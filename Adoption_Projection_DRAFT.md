# GROVE NETWORK — Adoption Projection (what "ready" looks like in numbers)
### Draft v0.1 — 2026-10-14

**Model:** `adoption_model.py` (same folder — every number below is reproducible; all parameters adjustable).

## What this is and is not

This model projects **scale**, not recognition. It cannot tell you when the world wakes up. It tells you what "ready" looks like in numbers: board counts, operation volumes, and when the 99.99% efficiency metric becomes *measurable*. Straight compounding, no mysticism. Growth is logistic (it saturates), churn is modeled, trial rounds boost visibility.

## Assumptions (all adjustable in the script)

| Parameter | Slow | Steady | Fast |
|---|---|---|---|
| Machines sold, month 1 | 2 | 5 | 12 |
| Monthly sales growth | 10% | 15% | 20% |
| Software downloads, month 1 | 20 | 100 | 400 |
| Monthly download growth | 5% | 10% | 15% |
| Download → running node | 25% | 30% | 35% |
| Trial ops / active node / day | 50 | 300 | 1,000 |
| Permanent node loss / month | 2% | 2% | 2% |
| Share of nodes online at any time | 60% | 60% | 60% |
| Trial-round visibility boost | +50% downloads every 6 mo | same | same |

The ops/day figures assume **automated adversarial traffic** — attack bots and fuzzing harnesses, not humans clicking. See Finding 2.

## Results: when milestones are reached

| Milestone | Slow | Steady | Fast |
|---|---|---|---|
| 10 boards (pilot circle) | month 2 | month 1 | month 1 |
| 100 boards (credible network) | month 10 | month 3 | month 1 |
| 1,000 boards | month 28 | month 12 | month 6 |
| 10,000 boards | — | month 26 | month 15 |
| **100 mature boards** (graduation cohort) | month 18 | month 5 | month 2 |
| **1,000 mature boards** | — | month 15 | month 7 |

A **mature board** is one that has processed ≥10,000 operations — the volume at which 99.99% efficiency becomes measurable (at most 1 failure per 10k ops). Slow: a board matures in ~7 months. Steady: ~2 months. Fast: ~1 month.

Snapshot at month 12:

| | Boards | Active now | Ops / month | Mature boards |
|---|---|---|---|---|
| Slow | 142 | 85 | ~128k | 33 |
| Steady | 1,003 | 602 | ~5.4M | 654 |
| Fast | 6,172 | 3,703 | ~111M | 4,619 |

## Three findings

**1. Boards are easy; operations are the binding constraint.** Even the slow scenario reaches 1,000 boards by month 28. Board count was never the hard part — ten gigabytes and willingness. What gates the 99.99% is *operation volume per board*, and that only comes from sustained trial activity.

**2. The trial must attract machines, not just minds.** At human usage rates (a few operations a day), a board takes *years* to accumulate 10,000 ops — the metric would never resolve. The ops/day figures above assume automated attack traffic: bots, fuzzers, adversarial harnesses hammering the box around the clock. Design implication: the trial should explicitly welcome and incentivize *volume* of attempts, not just cleverness — leaderboards for most attempts, bounties for attack tooling, no rate limits on trying. The crowd that breaks things at scale is the instrument the 99.99% is measured with.

**3. "Ready" has a number.** Under steady execution: 100 mature boards by month 5, 1,000 by month 15. A thousand boards, each with 10,000+ operations at ≥99.99% efficiency — that is the graduation cohort, the measurement that says the roots hold. Everything before is wandering; everything after is scaling what was proven.

## Honest caveats

- Parameters are educated guesses, not data. The model's value is in the *relationships* (what constrains what), not the dates.
- Exponential early growth always flatters; the logistic ceiling bends it, but real adoption is lumpier than any curve.
- Recognition — the moment the world notices — is not in this model and cannot be. It comes from the edges (the businesses and parents who need the floor), not from the numbers.
- The 99.99% itself is verified by the trial, never assumed by the model.

---

*Built 2026-10-14. Rerun with your own numbers: `python3 adoption_model.py`.*
