package main

// Adoption-trust tests for issue #1, Proof Log Entry 23, reconciled
// to the v04-phase1 branch model. Ported from the 2026-10-28 daily
// cycle's scratch tests (node/attack_untrusted_extension_test.go in
// the scratch tree); the assertions are the cycle's, the
// construction is the branch's.
//
// Entry 23 generalizes Entry 22's bootstrap rule to the node's
// whole life: a signature proves an account authorized an op, not
// that the op was funded or that the peer offering it is honest.
// The scratch victim started from an empty chain; the branch victim
// starts from the shared genesis entry, so its honest pre-attack
// state is entries=2 (genesis + deposit), not 1.
//
// Entry 24 is NOT ported as a mechanism. The cycle's fix
// canonicalizes keys of a per-peer node-id map (-peer-node-id,
// peerNodeFlag.Set, canonicalPeerNodeIDs) — the branch has no such
// map, flag, or per-peer binding: entry 0 is the operator's
// unsigned config payload, and network binding is enforced by
// CheckDomain plus prefix extension (the Oct 27 port of Entry 22
// already deferred node-id genesis binding to the Phase 2 design
// table). The safety property the cycle's Entry 24 test pins — a
// configured trust decision must never silently degrade into a
// fallback that admits what the operator meant to exclude — is
// reconciled as TestAttack_TrustedPeerForeignGenesisRefused below:
// there is no fallback path around CheckDomain or the prefix gate,
// so even an operator-trusted peer's foreign-network chain is
// refused structurally. The remaining Entry 24 spellings
// (hostname-case, default-port, leading-zero-port) are properties
// of the scratch mechanism and have no branch counterpart.
// Test assets only; no real value anywhere.

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

// postPeer registers url on s through the real unauthenticated
// POST /peers handler, the way a remote stranger would.
func postPeer(t *testing.T, s *server, url string) {
	t.Helper()
	body, _ := json.Marshal(peerReq{URL: url})
	rec := httptest.NewRecorder()
	s.handlePeers(rec, httptest.NewRequest(http.MethodPost, "/peers", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /peers: got %d, body %s", rec.Code, rec.Body.String())
	}
}

// TestAttack_UntrustedPeerExtensionInflatesFloor is the Entry 23
// headline hole: a stranger's unfunded but genuinely signed tip,
// offered as an extension by an untrusted peer, is adopted by a
// non-empty node and permanently raises its floor. Every pre-trust
// gate passes on this candidate — prefix, replay, signatures —
// because the attacker signs correctly with his own key. Only the
// peer's trust status can refuse it.
func TestAttack_UntrustedPeerExtensionInflatesFloor(t *testing.T) {
	victim := newTestServer(t)
	priv, pub, _ := identity.Generate()
	if rec := postDeposit(victim, signReq(t, identity.OpDeposit, priv, pub, 100, 1)); rec.Code != http.StatusOK {
		t.Fatalf("victim deposit: got %d", rec.Code)
	}

	// Attacker: exact copy of the victim's chain, plus one tip
	// genuinely signed by the attacker's OWN fresh account (nonce
	// 1, bound to this network's domain, exactly as the gate
	// requires). No victim key and no funding — the tip is built
	// with the vault's own machinery so hash chain, staged replay
	// and signature checks all pass.
	attacker := vault.New(ledger.New())
	attacker.Domain = testDomain()
	if err := attacker.Log.Adopt(victim.v.Log.Entries(), t.TempDir()+"/chain.json"); err != nil {
		t.Fatalf("attacker copy of victim chain: %v", err)
	}
	attacker.R, attacker.C, attacker.Floor = victim.v.R, victim.v.C, victim.v.Floor
	atkPriv, atkPub, _ := identity.Generate()
	atkSig, err := identity.Sign(atkPriv, identity.PayloadFor(testDomain(), identity.OpTip, atkPub, 1000000, 1))
	if err != nil {
		t.Fatalf("attacker sign: %v", err)
	}
	if err := attacker.TipFor(atkPub, 1, atkSig, 1000000); err != nil {
		t.Fatalf("attacker tip: %v", err)
	}

	srv := ledgerServer(&server{v: attacker})
	defer srv.Close()

	// The whole attack is remote and unauthenticated: register,
	// then trigger sync. The victim's trusted list is never touched.
	postPeer(t, victim, srv.URL)
	if len(victim.peers) != 0 {
		t.Fatalf("setup: attacker peer leaked into trusted list: %v", victim.peers)
	}
	rec := doSync(victim)
	if rec.Code != http.StatusOK {
		t.Fatalf("sync: got %d, body %s", rec.Code, rec.Body.String())
	}

	// Safe behavior: a stranger's unfunded extension must not move
	// the victim's reserves or its never-down floor.
	if victim.v.R != 100 || victim.v.Floor != 0 || victim.v.Log.Len() != 2 {
		t.Fatalf("HOLE DEMONSTRATED: unauthenticated untrusted-peer extension adopted: "+
			"R=%d (was 100), floor=%d (was 0), entries=%d (was 2); "+
			"a stranger with no trust and no funding permanently raised the victim's floor by 1,000,000; "+
			"sync response: %s", victim.v.R, victim.v.Floor, victim.v.Log.Len(), rec.Body.String())
	}
}

// TestAttack_TrustedPeerForeignGenesisRefused is the branch
// reconciliation of the cycle's Entry 24 test (see the file
// header): an operator who configures a peer as trusted must
// never silently end up adopting a chain from a different
// network. The branch gives that refusal structurally —
// CheckDomain and prefix extension admit no fallback — so even
// a trusted peer serving a foreign-genesis chain is refused.
func TestAttack_TrustedPeerForeignGenesisRefused(t *testing.T) {
	const foreignGenesis = `{"kind":"genesis","network":"foreign"}`
	peer := newTestServerGenesis(t, foreignGenesis)

	// The peer's deposit must be signed over the foreign network's
	// own domain (the foreign genesis entry's hash), the way an
	// honest node of that network would sign it.
	probe := ledger.New()
	g, err := probe.AppendGenesis(foreignGenesis)
	if err != nil {
		t.Fatalf("foreign genesis probe: %v", err)
	}
	priv, pub, _ := identity.Generate()
	sig, err := identity.Sign(priv, identity.PayloadFor(g.Hash, identity.OpDeposit, pub, 500, 1))
	if err != nil {
		t.Fatalf("foreign sign: %v", err)
	}
	body, _ := json.Marshal(signedReq{Account: pub, Amount: 500, Nonce: 1, Signature: sig})
	if rec := postDeposit(peer, body); rec.Code != http.StatusOK {
		t.Fatalf("peer genesis deposit: got %d, body %s", rec.Code, rec.Body.String())
	}
	srv := ledgerServer(peer)
	defer srv.Close()

	victim := newTestServer(t)
	victim.peers = []string{srv.URL} // operator-configured: trusted
	if rec := doSync(victim); rec.Code != http.StatusOK {
		t.Fatalf("sync: got %d, body %s", rec.Code, rec.Body.String())
	}

	// Safe behavior: a foreign network's chain must not be
	// adopted, from any peer, trusted or not.
	if victim.v.Log.Len() != 1 || victim.v.R != 0 {
		t.Fatalf("HOLE DEMONSTRATED: foreign-genesis chain adopted from a trusted peer: "+
			"entries=%d (was 1), R=%d (was 0); the peer %q serves a different network's "+
			"history and no configured trust may admit it", victim.v.Log.Len(), victim.v.R, srv.URL)
	}
}
