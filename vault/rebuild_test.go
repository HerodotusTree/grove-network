package vault

import (
	"testing"

	"github.com/HerodotusTree/grove-network/ledger"
)

// Phase 1 step 9 (OWN-01): a redeem is bounded by the redeemer's own
// balance, not by the vault's aggregate claims.
func TestRedeemBoundedByAccountBalance(t *testing.T) {
	v := New(ledger.New())
	if err := v.DepositFor("acctA", 1, "sig", 100); err != nil {
		t.Fatal(err)
	}
	if err := v.DepositFor("acctB", 1, "sig", 50); err != nil {
		t.Fatal(err)
	}
	if _, err := v.RedeemFor("acctB", 2, "sig", 100); err == nil {
		t.Fatal("B redeemed 100 against a balance of 50 (aggregate C=150 allowed it)")
	}
	if _, err := v.RedeemFor("acctB", 2, "sig", 50); err != nil {
		t.Fatalf("B redeeming its own 50: %v", err)
	}
	if _, err := v.RedeemFor("acctA", 2, "sig", 100); err != nil {
		t.Fatalf("A redeeming its own 100: %v", err)
	}
	if v.C != 0 || v.Balances["acctA"] != 0 || v.Balances["acctB"] != 0 {
		t.Fatalf("end state wrong: C=%d balances=%v", v.C, v.Balances)
	}
}

// Phase 1 step 9: Rebuild replays the ledger and refuses a chain whose
// recorded numbers do not recompute.
func TestRebuildDerivesAndVerifies(t *testing.T) {
	l := ledger.New()
	v := New(l)
	if _, err := l.AppendGenesis(`{"kind":"genesis","network":"trial-1"}`); err != nil {
		t.Fatal(err)
	}
	d, _ := l.Domain()
	v.Domain = d
	if err := v.DepositFor("acctA", 1, "sig", 100); err != nil {
		t.Fatal(err)
	}
	if err := v.TipFor("acctA", 2, "sig", 25); err != nil {
		t.Fatal(err)
	}
	v2 := New(l)
	if err := v2.Rebuild(); err != nil {
		t.Fatalf("rebuild of an honest chain: %v", err)
	}
	if v2.R != 125 || v2.C != 100 || v2.Floor != 25 || v2.Balances["acctA"] != 100 {
		t.Fatalf("rebuilt state wrong: R=%d C=%d Floor=%d balances=%v", v2.R, v2.C, v2.Floor, v2.Balances)
	}

	forged := ledger.New()
	forged.AppendSigned("", "acctA", 1, `{"kind":"deposit","amount":100,"reserves_after":999,"claims_after":100,"floor_after":0}`, "sig")
	v3 := New(forged)
	if err := v3.Rebuild(); err == nil {
		t.Fatal("rebuild accepted a chain whose recorded reserves do not recompute")
	}
}
