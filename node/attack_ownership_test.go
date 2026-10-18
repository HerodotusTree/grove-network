package main

// Independent attacker pass #2 (Claude), 2026-10-17 — ownership angle.
//
// The v0.3 identity layer proves WHO signed a request. It never checks
// that the signer OWNS the claims being redeemed: claims are one
// aggregate number C with no per-account balance. These tests assert
// the SAFE behavior and are failing-as-intended on v0.3. Test assets
// only — no real value anywhere.

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HerodotusTree/grove-network/identity"
)

func postRedeem(s *server, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/redeem", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.handleRedeem(rec, req)
	return rec
}

// B1. Stranger redeem. A fresh keypair that never deposited redeems the
// victim's entire deposit. Every check passes: the signature is valid,
// the nonce is the stranger's first (1), and the amount is <= aggregate C.
func TestAttack_StrangerRedeemsVictimsClaims(t *testing.T) {
	s := newTestServer(t)

	vPriv, vPub, _ := identity.Generate()
	if rec := postDeposit(s, signReq(t, identity.OpDeposit, vPriv, vPub, 100, 1)); rec.Code != http.StatusOK {
		t.Fatalf("victim deposit: got %d", rec.Code)
	}

	// Attacker: brand-new key, zero deposits, never seen by the node.
	aPriv, aPub, _ := identity.Generate()
	rec := postRedeem(s, signReq(t, identity.OpRedeem, aPriv, aPub, 100, 1))

	if rec.Code == http.StatusOK {
		t.Fatalf("STRANGER REDEEM ACCEPTED: account with zero deposits redeemed the victim's 100 (vault now R=%d C=%d; victim's claim is gone). Body: %s",
			s.v.R, s.v.C, rec.Body.String())
	}
}

// B2. The victim is then locked out of their own money. Whatever the
// fix, the victim's redeem must still succeed after a stranger tries.
func TestAttack_VictimLockedOutAfterStrangerRedeem(t *testing.T) {
	s := newTestServer(t)

	vPriv, vPub, _ := identity.Generate()
	postDeposit(s, signReq(t, identity.OpDeposit, vPriv, vPub, 100, 1))

	aPriv, aPub, _ := identity.Generate()
	postRedeem(s, signReq(t, identity.OpRedeem, aPriv, aPub, 100, 1))

	rec := postRedeem(s, signReq(t, identity.OpRedeem, vPriv, vPub, 100, 2))
	if rec.Code != http.StatusOK {
		t.Fatalf("VICTIM LOCKED OUT: owner's redeem of their own 100 got %d: %s", rec.Code, rec.Body.String())
	}
}
