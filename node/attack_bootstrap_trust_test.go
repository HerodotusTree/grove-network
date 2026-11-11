package main

// Bootstrap-trust tests for issue #1, Proof Log Entry 22 (AUTH-02),
// reconciled to the v04-phase1 branch model.
//
// The branch's fresh node holds the shared genesis entry from its
// config (entries=1), not an empty chain: a fabricated chain is
// built ON that genesis. The untrusted-refusal form is pinned by
// the branch's existing TestAttack_EmptyNodeAdoptsSelfSignedFabrication
// (attack_bootstrap_test.go), which this port turns from a
// demonstration into a pass; these tests pin the legitimate paths
// the trust gate must NOT break, plus the out-of-band genesis pin
// and the peer-listing contract.
//
// NOT ported from the cycle's scratch tests: node-id genesis
// binding (TestBootstrap_NodeIDBindsGenesisDeposit,
// TestBootstrap_NodeBoundChainAdoptsViaConfiguredPeer). The branch
// has no signed genesis op — entry 0 is the operator's unsigned
// config payload — so that mechanism has no branch counterpart; it
// is a design question for the architect, not a ported behavior.
// Test assets only.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HerodotusTree/grove-network/identity"
)

func postTip(s *server, body []byte) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	s.handleTip(rec, httptest.NewRequest(http.MethodPost, "/tip", bytes.NewReader(body)))
	return rec
}

// honestPeerWithHistory builds a peer holding two genuinely signed
// ops (a deposit and a tip) on the shared test genesis, and returns
// it plus its ledger-serving URL.
func honestPeerWithHistory(t *testing.T) (*server, string) {
	t.Helper()
	peer := newTestServer(t)
	priv, pub, _ := identity.Generate()
	if rec := postDeposit(peer, signReq(t, identity.OpDeposit, priv, pub, 1000, 1)); rec.Code != http.StatusOK {
		t.Fatalf("peer deposit: got %d", rec.Code)
	}
	if rec := postTip(peer, signReq(t, identity.OpTip, priv, pub, 25, 2)); rec.Code != http.StatusOK {
		t.Fatalf("peer tip: got %d", rec.Code)
	}
	srv := ledgerServer(peer)
	t.Cleanup(srv.Close)
	return peer, srv.URL
}

func syncAdopted(t *testing.T, s *server) bool {
	t.Helper()
	rec := doSync(s)
	if rec.Code != http.StatusOK {
		t.Fatalf("sync: got %d, body %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Adopted bool `json:"adopted"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	return resp.Adopted
}

// TestBootstrap_TrustedPeerStillBootstrapsFreshNode pins the
// legitimate bootstrap: a fresh node (genesis only) whose operator
// configured the peer (direct peers assignment — the in-process
// form of -trusted-peer) still adopts that peer's honestly signed
// history. The trust gate must refuse strangers, not break sync.
func TestBootstrap_TrustedPeerStillBootstrapsFreshNode(t *testing.T) {
	_, url := honestPeerWithHistory(t)
	victim := newTestServer(t)
	victim.peers = []string{url}
	if !syncAdopted(t, victim) {
		t.Fatal("trusted peer's history not adopted by fresh node")
	}
	if victim.v.R != 1025 || victim.v.Floor != 25 || victim.v.Log.Len() != 3 {
		t.Fatalf("trusted bootstrap broken: R=%d floor=%d entries=%d",
			victim.v.R, victim.v.Floor, victim.v.Log.Len())
	}
}

// TestBootstrap_TrustedGenesisGate pins the out-of-band genesis
// option: with a wrong trusted genesis configured, even a trusted
// peer's candidate is refused; with the candidate's actual entry-0
// hash configured, the same candidate adopts.
func TestBootstrap_TrustedGenesisGate(t *testing.T) {
	peer, url := honestPeerWithHistory(t)
	genesis := peer.v.Log.Entries()[0].Hash

	wrong := newTestServer(t)
	wrong.peers = []string{url}
	wrong.trustedGenesis = "deadbeef-wrong-genesis"
	if syncAdopted(t, wrong) {
		t.Fatal("wrong trusted genesis adopted")
	}
	if wrong.v.Log.Len() != 1 || wrong.v.R != 0 {
		t.Fatalf("wrong trusted genesis mutated node: entries=%d R=%d", wrong.v.Log.Len(), wrong.v.R)
	}

	right := newTestServer(t)
	right.peers = []string{url}
	right.trustedGenesis = genesis
	if !syncAdopted(t, right) {
		t.Fatal("matching trusted genesis not adopted")
	}
	if right.v.Log.Len() != 3 || right.v.R != 1025 {
		t.Fatalf("matching trusted genesis adoption wrong: entries=%d R=%d", right.v.Log.Len(), right.v.R)
	}
}

// TestBootstrap_PeersListingJoinsTrustedAndUntrusted pins the
// listing contract after the split: GET /peers shows both the
// operator-configured trusted list and the unauthenticated
// registrants, so monitoring still sees the whole table.
func TestBootstrap_PeersListingJoinsTrustedAndUntrusted(t *testing.T) {
	s := newTestServer(t)
	s.peers = []string{"http://127.0.0.1:9001"}
	body, _ := json.Marshal(peerReq{URL: "http://127.0.0.1:9002"})
	rec := httptest.NewRecorder()
	s.handlePeers(rec, httptest.NewRequest(http.MethodPost, "/peers", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("register: got %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	s.handlePeers(rec, httptest.NewRequest(http.MethodGet, "/peers", nil))
	var resp struct {
		Peers []string `json:"peers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("peers decode: %v", err)
	}
	want := map[string]bool{"http://127.0.0.1:9001": false, "http://127.0.0.1:9002": false}
	for _, p := range resp.Peers {
		want[p] = true
	}
	for p, seen := range want {
		if !seen {
			t.Fatalf("GET /peers missing %s (got %v)", p, resp.Peers)
		}
	}
}
