package vault

import (
	"testing"

	"github.com/HerodotusTree/grove-network/ledger"
)

// Phase 1 step 4: an operation's authorization envelope (account, nonce,
// signature) and the vault's domain are recorded in the ledger entry
// itself, so replay can verify who authorized what.
func TestEnvelopeRecordedInEntry(t *testing.T) {
	v := New(ledger.New())
	v.Domain = "domhash"
	if err := v.DepositFor("acctkey", 1, "sigbytes", 100); err != nil {
		t.Fatal(err)
	}
	entries := v.Log.Entries()
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Account != "acctkey" || e.Nonce != 1 || e.Signature != "sigbytes" || e.Domain != "domhash" {
		t.Fatalf("envelope not recorded: %+v", e)
	}
	if err := v.Log.Verify(); err != nil {
		t.Fatalf("chain should verify: %v", err)
	}
}
