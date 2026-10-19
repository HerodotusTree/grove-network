# Proof Log — Piece by Piece

Damien Hall's standing instruction (2026-10-17): keep track of every piece as it gets
proven, in the order it's proven, while the build continues. This is the
running record — not claims, not plans. Only what was demonstrated.

Format per entry: **Proven** (date, what was shown, how it was shown),
**Method** (the mechanism), **Limits** (what it does not prove).

---

## 1. Hash-chained ledger detects tampering (2026-10-16)
- **Proven:** Each entry's hash commits to the previous entry's hash; altering
  any entry breaks every link after it. Verification walks the full chain.
- **Method:** SHA-256 chain in `ledger/`, `TestLedger*` passing.
- **Limits:** Detects alteration of recorded history; does not prevent a
  fabricated-but-valid history from being written fresh.

## 2. 1:1 minting — deposits create exactly one unit of claim (2026-10-16)
- **Proven:** Deposit of N increases claims C by exactly N and reserves R by
  exactly N. No pre-mine, no discretionary issuance; supply equals fed floor.
- **Method:** `vault.Deposit`, live node run (deposit 100 → claims 100).
- **Limits:** Test assets only. No real-dollar issuance.

## 3. Integer overflow cannot wrap reserves or claims (2026-10-16)
- **Proven:** Deposits/tips that would overflow int64 are rejected before they
  can wrap; redeem uses overflow-safe 128-bit pro-rata math.
- **Method:** Commit `94fa275`, `TestOverflowRejected` passing. Found by first
  attacker (Johnohhh1, issue #2), fixed and closed same day.
- **Limits:** One boundary class covered; full boundary-test sweep still open.

## 4. Tips ratchet the floor without minting claims (2026-10-16)
- **Proven:** Deposit 100 + tip 25 → claims 100, floor 25, reserves 125,
  surplus 25. Tips feed reserves and the non-redeemable floor; no claims minted.
- **Method:** `vault.Tip`, `TestTipRatchetsFloor` passing, live node run.
- **Limits:** Volatile-to-cash conversion before minting still a design step.

## 5. The ledger survives restart with zero loss (2026-10-16)
- **Proven:** 10,000-op run (946 ops/sec), node restarted, all 9,954 accepted
  entries present afterward. Append-only journal + snapshot compaction;
  torn journal tail treated as crash mid-write, other corruption refuses startup.
- **Method:** `ledger/persist.go`, journal replay on startup, restart check.
- **Limits:** Single node. Disk failure between chain write and nonce write can
  leave a stale nonce table (known, documented).

## 6. 10,000 operations with zero invariant violations (2026-10-16)
- **Proven:** 9,954 accepted / 46 rejected, 0 invariant violations, chain valid
  after run. Rejected ops were audited as legitimate 422s, not silent failures.
- **Method:** `loadgen`, `/metrics` telemetry.
- **Limits:** Local benchmark only — not proof of production reliability,
  security, or the full 99.99% graduation bar. Rejected-op audit was manual.

## 7. Signed requests — no op without identity (2026-10-17)
- **Proven:** Every deposit/redeem/tip must carry a valid Ed25519 signature
  from the account's keypair; the node rejects unsigned, mis-signed, or
  replayed requests (per-account nonces, reject on reuse). Old unsigned
  request shape now returns 401.
- **Method:** `identity/` module, `node/main_test.go` (9 tests), live E2E:
  signed deposit accepted → exact replay rejected → unsigned rejected.
- **Limits:** Authentication only — accounting stays aggregate, no per-account
  balances yet. Applied to the workshop repo and pushed to the public repo
  2026-10-17 (commit `d761d15`); all five modules build/vet/test green on
  the workshop machine.

## 8. Journal torn-tail and corruption recovery tested (2026-10-17)
- **Proven:** A torn final journal line is truncated and recovered on load;
  a torn-only journal falls back to the snapshot; mid-journal corruption
  refuses startup instead of silently dropping history; a missing chain is
  a clean fresh start. Race detector clean across all five modules.
- **Method:** `ledger/persist_test.go` (5 new tests),
  `go test -race ./...` green in identity, ledger, loadgen, node, vault.
- **Limits:** The torn-write-can't-contain-newline invariant is assumed, not
  itself tested. fsync covers the journal file, not the directory rename —
  a crash exactly at rename could lose it on some filesystems. Single node.

## 9. Sync adopts fabricated-but-valid peer history — demonstrated live (2026-10-17)
- **Proven:** Unauthenticated POST /peers + POST /sync lets any network
  client force adoption of a longer hash-valid fabricated chain, rewriting
  vault state. Demo: honest node at R=100/C=100 adopted a 2-entry chain
  with a fabricated 1,000,000 tip → R=1000500/C=500/floor=1000000.
  Issue #1 is now demonstrated, not just described.
- **Method:** `node/attack_sync_test.go` — failing-as-intended regression
  test; all pre-existing tests still pass. Found by the cycle's attacker
  basin on the first run.
- **Limits:** Test assets only. Demonstrates the hole, not the fix. Also
  surfaced (static review, undemonstrated): unvalidated peer URLs (SSRF),
  sync holding the global mutex across outbound HTTP, Adopt discarding
  local history on honest forks, no request body-size caps, efficiency
  metrics mixing adopted history with local ops.

## 10. Identity layer's cross-node and post-sync guarantees broken — demonstrated (2026-10-17)
- **Proven:** Three holes, each with a failing-as-intended regression test,
  independently reproduced by Luna: (A1) a deposit signed for node A is
  accepted byte-for-byte by node B — the signed payload names no
  node/network and nonce tables are per-node; (A2) after B syncs from A,
  replaying the same nonce-1 deposit is accepted — R=100→200, the op
  applied twice — contradicting NODE.md's "a stale table can only reject,
  never replay" (that sentence must not ship as written); (A3) a node at
  floor 25 adopting a longer hash-valid chain drops to floor 0 while
  /metrics reports zero invariant violations — the ratchet stepping
  backward with telemetry silent.
- **Method:** Claude attacker basin (independent pass),
  `node/attack_crossnode_test.go` (3 tests), reproduced on Luna's side;
  all pre-existing tests still pass.
- **Limits:** Demonstrates holes, not fixes. In-process test nodes, not
  separate machines; attacker ran Go 1.24 with version lines lowered in a
  test-only copy. Root cause (static): ledger entries record kind/amount/
  R/C/floor only — no account, nonce, or signature — so identity is
  enforced at the HTTP door and forgotten after; no node can re-verify
  history or rebuild nonce tables from it.

---

*Next open pieces: sync hardening fix (#1 — three formal design questions
now with ChatGPT: chain id in payload, account/nonce/signature in ledger
entries, replay-and-refuse adoption rule), durable metrics, Hearth hardware.*

## 11. Sync adoption silently discards client-acknowledged local ops — demonstrated (2026-10-19)
- **Proven:** /sync adopts the longest hash-valid chain unconditionally; the re-lock guard only protects ops landing DURING the fetch, not ops acknowledged (HTTP 200, journaled) BEFORE it. Demonstrated with two HONEST nodes: victim B's 200-OK deposit vanished after syncing peer A's longer divergent chain — entry hash gone from ledger, R 100->1400, sync reports adopted:true, /metrics records nothing about the loss. No malice required — even honest divergent histories cause silent loss of acknowledged writes.
- **Method:** grove-cycle attacker basin, `node/attack_sync_discard_test.go` — failing-as-intended regression test; all previously-passing tests still pass (incl. the #6/#7 fixed-behavior tests: SyncFreezesSequencer, SyncSSRF, PeersStalledBody, PeerForceQueryCanonicalized, PeerURLValidation). Scratch clone only; never committed or pushed.
- **Limits:** Demonstrates the hole, not the fix. Fix direction: prefix-only adoption / staged replay with domain-bound entries (v0.4). Limit note found while writing the test: entry hashes commit to 1-second timestamps, so identical ops on two nodes within the same second hash identically. Also surfaced (static review, undemonstrated): unbounded request/response body decoding (DoS — 10s timeout bounds time, not bytes), unauthenticated + unbounded + non-persistent peer list with no removal endpoint, peer-URL validation syntactic only (bare 127.0.0.1 / [::1] / 169.254.169.254 still fetchable — residual SSRF surface on the #7 fix, DNS rebinding unaddressed).
