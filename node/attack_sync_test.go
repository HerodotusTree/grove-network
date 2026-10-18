package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HerodotusTree/grove-network/identity"
	"github.com/HerodotusTree/grove-network/ledger"
	"github.com/HerodotusTree/grove-network/vault"
)

// TestSyncAdoptsFabricatedPeerChain is a regression test for the
// unauthenticated /peers + /sync state-rewrite hole.
//
// Threat: ANY network client (no signature, no account) can register an
// arbitrary peer URL via POST /peers and trigger POST /sync. The node
// adopts the longest hash-valid chain it is shown — and hash-valid is
// cheap for an attacker to fabricate, since chain verification checks
// only hashes, never authorization. rebuildState then trusts the
// fabricated entries' R/C/Floor blindly.
//
// Desired behavior: the fabricated chain is refused and local state is
// untouched. Current behavior: it is adopted and the vault's state is
// rewritten to the attacker's numbers.
func TestSyncAdoptsFabricatedPeerChain(t *testing.T) {
	// Victim: one honest signed deposit, R=C=100, floor=0.
	s := newTestServer(t)
	priv, pub, _ := identity.Generate()
	if rec := postDeposit(s, signReq(t, identity.OpDeposit, priv, pub, 100, 1)); rec.Code != http.StatusOK {
		t.Fatalf("honest deposit: got %d", rec.Code)
	}

	// Attacker: a LONGER, hash-valid chain with fabricated economics —
	// a "tip" of one million that never happened, plus a deposit.
	atk := ledger.New()
	mkOp := func(kind string, amount, r, c, floor int64) string {
		b, _ := json.Marshal(vault.Op{Kind: kind, Amount: amount, R: r, C: c, Floor: floor})
		return string(b)
	}
	atk.Append(mkOp("tip", 1000000, 1000000, 0, 1000000))
	atk.Append(mkOp("deposit", 500, 1000500, 500, 1000000))
	dump, err := atk.Dump()
	if err != nil {
		t.Fatal(err)
	}
	var atkEntries []ledger.Entry
	if err := json.Unmarshal(dump, &atkEntries); err != nil {
		t.Fatal(err)
	}
	if err := ledger.VerifyEntries(atkEntries); err != nil {
		t.Fatalf("attacker chain should be hash-valid: %v", err)
	}

	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(dump)
	}))
	defer peer.Close()

	// Step 1: register the attacker's URL — no auth required.
	peerBody, _ := json.Marshal(peerReq{URL: peer.URL})
	rec := httptest.NewRecorder()
	s.handlePeers(rec, httptest.NewRequest(http.MethodPost, "/peers", bytes.NewReader(peerBody)))
	if rec.Code != http.StatusOK {
		t.Fatalf("peer registration: got %d, want 200 (unauthenticated)", rec.Code)
	}

	// Step 2: trigger a sync — no auth required.
	rec = httptest.NewRecorder()
	s.handleSync(rec, httptest.NewRequest(http.MethodPost, "/sync", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("sync: got %d, body %s", rec.Code, rec.Body.String())
	}
	t.Logf("sync response: %s", rec.Body.String())

	// Desired: local state untouched (R=100, C=100, floor=0).
	if s.v.R != 100 || s.v.C != 100 || s.v.Floor != 0 {
		t.Fatalf("HOLE DEMONSTRATED: unauthenticated /sync adopted the fabricated chain: "+
			"R=%d C=%d floor=%d (want R=100 C=100 floor=0)",
			s.v.R, s.v.C, s.v.Floor)
	}
}
