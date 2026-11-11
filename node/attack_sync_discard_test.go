package main

// Attacker pass (oscillation): re-attacking the sync-adoption proof from
// a new angle. The #1 hole (fabricated history) and the #4/#5 replay
// angles are already demonstrated in attack_sync_test.go,
// attack_crossnode_test.go and attack_atomicity_test.go. This file takes
// a different angle: SILENT DISCARD OF CLIENT-ACKNOWLEDGED OPERATIONS.
// No malice required — two HONEST nodes with divergent histories.
//
// Threat: /sync adopts the longest hash-valid chain unconditionally.
// The re-lock guard in handleSync (s.v.Log.Len() == localLen) only
// protects operations that landed DURING the network fetch. An
// operation acknowledged to a client BEFORE the sync — HTTP 200,
// persisted to the journal — is silently discarded when the adopted
// chain diverges from local history. The client was told the op
// succeeded; the ledger no longer contains it; /metrics records
// nothing about the loss (no counter, no invariant event).
//
// Desired: adoption must not discard locally-acknowledged entries —
// only a chain that extends local history is adoptable (the v0.4
// "staged replay before sync adoption" direction). Current: the
// acknowledged op vanishes without a trace.
import (
	"net/http"
	"testing"

	"github.com/HerodotusTree/grove-network/identity"
)

func TestAttack_SyncDiscardsAcknowledgedLocalOp(t *testing.T) {
	// Peer A: honest node, longer history (2 signed deposits of 700,
	// R=C=1400). Amounts differ from B's on purpose: entry hashes
	// commit to a 1-second-granularity timestamp, so same-amount ops on
	// two nodes within the same second hash IDENTICALLY — distinct
	// payloads keep the two histories distinguishable.
	a := newTestServer(t)
	privA, pubA, _ := identity.Generate()
	if rec := postDeposit(a, signReq(t, identity.OpDeposit, privA, pubA, 700, 1)); rec.Code != http.StatusOK {
		t.Fatalf("peer deposit 1: got %d", rec.Code)
	}
	if rec := postDeposit(a, signReq(t, identity.OpDeposit, privA, pubA, 700, 2)); rec.Code != http.StatusOK {
		t.Fatalf("peer deposit 2: got %d", rec.Code)
	}

	// Victim B: one honest signed deposit, ACKNOWLEDGED (HTTP 200) and
	// persisted to disk. From the client's perspective this is durable.
	b := newTestServer(t)
	privB, pubB, _ := identity.Generate()
	if rec := postDeposit(b, signReq(t, identity.OpDeposit, privB, pubB, 100, 1)); rec.Code != http.StatusOK {
		t.Fatalf("victim deposit: got %d, want 200", rec.Code)
	}
	acknowledged := b.v.Log.Entries()[0].Hash
	if b.v.R != 100 || b.v.C != 100 {
		t.Fatalf("setup: victim state R=%d C=%d, want 100/100", b.v.R, b.v.C)
	}

	// B syncs from honest peer A, whose chain is longer but divergent
	// (different genesis: different timestamps, different hashes).
	srv := ledgerServer(a)
	defer srv.Close()
	b.peers = []string{srv.URL}
	rec := doSync(b)
	if rec.Code != http.StatusOK {
		t.Fatalf("sync: got %d, body %s", rec.Code, rec.Body.String())
	}

	// Desired: the acknowledged entry survives the sync — adoption must
	// not silently discard entries the node already told a client were
	// accepted.
	for _, e := range b.v.Log.Entries() {
		if e.Hash == acknowledged {
			return // survived: safe behavior
		}
	}
	t.Fatalf("HOLE DEMONSTRATED: /sync adopted a longer divergent chain and silently discarded "+
		"a client-acknowledged deposit (client got HTTP 200; entry %.16s... vanished from the ledger; "+
		"now R=%d C=%d entries=%d; sync response: %s)",
		acknowledged, b.v.R, b.v.C, b.v.Log.Len(), rec.Body.String())
}
