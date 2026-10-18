package main

// Independent attacker pass (workflow cycle), 2026-10-18 —
// availability angle. Failing-as-intended on v0.3: each test fails
// while the hole exists and should pass once it is closed.
// Test assets only — no real value anywhere.
//
// The Builder's patch was empty this cycle, so per the oscillation
// rule this pass re-attacks earlier proofs from a new angle: not the
// signed-operation path (already covered by the ownership/atomicity/
// cross-node tests) but the UNAUTHENTICATED /peers + /sync path.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// D1. Unauthenticated /sync freezes the whole sequencer.
//
// handleSync takes s.mu (the single global lock guarding deposits,
// redeems, tips, state, metrics and the ledger) and holds it for the
// entire peer loop — including the network GET to each peer, which has
// a 10-second timeout. Peer registration via POST /peers is also
// unauthenticated and unbounded. So any network client, with no key
// and no signature, can:
//
//  1. POST /peers {"url": "<slow host>"}   (no auth)
//  2. POST /sync                            (no auth)
//
// ...and freeze EVERY sequencer operation for up to 10s per peer.
// Deposits, redeems, tips, /state, /metrics: all blocked behind the
// lock while the node waits on attacker-chosen network I/O.
//
// Desired behavior: sync fetches peers without holding the op lock
// (or runs in a worker), so one slow peer cannot halt the sequencer.
// Current behavior: total freeze, demonstrated below.
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

	// Let the sync get into its network wait, then try a trivial read.
	time.Sleep(300 * time.Millisecond)
	start := time.Now()
	srec := httptest.NewRecorder()
	s.handleState(srec, httptest.NewRequest(http.MethodGet, "/state", nil))
	elapsed := time.Since(start)

	<-syncDone

	if elapsed >= 1500*time.Millisecond {
		t.Fatalf("HOLE DEMONSTRATED: unauthenticated /sync froze the sequencer — "+
			"a plain GET /state took %v while the node waited on a slow peer (threshold 1.5s). "+
			"Deposits, redeems and metrics were blocked the whole time; freeze scales with "+
			"10s x <number of registered peers>, and peer registration is unauthenticated.",
			elapsed.Round(time.Millisecond))
	}
	t.Logf("/state answered in %v while a sync was in flight", elapsed.Round(time.Millisecond))
}

// D2. SSRF via unauthenticated /peers + /sync (issue #7).
//
// handleSync issues client.Get(peer + "/ledger") for every registered
// peer. The peer URL is entirely attacker-chosen (host, port, and URL
// prefix — only the trailing "/ledger" is fixed), and registering it
// needs no auth. The node will therefore issue outbound HTTP requests
// to arbitrary attacker-chosen destinations on demand.
//
// Desired behavior: peer URLs are validated/restricted, or sync is
// authenticated. Current behavior: the node fetches whatever it is
// told to fetch, demonstrated below with a honeypot listener.
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

	// Attacker-chosen URL prefix, registered with zero auth.
	evilURL := honeypot.URL + "/internal-prefix"
	peerBody, _ := json.Marshal(peerReq{URL: evilURL})
	rec := httptest.NewRecorder()
	s.handlePeers(rec, httptest.NewRequest(http.MethodPost, "/peers", bytes.NewReader(peerBody)))
	if rec.Code != http.StatusOK {
		t.Fatalf("peer registration (unauthenticated): got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	s.handleSync(rec, httptest.NewRequest(http.MethodPost, "/sync", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("sync: got %d", rec.Code)
	}

	if gotMethod == http.MethodGet && gotPath == "/internal-prefix/ledger" {
		t.Fatalf("HOLE DEMONSTRATED: unauthenticated /sync made the node issue an "+
			"outbound GET to attacker-chosen destination %s%s (host %v). "+
			"In production the peer URL could point at internal services; "+
			"registration and trigger both required no auth.", evilURL, "/ledger", gotHost.Load())
	}
	t.Logf("honeypot saw %s %s (host %v)", gotMethod, gotPath, gotHost.Load())
}