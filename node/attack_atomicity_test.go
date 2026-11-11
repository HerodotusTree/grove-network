package main

// Independent attacker pass #2 (Claude), 2026-10-17 — atomicity angle.
// Phase 1 step 8 (ATOM-01): the nonce file this attack used to sabotage
// no longer exists. Nonce state is derived from the ledger alone, so
// the split-write gap has nowhere to live. The attack is re-aimed at
// the new design: deposit, restart with the chain as the only truth,
// replay the exact same signed bytes - they must not apply twice.

import (
	"net/http"
	"testing"

	"github.com/HerodotusTree/grove-network/identity"
)

// C1 (re-aimed). One signature, one deposit - across a restart.
func TestAttack_SplitWriteDoubleApplyAfterRestart(t *testing.T) {
	s := newTestServer(t)
	priv, pub, _ := identity.Generate()
	body := signReq(t, identity.OpDeposit, priv, pub, 100, 1)

	rec := postDeposit(s, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("setup: deposit should succeed, got %d", rec.Code)
	}

	// Restart: fresh server, same data dir, disk is the only truth.
	s2 := &server{dataDir: s.dataDir, chainPath: s.chainPath}
	v, err := s2.loadOrNew()
	if err != nil {
		t.Fatalf("restart: %v", err)
	}
	s2.v = v
	if err := s2.applyGenesis(testGenesis); err != nil {
		t.Fatalf("restart genesis: %v", err)
	}
	s2.deriveNonces()

	// Replay the exact same signed bytes.
	rec = postDeposit(s2, body)
	if rec.Code == http.StatusOK {
		t.Fatalf("DOUBLE APPLY: one signature produced two deposits across a restart (R=%d, ledger entries=%d)",
			s2.v.R, s2.v.Log.Len())
	}
	if s2.v.R != 100 {
		t.Fatalf("replay mutated the vault: R=%d, want 100", s2.v.R)
	}
}
