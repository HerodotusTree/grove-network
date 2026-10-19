# The Grove

![The Grove emblem](grove-emblem.webp)

*One box. One rule. Break it if you can.*

The Grove explores a distributed vault design: deposits in, 1:1 receipts out, redemptions recorded in a replicated ledger. This repository currently contains an experimental Go prototype using valueless trial accounting. The intended network guarantees are being developed and tested.

This repository contains the architecture, trial specifications, and runnable node code. See [NODE.md](NODE.md) for the implemented interfaces and [CONTRIBUTING.md](CONTRIBUTING.md) for setup, test expectations, and contribution guidance.

## The idea in one paragraph

The intended design introduces no new coin: people would bring existing digital assets, the network would lock them 1:1, and receipts would circulate and be redeemable for the underlying value. The proposed 1:1 guarantee concerns redemption rather than a market-price peg. Real-asset custody and these network guarantees remain outside the current trial prototype.

## Documents

| File | What it is |
|---|---|
| `Vault_Trial_Spec_DRAFT.md` | The public challenge: rules, prizes, rounds, and the 99.99% graduation metric |
| `Grove_Node_v01_Dev_Spec_DRAFT.md` | Technical spec for the node software (what a developer builds) |
| `Adoption_Projection_DRAFT.md` | Growth model: what "ready" looks like in numbers (+ `adoption_model.py`, runnable) |
| `Node-1-Build-Plan.md` | The Hearth: build plan for the first physical node |
| `The_Master_Architectural_Specification_1_j30z.md` | The full system architecture |
| `The_Universal_Conservation_Ledger_0_m8zj.pdf` | The formal framework compendium |

## Status

The prototype implements signed HTTP operations, aggregate vault arithmetic,
hash-chained storage with journal recovery, and peer sync. It is under active
review and includes regression tests for unresolved security properties.

Current work includes per-account receipt ownership, atomic operation and nonce
persistence, replay protection across sync, and validation of authoritative peer
history. See the [issue tracker](https://github.com/HerodotusTree/grove-network/issues)
and the [test baseline](CONTRIBUTING.md#current-security-test-baseline) for the
specific behaviors still being repaired. Trial accounting does not establish
real-asset custody or production security.

## The graduation rule

No master key is generated — for anyone, including the founder — until the network demonstrates **99.99% efficiency across all boards**: every board settling its operations at exactly 1:1, measured over 10,000+ operations each. The system is sealed by measurement, not by anyone's decision.

## License

MIT. The trial is white-box by design — the code must be auditable for the proof to mean anything.

## Disclaimer

Nothing here is operational, offered, or solicited. The network does not exist yet. Trial assets are and will remain valueless test instruments, never redeemable for dollars. This is a plan being built in public.
