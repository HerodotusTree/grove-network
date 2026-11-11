package main

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/HerodotusTree/grove-network/identity"
	"github.com/HerodotusTree/grove-network/ledger"
	"github.com/HerodotusTree/grove-network/vault"
)

// Phase 1 step 12 (N1/N5/P5): exactly one sequencer writes; followers
// are read-only, never create a genesis, and pull only from the
// configured sequencer.
func TestFollowerRefusesWritesAndAwaitsGenesis(t *testing.T) {
	dir := t.TempDir()
	f := &server{
		v:         vault.New(ledger.New()),
		nonces:    map[string]uint64{},
		dataDir:   dir,
		chainPath: filepath.Join(dir, "chain.json"),
		role:      "follower",
	}
	if err := f.applyGenesis(testGenesis); err != nil {
		t.Fatal(err)
	}
	if f.v.Log.Len() != 0 {
		t.Fatalf("follower must not create a genesis entry, has %d entries", f.v.Log.Len())
	}
	if f.v.Domain != testDomain() {
		t.Fatal("follower must still learn the domain from its genesis file")
	}
	priv, pub, _ := identity.Generate()
	rec := postDeposit(f, signReq(t, identity.OpDeposit, priv, pub, 100, 1))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("follower deposit: got %d, want 403", rec.Code)
	}
}

func TestFollowerAdoptsSequencerHistory(t *testing.T) {
	seq := newTestServer(t)
	priv, pub, _ := identity.Generate()
	if rec := postDeposit(seq, signReq(t, identity.OpDeposit, priv, pub, 100, 1)); rec.Code != http.StatusOK {
		t.Fatalf("sequencer deposit: %d", rec.Code)
	}
	srv := ledgerServer(seq)
	defer srv.Close()

	dir := t.TempDir()
	f := &server{
		v:            vault.New(ledger.New()),
		nonces:       map[string]uint64{},
		dataDir:      dir,
		chainPath:    filepath.Join(dir, "chain.json"),
		role:         "follower",
		sequencerURL: srv.URL,
	}
	if err := f.applyGenesis(testGenesis); err != nil {
		t.Fatal(err)
	}
	if rec := doSync(f); rec.Code != http.StatusOK {
		t.Fatalf("follower sync: %d %s", rec.Code, rec.Body.String())
	}
	if f.v.Log.Len() != 2 || f.v.R != 100 {
		t.Fatalf("follower should hold the sequencer's history: entries=%d R=%d", f.v.Log.Len(), f.v.R)
	}
	rec := postDeposit(f, signReq(t, identity.OpDeposit, priv, pub, 100, 1))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("follower writes stay refused after sync: got %d", rec.Code)
	}
}
