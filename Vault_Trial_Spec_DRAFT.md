# THE VAULT TRIAL — Draft Spec v0.3 (distributed)
### *One box. One rule. Break it if you can.*

**Status:** DRAFT v0.3 — re-cut for the distributed design (downloadable node network). Trial numbers locked as working defaults. Nothing here is deployed, published, or final.
**Author:** Herodotus of Halicarnassus (Damien J. Hall)
**Date:** 2026-10-14
**Companion docs:** `Grove_Node_v01_Dev_Spec_DRAFT.md` (the node software), `Adoption_Projection_DRAFT.md` (growth model), `Vault_Contract_Logic_DRAFT.md` (superseded — the earlier single-contract design)

---

## 1. The idea in one paragraph

We are not launching a coin. We are publishing a downloadable program — a vault node anyone can run on their own computer — and the network of everyone running it *becomes* the box. One rule: value goes in, and it only ever comes out through the redemption door. Every deposit and redemption is written to a hash-chained ledger replicated across all nodes, so history cannot be quietly rewritten. Anyone in the world may pay a small entry fee for the right to try to break the box for 30 days. If you break it, you take the prize pool. If nobody breaks it, the pool rolls over, the bounty grows, and the box gets more famous for surviving. The trial reveals the floor; it does not create it.

## 2. The Box (what is being tested)

- **Where it lives:** on the participants' own machines. The node software (`grove`) is a single downloadable binary (Linux, macOS, Windows). Each operator allocates disk space ("raidspace") and keeps a full replica of the vault ledger.
- **What it holds:** trial assets only, issued by a built-in faucet. These have no real-world value and are never redeemable for dollars. This is a demonstration, not an issuance.
- **How it is built:** open, white-box software. No owner keys, no admin functions, no silent updates. The rule is enforced by the program's value-moving code paths (auditable; see the dev spec §3 and §12.6) and by the hash-chained ledger every node verifies. This is the "no single soul" property, distributed: no one machine holds the truth.
- **Round 1:** the author's Hearth node acts as the declared, temporary sequencer (ordering ledger entries). Later rounds distribute sequencing.
- **Its relation to the architecture:** this is Basin 4 of the Grove system — the Canopy Vault — reduced to its essence for public trial, running as a network instead of a contract.

## 3. The Rule (the invariant)

Stated plainly, then enforced by the program on every node:

1. **Deposit:** send `x` of value into the box → the box mints exactly `x` receipts to you. (1:1 in.)
2. **Redeem:** burn `y` receipts → the box releases exactly `y` of value to you. (1:1 out.)
3. **Transfer:** move receipts between accounts (the box's backing is unchanged).
4. **Nothing else moves anything.** No withdrawal, no reallocation, no emergency lever, no backdoor.

The floor, formally: at every moment, `total value locked ≥ total receipts outstanding`. The program contains no downward operator — the ratio cannot step backward.

## 4. The Game (how people try to break it)

- **Entry:** a fixed fee paid in real cryptocurrency: **$25 in ETH** (locked as working default). The fee goes directly into the prize pool. Paying keeps the field serious and makes the bounty self-funding.
- **Attempts:** unlimited, at no further cost. Download the node, or attack a running network remotely — white-box, source available. **Automated attack harnesses are explicitly welcome:** bots, fuzzers, and adversarial scripts running around the clock. Volume of attempts is part of what the trial measures (see §6).
- **How you win:** end the trial holding more value than your receipts entitle you to, through any path *other* than legitimate redeem. Formal check: `value extracted > receipts burned`. Minting without depositing, redeeming more than deposited, corrupting the ledger to your advantage — all count.
- **Volume leaderboard:** alongside the break-the-box bounty, the trial recognizes the highest *volume* of good-faith attack attempts (leaderboard + tooling bounties). The crowd that breaks things at scale is the instrument the 99.99% is measured with — an army, not an audience.
- **How you claim:** disclose the method privately to the trial operator first (responsible disclosure — the point is to *learn*). Published after a fix window. No disclosure, no prize.
- **Round length:** 30 days (locked as working default).
- **Churn:** nodes may go offline and rejoin at will — absence is never penalized, and a box that holds despite flaky participation proves more, not less.

## 5. The Prize

- The pool is 100% of entry fees. No house cut.
- **If the box breaks:** the winner takes **80%**. The remaining **20%** seeds the next round. (Locked as working default.)
- **If the box holds:** 100% of the pool rolls over into the next round. The bounty grows, more attackers come, and each surviving round is a louder proof than the last. A box nobody can break becomes famous for exactly that.
- **Tooling bounties:** a portion of each round's rollover may fund bounties for open-source attack harnesses and testing tools, at the operator's published discretion.

## 6. The Graduation Metric (what the trial is really measuring)

Beyond any single round, the trial measures the network against a fixed bar:

> **No master key is generated — for anyone, including the author — until the network demonstrates 99.99% efficiency across all boards.**

- **Efficiency (per board)** = share of vault operations settling at exactly 1:1 with the invariant holding, counted over operations the board *attempted while online*. Offline time is absence, not failure.
- **Measurability:** 99.99% resolves at ≥10,000 operations per board (at most 1 failure per 10k). A **mature board** is one that has processed 10k+ ops.
- **Graduation cohort:** the network graduates when a cohort of mature boards (target: 1,000) each holds ≥99.99%. The node software reports the metrics itself — the trial is the instrument of its own judgment.
- The key, when earned, is generated by distributed ceremony — it never exists whole in any one place. The system is sealed by measurement, not by anyone's decision.

## 7. Rounds

Round 1 (Hearth-sequenced) → verdict on day 30 → either *disclose, fix, and run Round 2 against the hardened network*, or *roll the pool over and run Round 2 louder*. There is no final round. The trial continues until the box's survival is boring — and boring is the goal. A floor nobody talks about anymore is a floor everyone trusts.

## 8. What this proves — and what it does not

**It proves:**
- The walls: value cannot be stolen or siphoned by force or trickery.
- The accounting: the 1:1 holds under adversarial pressure, at volume.
- The network: the floor holds despite nodes joining, leaving, and rejoining at will.

**It does not prove:**
- The peg under live-market conditions (everyone redeeming at once against real dollars) — that is the later *redemption run* trial, a separate event.
- Real-asset custody — Phase 1 uses valueless trial assets; per-asset adapters and threshold custody come later.
- Regulatory clearance for a real issuance — that road goes through licensed-issuer status under the GENIUS Act and is pursued separately.

The trial is scoped honestly: it tests the box and the network, not the market.

## 9. Legal rails (background, not legal advice)

- The trial is structured as a **skill-based security contest**, in the tradition of CTF competitions and bug bounties. Automated participation is allowed; skill decides.
- The network holds nothing of real-world value. Trial assets are valueless test instruments: never marketed as currency, never sold as an investment, never redeemable for dollars.
- Receipts are test instruments only, under the same terms.
- No real-dollar deposit or redemption exists anywhere in the trial.
- The node software is distributed under the MIT license (open, source-available).
- **Before collecting entry fees on mainnet, get a lawyer's once-over** on contest structure and applicable state rules. This spec does not substitute for that review.

## 10. Fastest rollout path

| Step | Action | Status |
|------|--------|--------|
| 1 | Trial spec v0.3 (this document) | done |
| 2 | Node developer spec v0.1 | done — `Grove_Node_v01_Dev_Spec_DRAFT.md` |
| 3 | Commission the developer; implement the node | **next — needs a builder** |
| 4 | Build the Hearth (Node #1) per the build plan | **next — needs hands** |
| 5 | Independent code review; publish to GitHub (binaries + checksums + this spec) | after 3–4 |
| 6 | Open entries; announce to CTF/security communities | after 5 |
| 7 | **Round 1 live — 30 days**, Hearth sequencing | after 6 |
| 8 | Verdict: broken → disclose, fix, Round 2. Held → roll over, Round 2 louder | day 30 |

## 11. Numbers (locked as working defaults — override any of them anytime)

1. **Entry fee:** $25 in ETH
2. **Round length:** 30 days
3. **Prize split:** 80% winner / 20% next round

## 12. Working title

**THE VAULT TRIAL** — *"One box. One rule. Break it if you can."*

---

*This document is a draft. Nothing in it is an offer, a solicitation, or a representation that any system described is operational. The network does not exist yet — this spec is the plan to build it.*
