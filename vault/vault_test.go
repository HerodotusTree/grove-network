package vault

import (
	"testing"

	"github.com/HerodotusTree/grove-network/ledger"
)

func TestHealthyFlow(t *testing.T) {
	v := New(ledger.New())
	if err := v.Deposit(100); err != nil {
		t.Fatal(err)
	}
	if v.Surplus() != 0 {
		t.Fatalf("want S=0, got %d", v.Surplus())
	}
	payout, err := v.Redeem(25)
	if err != nil {
		t.Fatal(err)
	}
	if payout != 25 {
		t.Fatalf("want payout 25, got %d", payout)
	}
	if v.Surplus() != 0 {
		t.Fatalf("want S=0, got %d", v.Surplus())
	}
	if v.Log.Len() != 2 {
		t.Fatalf("want 2 ledger entries, got %d", v.Log.Len())
	}
	if err := v.Log.Verify(); err != nil {
		t.Fatalf("ledger verify: %v", err)
	}
}

func TestImpairment(t *testing.T) {
	v := New(ledger.New())
	// Force impairment directly (simulates a loss event). The claims
	// belong to an account: under OWN-01 there are no ownerless claims.
	v.R, v.C = 80, 100
	v.Balances["acct"] = 100
	if err := v.Deposit(10); err == nil {
		t.Fatal("deposit during impairment should be rejected")
	}
	payout, err := v.RedeemFor("acct", 1, "sig", 50)
	if err != nil {
		t.Fatal(err)
	}
	// pro-rata: 50 * 80 / 100 = 40
	if payout != 40 {
		t.Fatalf("want pro-rata payout 40, got %d", payout)
	}
	// shortfall shrank: S was -20, now (80-40)-(100-50) = -10
	if v.Surplus() != -10 {
		t.Fatalf("want S=-10, got %d", v.Surplus())
	}
}

func TestTipRatchetsFloor(t *testing.T) {
	v := New(ledger.New())
	if err := v.Deposit(100); err != nil {
		t.Fatal(err)
	}
	if err := v.Tip(25); err != nil {
		t.Fatal(err)
	}
	if v.Floor != 25 {
		t.Fatalf("want floor 25, got %d", v.Floor)
	}
	if v.C != 100 {
		t.Fatalf("tip must not mint claims: want C=100, got %d", v.C)
	}
	if v.Surplus() != 25 {
		t.Fatalf("want S=25, got %d", v.Surplus())
	}
	// The floor holds through redemption: redeem everything, floor stays.
	if _, err := v.Redeem(100); err != nil {
		t.Fatal(err)
	}
	if v.Floor != 25 {
		t.Fatalf("floor must survive redemption: want 25, got %d", v.Floor)
	}
	if v.Surplus() != 25 {
		t.Fatalf("want S=25, got %d", v.Surplus())
	}
}
func TestOverflowRejected(t *testing.T) {
	const maxInt64 = int64(^uint64(0) >> 1)
	v := New(ledger.New())
	if err := v.Deposit(maxInt64); err != nil {
		t.Fatalf("max-int64 deposit should be accepted: %v", err)
	}
	if err := v.Deposit(1); err == nil {
		t.Fatal("deposit past int64 range must be rejected")
	}
	if v.R != maxInt64 || v.C != maxInt64 {
		t.Fatalf("rejected op changed state: R=%d C=%d", v.R, v.C)
	}
	if err := v.Tip(1); err == nil {
		t.Fatal("tip past int64 range must be rejected")
	}
	if p, err := v.Redeem(maxInt64); err != nil || p != maxInt64 {
		t.Fatalf("redeem at max int64: payout=%d err=%v", p, err)
	}
	if v.R != 0 || v.C != 0 || v.Surplus() != 0 {
		t.Fatalf("after full redeem: R=%d C=%d S=%d", v.R, v.C, v.Surplus())
	}
}
