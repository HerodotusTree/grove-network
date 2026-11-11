package main

// Adapted to the Phase 1 server shape (2026-10-23): the nonce file is
// gone (ATOM-01 — nonces are derived from ledger replay), and a restart
// mirrors main()'s startup: loadOrNew, keep the vault, re-bind the
// genesis (applyGenesis), then deriveNonces. The journal itself is
// owned by the ledger package: journal.jsonl beside the chain.json
// snapshot.

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HerodotusTree/grove-network/identity"
)

func journalFile(s *server) string {
	return filepath.Join(filepath.Dir(s.chainPath), "journal.jsonl")
}

// restartOverDataDir rebuilds a server over s's existing data dir the
// way main() does at startup: loadOrNew to rebuild the vault from the
// persisted chain, applyGenesis to re-bind the network domain (on an
// existing chain it verifies the chain belongs to the configured
// genesis — the A3 check), then deriveNonces to rebuild the nonce
// table from the ledger. Fails the test if any startup step refuses.
func restartOverDataDir(t *testing.T, s *server) *server {
	t.Helper()
	s2 := &server{
		nonces:    map[string]uint64{},
		dataDir:   s.dataDir,
		chainPath: s.chainPath,
	}
	v, err := s2.loadOrNew()
	if err != nil {
		t.Fatalf("startup over existing data dir refused: %v", err)
	}
	s2.v = v
	if err := s2.applyGenesis(testGenesis); err != nil {
		t.Fatalf("startup genesis binding refused: %v", err)
	}
	s2.deriveNonces()
	return s2
}

// TestStartupTruncatesTornJournalTail simulates a crash mid-write: a
// signed, acknowledged deposit sits in the journal, then a torn
// (unterminated, partial) tail is left by the crash. The node must
// start, discard only the torn write, and rebuild the vault state
// from the acknowledged deposit.
func TestStartupTruncatesTornJournalTail(t *testing.T) {
	s := newTestServer(t)
	priv, pub, _ := identity.Generate()
	if rec := postDeposit(s, signReq(t, identity.OpDeposit, priv, pub, 100, 1)); rec.Code != http.StatusOK {
		t.Fatalf("deposit: %d", rec.Code)
	}
	f, err := os.OpenFile(journalFile(s), os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"index":1,"payload":"torn-crash-wr`); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	s2 := restartOverDataDir(t, s)
	if s2.v.R != 100 || s2.v.C != 100 {
		t.Fatalf("state after recovery R=%d C=%d, want 100/100", s2.v.R, s2.v.C)
	}
	rest, err := os.ReadFile(journalFile(s))
	if err != nil {
		t.Fatal(err)
	}
	if len(rest) > 0 && !strings.HasSuffix(string(rest), "\n") {
		t.Fatal("torn tail was not truncated from the journal")
	}
	// The recovered node must keep serving: the next nonce appends
	// cleanly and survives another restart.
	if rec := postDeposit(s2, signReq(t, identity.OpDeposit, priv, pub, 50, 2)); rec.Code != http.StatusOK {
		t.Fatalf("post-recovery deposit: %d", rec.Code)
	}
	s3 := restartOverDataDir(t, s)
	if s3.v.R != 150 || s3.v.C != 150 {
		t.Fatalf("state after second restart R=%d C=%d, want 150/150", s3.v.R, s3.v.C)
	}
}

// TestStartupRefusesCorruptJournalRecord: a newline-terminated but
// malformed journal record is corruption, not a torn write — the node
// must refuse to start rather than run on a history it cannot verify.
func TestStartupRefusesCorruptJournalRecord(t *testing.T) {
	s := newTestServer(t)
	priv, pub, _ := identity.Generate()
	if rec := postDeposit(s, signReq(t, identity.OpDeposit, priv, pub, 100, 1)); rec.Code != http.StatusOK {
		t.Fatalf("deposit: %d", rec.Code)
	}
	f, err := os.OpenFile(journalFile(s), os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("{corrupt complete record}\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	s2 := &server{nonces: map[string]uint64{}, dataDir: s.dataDir, chainPath: s.chainPath}
	if _, err := s2.loadOrNew(); err == nil {
		t.Fatal("node started with a newline-terminated corrupt journal record")
	}
}
