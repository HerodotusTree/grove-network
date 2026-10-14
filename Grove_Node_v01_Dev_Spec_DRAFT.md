# GROVE NODE v0.1 — Developer Technical Specification
### The downloadable vault node for the Vault Trial (distributed)

**Status:** DRAFT v0.1 — for implementation by a commissioned developer. Not audited. Trial software only.
**Author:** Herodotus of Halicarnassus (Damien J. Hall) — architecture
**Spec drafted:** 2026-10-14 (with Luna)
**Companion docs:** `Vault_Trial_Spec_DRAFT.md` (trial rules), `Vault_Contract_Logic_DRAFT.md` (superseded by this spec for the distributed design)

---

## 1. What this software is

A single downloadable program. Anyone who runs it joins the Grove trial network as a node: they allocate disk space ("raidspace"), keep a replica of the vault ledger, and can deposit trial assets into the vault, receive 1:1 receipts, transfer receipts, and redeem receipts for the underlying asset at any time. The network of running nodes *is* the trial venue — there is no central server beyond the bootstrap sequencer (see §6).

**v0.1 scope is deliberately narrow:** trial assets only (valueless, faucet-issued), one vault, one receipt type, simple sequencer-based ordering. Real-asset custody, threshold signatures, distributed key generation, and erasure coding are explicitly out of scope (see §11).

## 2. Recommended stack

- **Language:** Go (recommended) — single static binary, strong networking stdlib, trivial cross-compilation. Rust acceptable.
- **Local store:** embedded database (SQLite via modernc, or bbolt). No external DB dependency.
- **Crypto:** ed25519 for account keys and signatures; SHA-256 for the hash chain.
- **Distribution:** versioned GitHub releases; binaries for `linux/amd64`, `darwin/arm64`, `windows/amd64`; SHA-256 checksums published alongside.

## 3. The invariant (the floor)

The entire program exists to uphold one property, at every state, on every node:

```
total_locked >= total_receipts_outstanding
```

State transitions allowed — and these are the *only* state transitions that may move value:

| Operation | Precondition | Effect |
|---|---|---|
| `deposit(account, x)` | x > 0; account signed | `locked += x`; mint `x` receipts to account |
| `redeem(account, y)` | y > 0; account holds ≥ y receipts; account signed | burn `y` receipts; `locked -= y`; release `y` asset to account |
| `transfer(from, to, z)` | z > 0; from holds ≥ z receipts; from signed | move `z` receipts from → to (locked unchanged) |

No admin functions. No minting outside `deposit`. No release outside `redeem`. The program must not contain any code path that moves value otherwise — this is verifiable by audit and is the core trial claim.

## 4. Data model

**Accounts** (local to each node operator; keys never leave the machine):
- `account_id` = ed25519 public key (hex)
- `receipt_balance`

**Vault state** (global, replicated):
- `total_locked` (sum of all deposits minus all redemptions)
- `total_receipts` (sum of all outstanding receipt balances)

**Ledger entries** (append-only, hash-chained):
```
{
  index: uint64,            // sequence number from sequencer
  timestamp: unix_ms,
  type: "deposit" | "redeem" | "transfer" | "faucet",
  account: pubkey_hex,
  counterparty: pubkey_hex | null,   // for transfers
  amount: uint64,           // integer minor units; no floats anywhere
  prev_hash: hex,           // hash of previous entry
  hash: hex                 // SHA-256 over canonical encoding of the above
}
```

**Faucet entries** credit trial assets to an account for testing (rate-limited; trial assets are valueless and faucet abuse only wastes the attacker's time).

## 5. The ledger (tamper-evidence)

- Every value-moving operation is appended as a ledger entry *before* it is considered settled.
- Each entry commits to the previous entry's hash. Any alteration of history breaks every subsequent hash — visibly, on every node holding a replica.
- On startup and on sync, each node **verifies the full chain from genesis** (or from the latest sequencer checkpoint) and **recomputes** `total_locked` and `total_receipts` from the entries, refusing to start if recomputed state ≠ claimed state or if the invariant is violated.
- A node that detects a broken chain or invariant violation halts and reports; it does not paper over it. A halted node is a successful trial signal, not a failure.

## 6. Networking (v0.1: simple and honest)

- **Bootstrap:** the program ships with the Hearth's address as the default bootstrap peer (overridable in config). New nodes join by connecting to any known peer.
- **Gossip:** accepted ledger entries are flooded to connected peers. Nodes verify the hash chain and sequencer signature before relaying or applying.
- **Sequencing (v0.1):** the Hearth acts as the sequencer — it assigns `index` order and signs checkpoints. This is declared, temporary centralization (see the trial spec's phasing). Nodes accept entries only in sequencer order; conflicting orders are resolved in favor of the latest sequencer-signed checkpoint.
- **Churn:** participation is voluntary — nodes may go offline and rejoin at will, no uptime obligation. A rejoining node syncs from peers, verifies the chain from the latest sequencer checkpoint (fast catch-up), recomputes state, and resumes. Absence is never penalized; the ledger, not any node's memory, is the source of truth. Note the honest limit: in v0.1, *history* survives any churn, but *progress* needs the sequencer awake — if the Hearth goes offline, the ledger halts safely (no corruption, no divergence) until it returns. Sequencer failover/rotation is a later version.
- **Later versions:** sequencer rotation / distributed ordering. Not v0.1.

## 7. Raidspace (disk allocation)

- On first run, the operator configures a data directory and a size cap (recommended default: 10 GB; the ledger itself is tiny — the allocation is forward capacity for the network's data layer plus a commitment signal).
- The node stores: the embedded DB, the full ledger replica, and (later versions) erasure-coded shards for the network pool.
- The program must refuse to run if its data directory is full or unwritable, and must report disk usage via the status endpoint.

## 8. Interfaces

**CLI (local):**
- `grove init` — generate account keypair (local only), configure raidspace
- `grove faucet` — claim trial assets (rate-limited)
- `grove deposit <amount>` / `grove redeem <amount>` / `grove transfer <to> <amount>`
- `grove balance` — receipt balance + account asset balance
- `grove verify` — verify local chain from genesis, report invariant status

**HTTP API (local + public read endpoints):**
- `GET /status` — locked vs. receipts outstanding (the floor, live), node height, peer count
- `GET /metrics` — operation counts, success/failure breakdown per §9
- `GET /ledger?from=&to=` — paginated public ledger (the transparency that makes the trial credible)

## 9. The efficiency metric (the 99.99%)

The program must measure what the graduation threshold requires. Every node exposes:

- `ops_total`, `ops_success`, `ops_failed` — per operation type
- `invariant_violations` — must remain 0; any nonzero value is a critical event
- An operation counts as **successful** iff it settles at exactly 1:1 with the invariant holding afterward.

**Efficiency (per board)** = `ops_success / ops_total`, computed over a rolling window, reported via `/metrics`. Crucially, the denominator counts only operations the board *attempted while online and participating* — offline time is absence, not failure. A board that is switched off is not failing; it is simply not playing. The trial's graduation criterion (99.99% per board over qualifying volume) is evaluated from these metrics — the software is the instrument of its own judgment.

## 10. Security requirements

- Account keys are generated and held locally; the node software never transmits private keys.
- All state-changing operations require a valid ed25519 signature + per-account nonce (replay protection).
- Faucet rate-limiting per account and per IP.
- No remote admin interface. No auto-update that can push code silently (updates are manual binary replacement, checksum-verified).
- Integer arithmetic only for all value handling. No floating point anywhere near money.

## 11. Explicitly out of scope for v0.1

Real-asset custody. Threshold signatures / DKG. Distributed sequencing. Erasure coding. Privacy features. Token economics or incentives beyond the trial prize. Governance. Anything not required to run the trial.

### Future extension: multi-asset adapters (post-v0.1)

The vault core (§3) is asset-agnostic — the 1:1 accounting does not care what the asset is. Real multi-currency support arrives via **per-asset adapters**, one per external chain, each providing:

- a **deposit observer** (watches the external chain for deposits to the vault's addresses; light client preferred, trusted relayer acceptable in early phases), and
- a **custodian** (threshold-signature / MPC key shares across nodes — the hard part; no single node ever holds a full key).

Each supported asset gets its own vault pool and receipt type (1 BTC pool → BTC receipts, etc.), all obeying the same invariant. v0.1 ships exactly one adapter: the **mock adapter** (faucet-issued trial asset, valueless). The developer should keep the adapter behind a clean interface so real adapters plug in later without touching the vault core. Multi-asset mainnet support also triggers money-transmitter and custody regulatory review before it opens.

## 12. Acceptance criteria (definition of done)

1. Two nodes sync the same ledger from genesis; both recompute identical vault state.
2. Fuzzed random sequences of deposit/redeem/transfer (≥100k ops) never violate the invariant on any node.
3. A node fed a tampered ledger history halts instead of accepting it.
4. `grove verify` passes on a fresh sync; `/status` shows locked vs. receipts live.
5. Binaries build reproducibly for all three target platforms; checksums published.
6. An independent reviewer (even informal) reads the value-moving code paths and confirms §3 exhaustiveness.

## 13. Milestones

1. This spec → developer implements v0.1
2. Two-node internal test on the Hearth + one more machine
3. Independent code review (§12.6)
4. Public release: GitHub, binaries, checksums, trial rules announcement
5. Round 1 of the Vault Trial goes live; the Hearth sequences

## 14. Minimum machine requirements (node operator)

The v0.1 workload is tiny — hashing, signature verification, an embedded database, and gossiping small ledger entries. Any working 64-bit computer from the last ~15 years qualifies.

**Hardware:**
- CPU: any 64-bit dual-core or better (2009+). No GPU needed.
- RAM: 2 GB minimum, 4 GB comfortable.
- Disk: 10 GB allocated raidspace (default), 20 GB total free recommended. The ledger itself is kilobytes-to-megabytes at trial scale; the allocation is forward capacity plus commitment.
- Power: intended to stay on (or mostly on). Typical idle draw for this class of machine: 10–30W.

**Software:**
- OS (64-bit): Ubuntu 24.04 LTS (recommended), macOS (ARM64), or Windows 10/11.
- The single `grove` binary + a config file. No Docker, no database server, no other dependencies.

**Network:**
- Stable internet; wired Ethernet preferred.
- Must be reachable by peers: one inbound TCP port open (configurable; UPnP attempted automatically, manual port-forward documented as fallback).
- Bandwidth needs are negligible at trial scale (ledger entries are hundreds of bytes).

**The honest bottom line:** the barrier is not hardware, it is willingness. The refurbished machines in the Grove product line exceed these requirements several times over — which is exactly why the e-waste arbitrage works: scrap-value hardware runs this effortlessly. Future phases (erasure-coded shards, threshold signing) raise disk and CPU needs modestly, still within the same hardware class.

---

*Spec drafted 2026-10-14. This is a build-to document for a professional developer — not a finished program. The architecture is the author's; the implementation will be the builder's; the trial belongs to whoever tries to break it.*
