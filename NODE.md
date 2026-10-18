# Running a Grove Node

## Build

```bash
cd node
go mod tidy
go build -o grove-node .
```

## Identity: signed requests

`/deposit`, `/redeem` and `/tip` no longer take a bare amount. Every
mutating request must be signed by the account acting — no signature,
no signature check, no op. This closes the "anyone can move anyone's
money" gap: the node holds no passwords and never sees a private key.

**Step 1 — generate your key.** Your account IS your public key.

```bash
cd node
go run . -keygen
```

It prints (private key goes to you once — store it somewhere safe,
it cannot be recovered):

```json
{"private_key":"<64 hex chars — the secret>","account":"<64 hex chars — your account id>"}
```

**Step 2 — sign each request.** The signature covers the canonical
payload, so it can never be moved to a different operation, amount,
account, or replayed:

```
grove-v1|<op>|<account>|<amount>|<nonce>
```

- `op` is exactly `deposit`, `redeem` or `tip`
- `account` is your public-key hex
- `nonce` starts at 1 for a new account and must increase by exactly 1
  every accepted op (a replayed or skipped nonce is rejected with 401)
- Sign the payload with Ed25519 and send the signature as hex

Request shape for `/deposit`, `/redeem`, `/tip`:

```json
{"account":"<hex pubkey>","amount":100,"nonce":1,"signature":"<hex sig>"}
```

**Worked example** (throwaway keys — do NOT use these, generate your own):

```bash
# canonical payload that was signed:
# grove-v1|deposit|37d441aa512e445627c728c65e8363bb560daa3306dcef17ba177d52f551e433|100|1

curl -X POST localhost:8080/deposit -d '{
  "account": "37d441aa512e445627c728c65e8363bb560daa3306dcef17ba177d52f551e433",
  "amount": 100,
  "nonce": 1,
  "signature": "7ad53ac89168069ec9a6f7e2151382d4275be8efca812636abcc58925aa0ec19b0869efdce0f32289a2aca9c5ba371377c8ef7c6098527a04ccf9eee67ad340e"
}'
```

Any tool that speaks Ed25519 can sign — the format above is all you
need. Nonces are tracked per account in `<data>/nonces.json`
(atomic writes); a corrupt nonce file refuses startup, same as a
corrupt chain.

Client rule that will save you hours: **number acceptances, not
attempts.** Advance your nonce only when the node answers 200. If an
op is rejected (say a redeem bigger than your claims), retry with the
*same* nonce — skipping ahead desyncs the account and every later op
comes back 401 until you go back to the right number.

## Endpoints

| Method | Path      | What it does                                                        |
|--------|-----------|---------------------------------------------------------------------|
| POST   | /deposit  | signed `{"account":..,"amount":100,"nonce":1,"signature":..}` — deposit, mints equal claim |
| POST   | /redeem   | signed — redeem, 1:1 (pro-rata if impaired)                         |
| POST   | /tip      | signed — tip the floor, mints no claims                             |
| GET    | /state    | reserves, claims, surplus                                           |
| GET    | /ledger   | full hash-chained operation log (verified first)                    |
| GET    | /metrics  | op telemetry: attempted/succeeded/rejected, invariant violations, uptime |
| GET    | /peers    | list known peers                                                    |
| POST   | /peers    | `{"url":"http://host:port"}` — register a peer (bare base URL; validated) |
| POST   | /sync     | adopt the longest valid peer chain                                  |

## Try it

```bash
# 1. start the node
./grove-node -port 8080 -data grove-data &

# 2. make yourself an account
go run . -keygen

# 3. sign a deposit (see "Identity" above for the payload format),
#    then check state
curl localhost:8080/state
curl localhost:8080/metrics
```

## Run two nodes and sync them

```bash
./grove-node -port 8080 -data grove-data-a &
./grove-node -port 8081 -data grove-data-b &
curl -X POST localhost:8080/peers -d '{"url":"http://localhost:8081"}'
curl -X POST localhost:8080/sync
```

## The rules (enforced in code, not promises)

- Absolute surplus S = R − C never steps backward.
- Deposits move reserves and claims equally; S unchanged.
- Intake closes while impaired (R < C); redemptions go pro-rata at R/C.
- Every accepted operation is appended to the hash-chained ledger.
- Tips feed the non-redeemable floor; claims are never minted from tips.
- Integer minor units only. No floating point.
- Mutating ops require a valid Ed25519 signature and the next expected
  nonce for the account; failures answer 401 and count as rejected.

## Known limits (v0.3)

- Nonce tables are per-node. `/sync` adopts a hash-valid chain without
  re-verifying its signatures, and it does not import the other node's
  nonce table. After a sync, an account that acted on another node will
  get 401 `bad nonce` from the adopting node until the client realigns
  to that node's expected nonce. This fails closed: a stale table can
  only reject, never replay.
- Transport is plain HTTP. Signatures prove who signed the request;
  they do not encrypt anything on the wire.
- The private key IS the account. The node never stores it, so there is
  no recovery, no reset, no admin override — lose it and the account is
  gone with it.

---

## Storage: Journal Recovery and Crash Consistency

The ledger enforces crash consistency across process termination, power loss,
and mid-write disruptions using an append-only journal (journal.jsonl) paired
with snapshot state (chain.json).

**Proven behavior**

- *Torn-tail truncation:* a partial, unterminated journal write (missing a
  final newline) caused by a crash mid-append is truncated on reload. All
  valid preceding records are recovered, the corrupted tail is discarded,
  and the chain verifies cleanly.
- *Snapshot fallback:* if a journal contains only torn bytes, the node
  safely falls back to the existing snapshot state, zeroes the torn journal,
  and recovers without failing.
- *Interrupted compaction:* journal entries already present in the snapshot
  are skipped only when every field matches. Conflicting overlap is rejected.
- *Corruption rejection:* non-tail corruption (e.g. malformed JSON in a
  middle record) causes the node to refuse startup with a corruption error
  rather than silently omitting history.
- *Record boundaries:* malformed newline-terminated records are corruption,
  including the final record. An unterminated tail is discarded even when
  its JSON is complete, so the next append starts at a valid record boundary.
- *Clean state initialization:* a missing chain path is treated as a clean
  first run (os.IsNotExist) and initializes a fresh ledger.
- *Concurrency safety:* core storage operations pass Go race detection
  cleanly across all modules (-race).

**How to reproduce**

Run the automated persistence test suite:

```
cd ledger
go test -v -race -run 'Test(SaveLoadRoundtrip|TornTail|CorruptMiddle|LoadMissing|Recovery)'
```

**Stated limits**

- Assumes torn writes do not contain embedded newlines.
- Filesystem fsync covers the active journal file but does not guarantee
  directory rename durability across every filesystem type in the event of
  an immediate system crash.
- Verified for a single node instance only.

---

## Network Sync: History Adoption and Threat Profile

The /peers and /sync endpoints allow nodes to discover peers and synchronize
ledger state across the network.

**Proven behavior & demonstrated vulnerability (issue #1)**

- *Unauthenticated chain adoption:* as demonstrated in
  node/attack_sync_test.go, unauthenticated POST /peers combined with POST
  /sync allows an arbitrary network client to force a node to adopt an
  alternate, longer, hash-valid chain.
- *State rewriting:* adopting a longer fabricated chain completely replaces
  local ledger history and mutates aggregate vault state (reserves, claims,
  floor) without cryptographic proof of initial deposit collateral.
- *Demonstration vector:* an honest node operating at R=100, C=100 was
  forced via /sync to adopt a 2-entry fabricated peer chain containing an
  unbacked 1,000,000 unit tip, mutating local state to R=1,000,500, C=500,
  floor=1,000,000.

**How to reproduce**

Run the sync attack regression test:

```
go test -v ./node -run TestSyncAdoptsFabricatedPeerChain
```

(Note: this test is designed to demonstrate the unhardened sync behavior
and will fail until sync hardening acceptance rules are implemented.)

**Stated limits**

- *Hole demonstrated, not the fix:* this test demonstrates the
  vulnerability; it does not implement the hardened acceptance policy.
- *Test assets only:* validated entirely within local test harnesses using
  non-monetary test accounting.
- *Open static surface:* static analysis identified additional unverified
  vectors in the sync path that remain open for formal testing:
  - lack of URL validation on registered peers (potential SSRF) —
    HARDENED: POST /peers now enforces the documented
    {"url":"http://host:port"} contract (bare http(s) base URL only;
    paths, queries, fragments, userinfo, and non-http(s) schemes are
    rejected with 400), fetches never follow redirects, and peer URLs
    are revalidated at fetch time. Residual: a bare
    http://internal-host:port is still fetchable — full peer
    authorization policy is AUTH-02's call.
  - holding the global node mutex across outbound peer HTTP calls —
    HARDENED: /sync snapshots the peer list under the op lock and does
    all network fetches with the lock released; adoption re-acquires
    the lock and proceeds only if nothing was appended locally during
    the fetch (a longer peer chain never silently discards operations
    that landed mid-sync).
  - unconditional local history discard on valid peer forks;
  - unbounded request body sizes on sync payloads;
  - telemetry metrics conflating adopted peer history with local
    operational throughput.
