package main

// Independent attacker pass (workflow cycle), 2026-10-18 —
// availability angle. Test assets only — no real value anywhere.
//
// CONTRACT CHANGE (hardening patch, issues #6/#7): these tests were
// written against the vulnerable contract — unauthenticated /peers
// accepted any string and unauthenticated /sync fetched it while
// holding the global op lock. The fix changed that contract, so the
// tests below now assert the FIXED behavior instead of demonstrating
// the hole:
//
//  1. /sync no longer holds the op lock during peer network I/O
//     (issue #6). TestAttack_SyncFreezesSequencer now asserts that
//     /state and a signed /deposit both answer promptly (<1.5s) while
//     a sync is in flight against a slow peer. It PASSES on fixed
//     code and FAILS on the old code (where /state blocked ~2.7s
//     behind the lock).
//  2. POST /peers validates the URL against the documented contract
//     {"url":"http://host:port"} (issue #7). The demonstrated SSRF
//     rode on a URL *path prefix* — the node fetches peer+"/ledger"
//     verbatim, so "http://host:port/internal-prefix" became a fetch
//     of "/internal-prefix/ledger" on an attacker-chosen host.
//     Registration now rejects paths, queries, fragments, userinfo,
//     and non-http(s) schemes with 400, and sync never follows
//     redirects. TestAttack_SyncSSRF now asserts the evil URL is
//     rejected at registration, the peer list is unchanged, no
//     outbound fetch happens — and (positive control) that a bare
//     base URL still registers and IS fetched at exactly "/ledger".
//     It PASSES on fixed code and FAILS on the old code (where the
//     evil URL registered with 200 and was fetched).

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/HerodotusTree/grove-network/identity"
)

// D1. Unauthenticated /sync must not freeze the sequencer (issue #6).
//
// Old behavior: handleSync took s.mu (the single global lock guarding
// deposits, redeems, tips, state, metrics and the ledger) and held it
// for the entire peer loop — including the network GET to each peer,
// with a 10-second timeout. Any network client could freeze EVERY
// sequencer operation for up to 10s per registered peer.
//
// Fixed behavior: sync snapshots the peer list under the lock, releases
// it, and does the fetches outside the lock. This test asserts the
// fixed behavior: a plain GET /state and a signed /deposit must both
// answer in well under 1.5s while a sync is waiting on a 3s-stall peer.
func TestAttack_SyncFreezesSequencer(t *testing.T) {
	s := newTestServer(t)

	// A peer that answers /ledger after a 3s stall — well within the
	// node's own 10s client timeout, so this is "normal" slowness,
	// not even a malicious hang.
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	}))
	defer slow.Close()

	// Step 1: register the peer — UNAUTHENTICATED, no signature anywhere.
	peerBody, _ := json.Marshal(peerReq{URL: slow.URL})
	rec := httptest.NewRecorder()
	s.handlePeers(rec, httptest.NewRequest(http.MethodPost, "/peers", bytes.NewReader(peerBody)))
	if rec.Code != http.StatusOK {
		t.Fatalf("peer registration (unauthenticated): got %d", rec.Code)
	}

	// Step 2: trigger the sync in the background — UNAUTHENTICATED.
	syncDone := make(chan struct{})
	go func() {
		defer close(syncDone)
		r := httptest.NewRecorder()
		s.handleSync(r, httptest.NewRequest(http.MethodPost, "/sync", nil))
	}()

	// Let the sync get into its network wait, then try a trivial read
	// and a real signed operation. Neither may block on the sync.
	time.Sleep(300 * time.Millisecond)

	start := time.Now()
	srec := httptest.NewRecorder()
	s.handleState(srec, httptest.NewRequest(http.MethodGet, "/state", nil))
	stateElapsed := time.Since(start)

	priv, pub, _ := identity.Generate()
	start = time.Now()
	drec := postDeposit(s, signReq(t, identity.OpDeposit, priv, pub, 100, 1))
	depositElapsed := time.Since(start)

	<-syncDone

	if srec.Code != http.StatusOK {
		t.Fatalf("/state during sync: got %d", srec.Code)
	}
	if drec.Code != http.StatusOK {
		t.Fatalf("signed deposit during sync: got %d, body %s", drec.Code, drec.Body.String())
	}
	if stateElapsed >= 1500*time.Millisecond {
		t.Fatalf("REGRESSION (#6): /state took %v while a sync was in flight "+
			"against a slow peer (threshold 1.5s) — the op lock is being "+
			"held across peer network I/O again.", stateElapsed.Round(time.Millisecond))
	}
	if depositElapsed >= 1500*time.Millisecond {
		t.Fatalf("REGRESSION (#6): signed /deposit took %v while a sync was "+
			"in flight against a slow peer (threshold 1.5s) — sequencer "+
			"operations are blocked on peer network I/O again.", depositElapsed.Round(time.Millisecond))
	}
	t.Logf("/state answered in %v and a signed deposit in %v while a sync was in flight",
		stateElapsed.Round(time.Millisecond), depositElapsed.Round(time.Millisecond))
}

// D2. SSRF via /peers + /sync (issue #7).
//
// Old behavior: handleSync issued client.Get(peer + "/ledger") for every
// registered peer, and the peer URL was entirely attacker-chosen (host,
// port, AND url prefix — only the trailing "/ledger" was fixed).
// Registering it needed no auth, so the node issued outbound HTTP
// requests to arbitrary attacker-chosen destinations on demand.
//
// Fixed behavior: POST /peers enforces the documented contract
// {"url":"http://host:port"} — a bare base URL. Paths, queries,
// fragments, userinfo, and non-http(s) schemes are rejected with 400,
// fetches never follow redirects, and peer URLs are revalidated at
// fetch time. This test asserts the evil URL (with a path prefix) is
// rejected at registration and never fetched — and that a legitimate
// bare base URL still syncs, so the fix didn't just disable sync.
func TestAttack_SyncSSRF(t *testing.T) {
	s := newTestServer(t)

	var gotMethod, gotPath string
	var gotHost atomic.Value
	honeypot := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotHost.Store(r.Host)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	}))
	defer honeypot.Close()

	// Phase 1: the demonstrated attack URL — a bare host:port plus an
	// attacker-chosen path prefix. Registration must now refuse it.
	evilURL := honeypot.URL + "/internal-prefix"
	peerBody, _ := json.Marshal(peerReq{URL: evilURL})
	rec := httptest.NewRecorder()
	s.handlePeers(rec, httptest.NewRequest(http.MethodPost, "/peers", bytes.NewReader(peerBody)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("REGRESSION (#7): evil peer URL with path prefix registered with %d, "+
			"want 400 — the node would fetch %s%s on demand.", rec.Code, evilURL, "/ledger")
	}

	// The rejected URL must not be in the peer list...
	rec = httptest.NewRecorder()
	s.handlePeers(rec, httptest.NewRequest(http.MethodGet, "/peers", nil))
	var listed struct {
		Peers []string `json:"peers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("list peers: %v", err)
	}
	for _, p := range listed.Peers {
		if p == evilURL {
			t.Fatalf("REGRESSION (#7): rejected evil URL is in the peer list")
		}
	}

	// ...and a sync must not fetch anything: the honeypot stays silent.
	rec = httptest.NewRecorder()
	s.handleSync(rec, httptest.NewRequest(http.MethodPost, "/sync", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("sync: got %d", rec.Code)
	}
	if gotMethod != "" {
		t.Fatalf("REGRESSION (#7): node fetched %s %s (host %v) for a peer "+
			"that was never registered.", gotMethod, gotPath, gotHost.Load())
	}

	// Phase 2 (positive control): a bare base URL is the documented
	// contract and must still register and sync — the fix constrains
	// sync, it does not disable it.
	peerBody, _ = json.Marshal(peerReq{URL: honeypot.URL})
	rec = httptest.NewRecorder()
	s.handlePeers(rec, httptest.NewRequest(http.MethodPost, "/peers", bytes.NewReader(peerBody)))
	if rec.Code != http.StatusOK {
		t.Fatalf("legitimate bare peer URL rejected: got %d, body %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	s.handleSync(rec, httptest.NewRequest(http.MethodPost, "/sync", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("sync with legitimate peer: got %d", rec.Code)
	}
	if gotMethod != http.MethodGet || gotPath != "/ledger" {
		t.Fatalf("legitimate sync broken: honeypot saw %s %s (host %v), want GET /ledger",
			gotMethod, gotPath, gotHost.Load())
	}
	t.Logf("evil URL rejected at registration (400), nothing fetched; bare peer URL synced via GET /ledger as contracted")
}
