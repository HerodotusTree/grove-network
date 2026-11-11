package main

// Attacker pass on the grove-v2 genesis-binding build (2026-10-26).
//
// What v2 fixes: a signature captured from one network no longer
// verifies on another, because it binds that network's domain. What
// v2 does NOT fix, and this test pins: a fresh node has almost no
// history to protect, and /peers + /sync are unauthenticated, so an
// attacker who was never given any victim signature at all can still
// poison a fresh node. He signs his OWN fabricated history for the
// network's (public) genesis domain — every gate check passes,
// because the gate authenticates entries, and they ARE authenticated,
// by the attacker himself. A signature proves authorization by SOME
// key, not funding or provenance. This is issue #1 in its bootstrap
// form, surviving the transplant fix; closing it needs peer
// authentication (AUTH-02), not genesis binding.
//
// RECONCILED 2026-10-26 against the v04-phase1 branch: the branch's
// fresh node carries the shared genesis entry (from its config), so
// the fabricated chain is built ON that genesis rather than from
// nothing, and the victim's expected state is entries=1, not 0.
//
// This test asserts the SAFE behavior and FAILS on the current
// build. Test assets only; no real value anywhere.

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

func TestAttack_EmptyNodeAdoptsSelfSignedFabrication(t *testing.T) {
	// Attacker builds his own chain on the network's public genesis.
	// No victim key, no captured signature, no funding anywhere.
	priv, pub, _ := identity.Generate()
	attacker := vault.New(ledger.New())
	attacker.Domain = testDomain()
	if _, err := attacker.Log.AppendGenesis(testGenesis); err != nil {
		t.Fatal(err)
	}
	sig1, err := identity.Sign(priv, identity.PayloadFor(testDomain(), identity.OpDeposit, pub, 1000000, 1))
	if err != nil {
		t.Fatal(err)
	}
	if err := attacker.DepositFor(pub, 1, sig1, 1000000); err != nil {
		t.Fatalf("attacker deposit: %v", err)
	}
	sig2, err := identity.Sign(priv, identity.PayloadFor(testDomain(), identity.OpTip, pub, 250, 2))
	if err != nil {
		t.Fatal(err)
	}
	if err := attacker.TipFor(pub, 2, sig2, 250); err != nil {
		t.Fatalf("attacker tip: %v", err)
	}
	if err := attacker.Log.Verify(); err != nil {
		t.Fatalf("attacker chain must be hash/domain valid to isolate the gate: %v", err)
	}

	srv := ledgerServer(&server{v: attacker})
	defer srv.Close()

	// Victim: a fresh node holding only the genesis entry. Attacker
	// registers himself as a peer — no auth is required — and
	// triggers /sync, also no auth.
	victim := newTestServer(t)
	peerBody, _ := json.Marshal(peerReq{URL: srv.URL})
	rec := httptest.NewRecorder()
	victim.handlePeers(rec, httptest.NewRequest(http.MethodPost, "/peers", bytes.NewReader(peerBody)))
	if rec.Code != http.StatusOK {
		t.Fatalf("peer registration: got %d, body %s", rec.Code, rec.Body.String())
	}
	syncRec := doSync(victim)
	if syncRec.Code != http.StatusOK {
		t.Fatalf("sync: got %d, body %s", syncRec.Code, syncRec.Body.String())
	}
	var syncResp struct {
		Adopted bool `json:"adopted"`
	}
	_ = json.Unmarshal(syncRec.Body.Bytes(), &syncResp)

	// Safe behavior: a fresh node must not adopt an unauthenticated
	// peer's self-signed history as its state.
	if syncResp.Adopted || victim.v.R != 0 || victim.v.Log.Len() != 1 {
		t.Fatalf("HOLE DEMONSTRATED: fresh node adopted an attacker's fully self-signed fabricated chain "+
			"(no victim signature ever captured): adopted=%v, victim reserves=%d (was 0), floor=%d (was 0), "+
			"entries=%d (was 1, genesis only), attacker nonce now held=%d; grove-v2 binds signatures to a "+
			"domain but nothing binds that domain's history to a trusted origin, and /peers+/sync carry "+
			"no endpoint auth.",
			syncResp.Adopted, victim.v.R, victim.v.Floor, victim.v.Log.Len(), victim.nonces[pub])
	}
}
