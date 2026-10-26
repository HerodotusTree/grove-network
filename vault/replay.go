// Staged replay for sync adoption (issue #1).
//
// Replay validates a candidate chain as a vault history WITHOUT
// mutating anything and WITHOUT touching disk. It is the check that
// runs before Adopt persists anything: every entry's payload must be a
// well-formed vault op, every op must be legal in the state the
// preceding entries produced, and every entry's claimed R/C/Floor must
// equal the state its own op actually produces. A chain that passes is
// internally consistent — its economics follow from its ops. A chain
// that fails is refused wholesale; nothing is partially applied.
//
// What replay can NOT do, and this matters: on its own it cannot
// tell a valid history that really happened from a valid history an
// attacker fabricated, because it checks economics, not authorship.
// That provenance check now lives in the node's adoption gate
// (verifyAdoptionAuth in node/main.go): every NEW entry of an
// adopted chain must be domain-bound — genesis hash, account,
// sequential nonce, signature — and adoption advances the nonce
// table, so a self-consistent fabrication with no signature, or
// with a nonce already spent, is refused there. Replay itself
// deliberately stays signature-agnostic (this package has no identity
// dependency): a self-consistent fabrication still replays cleanly
// here, and must never be quoted as proof of provenance on its own.
// The remaining bootstrap gap is narrower but real: a fresh node
// adopting a first chain gets authenticated entries from a peer
// that is itself unauthenticated (/peers and /sync carry no endpoint
// auth), and a signature proves authorization, not off-chain
// funding. Peer authorization is still owed.
package vault

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/HerodotusTree/grove-network/ledger"
)

// Replay simulates entries from the empty vault and returns the final
// R, C, and Floor. It mirrors Deposit, Tip, and Redeem exactly,
// including their overflow guards and the surplus-never-decreases
// rule, but applies them to local variables instead of the vault.
func Replay(entries []ledger.Entry) (r, c, floor int64, err error) {
	fail := func(i int, format string, args ...any) (int64, int64, int64, error) {
		return 0, 0, 0, fmt.Errorf("replay: entry %d: %s", i, fmt.Sprintf(format, args...))
	}
	for i, e := range entries {
		if e.Index != uint64(i) {
			return fail(i, "index %d, want %d (entries must be contiguous from genesis)", e.Index, i)
		}
		var op Op
		if err := json.Unmarshal([]byte(e.Payload), &op); err != nil {
			return fail(i, "payload is not a vault op: %v", err)
		}
		if op.Amount <= 0 {
			return fail(i, "amount %d must be positive", op.Amount)
		}
		before := r - c
		switch op.Kind {
		case "deposit":
			if r < c {
				return fail(i, "deposit while impaired (R=%d < C=%d)", r, c)
			}
			if r > math.MaxInt64-op.Amount || c > math.MaxInt64-op.Amount {
				return fail(i, "deposit %d would overflow reserves/claims", op.Amount)
			}
			r += op.Amount
			c += op.Amount
		case "tip":
			if r > math.MaxInt64-op.Amount || floor > math.MaxInt64-op.Amount {
				return fail(i, "tip %d would overflow reserves/floor", op.Amount)
			}
			r += op.Amount
			floor += op.Amount
		case "redeem":
			if op.Amount > c {
				return fail(i, "redeem %d exceeds claims %d", op.Amount, c)
			}
			payout := op.Amount
			if r < c {
				payout = mulDiv(op.Amount, r, c)
			}
			r -= payout
			c -= op.Amount
			if r-c < before {
				return fail(i, "redeem would reduce surplus")
			}
		default:
			return fail(i, "unknown kind %q", op.Kind)
		}
		if op.R != r || op.C != c || op.Floor != floor {
			return fail(i, "claimed state R=%d C=%d floor=%d does not follow from its ops (replayed R=%d C=%d floor=%d)",
				op.R, op.C, op.Floor, r, c, floor)
		}
	}
	return r, c, floor, nil
}
