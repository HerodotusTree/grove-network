package ledger

import "testing"

func TestChain(t *testing.T) {
	l := New()
	l.Append("genesis: floor established")
	l.Append("deposit: 100")
	l.Append("redeem: 25")
	if err := l.Verify(); err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if l.Len() != 3 {
		t.Fatalf("want 3 entries, got %d", l.Len())
	}
	// Tamper with history: verification must fail.
	l.entries[1].Payload = "deposit: 1000000"
	if err := l.Verify(); err == nil {
		t.Fatal("tampered chain verified clean; want error")
	}
}

