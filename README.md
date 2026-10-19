# The Grove

![The Grove emblem](grove-emblem.webp)

*One box. One rule. Break it if you can.*

The Grove is a vault network nobody owns. This repository holds running node software: download it, run it on your own machine, hold your own keys, verify the ledger yourself. No account, no permission, nobody can shut your node off.

It runs on **valueless test assets only**. There is no coin, no money, nothing to invest in. The challenge is the point: the box is open, come break it.

See [NODE.md](NODE.md) for the implemented interfaces and [CONTRIBUTING.md](CONTRIBUTING.md) for setup, test expectations, and contribution guidance.

## What runs today (v0.3)

- **Signed operations** — every deposit, redeem, and tip is Ed25519-signed; the public key *is* the account. No signature, no state change.
- **Hash-chained ledger** — every entry links to the last; tampering is detectable. Journal recovery survives crashes.
- **The vault** — deposits in, 1:1 receipts out, redemptions honored, tips ratchet the floor upward without minting claims.
- **Peer sync** — nodes share history over HTTP.

Measured locally: 10,000 operations, 946 ops/sec, 9,954 accepted, 46 rejected, 0 invariant violations, all entries present after restart. A local benchmark, not a production claim.

## What the next version is (v0.4 spec — frozen)

The v0.4 builder packet specifies per-account claim ownership, replay-rejecting authorization, atomic durable transitions, and sync-adoption rules. Before a line of code was written, it went through **13 revisions, 7 independent attack rounds, and 3 formal reviews** — the last attacker declared no further findings, and the formalizer's verdict is READY-WITH-NOTES (specification level only; the tests still have to be executed).

- [`docs/Phase1_Builder_Packet.md`](docs/Phase1_Builder_Packet.md) — the frozen spec, with every revision and ruling on the record
- [`docs/Proof_Log.md`](docs/Proof_Log.md) — demonstrated results only: what was proven, how, and the limits

## The idea in one paragraph

No new coin. People bring the digital assets they already hold; the network locks them 1:1 and issues receipts that circulate. Anyone can always redeem — "break the box" — and take the underlying value back. The 1:1 is a redemption guarantee, not a price peg. Volatility rages outside; the ratio holds inside.

## The challenge

Open issues are listed in the [issue tracker](https://github.com/HerodotusTree/grove-network/issues) — fabricated-history adoption, cross-node replay, nonce durability. Each has a failing regression test waiting. If you can break the box, that's the proof working as intended.

## Documents

| File | What it is |
|---|---|
| `docs/Phase1_Builder_Packet.md` | v0.4 spec, frozen at Revision 13 (READY-WITH-NOTES) — the next build |
| `docs/Proof_Log.md` | Demonstrated results: proven / method / limits |
| `Vault_Trial_Spec_DRAFT.md` | The public challenge: rules, rounds, and the 99.99% graduation metric |
| `Grove_Node_v01_Dev_Spec_DRAFT.md` | Earlier technical spec for the node software (superseded by the v0.4 packet) |
| `Adoption_Projection_DRAFT.md` | Growth model: what "ready" looks like in numbers (+ `adoption_model.py`, runnable) |
| `Node-1-Build-Plan.md` | The Hearth: build plan for the first physical node |
| `The_Master_Architectural_Specification_1_j30z.md` | The full system architecture |
| `The_Universal_Conservation_Ledger_0_m8zj.pdf` | The formal framework compendium |

## Status

v0.3 node software is public and running (test assets). The v0.4 spec is frozen and verified at the specification level — implementation is next. The physical Hearth (Node #1) is unbuilt. No live network exists yet; no coin has been issued; no real dollars are involved anywhere.

Trial accounting does not establish real-asset custody or production security. See the [test baseline](CONTRIBUTING.md#current-security-test-baseline) for the specific behaviors still being repaired.

## The graduation rule

No master key is generated — for anyone, including the founder — until the network demonstrates **99.99% efficiency**: operations settling at exactly 1:1 with the invariant holding, measured over 10,000+ operations. The system is sealed by measurement, not by anyone's decision.

## License

MIT. The trial is white-box by design — the code must be auditable for the proof to mean anything.

## Disclaimer

No live network exists. Nothing here is offered or solicited. Trial assets are and will remain valueless test instruments, never redeemable for dollars. This is a build in public, not an investment.
