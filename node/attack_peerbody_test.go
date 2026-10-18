package main

// Attacker regression tests for bypasses found in the sync-hardening
// patch review (issues #6/#7 follow-ups), 2026-10-18.
// Each test FAILS on the pre-fix code and PASSES once fixed.
// Test assets only — no real value anywhere.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/HerodotusTree/grove-network/identity"
	"github.com/HerodotusTree/grove-network/ledger"
)

// R1. A stalled request body on the UNAUTHENTICATED POST /peers must not
// freeze the sequencer. The handler used to take s.mu before
// json.Decode(r.Body) — body bytes are network I/O, so any client could
// hold the global op lock indefinitely with a dripping body, blocking
// deposits, redeems, tips, /state and /metrics (issue #6 class).
func TestAttack_PeersStalledBodyKeepsSequencerLive(t *testing.T) {
	s := newTestServer(t)

	pr, pw := io.Pipe()
	peersDone := make(chan struct{})
	go func() {
		defer close(peersDone)
		s.handlePeers(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/peers", pr))
	}()
	time.Sleep(500 * time.Millisecond) // handler takes the lock, blocks in Decode

	// Release the body 1.5s after measurement starts.
	go func() {
		time.Sleep(1500 * time.Millisecond)
		pw.Write([]byte(`{"url":"http://127.0.0.1:9"}`))
	}()

	start := time.Now()
	srec := httptest.NewRecorder()
	s.handleState(srec, httptest.NewRequest(http.MethodGet, "/state", nil))
	stateElapsed := time.Since(start)
	if srec.Code != http.StatusOK {
		t.Fatalf("/state during stalled /peers body: got %d", srec.Code)
	}

	priv, pub, _ := identity.Generate()
	start = time.Now()
	drec := postDeposit(s, signReq(t, identity.OpDeposit, priv, pub, 100, 1))
	depositElapsed := time.Since(start)
	if drec.Code != http.StatusOK {
		t.Fatalf("deposit during stalled /peers body: got %d", drec.Code)
	}
	<-peersDone

	if stateElapsed >= 1000*time.Millisecond {
		t.Fatalf("REGRESSION: /state took %v while a POST /peers body was stalled — "+
			"the op lock is held across request-body network I/O again.",
			stateElapsed.Round(time.Millisecond))
	}
	if depositElapsed >= 1000*time.Millisecond {
		t.Fatalf("REGRESSION: signed /deposit took %v while a POST /peers body was stalled.",
			depositElapsed.Round(time.Millisecond))
	}
	t.Logf("/state answered in %v and a signed deposit in %v with a stalled /peers body in flight",
		stateElapsed.Round(time.Millisecond), depositElapsed.Round(time.Millisecond))
}

// R2. A trailing "?" must not survive peer canonicalization.
// validatePeerURL used to accept "http://host?" and keep the "?" (Go's
// ForceQuery), so "http://host" and "http://host?" registered as two
// peers — and sync fetched "http://host?/ledger", i.e. GET / with query
// "/ledger", instead of the contracted GET /ledger (issue #7 class).
func TestAttack_PeerForceQueryCanonicalized(t *testing.T) {
	if got, err := validatePeerURL("http://127.0.0.1:1?"); err != nil || got != "http://127.0.0.1:1" {
		t.Fatalf("REGRESSION: validatePeerURL(%q) = %q, err=%v; want %q with no trailing ?",
			"http://127.0.0.1:1?", got, err, "http://127.0.0.1:1")
	}

	var gotPath, gotQuery atomic.Value
	honeypot := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath.Store(r.URL.Path)
		gotQuery.Store(r.URL.RawQuery)
		json.NewEncoder(w).Encode([]ledger.Entry{})
	}))
	defer honeypot.Close()

	s := newTestServer(t)
	for _, u := range []string{honeypot.URL, honeypot.URL + "?"} {
		body, _ := json.Marshal(peerReq{URL: u})
		rec := httptest.NewRecorder()
		s.handlePeers(rec, httptest.NewRequest(http.MethodPost, "/peers", bytes.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("register %q: got %d", u, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	s.handlePeers(rec, httptest.NewRequest(http.MethodGet, "/peers", nil))
	var listed struct {
		Peers []string `json:"peers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("list peers: %v", err)
	}
	if len(listed.Peers) != 1 {
		t.Fatalf("REGRESSION: %q and %q registered as %d peers, want dedup to 1",
			honeypot.URL, honeypot.URL+"?", len(listed.Peers))
	}

	s.handleSync(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/sync", nil))
	p, _ := gotPath.Load().(string)
	q, _ := gotQuery.Load().(string)
	if p != "/ledger" || q != "" {
		t.Fatalf("REGRESSION: sync fetched path=%q query=%q, want exactly GET /ledger with no query", p, q)
	}
	t.Logf("trailing ? canonicalized away; peer deduped; sync fetched /ledger with no query")
}
