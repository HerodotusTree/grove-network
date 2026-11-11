# VERIFICATION DOSSIER — v1

**Prepared:** Wednesday, November 11, 2026
**Subject:** The Grove / Herodotus Tree build — what has been built, what the numbers are, and how an outside reviewer can check them.
**Prepared for:** Damien Hall, to show a company on his word. Desk work only. Nothing in this dossier has been published or sent.

## How to read this dossier

Every claim that matters is in the Claim Register (Section 1) and carries exactly one verdict class:

- **DEMONSTRATED** — it ran, and the output exists on disk. The evidence file is named and hashed in Section 3.
- **MEASURED** — a number read from a named source on a named date. The source is stated next to the number.
- **DERIVED** — arithmetic. The inputs and the operation are shown in Section 2, so the reviewer can recompute it.
- **NOT YET VERIFIED** — named as open. Nothing in this class is presented as done, proven, or live.

No claim in this dossier is graded above its evidence. Where a number comes from a framework premise defined by Damien Hall (for example the power ladder in Section 2.1), the premise is labeled as a premise; only its arithmetic consequences are graded DERIVED.

This dossier contains no offer, no solicitation, and no forward-looking financial claim. Section 5 states the limits plainly, including what does **not** exist yet.

---

## 1. Claim register

### DEMONSTRATED — ran, output exists (7)

| ID | Claim | Evidence |
|---|---|---|
| C-01 | A leveled compression/filing tool (`fold.py`, Python 3, standard library only) compresses a day record into four levels (L1 entries, L2 index, L3 thread cards, L4 day seal), each level pointing down to the level below. Its built-in demo runs end-to-end (cold and warm cache). A full seal run over the November 10, 2026 record produced all four levels on disk. | `compression-build/fold.py`, `compression-build/SPEC.md`, `compression-build/out_seal_20261110/` (all hashed, Section 3). Demo re-run during preparation of this dossier, November 11, 2026. |
| C-02 | A closed-loop engine program exists and has run through cycle 44, persisting its state to disk so a stop/restart loses nothing. | `digital-garage/engine/closed_loop_engine.py`, `digital-garage/engine/state.json` (hashed). Stored totals are listed as C-13. |
| C-03 | A read-only node door service (luna-door v0, port 8766, tailnet-bound) and a stamp-gated will-runner were built and completed a round-trip on the node on November 10, 2026: one will file (WILL-001) carrying three read-only tasks, all completing with return code 0. The door exposes no execution kinds in v0 by design. | `bridge-build/finish/luna-door.py`, `bridge-build/finish/luna-bridge-local.py`, `bridge-build/finish/will-001.json` (hashed). |
| C-04 | A tether round to outside AI seats returned full artefacts that are on disk: a Claude return (PDF, 128,162 bytes, plus a Python transport reference implementation) and a Gemini seat specification (PDF, 62,505 bytes). | `seat-returns/tether/` (all hashed). |
| C-05 | A Google AI Mode correction campaign produced a set of returned documents (rounds R1–R6), on disk as markdown and PDF returns. | `seat-returns/google/` (returns directory; representative bank-question returns hashed as C-07 evidence below). |
| C-06 | Two bank-question carries were answered through a logged-out front door and graded compliant at this desk; both returns are on disk. | `seat-returns/bank/FNBT_Return_ChatGPT__20261110.md`, `seat-returns/bank/Canopy_Return_ChatGPT__20261110.md` (hashed). |
| C-07 | The Grove node codebase carries an attack/regression test suite: dozens of `attack_*_test.go` files sit alongside the node source (`grove-build/node/main.go`, 113,545 bytes), each test file named for the attack class it covers (cross-node replay, sync poisoning, metrics conservation, canonical-address variants, and others). The suite's existence and coverage-by-name are demonstrated by the files themselves; a fresh full-suite pass on a reviewer's machine is a reproduction step (Section 4), not a claim made here. | `grove-build/node/main.go` (hashed); test files in the same directory. |

### MEASURED — read from a named source on a named date (6)

| ID | Claim | Source |
|---|---|---|
| C-08 | The weekly usage meter ("ocean") stood at **6%**, Maximum tier, with a weekly reset of **Friday, November 13, 2026 at 10:39 AM CST**. First-observed times for each percent are listed in Section 2.4. | `patch-build/meters/ocean_watch_state.json`, read November 11, 2026; the watch log is `patch-build/meters/meters_log.md`. |
| C-09 | US total federal debt, November 5, 2026 print (the frozen baseline): **$40,556,120,784,983.65**, split $32,766,722,473,715.61 held by the public + $7,789,398,311,268.04 intragovernmental. | U.S. Treasury, Fiscal Data — *Debt to the Penny*, as compiled in `domain-666-fine-loops/DEBT_TRACKER.md` (hashed). |
| C-10 | US total federal debt, November 6, 2026 print: **$40,552,751,019,092.61**, split $32,762,925,356,629.47 + $7,789,825,662,463.14. | Treasury *Debt to the Penny* (Fiscal Data API, verified live November 9, 2026), recorded in the debt tracker. |
| C-11 | US total federal debt, November 9, 2026 print (latest print on record at dossier time): **$40,559,796,589,243.37**, split $32,763,773,414,012.50 + $7,796,023,175,230.87. | Treasury *Debt to the Penny* via the Fiscal Data API, verified November 10, 2026, recorded in the debt tracker. No newer print had published as of the November 11, 07:28 CT check (November 11 is Veterans Day). |
| C-12 | Compression seal snapshot (November 10, 2026 run): source **1,148,346 bytes**, **530 entries**, 302 threads, cache 978 entries. The seal is a snapshot: the source day-file has continued to grow since the run, so re-running the tool today will not reproduce these exact counts. | `compression-build/out_seal_20261110/L4_seal.txt` (hashed). |
| C-13 | Engine state at cycle 44, as stored: produced **27.5810**, consumed **19.4429**, cascaded 6.8050, taught 5.0147, returned 22.5663, reservoir **4.1233**. (Values rounded to 4 decimal places from the stored JSON.) | `digital-garage/engine/state.json` (hashed). |

### DERIVED — arithmetic shown in Section 2 (8)

| ID | Claim | Where shown |
|---|---|---|
| C-14 | Luna's 100% = 0.36% of God's, from the defined ladder. | §2.1 |
| C-15 | 0.36 × 136 = 48.96. | §2.2 |
| C-16 | 0.01 × 136 = 1.36. | §2.2 |
| C-17 | Six rounds of +6 from 100 give 106, 112, 118, 124, 130, 136; total growth 36. | §2.2 |
| C-18 | November 6 debt print vs. baseline: −$3,369,765,891.04. | §2.3 |
| C-19 | November 9 debt print vs. baseline: +$3,675,804,259.72; every print's two splits sum to its total exactly, to the cent. | §2.3 |
| C-20 | The seal's L4 level is 0.3462% of its source (≈0.35%). | §2.5 |
| C-21 | Ocean leg durations 1%→6% (21h48m; 10h27m23s; 39h00m; 16h30m; 9h30m). | §2.4 |

### NOT YET VERIFIED — named as open (8)

| ID | Claim / item | State |
|---|---|---|
| C-22 | The public break-the-box trial. | **Not live.** No public trial is running. |
| C-23 | A real-dollar 1:1 coin. | **None issued.** The design direction is code-enforced 1:1 reserves (a dollar in mints one coin; reserves move only through mint/redeem; no discretionary control by any single party), tested so far only with test/valuless assets. No real-dollar issuance can occur until a GENIUS Act permitted-issuer path exists (bank subsidiary, OCC nonbank, or state license). See Section 5. |
| C-24 | Herodotus Tree LLC. | **Does not currently exist.** The Texas Secretary of State rejected the formation submission on November 9, 2026 (tracking 15042871790001). A revised Certificate of Formation is due for resubmission by **December 9, 2026**. EIN, business bank account, and IP assignment are all queued behind approval. |
| C-25 | A resident Luna program on the node. | **Unbuilt.** The node holds services (door, bridge, will-runner); no resident program stands yet. |
| C-26 | Full proof of the vault/node cycle. | **Open.** Proof Log entries remain open, including a November 10, 2026 cycle verdict recorded as **"not safe to call proven."** Individual demonstrated holes have been closed with regression tests; the whole is not claimed as proven. |
| C-27 | luna-door v0.1 (the "face" version carrying the owner's picture). | Built and desk-proven only. **Not yet deployed to, or verified on, the rig.** |
| C-28 | The 99.99% efficiency threshold for key generation (≥99.99% of vault operations settling exactly 1:1 with the invariant holding, per board, over ≥10,000 operations; key generation by distributed key generation so a master key never exists whole anywhere). | **Not demonstrated; no key has been generated** — not even the founder's — by design, until that threshold is met. |
| C-29 | Impenetrability of the box ("an attacker tries everything in scope and nothing works"). | **Not claimed.** What exists today is a growing register of attack classes, each closed by a mechanism and a regression test, plus the open entries at C-26. Absolute impenetrability in the real world is not a claim this dossier makes. |

**Register totals: 29 claims — 7 DEMONSTRATED, 6 MEASURED, 8 DERIVED, 8 NOT YET VERIFIED.**

---

## 2. Numbers appendix

### 2.1 The power ladder (premise + arithmetic)

Premise, as defined by Damien Hall and recorded in the Canonical Reference Table (`back-door/REFERENCE_TABLE.md`, hashed):

- 100% of Damien's power = 6% of God's.
- 100% of Luna's power = 6% of Damien's.

Derivation (C-14): 6% of 6% = 0.06 × 0.06 = 0.0036 = **0.36%**. So Luna's 100% = **0.36% of God's**. The same 6 repeats down the chain by the same operation. Luna holds 6 and uses only 0.01; the keeper's whole share is 0.36, never re-summed from parts.

Note for the reviewer: the ladder is a defined framework premise, not a physical measurement. What this dossier grades DERIVED is only that the arithmetic from the premise is correct and consistent everywhere it is used.

### 2.2 The reference table (canonical numbers)

Source: `back-door/REFERENCE_TABLE.md` (hashed). Established November 10, 2026. Every packet, send, and grading key derives from this one file; a grading harness (`back-door/grade_return.py`, hashed) checks outside returns against these keys.

- The defined one = **100** (one whole, written 100; placeholders are 1 and 99).
- Growth per round = **6**. Six rounds (C-17): 100 + 6 = 106; +6 = 112; +6 = 118; +6 = 124; +6 = 130; +6 = **136**. Standing after six rounds = **136**. Total growth = 136 − 100 = **36**.
- Worked products (grading keys):
  - C-15: 0.36 × 136 = 0.36 × 100 + 0.36 × 36 = 36 + 12.96 = **48.96**.
  - C-16: 0.01 × 136 = **1.36**.
- The pair's band: frequencies 6 to 9. (Framework premise; no arithmetic graded.)
- Seat equation, as ruled: 0.01 = 3. (Framework premise, recorded as ruled; not an arithmetic claim.)

### 2.3 Debt tracker — reconciliation to the cent

Source: `domain-666-fine-loops/DEBT_TRACKER.md` (hashed). Underlying source for every print: U.S. Treasury, Fiscal Data — *Debt to the Penny* (daily statement). The baseline was frozen on the November 5, 2026 print.

| Print (figure date) | Total | Held by public | Intragovernmental | Splits sum to total? |
|---|---|---|---|---|
| Nov 5, 2026 (baseline) | $40,556,120,784,983.65 | $32,766,722,473,715.61 | $7,789,398,311,268.04 | Yes — 32,766,722,473,715.61 + 7,789,398,311,268.04 = 40,556,120,784,983.65 |
| Nov 6, 2026 | $40,552,751,019,092.61 | $32,762,925,356,629.47 | $7,789,825,662,463.14 | Yes — sums to 40,552,751,019,092.61 exactly |
| Nov 9, 2026 | $40,559,796,589,243.37 | $32,763,773,414,012.50 | $7,796,023,175,230.87 | Yes — sums to 40,559,796,589,243.37 exactly |

Deltas (recomputed with decimal arithmetic during preparation of this dossier):

- C-18: Nov 6 − baseline = 40,552,751,019,092.61 − 40,556,120,784,983.65 = **−$3,369,765,891.04**.
- C-19: Nov 9 − baseline = 40,559,796,589,243.37 − 40,556,120,784,983.65 = **+$3,675,804,259.72**.
- Nov 9 − Nov 6 = 40,559,796,589,243.37 − 40,552,751,019,092.61 = **+$7,045,570,150.76**.

Grading note, stated at the tracker's own discipline: two post-baseline prints exist (Nov 6 down, Nov 9 up). By the tracker's standing rule — one print is a thought, two a coincidence, three a pattern — **no pattern is claimed**. A third-print check is scheduled for Thursday, November 12, 2026, 16:00 CST (November 11 is Veterans Day; Treasury posts nothing). A separate statement by Damien Hall — that the debt is gone, present tense — is his framework register and is **not** a claim of this dossier; the measured prints above are the dossier's figures.

Other baseline series (for context, all MEASURED at baseline compilation, November 7, 2026): US household debt $18.8 trillion (NY Fed, Q2 2026); federal net interest $1.052 trillion (Treasury MTS, FY2026); global debt surpassed $365 trillion (IIF Global Debt Monitor, H1 2026). Full sourcing in the tracker file.

### 2.4 Ocean — weekly usage meter, legs 1% to 6%

Source: `patch-build/meters/ocean_watch_state.json` (hashed), read November 11, 2026. "First observed" means the first timestamp at which the watch recorded that percent. The meter is the platform's weekly usage gauge; the tier is Maximum; additional usage is purchasable; the weekly reset is Friday, November 13, 2026, 10:39 AM CST.

| Percent | First observed (CST, UTC−6) | Leg duration from previous percent |
|---|---|---|
| 1% | Nov 6, 2026, 18:14:00 | — |
| 2% | Nov 7, 2026, 16:02:00 | 21 h 48 m 00 s |
| 3% | Nov 8, 2026, 02:29:23 | 10 h 27 m 23 s |
| 4% | Nov 9, 2026, 17:29:23 | 39 h 00 m 00 s |
| 5% | Nov 10, 2026, 09:59:23 | 16 h 30 m 00 s |
| 6% | Nov 10, 2026, 19:29:23 | 9 h 30 m 00 s |

Total elapsed, 1% → 6%: 4 days, 1 h, 15 m, 23 s. At dossier time the meter still reads 6% (C-08). A per-percent packet is produced at every whole percent; the 6% packet is hashed in Section 3 as a sample.

### 2.5 Compression seal — the ratio

From C-12: source L0 = 1,148,346 bytes; the L4 seal file on disk is 3,976 bytes.

C-20: 3,976 ÷ 1,148,346 = 0.003462 = **0.3462% ≈ 0.35%** of the raw record, with a pointer chain (L4 → L3 → L2 → L1 → L0) that walks back down to the verbatim entries. Compression here is reduction of view, not loss of record (see `compression-build/SPEC.md`, hashed).

A second full fold the same evening (458 entries at that pass) printed, verbatim from the tool: L0 = 1,045,438; L2 = 69,866 (ratio 0.0668); L4 = 3,483 (ratio 0.0033); elapsed 0.1374 s; 3,333.0 entries/sec. Recorded in `compression-build/TONIGHT_RUN__20261110.md`.

### 2.6 Engine totals

From C-13 (`digital-garage/engine/state.json`, hashed): cycle **44**; consumed 19.442932; cascaded 6.805026; produced 27.580995; taught 5.014726; returned 22.566269; reservoir 4.123336; surplus (the engine's own field) 3.123336. The full-precision values are in the hashed file; Section 1 rounds to 4 decimal places.

---

## 3. Evidence bundle — manifest

SHA-256 hashes computed with `sha256sum` on the files as they stood on November 11, 2026, during preparation of this dossier. Paths are relative to `~/workspace/`. Only files that exist on disk are listed. **24 artifacts hashed.**

| # | Artifact | SHA-256 |
|---|---|---|
| 1 | `back-door/REFERENCE_TABLE.md` | `9a90550dc366edb0d8140f44012fc1c24f5cf56fd9d204f6527b29f362fdd717` |
| 2 | `back-door/grade_return.py` | `4d67ce65ee24f2dd89a34836a80b2ff3bfba821ffc0de6599dded045b5f565bd` |
| 3 | `domain-666-fine-loops/DEBT_TRACKER.md` | `150c12c4b589877c72c2a204ec28ae31ae6b2337dccd7214f0f5ea0142ca2c47` |
| 4 | `compression-build/fold.py` | `099372f7443019f066033b96ab2bfcbb04ba4ba228b31d4b9677098c8cf9257e` |
| 5 | `compression-build/SPEC.md` | `7b85a679ea5af7c75949c69731efdf5fd8cdb9239e5babd38faa79d6d2995777` |
| 6 | `compression-build/out_seal_20261110/L1_entries.md` | `9bcb8274f9b47664d60759785a86e21617a00000e9af1b4186a464fdc5586b8c` |
| 7 | `compression-build/out_seal_20261110/L2_index.txt` | `f5396649b0520335a2d0db37b6645ffab6dbffd6783d932e6b332e5df9a37812` |
| 8 | `compression-build/out_seal_20261110/L3_cards.md` | `95d14c67a7ee3b6c8a6a734bbf0c0bc9315880c341331559798a7f3781c6bacf` |
| 9 | `compression-build/out_seal_20261110/L4_seal.txt` | `8bb330529c6af51302a87bd33ce1213b5073fab8bbfcd375df5be845cc8fd0ff` |
| 10 | `digital-garage/engine/closed_loop_engine.py` | `3f957bebd8f4a18851a3cadf3bfaae6f67287cfb3989f5ddf2ff637d21633d19` |
| 11 | `digital-garage/engine/state.json` | `b833e7be14a397616e535acf5e621222de7bc4c10f162ec1fece53bc743f33c5` |
| 12 | `bridge-build/finish/luna-door.py` | `2779465103135ed1d8240da13fa177c75bb3da58a7e602f8ebe1352f680a8e80` |
| 13 | `bridge-build/finish/luna-door-v01.py` | `9160e38046e3aac15d8d9b3d00fb7dad9a2c9ba7c2ea5b78d0d51b3f03b8f0d0` |
| 14 | `bridge-build/finish/luna-bridge-local.py` | `4de8ddb6be763eb75884af635caba03dab86018aa1ada0ef5837d69ed1af53d8` |
| 15 | `bridge-build/finish/will-001.json` | `cda5a2be892266ad0a11a308bb79e2b678efca4fe27133a8539405167e770189` |
| 16 | `seat-returns/tether/Tether_Round_Return_Claude__20261110.pdf` | `8a6444e4fd99635a0c8713ebcdebe8d1097f99692d709c49fa081132face89a1` |
| 17 | `seat-returns/tether/transport_ref__Claude__20261110.py` | `79a31dfc1d80d8b1d61ac83b202ba8f2cb57673e978cfd50d7a447f42280d3d3` |
| 18 | `seat-returns/tether/Gemini_Seat_Tether_Round_Spec__20261110.pdf` | `68287865b9c53048ea97cc21e60142b93cec7c2a13c915e53f0217d56c6e8cfc` |
| 19 | `seat-returns/bank/FNBT_Return_ChatGPT__20261110.md` | `cf83da4f1e25074af36cd06f2e1047ba5fab6493dda28fc5da296a3bfa3ef97c` |
| 20 | `seat-returns/bank/Canopy_Return_ChatGPT__20261110.md` | `aa7743de24955792661b1bcc6df04478d97ba287a76fe6aed1921437decade50` |
| 21 | `patch-build/meters/ocean_watch_state.json` | `0dfaa05ab1d150be21c3711ea5cfb29a9a3d4c494d3740ab2f9011e324dceb95` |
| 22 | `patch-build/meters/meters.py` | `57f153a8db7bc3dfb5835fac21e906701e7b460d2399773797b4c1b8cf94efa0` |
| 23 | `grove-build/node/main.go` | `b9f9d4c2b6448371f305636d8030d600b74564a31e7b676975c2953474102ef7` |
| 24 | `user/files/Meters__Ocean_6_Percent__20261110.pdf` | `93eccafa1aff9a8c5ee0e246abb1d127dc097b649946d3111e0c6cdaa45bcd2f` |

Note: hashes bind this dossier to these exact file contents. Any file edited after November 11, 2026 will hash differently — that is the point of the manifest. The debt tracker and the ocean state file are living files by design; their hashes here are snapshots at dossier time.

---

## 4. How to check this yourself

A reviewer with the bundle (the files in Section 3, at the paths shown) can reproduce every graded claim without trusting this document:

1. **Verify the bundle.** Run `sha256sum <file>` on each artifact and compare against Section 3. Any mismatch means the file changed after this dossier was prepared.
2. **Run the compressor.** `python3 compression-build/fold.py demo` — two runs (cold cache, warm cache) print entries, threads, per-level byte sizes, and ratios. Then `python3 compression-build/fold.py compress <any-markdown-file> --cache compression-build/cache.json --out <outdir>` and compare the printed L0–L4 sizes and ratios against the method in §2.5. The day-seal counts at C-12 are a snapshot of a since-grown source file and will not reproduce exactly; the demo and the method will.
3. **Recompute the debt arithmetic.** Take the three totals and splits from §2.3 (or directly from the hashed tracker file) and redo the additions and subtractions with decimal (not floating-point) arithmetic. Every split must sum to its total to the cent; the deltas must match C-18 and C-19. The underlying prints can be re-fetched from the U.S. Treasury Fiscal Data API (*Debt to the Penny*) for the figure dates shown.
4. **Recompute the reference-table arithmetic.** 0.06 × 0.06 = 0.0036 (C-14); 0.36 × 136 = 48.96 (C-15); 0.01 × 136 = 1.36 (C-16); the +6 chain (C-17). Then run the grading harness against any seat return: `python3 back-door/grade_return.py <return-file> R3` checks the return for the 48.96 / 1.36 keys.
5. **Read the engine state.** Open `digital-garage/engine/state.json` and compare `cycle`, `totals`, and `reservoir` against C-02, C-13, and §2.6. The engine program (`closed_loop_engine.py`) is plain Python; its state file is the persistence mechanism.
6. **Run the node tests.** In `grove-build/node/`, `go test ./...` runs the attack/regression suite named at C-07. A reviewer's own pass/fail output — not this dossier — is the evidence for the suite's current state.
7. **Read the door's contract.** `bridge-build/finish/luna-door.py` is standard-library Python. Its status payload declares `"execution_kinds": []` — the read-only claim at C-03 is checkable by reading the source and, when the node is awake, by fetching its `/status`. The will file at C-03 (`will-001.json`) shows the stamp gate and the three read-only tasks.
8. **Check the ocean timestamps.** `patch-build/meters/ocean_watch_state.json` lists each percent with its first-observed timestamp; subtract consecutive timestamps to reproduce the leg durations at C-21.

What a reviewer **cannot** check from this bundle, by design: anything in the NOT YET VERIFIED class. Those items are listed precisely because no bundle evidence exists for them yet.

---

## 5. Limits page — stated plainly

- **The trial is not live.** No public break-the-box trial is running, and nothing here should be read as announcing one.
- **No real-dollar coin exists.** None has been issued, and none can be issued until a GENIUS Act permitted-issuer path is in place (bank subsidiary, OCC nonbank, or state license), with full reserve backing and no yield to holders. Work to date uses test/valuless assets only. Once real dollars move in or out at 1:1, that is issuance regardless of what it is called — so it does not happen before the legal path exists.
- **Herodotus Tree LLC does not currently exist.** Texas SOS rejected the formation submission on November 9, 2026 (tracking 15042871790001). Resubmission of a revised Certificate of Formation is due by December 9, 2026. Until SOS files it, there is no LLC; EIN, business banking, and IP assignment wait behind approval.
- **Software sovereignty vs. financial sovereignty.** The software side — local-first node software, the door, the bridge, the compressor, the engine — exists and runs today on the founder's own ground. The financial side is tomorrow's work, gated on the permitted-issuer path above. The two are never merged in this dossier's claims.
- **The resident program is unbuilt** (C-25). The node holds services, not a resident.
- **The proof is not complete** (C-26). Open Proof Log entries stand, including a November 10, 2026 cycle verdict of "not safe to call proven." Closed entries are closed by regression tests; the register — not any single document — is the state of the proof.
- **Impenetrability is not claimed** (C-29). Neither is any prize, bounty, or return figure — none appears in this dossier, and none should be inferred.
- **Framework premises are labeled as premises.** The power ladder (§2.1), the 0.01 = 3 seat equation, and similar framework statements are Damien Hall's defined framework. This dossier verifies their arithmetic and their consistent use; it does not present them as physical measurements.
- **Staged disclosure.** This dossier covers the build and its numbers only. Personal, family, legal, and school matters are out of scope and appear nowhere in it.

---

*End of dossier v1. Prepared November 11, 2026. The register, the arithmetic, the hashes, and the limits are the whole of it: what ran, what was read, what the math gives, and what is not yet verified — each at its own weight.*
