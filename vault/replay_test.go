package vault

import (
	"encoding/json"
	"testing"

	"github.com/HerodotusTree/grove-network/ledger"
)

func replayChain(t *testing.T, ops []Op) []ledger.Entry {
	t.Helper()
	l := ledger.New()
	for _, op := range ops {
		b, err := json.Marshal(op)
		if err != nil {
			t.Fatal(err)
		}
		l.Append(string(b))
	}
	return l.Entries()
}

func TestReplayValidChain(t *testing.T) {
	entries := replayChain(t, []Op{
		{Kind: "deposit", Amount: 100, R: 100, C: 100, Floor: 0},
		{Kind: "tip", Amount: 25, R: 125, C: 100, Floor: 25},
		{Kind: "redeem", Amount: 40, R: 85, C: 60, Floor: 25},
	})
	r, c, floor, err := Replay(entries)
	if err != nil {
		t.Fatalf("valid chain rejected: %v", err)
	}
	if r != 85 || c != 60 || floor != 25 {
		t.Fatalf("replayed state R=%d C=%d floor=%d, want 85/60/25", r, c, floor)
	}
}

func TestReplayRejectsClaimedStateMismatch(t *testing.T) {
	// Fabricated economics with correct op kinds but numbers that do
	// not follow from the ops: a "tip" of one million that the chain
	// itself does not account for.
	entries := replayChain(t, []Op{
		{Kind: "tip", Amount: 1000000, R: 1000000, C: 0, Floor: 1000000},
		{Kind: "deposit", Amount: 500, R: 1000500, C: 500, Floor: 1000000},
	})
	// That chain is self-consistent, so it replays — the limit is
	// stated in replay.go: replay cannot catch a self-consistent
	// fabrication. What it must catch is an inconsistent one:
	bad := replayChain(t, []Op{
		{Kind: "deposit", Amount: 100, R: 100, C: 100, Floor: 0},
		{Kind: "deposit", Amount: 50, R: 999, C: 150, Floor: 0}, // R should be 150
	})
	if _, _, _, err := Replay(bad); err == nil {
		t.Fatal("chain with claimed R that does not follow from its ops was accepted")
	}
	// And the self-consistent fabrication is honestly reported as
	// replay-valid here, so no later reader mistakes replay for
	// proof of provenance.
	if _, _, _, err := Replay(entries); err != nil {
		t.Fatalf("self-consistent chain should replay (provenance is a separate question): %v", err)
	}
}

func TestReplayRejectsPoisonAndIllegalOps(t *testing.T) {
	l := ledger.New()
	l.Append("this is not a vault op")
	if _, _, _, err := Replay(l.Entries()); err == nil {
		t.Fatal("non-vault payload accepted by replay")
	}

	l2 := ledger.New()
	b, _ := json.Marshal(Op{Kind: "mint", Amount: 1, R: 1, C: 0, Floor: 0})
	l2.Append(string(b))
	if _, _, _, err := Replay(l2.Entries()); err == nil {
		t.Fatal("unknown op kind accepted by replay")
	}

	l3 := ledger.New()
	b, _ = json.Marshal(Op{Kind: "redeem", Amount: 10, R: 0, C: 0, Floor: 0})
	l3.Append(string(b))
	if _, _, _, err := Replay(l3.Entries()); err == nil {
		t.Fatal("redeem exceeding claims accepted by replay")
	}

	// Index gap: entries must be contiguous from genesis.
	entries := replayChain(t, []Op{{Kind: "deposit", Amount: 100, R: 100, C: 100, Floor: 0}})
	entries[0].Index = 7
	if _, _, _, err := Replay(entries); err == nil {
		t.Fatal("index gap accepted by replay")
	}
}
