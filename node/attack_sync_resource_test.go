package main

// Attacker pass (oscillation), 2026-10-21 — resource-exhaustion angle on
// the NEW sync hardening (issue #6/#7 fix in the working tree).
//
// The new code moved peer fetches out from under the op lock and locked
// down the peer-URL contract, closing the demonstrated vectors. But the
// fetch path has NO resource bounds at all: fetchPeerLedger decodes the
// peer's response body with no size cap, and Adopt re-verifies the whole
// chain under the op lock. /peers and /sync are both unauthenticated, so
// ANY network client can register an attacker peer and trigger a sync.
// Failing-as-intended on the current tree; test assets only.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/HerodotusTree/grove-network/ledger"
)

// bigPeerChain builds a hash-valid chain of n entries with tiny payloads
// and returns its JSON dump. Each entry is ~190 bytes on the wire.
func bigPeerChain(t *testing.T, n int) []byte {
	t.Helper()
	l := ledger.New()
	for i := 0; i < n; i++ {
		l.Append(`{"kind":"tip","amount":1,"n":` + strconv.Itoa(i) + `}`)
	}
	dump, err := l.Dump()
	if err != nil {
		t.Fatal(err)
	}
	return dump
}

// R1. fetchPeerLedger reads the entire peer response body with no size
// limit. The attacker peer answers with a body of arbitrary size and the
// node buffers it all in memory before even looking at it.
//
// Desired: bodies over a sane cap are refused before decode. Current:
// fully decoded — a registered peer plus an unauthenticated /sync is a
// remote OOM button (the 10s client timeout bounds bandwidth, not the
// buffered total).
func TestAttack_SyncPeerBodyUnbounded(t *testing.T) {
	dump := bigPeerChain(t, 150000)
	t.Logf("attacker peer body: %d bytes (%d entries)", len(dump), 150000)
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(dump)
	}))
	defer peer.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	got, err := fetchPeerLedger(client, peer.URL)
	if err == nil {
		t.Fatalf("HOLE DEMONSTRATED: fetchPeerLedger decoded a %d-byte attacker body "+
			"(%d entries) with no size limit — an unauthenticated /sync against a "+
			"registered malicious peer lets any network client push unbounded "+
			"memory into the sequencer", len(dump), len(got))
	}
	t.Logf("oversized peer body refused: %v", err)
}

// R2. Adopt rewrites the snapshot (temp file + rename) and THEN truncates
// the journal — two separate durable steps. A crash (kill -9, power loss)
// between them leaves the NEW snapshot beside the OLD chain's journal.
// On restart, Load replays the old journal onto the new snapshot, finds
// conflicting entries at the same indexes, and refuses to start. The node
// is bricked until an operator hand-edits the data dir — and this path is
// reachable exactly when /sync adopts a divergent chain (the common case:
// different genesis/timestamps mean the old journal can never match the
// new snapshot's prefix).
//
// Desired: Adopt is crash-atomic (single durable step, or recovery
// reconciles the torn adopt). Current: torn state bricks the node.
func TestAttack_AdoptCrashWindowBricksNode(t *testing.T) {
	dir := t.TempDir()
	chainPath := filepath.Join(dir, "chain.json")

	// Old chain: one real op, journaled to disk.
	old := ledger.New()
	old.Append(`{"kind":"deposit","amount":100,"reserves_after":100,"claims_after":100,"floor_after":0}`)
	if err := old.Save(chainPath); err != nil {
		t.Fatal(err)
	}

	// Simulate Adopt's snapshot rename completing, then a crash before
	// the journal truncate: new snapshot on disk, old journal untouched.
	// Adopt writes its commit marker (length + tip hash) BEFORE the
	// rename, so the faithful crash-point state includes the marker —
	// it is what lets Load tell this torn adopt apart from a corrupted
	// journal (which has no matching marker and must still refuse).
	adopted := ledger.New()
	adopted.Append(`{"kind":"deposit","amount":700,"reserves_after":700,"claims_after":700,"floor_after":0}`)
	adopted.Append(`{"kind":"deposit","amount":700,"reserves_after":1400,"claims_after":1400,"floor_after":0}`)
	data, err := adopted.Dump()
	if err != nil {
		t.Fatal(err)
	}
	adoptedEntries := adopted.Entries()
	marker, err := json.Marshal(map[string]any{
		"entries": len(adoptedEntries),
		"tip":     adoptedEntries[len(adoptedEntries)-1].Hash,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(chainPath+".adopt", marker, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(chainPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	// journal.jsonl still holds the OLD chain's entry — the exact on-disk
	// state of a crash between Adopt's rename and its journal truncate.

	if _, err := ledger.Load(chainPath); err == nil {
		return // safe: recovery reconciled the torn adopt
	} else {
		t.Fatalf("HOLE DEMONSTRATED: a crash between Adopt's snapshot rename "+
			"and its journal truncate leaves an unrecoverable data dir — the "+
			"node refuses to start: %v", err)
	}
}
