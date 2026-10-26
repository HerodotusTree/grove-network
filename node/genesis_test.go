package main

import (
	"path/filepath"
	"testing"
)

// Phase 1 step 6: the node binds to its genesis at startup - writing
// entry 0 on a fresh chain, re-binding on restart, and refusing a
// chain that belongs to a different genesis.
func TestApplyGenesisBindsNode(t *testing.T) {
	dir := t.TempDir()
	const payload = `{"kind":"genesis","network":"trial-1"}`

	s := &server{dataDir: dir, chainPath: filepath.Join(dir, "chain.json")}
	v, err := s.loadOrNew()
	if err != nil {
		t.Fatal(err)
	}
	s.v = v
	if err := s.applyGenesis(payload); err != nil {
		t.Fatal(err)
	}
	if s.v.Log.Len() != 1 {
		t.Fatalf("want genesis entry only, got %d entries", s.v.Log.Len())
	}
	domain := s.v.Domain
	if domain == "" {
		t.Fatal("domain must be set after genesis")
	}

	s2 := &server{dataDir: dir, chainPath: filepath.Join(dir, "chain.json")}
	v2, err := s2.loadOrNew()
	if err != nil {
		t.Fatal(err)
	}
	s2.v = v2
	if err := s2.applyGenesis(payload); err != nil {
		t.Fatal(err)
	}
	if s2.v.Log.Len() != 1 || s2.v.Domain != domain {
		t.Fatal("restart must rebind to the same domain without a second genesis")
	}

	s3 := &server{dataDir: dir, chainPath: filepath.Join(dir, "chain.json")}
	v3, err := s3.loadOrNew()
	if err != nil {
		t.Fatal(err)
	}
	s3.v = v3
	if err := s3.applyGenesis(`{"kind":"genesis","network":"other"}`); err == nil {
		t.Fatal("foreign genesis must refuse to start")
	}
}
