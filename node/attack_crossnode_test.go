package main

// Independent attacker pass (Claude), 2026-10-17.
//
// These tests assert the SAFE behavior. They are failing-as-intended:
// each one fails on the v0.3 patch and should pass once the hole is
// closed. Test assets only — no real value anywhere.

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

func ledgerServer(s *server) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(s.handleLedger))
}

func doSync(s *server) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	s.handleSync(rec, httptest.NewRequest(http.MethodPost, "/sync", nil))
	return rec
}

// A1. Cross-node replay. Signatures carry no chain or node binding, and
// nonce tables are per-node. A signed request captured once (plain HTTP)
// is good once PER NODE, not once total.
func TestAttack_CrossNodeReplay(t *testing.T) {
	a, b := newTestServer(t), newTestServer(t)
	priv, pub, _ := identity.Generate()
	body := signReq(t, identity.OpDeposit, priv, pub, 100, 1)

	if rec := postDeposit(a, body); rec.Code != http.StatusOK {
		t.Fatalf("victim's deposit on A: got %d", rec.Code)
	}
	// Attacker replays the exact captured bytes to B. Victim never sent them to B.
	rec := postDeposit(b, body)
	if rec.Code == http.StatusOK {
		t.Fatalf("REPLAY ACCEPTED on node B: request signed once for A was applied again on B (B R=%d)", b.v.R)
	}
}

// A2. The patch's own NODE.md "Known limits (v0.3)" says that after a
// sync, a stale nonce table "fails closed: a stale table can only
// reject, never replay." This tests that sentence directly.
func TestAttack_ReplayAfterSync_ContradictsKnownLimits(t *testing.T) {
	a, b := newTestServer(t), newTestServer(t)
	priv, pub, _ := identity.Generate()
	body := signReq(t, identity.OpDeposit, priv, pub, 100, 1)

	if rec := postDeposit(a, body); rec.Code != http.StatusOK {
		t.Fatalf("deposit on A: got %d", rec.Code)
	}
	srv := ledgerServer(a)
	defer srv.Close()
	b.peers = []string{srv.URL}
	if rec := doSync(b); rec.Code != http.StatusOK {
		t.Fatalf("sync: %d %s", rec.Code, rec.Body.String())
	}
	if b.v.R != 100 || b.v.Log.Len() != 1 {
		t.Fatalf("setup: B should hold A's history after sync, got R=%d entries=%d", b.v.R, b.v.Log.Len())
	}
	// B now holds the ledger entry for the victim's nonce-1 deposit, but its
	// nonce table never saw it. Replay the captured request.
	rec := postDeposit(b, body)
	if rec.Code == http.StatusOK {
		t.Fatalf("REPLAY ACCEPTED after sync: the op already in B's ledger was applied a second time "+
			"(B R=%d, ledger entries=%d). NODE.md claims a stale table can never replay.", b.v.R, b.v.Log.Len())
	}
}

// A3. The floor ratchet ("absolute surplus never steps backward", floor
// never down) is checked per local op, not across adopted history.
// A longer, hash-valid chain whose LAST entry reports lower numbers is
// adopted, the floor steps backward, and /metrics still reports zero
// invariant violations.
func TestAttack_SyncStepsFloorBackward_MetricsStaySilent(t *testing.T) {
	b := newTestServer(t)
	priv, pub, _ := identity.Generate()
	if rec := postDeposit(b, signReq(t, identity.OpDeposit, priv, pub, 100, 1)); rec.Code != http.StatusOK {
		t.Fatalf("deposit: %d", rec.Code)
	}
	tip, _ := json.Marshal(signedReq{Account: pub, Amount: 25, Nonce: 2,
		Signature: mustSig(t, priv, identity.Payload(identity.OpTip, pub, 25, 2))})
	trec := httptest.NewRecorder()
	b.handleTip(trec, httptest.NewRequest(http.MethodPost, "/tip", bytes.NewReader(tip)))
	if trec.Code != http.StatusOK {
		t.Fatalf("tip: %d", trec.Code)
	}
	floorBefore, sBefore := b.v.Floor, b.v.Surplus() // 25, 25

	// Fabricated peer: 3 hash-valid entries, last one reports floor 0, surplus 0.
	forged := ledger.New()
	for _, op := range []vault.Op{
		{Kind: "deposit", Amount: 100, R: 100, C: 100},
		{Kind: "deposit", Amount: 1, R: 101, C: 101},
		{Kind: "deposit", Amount: 1, R: 102, C: 102},
	} {
		p, _ := json.Marshal(op)
		forged.Append(string(p))
	}
	fs := &server{v: vault.New(forged)}
	srv := ledgerServer(fs)
	defer srv.Close()
	b.peers = []string{srv.URL}
	doSync(b)

	var m Metrics
	mrec := httptest.NewRecorder()
	b.handleMetrics(mrec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	_ = json.Unmarshal(mrec.Body.Bytes(), &m)

	if b.v.Floor < floorBefore || b.v.Surplus() < sBefore {
		t.Fatalf("FLOOR STEPPED BACKWARD via sync: floor %d -> %d, surplus %d -> %d; "+
			"metrics invariant_violations=%d", floorBefore, b.v.Floor, sBefore, b.v.Surplus(), m.InvariantViolations)
	}
}

func mustSig(t *testing.T, priv, payload string) string {
	t.Helper()
	s, err := identity.Sign(priv, payload)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
