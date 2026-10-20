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
	"math"
	"math/bits"

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
	// Domain is the network's genesis hash, stamped into every entry
	// this vault records. The node sets it from its genesis config.
	Domain string
}

// New returns a vault wired to the given ledger.
func New(log *ledger.Ledger) *Vault { return &Vault{Log: log} }

// Surplus returns S = R - C.
func (v *Vault) Surplus() int64 { return v.R - v.C }

func (v *Vault) record(kind string, amount int64, account string, nonce uint64, sig string) {
	rec, _ := json.Marshal(Op{Kind: kind, Amount: amount, R: v.R, C: v.C, Floor: v.Floor})
	v.Log.AppendSigned(v.Domain, account, nonce, string(rec), sig)
}

// Deposit adds reserves and mints an equal claim. S is unchanged.
// Intake is closed while impaired (R < C).
// Deposit is the legacy amount-only form, kept until the node migrates
// to DepositFor; it records an empty authorization envelope.
func (v *Vault) Deposit(amount int64) error {
	return v.DepositFor("", 0, "", amount)
}

// DepositFor is Deposit carrying its authorization envelope (account,
// nonce, signature), recorded in the ledger entry itself.
func (v *Vault) DepositFor(account string, nonce uint64, sig string, amount int64) error {
	if amount <= 0 {
		return fmt.Errorf("deposit: amount must be positive")
	}
	if v.R < v.C {
		return fmt.Errorf("deposit: intake closed while impaired (R=%d < C=%d)", v.R, v.C)
	}
	if v.R > math.MaxInt64-amount || v.C > math.MaxInt64-amount {
		return fmt.Errorf("deposit: amount %d would overflow reserves/claims", amount)
	}
	v.R += amount
	v.C += amount
	v.record("deposit", amount, account, nonce, sig)
	return nil
}

// Tip is a gift to the floor: it adds to reserves and to the
// non-redeemable floor ledger, minting no claims. S grows by amount.
// Tips are how the floor ratchets up; without them it holds flat,
// never down.
// Tip is the legacy amount-only form; see Deposit.
func (v *Vault) Tip(amount int64) error {
	return v.TipFor("", 0, "", amount)
}

// TipFor is Tip carrying its authorization envelope.
func (v *Vault) TipFor(account string, nonce uint64, sig string, amount int64) error {
	if amount <= 0 {
		return fmt.Errorf("tip: amount must be positive")
	}
	if v.R > math.MaxInt64-amount || v.Floor > math.MaxInt64-amount {
		return fmt.Errorf("tip: amount %d would overflow reserves/floor", amount)
	}
	v.R += amount
	v.Floor += amount
	v.record("tip", amount, account, nonce, sig)
	return nil
}

// mulDiv returns floor(a*b/c) without overflow. All args must be
// non-negative, c must be positive, and the true quotient must fit
// in an int64 (guaranteed by the caller: amount <= C, so the
// pro-rata payout never exceeds R).
func mulDiv(a, b, c int64) int64 {
	hi, lo := bits.Mul64(uint64(a), uint64(b))
	q, _ := bits.Div64(hi, lo, uint64(c))
	return int64(q)
}

// Redeem burns claims and pays out reserves 1:1 while healthy.
// While impaired (R < C) it pays pro-rata at R/C.
// Redeem is the legacy amount-only form; see Deposit.
func (v *Vault) Redeem(amount int64) (int64, error) {
	return v.RedeemFor("", 0, "", amount)
}

// RedeemFor is Redeem carrying its authorization envelope.
func (v *Vault) RedeemFor(account string, nonce uint64, sig string, amount int64) (int64, error) {
	if amount <= 0 {
		return 0, fmt.Errorf("redeem: amount must be positive")
	}
	if amount > v.C {
		return 0, fmt.Errorf("redeem: amount %d exceeds claims %d", amount, v.C)
	}
	payout := amount
	if v.R < v.C {
		payout = mulDiv(amount, v.R, v.C) // pro-rata in impairment, overflow-safe
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
	v.record("redeem", amount, account, nonce, sig)
	return payout, nil
}
