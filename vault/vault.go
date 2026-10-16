// Package vault implements the Grove trial vault: 1:1 redeemable claims
// with the absolute-surplus invariant S = R - C, non-decreasing.
// All amounts are integer minor units. Every accepted operation is
// appended to the hash-chained ledger.
//
// Tips feed the floor: they add to reserves and to the non-redeemable
// floor ledger, ratcheting the floor up without minting claims.
package vault

import (
	"encoding/json"
	"fmt"

	"github.com/HerodotusTree/grove-network/ledger"
)

// Op is a vault operation recorded on the ledger.
type Op struct {
	Kind   string `json:"kind"` // "deposit", "redeem", or "tip"
	Amount int64  `json:"amount"`
	R      int64  `json:"reserves_after"`
	C      int64  `json:"claims_after"`
	Floor  int64  `json:"floor_after"`
}

// Vault holds reserves R, redeemable claims C, and the non-redeemable floor.
type Vault struct {
	R     int64
	C     int64
	Floor int64
	Log   *ledger.Ledger
}

// New returns a vault wired to the given ledger.
func New(log *ledger.Ledger) *Vault { return &Vault{Log: log} }

// Surplus returns S = R - C.
func (v *Vault) Surplus() int64 { return v.R - v.C }

func (v *Vault) record(kind string, amount int64) {
	rec, _ := json.Marshal(Op{Kind: kind, Amount: amount, R: v.R, C: v.C, Floor: v.Floor})
	v.Log.Append(string(rec))
}

// Deposit adds reserves and mints an equal claim. S is unchanged.
// Intake is closed while impaired (R < C).
func (v *Vault) Deposit(amount int64) error {
	if amount <= 0 {
		return fmt.Errorf("deposit: amount must be positive")
	}
	if v.R < v.C {
		return fmt.Errorf("deposit: intake closed while impaired (R=%d < C=%d)", v.R, v.C)
	}
	v.R += amount
	v.C += amount
	v.record("deposit", amount)
	return nil
}

// Tip is a gift to the floor: it adds to reserves and to the
// non-redeemable floor ledger, minting no claims. S grows by amount.
// Tips are how the floor ratchets up; without them it holds flat,
// never down.
func (v *Vault) Tip(amount int64) error {
	if amount <= 0 {
		return fmt.Errorf("tip: amount must be positive")
	}
	v.R += amount
	v.Floor += amount
	v.record("tip", amount)
	return nil
}

// Redeem burns claims and pays out reserves 1:1 while healthy.
// While impaired (R < C) it pays pro-rata at R/C.
func (v *Vault) Redeem(amount int64) (int64, error) {
	if amount <= 0 {
		return 0, fmt.Errorf("redeem: amount must be positive")
	}
	if amount > v.C {
		return 0, fmt.Errorf("redeem: amount %d exceeds claims %d", amount, v.C)
	}
	payout := amount
	if v.R < v.C {
		payout = amount * v.R / v.C // pro-rata in impairment
	}
	before := v.Surplus()
	v.C -= amount
	v.R -= payout
	if v.Surplus() < before {
		// Should be unreachable: revert rather than violate the invariant.
		v.C += amount
		v.R += payout
		return 0, fmt.Errorf("redeem: rejected, would reduce surplus")
	}
	v.record("redeem", amount)
	return payout, nil
}
