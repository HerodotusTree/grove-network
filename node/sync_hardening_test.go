package main

// Sync-hardening acceptance tests for issue #1 (builder pass): the
// adoption gate must refuse divergent, poisoned, and inconsistent
// chains — and must still adopt the one legitimate case, an honest
// chain that extends local history exactly. Test assets only.

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HerodotusTree/grove-network/identity"
	"github.com/HerodotusTree/grove-network/ledger"
	"github.com/HerodotusTree/grove-network/vault"
)

// TestSyncAdoptsHonestPrefixExtension pins the legitimate sync case:
// the peer chain is the victim's own entry plus one more valid op.
// Prefix-only adoption must not break real chain growth.
//
// Two independent ledgers produce a byte-identical first entry only
// when the payload and the one-second-granularity timestamp match, so
// the setup retries across a second boundary (bounded, usually first
// try) rather than depending on a lucky timestamp.
func TestSyncAdoptsHonestPrefixExtension(t *testing.T) {
	var victim, peer *server
	var priv, pub string
	aligned := false
	for attempt := 0; attempt < 10 && !aligned; attempt++ {
		victim, peer = newTestServer(t), newTestServer(t)
		priv, pub, _ = identity.Generate()
		if rec := postDeposit(victim, signReq(t, identity.OpDeposit, priv, pub, 100, 1)); rec.Code != http.StatusOK {
			t.Fatalf("victim deposit: got %d", rec.Code)
		}
		if rec := postDeposit(peer, signReq(t, identity.OpDeposit, priv, pub, 100, 1)); rec.Code != http.StatusOK {
			t.Fatalf("peer deposit: got %d", rec.Code)
		}
		aligned = peer.v.Log.Entries()[1] == victim.v.Log.Entries()[1]
	}
	if !aligned {
		t.Skip("could not align one-second timestamps across a second boundary; rerun")
	}
	if rec := postDeposit(peer, signReq(t, identity.OpDeposit, priv, pub, 50, 2)); rec.Code != http.StatusOK {
		t.Fatalf("peer extension deposit: got %d", rec.Code)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dump, _ := peer.v.Log.Dump()
		w.Header().Set("Content-Type", "application/json")
		w.Write(dump)
	}))
	defer srv.Close()
	victim.peers = []string{srv.URL}
	rec := doSync(victim)
	if rec.Code != http.StatusOK {
		t.Fatalf("sync: got %d, body %s", rec.Code, rec.Body.String())
	}
	if victim.v.R != 150 || victim.v.C != 150 || victim.v.Log.Len() != 3 {
		t.Fatalf("honest extension not adopted: R=%d C=%d entries=%d, sync %s",
			victim.v.R, victim.v.C, victim.v.Log.Len(), rec.Body.String())
	}
}

// TestSyncGateRefusesInconsistentExtension: a candidate that extends
// local history byte-identically but whose extension op claims state
// that does not follow from the ops is refused at the gate, even
// though its hashes are valid (Append computed them).
//
// RECONCILED 2026-10-26: the cycle's build had a standalone
// validateSyncCandidate gate function; the branch's gate lives in
// handleSync (VerifyEntries -> CheckDomain -> IsPrefixExtension ->
// staged Rebuild -> extension signature check), so this drives the
// real sync path end to end and asserts the victim is untouched.
func TestSyncGateRefusesInconsistentExtension(t *testing.T) {
	b := newTestServer(t)
	priv, pub, _ := identity.Generate()
	if rec := postDeposit(b, signReq(t, identity.OpDeposit, priv, pub, 100, 1)); rec.Code != http.StatusOK {
		t.Fatalf("victim deposit: got %d", rec.Code)
	}
	candidate := ledger.FromEntries(b.v.Log.Entries())
	candidate.Append(`{"kind":"deposit","amount":50,"reserves_after":999,"claims_after":150,"floor_after":0}`)

	srv := ledgerServer(&server{v: vault.New(candidate)})
	defer srv.Close()
	b.peers = []string{srv.URL}
	if rec := doSync(b); rec.Code != http.StatusOK {
		t.Fatalf("sync: got %d, body %s", rec.Code, rec.Body.String())
	}
	if b.v.R != 100 || b.v.C != 100 || b.v.Log.Len() != 2 {
		t.Fatalf("extension with inconsistent claimed state was adopted: R=%d C=%d entries=%d, want 100/100/2",
			b.v.R, b.v.C, b.v.Log.Len())
	}
}
