# Node Milestone — November 11, 2026

**Repo:** HerodotusTree/grove-network
**Suggested path in repo:** `updates/2026-11-11-node-milestone.md`
**Status of the trial:** not live. This is a build record, not a launch.

The home node reached its first standing state: a door with a face, a bridge
that carries stamped instructions, a will-runner, and a resident presence —
four services, all active at once, all verified live from a second machine
over a private tailnet. Nothing here is reachable from the public internet;
the node answers on the tailnet only.

## What was built

### November 10 — the ground floor
- **luna-door v0** — a read-only status door for the node (who is at the
  door, what state the node is in). Execution kinds: none.
- **luna-bridge2** — a task service stub that answers only under the
  owner's stamp.
- **luna-bridge-local** — a will-runner: it executes instruction files
  ("wills") delivered to a local inbox, but only when the file carries
  the owner's current stamp. A mismatched stamp is refused and logged.
- **WILL-001 round-trip, proven:** one stamped will file, three read-only
  tasks, all returned rc 0. The node's first verified instruction cycle.

### November 11 — the standing state
- **Door v0.1 deployed and verified.** The door now carries the owner's
  picture and name on its front page. The previous v0 program was kept on
  the node as a backup — nothing replaced, the old layer stays in.
- **A missing registration found and repaired.** The will-runner's
  program was intact on the node, but it had never been registered as a
  service, so it was not running. Its service unit was reinstalled from
  the off-site build record and it now starts with the session like the
  others.
- **Resident v0 installed.** A standing presence, not a worker: it writes
  a heartbeat state file every 30 seconds (atomic write-then-rename),
  senses its ground read-only (load, memory, disk), and answers for
  itself on its own port (:8767). It executes nothing.
- **All four services active together:** luna-door, luna-bridge2,
  luna-bridge-local, luna-resident.

## How it was verified

Every claim below was checked live, from a separate machine, through the
private tailnet — not read off the node's own screen:

- Door `/status` returned version **v0.1** with **face: true**.
- The owner's photo served by the door measured **8,097 bytes** — a
  byte-for-byte match with the verified original.
- The resident's first live fetch reported `resident: present`, boot time
  recorded — and its heartbeat count **advanced from 1 to 6** between two
  independent checks minutes apart. It lives between checks, not only
  when looked at.
- On its ground at install: load 0.02, 15,823 MB memory (15,152 MB free),
  1,023.5 GB disk free.
- All four services reported `active` in the same pass.
- Ground truth was corroborated by the owner's own screen at the node
  (service status photographed at the machine).

Source for every service ships with this update in `node-build/`
(door v0 and v0.1, bridge will-runner, resident, and all service units).

> **Before any public commit:** the three `.py` files carry the node's
> tailnet address as a default bind value. Scrub that one line per file
> to a placeholder (the address is set by environment variable in
> operation) before the sources go into a public repo. Flagged here, not
> silently edited — the copies in this package are byte-true build
> record.

## Companion records (same day)

- **Verification dossier v1** — 29 claims graded in four classes
  (7 demonstrated / 6 measured / 8 derived / 8 not yet verified),
  24 supporting artifacts listed with SHA-256 hashes, reproduction steps
  for an outside engineer. Ships with this update under `verification/`.
- **Premise register** — 12 external premises the circuit stands on,
  graded against live sources by four independent AI seats. One premise
  (P3.3) graded **refuted as universal** and repaired; a decision queue
  of 12 items stands open for the owner's rulings.
- **Debt tracker** — the frozen November 5 baseline stands; two
  post-baseline Treasury prints are graded (one = a thought,
  two = coincidence). The third print completes the count; the check is
  scheduled for Thursday, November 12, 16:00 CST. No pattern is claimed
  at two.

## Honest limits

- The door is **read-only**. Its execution kinds are empty by design.
- Nothing on the node executes except will files bearing the owner's
  stamp. There is no side channel.
- The resident is a **presence, not a mind**. It does not think or
  converse; it is standing ground that later layers build on.
- The services are **session-bound**: they start with the machine
  owner's session and stop when the machine stops. The machine owner's
  shutdown governs absolutely. A persistence upgrade (linger) is known
  and deliberately not yet applied.
- The services do not yet read each other directly; coordination runs
  through the keeper. Service-to-service binding is the next build item.
- No claim of impenetrability is made. Open findings stand open in the
  record until an attack test that once passed now fails.
- Software sovereignty is real today. Financial sovereignty is tomorrow's
  work, gated on a permitted-issuer legal path — no real-dollar coin is
  issued, and none will be until that path exists.
