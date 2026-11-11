package main

// Attacker pass on issue #1, Entry 18 (signed-transplant variant).
//
// THE HOLE (grove-v1): the signed payload covered NO genesis hash, so
// a victim's captured signature verified on ANY chain the attacker
// built around it, provided the victim's nonce was next-sequential
// on the target.
//
// RECONCILED 2026-10-26 against the v04-phase1 branch: the branch's
// Phase 1 format is grove-v2|<domain>|<op>|<account>|<amount>|<nonce>
// (identity.PayloadFor), where the domain is the hash of the genesis
// entry built from the network's genesis config file. checkAuth
// binds direct submissions to the node's domain, but the sync
// adoption gate originally replayed candidate ops WITHOUT verifying
// their signatures — a signature made for a foreign domain could be
// re-homed into an entry stamped with this network's domain and
// adopted. The gate now verifies every extension entry's signature
// against the local domain (verifyExtensionSignatures in main.go,
// ported from the daily cycle's Oct 25 adoption gate). These tests
// assert the SAFE behavior (refusal). Test assets only.

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/HerodotusTree/grove-network/identity"
	"github.com/HerodotusTree/grove-network/ledger"
	"github.com/HerodotusTree/grove-network/vault"
)

// foreignDomain returns the domain of a different network (a genesis
// config this node does not share).
func foreignDomain(t *testing.T) string {
	t.Helper()
	probe := ledger.New()
	g, err := probe.AppendGenesis(`{"kind":"genesis","network":"foreign"}`)
	if err != nil {
		t.Fatal(err)
	}
	return g.Hash
}

// TestAttack_TransplantedSignatureAdoptedViaSync: the target holds
// the network genesis plus one operator deposit. The attacker builds
// a candidate that shares that exact prefix, then appends the
// victim's signed nonce-1 deposit — captured from a DIFFERENT
// network, never submitted to this target — as the next entry,
// stamped with THIS network's domain. Prefix rule, domain check,
// and staged replay all pass; only the signature's foreign domain
// binding can refuse it.
func TestAttack_TransplantedSignatureAdoptedViaSync(t *testing.T) {
	target := newTestServer(t)
	opPriv, opPub, _ := identity.Generate()
	if rec := postDeposit(target, signReq(t, identity.OpDeposit, opPriv, opPub, 10, 1)); rec.Code != http.StatusOK {
		t.Fatalf("genesis deposit: got %d", rec.Code)
	}
	if target.v.Log.Len() != 2 || target.v.R != 10 {
		t.Fatalf("setup: target R=%d entries=%d", target.v.R, target.v.Log.Len())
	}

	// Victim's signature, made for a deposit of 100 at nonce 1 on a
	// DIFFERENT network — the attacker never holds the victim's key;
	// this is a captured request / a signature copied from another
	// network's public ledger.
	victimPriv, victimPub, _ := identity.Generate()
	victimSig, err := identity.Sign(victimPriv, identity.PayloadFor(foreignDomain(t), identity.OpDeposit, victimPub, 100, 1))
	if err != nil {
		t.Fatal(err)
	}

	peer := newTestServer(t)
	if err := peer.v.Log.Adopt(target.v.Log.Entries(), peer.chainPath); err != nil {
		t.Fatalf("peer seed history: %v", err)
	}
	op, _ := json.Marshal(vault.Op{Kind: "deposit", Amount: 100, R: 110, C: 110, Floor: 0})
	peer.v.Log.AppendSigned(testDomain(), victimPub, 1, string(op), victimSig)

	srv := ledgerServer(peer)
	defer srv.Close()
	target.peers = []string{srv.URL}
	if rec := doSync(target); rec.Code != http.StatusOK {
		t.Fatalf("sync: got %d, body %s", rec.Code, rec.Body.String())
	}
	if target.v.R != 10 || target.v.Log.Len() != 2 {
		t.Fatalf("TRANSPLANT ADOPTED via sync: target R=%d entries=%d, want R=10 entries=2 (victim's captured foreign-domain deposit credited on a history it never joined)", target.v.R, target.v.Log.Len())
	}
}

// TestAttack_TransplantedSignatureAcceptedOnFreshGenesis: the same
// victim signature, carried as the second extension entry of a
// candidate built on the target's own genesis (the first extension
// entry is the attacker's own, validly signed for this domain — the
// gate must refuse the chain on the VICTIM's entry, not on his).
func TestAttack_TransplantedSignatureAcceptedOnFreshGenesis(t *testing.T) {
	target := newTestServer(t)

	atkPriv, atkPub, _ := identity.Generate()
	atkSig, err := identity.Sign(atkPriv, identity.PayloadFor(testDomain(), identity.OpDeposit, atkPub, 5, 1))
	if err != nil {
		t.Fatal(err)
	}
	victimPriv, victimPub, _ := identity.Generate()
	victimSig, err := identity.Sign(victimPriv, identity.PayloadFor(foreignDomain(t), identity.OpDeposit, victimPub, 100, 1))
	if err != nil {
		t.Fatal(err)
	}

	chain := ledger.New()
	if _, err := chain.AppendGenesis(testGenesis); err != nil {
		t.Fatal(err)
	}
	op0, _ := json.Marshal(vault.Op{Kind: "deposit", Amount: 5, R: 5, C: 5, Floor: 0})
	chain.AppendSigned(testDomain(), atkPub, 1, string(op0), atkSig)
	// Victim's op, re-homed under this chain's domain — the attacker
	// just recomputes hashes, no key needed.
	op1, _ := json.Marshal(vault.Op{Kind: "deposit", Amount: 100, R: 105, C: 105, Floor: 0})
	chain.AppendSigned(testDomain(), victimPub, 1, string(op1), victimSig)

	srv := ledgerServer(&server{v: vault.New(chain)})
	defer srv.Close()
	target.peers = []string{srv.URL}
	if rec := doSync(target); rec.Code != http.StatusOK {
		t.Fatalf("sync: got %d, body %s", rec.Code, rec.Body.String())
	}
	if target.v.R != 0 || target.v.Log.Len() != 1 {
		t.Fatalf("TRANSPLANT ADOPTED ON FRESH GENESIS: target R=%d entries=%d, want R=0 entries=1 (victim's signature verified on a chain whose domain it never signed for)", target.v.R, target.v.Log.Len())
	}
}
