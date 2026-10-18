package main

// Independent attacker pass #2 (Claude), 2026-10-17 — atomicity angle.
// Failing-as-intended on v0.3. Test assets only.

import (
	"net/http"
	"os"
	"testing"

	"github.com/HerodotusTree/grove-network/identity"
)

// C1. Chain and nonce are two separate writes. If the nonce write fails
// after the chain write succeeds, the client is told 500 (failure) but
// the money already moved and is durable. After a restart the nonce on
// disk is stale, so the SAME signed bytes are accepted again: one
// signature, two deposits.
func TestAttack_SplitWriteDoubleApplyAfterRestart(t *testing.T) {
	s := newTestServer(t)
	// Make the nonce write fail: a directory where the temp file goes.
	if err := os.Mkdir(s.noncePath+".tmp", 0700); err != nil {
		t.Fatal(err)
	}
	priv, pub, _ := identity.Generate()
	body := signReq(t, identity.OpDeposit, priv, pub, 100, 1)

	rec := postDeposit(s, body)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("setup: expected nonce-write failure (500), got %d", rec.Code)
	}
	if s.v.R == 100 {
		t.Logf("client was told 500, but the deposit is in the chain (R=%d)", s.v.R)
	}

	// Restart: fresh server, same data dir, disk is the only truth.
	os.Remove(s.noncePath + ".tmp")
	s2 := &server{dataDir: s.dataDir, chainPath: s.chainPath, noncePath: s.noncePath}
	v, err := s2.loadOrNew()
	if err != nil {
		t.Fatalf("restart: %v", err)
	}
	s2.v = v
	if s2.nonces, err = loadNonces(s2.noncePath); err != nil {
		t.Fatalf("restart nonces: %v", err)
	}

	// Replay the exact same signed bytes.
	rec = postDeposit(s2, body)
	if rec.Code == http.StatusOK {
		t.Fatalf("DOUBLE APPLY: one signature produced two deposits across a restart (R=%d, ledger entries=%d)",
			s2.v.R, s2.v.Log.Len())
	}
}
