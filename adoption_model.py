#!/usr/bin/env python3
"""Grove Network adoption projection — straight compounding, no mysticism.

Models node growth across three scenarios and reports when the network
crosses key thresholds: board counts, and the operations volume needed for
the 99.99% efficiency metric to be meaningful (>=10k ops per board).

This model projects SCALE, not recognition. It cannot tell you when the
world wakes up — only what "ready" looks like in numbers.

All parameters adjustable below.
"""

# ---------------------------------------------------------------- parameters
SCENARIOS = {
    "slow": dict(  # word of mouth only, no press
        machines_0=2, machine_growth=0.10,      # machines sold, month 1 and monthly growth
        downloads_0=20, download_growth=0.05,    # software downloads, month 1 and growth
        conversion=0.25,                          # downloads -> running nodes
        ops_per_day=50,                           # trial ops per active node/day (incl. attack bots)
        carrying=50_000,                          # logistic ceiling: saturating audience
    ),
    "steady": dict(  # consistent effort + trial visibility
        machines_0=5, machine_growth=0.15,
        downloads_0=100, download_growth=0.10,
        conversion=0.30,
        ops_per_day=300,
        carrying=500_000,
    ),
    "fast": dict(  # trial goes viral, strong word of mouth
        machines_0=12, machine_growth=0.20,
        downloads_0=400, download_growth=0.15,
        conversion=0.35,
        ops_per_day=1000,
        carrying=5_000_000,
    ),
}

PERMANENT_CHURN = 0.02   # share of nodes permanently lost per month (dead hw, lost interest)
ACTIVE_FRACTION = 0.60   # voluntary churn: share of live nodes online at any time
TRIAL_BOOST_EVERY = 6    # months between trial-round visibility boosts
TRIAL_BOOST = 1.5        # download jump after each survived round ("the box nobody can break")
MONTHS = 36
OPS_FOR_METRIC = 10_000  # ops per board for 99.99% to resolve (<=1 failure per 10k ops)


# ---------------------------------------------------------------- model
def simulate(p):
    machines, downloads = p["machines_0"], p["downloads_0"]
    total = 0.0
    cohorts = []          # new nodes per month, for maturity tracking
    milestones, rows = {}, {}
    # months for a consistently-active board to reach 10k ops
    k_mat = max(1, -(-OPS_FOR_METRIC // int(p["ops_per_day"] * 30)))
    for m in range(1, MONTHS + 1):
        if m > 1:
            machines *= (1 + p["machine_growth"])
            downloads *= (1 + p["download_growth"])
        if m % TRIAL_BOOST_EVERY == 0:
            downloads *= TRIAL_BOOST
        new_nodes = (machines + downloads * p["conversion"]) * max(0.0, 1 - total / p["carrying"])
        cohorts.append(new_nodes)
        total = total * (1 - PERMANENT_CHURN) + new_nodes
        active = total * ACTIVE_FRACTION
        ops = active * p["ops_per_day"] * 30
        # mature boards: joined >=k_mat months ago, survived churn since
        mature = sum(c * (1 - PERMANENT_CHURN) ** (m - (i + 1))
                     for i, c in enumerate(cohorts) if m - (i + 1) >= k_mat)
        rows[m] = (total, active, ops, mature)
        for name, thresh in (("10 boards (pilot circle)", 10),
                             ("100 boards (credible network)", 100),
                             ("1,000 boards", 1000),
                             ("10,000 boards", 10000)):
            if name not in milestones and total >= thresh:
                milestones[name] = m
        for name, thresh in (("100 mature boards (graduation cohort)", 100),
                             ("1,000 mature boards", 1000)):
            if name not in milestones and mature >= thresh:
                milestones[name] = m
    return rows, milestones, k_mat


def fmt(m):
    return f"month {m}" if m else "not reached in 36 mo"


# ---------------------------------------------------------------- report
for name, p in SCENARIOS.items():
    rows, milestones, k_mat = simulate(p)
    print(f"\n=== SCENARIO: {name.upper()} "
          f"(board matures in ~{k_mat} mo at {p['ops_per_day']} ops/day) ===")
    print(f"{'milestone':42s} reached")
    for k in ("10 boards (pilot circle)", "100 boards (credible network)",
              "1,000 boards", "10,000 boards",
              "100 mature boards (graduation cohort)", "1,000 mature boards"):
        print(f"  {k:40s} {fmt(milestones.get(k))}")
    print(f"\n  {'month':>5s} {'boards':>8s} {'active':>8s} {'ops/mo':>10s} {'mature':>8s}")
    for m in (6, 12, 18, 24, 36):
        total, active, ops, mature = rows[m]
        print(f"  {m:5d} {total:8.0f} {active:8.0f} {ops:10.0f} {mature:8.0f}")
