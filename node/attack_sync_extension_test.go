package main

// Attacker pass on the #1 sync-hardening build (2026-10-24).
//
// The builder's prefix rule stops a longer DIVERGENT chain from
// displacing acknowledged local history. It does not stop a chain
// that EXTENDS local history, because v0.3 ledger entries carry no
// account, nonce, or signature, and /ledger, /peers and /sync are
// all unauthenticated. An attacker who can read the victim's public
// ledger can therefore fabricate a self-consistent extension to it,
// and staged replay cannot tell that extension from an honest one.
//
// Both tests below assert the SAFE behavior and fail on the current
// build. Test assets only; no real value anywhere.

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/HerodotusTree/grove-network/identity"
	"github.com/HerodotusTree/grove-network/ledger"
	"github.com/HerodotusTree/grove-network/vault"
)

// TestAttack_SyncAdoptsFabricatedPrefixExtension: the attacker reads
// the victim's own (public, unauthenticated) /ledger, copies it
// byte-identically, and appends a self-consistent tip of 1,000,000
// that no one ever signed or funded. Prefix + hash + replay all
// pass, so the victim adopts it and its floor/reserves inflate.
func TestAttack_SyncAdoptsFabricatedPrefixExtension(t *testing.T) {
	victim := newTestServer(t)
	priv, pub, _ := identity.Generate()
	if rec := postDeposit(victim, signReq(t, identity.OpDeposit, priv, pub, 100, 1)); rec.Code != http.StatusOK {
		t.Fatalf("victim deposit: got %d", rec.Code)
	}

	// Attacker: exact copy of the victim's chain, plus one fabricated
	// but self-consistent extension entry. No key, no signature, no
	// funding is needed to mint this entry — Append computes its hash.
	attacker := ledger.New()
	if err := attacker.Adopt(victim.v.Log.Entries(), t.TempDir()+"/chain.json"); err != nil {
		t.Fatalf("attacker copy of victim chain: %v", err)
	}
	tip, _ := json.Marshal(vault.Op{Kind: "tip", Amount: 1000000, R: 1000100, C: 100, Floor: 1000000})
	attacker.Append(string(tip))

	srv := ledgerServer(&server{v: vault.New(attacker)})
	defer srv.Close()
	victim.peers = []string{srv.URL}
	rec := doSync(victim)
	if rec.Code != http.StatusOK {
		t.Fatalf("sync: got %d, body %s", rec.Code, rec.Body.String())
	}

	// Safe behavior: an unsigned, unfunded extension must not move
	// the victim's reserves or floor, however self-consistent it is.
	if victim.v.R != 100 || victim.v.Floor != 0 || victim.v.Log.Len() != 2 {
		t.Fatalf("HOLE DEMONSTRATED: /sync adopted a fabricated prefix extension: "+
			"R=%d (was 100), floor=%d (was 0), entries=%d (was 2); "+
			"an unauthenticated attacker inflated the vault by 1,000,000 with no key and no funding; sync response: %s",
			victim.v.R, victim.v.Floor, victim.v.Log.Len(), rec.Body.String())
	}
}

// TestAttack_ReplayAfterHonestPrefixExtensionAdoption: the #1 fix's
// new legitimate case is itself a replay vector. The victim adopts
// an honest prefix extension whose second op was signed once (nonce
// 2 on the peer). Adoption installs the ledger entry but never
// advances the victim's nonce table — v0.3 entries carry no account
// or nonce to advance it from — so the captured nonce-2 request
// replays on the victim and mints the same 50 a second time.
func TestAttack_ReplayAfterHonestPrefixExtensionAdoption(t *testing.T) {
	victim := newTestServer(t)
	priv, pub, _ := identity.Generate()
	if rec := postDeposit(victim, signReq(t, identity.OpDeposit, priv, pub, 100, 1)); rec.Code != http.StatusOK {
		t.Fatalf("victim deposit: got %d", rec.Code)
	}

	// Peer with the victim's exact history, then one honest signed
	// extension op (nonce 2, amount 50). Adopt + state restore give
	// the peer the victim's history without a timestamp race.
	peer := newTestServer(t)
	if err := peer.v.Log.Adopt(victim.v.Log.Entries(), peer.chainPath); err != nil {
		t.Fatalf("peer adopt victim history: %v", err)
	}
	peer.v.R, peer.v.C, peer.v.Floor = 100, 100, 0
	peer.nonces[pub] = 1 // peer has seen nonce 1, as the victim has
	extension := signReq(t, identity.OpDeposit, priv, pub, 50, 2)
	if rec := postDeposit(peer, extension); rec.Code != http.StatusOK {
		t.Fatalf("peer extension deposit: got %d, body %s", rec.Code, rec.Body.String())
	}

	srv := ledgerServer(peer)
	defer srv.Close()
	victim.peers = []string{srv.URL}
	if rec := doSync(victim); rec.Code != http.StatusOK {
		t.Fatalf("sync: got %d, body %s", rec.Code, rec.Body.String())
	}
	if victim.v.R != 150 || victim.v.Log.Len() != 3 {
		t.Fatalf("setup: honest extension not adopted: R=%d entries=%d", victim.v.R, victim.v.Log.Len())
	}

	// The extension op is already in the victim's ledger (entry 1).
	// Its signed request must now be dead on the victim.
	rec := postDeposit(victim, extension)
	if rec.Code == http.StatusOK {
		t.Fatalf("HOLE DEMONSTRATED: signed op replayed after prefix-extension adoption: "+
			"nonce-2 deposit applied a second time (R=%d, was 150; entries=%d, was 3). "+
			"Adoption installed the entry but left the victim's nonce table at 1, so the "+
			"already-adopted op was spendable again.", victim.v.R, victim.v.Log.Len())
	}
	if victim.v.R != 150 || victim.v.Log.Len() != 3 {
		t.Fatalf("replay mutated state despite non-200: R=%d entries=%d, sync replay code %d", victim.v.R, victim.v.Log.Len(), rec.Code)
	}
}
