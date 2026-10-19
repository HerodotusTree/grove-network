# Contributing to Grove

Grove is an experimental trial-asset prototype. Contributions are useful when
they make an implemented behavior easier to verify, fix a reproducible failure,
or explain the difference between the current code and the draft design.
Read [README.md](README.md) for project status and [NODE.md](NODE.md) for the
implemented node interfaces. The architecture documents describe intended work;
they are not evidence that every described feature is implemented.

## Set up a local checkout

Fork the repository, clone your fork, and create a branch for one coherent
change. Use the Go version required by the module files, currently `1.27.2`,
and include `go version` in bug reports and PR validation notes.

The repository has separate Go modules, with local replacements connecting
them. There is no root Go module, so run commands inside the relevant module.

| Directory | Responsibility |
| --- | --- |
| `identity/` | Key generation, signing, and signature verification |
| `ledger/` | Hash-chained entries, journal persistence, and recovery |
| `vault/` | Aggregate reserve, claim, and floor arithmetic |
| `node/` | HTTP handlers, peer sync, and nonce persistence |
| `loadgen/` | Local trial-operation load generator |

Build the node from the repository root:

```bash
cd node
go build -o grove-node .
```

Use disposable data directories and valueless trial amounts for reproductions.
Test network behavior on nodes you control, or within separately authorized
test scope. Do not include private keys, credentials, or personal data in reports.

## Run checks

From the repository root, run each command and record its result:

```bash
(cd identity && go test -race ./...)
(cd ledger && go test -race ./...)
(cd vault && go test -race ./...)
(cd node && go test -race ./...)
(cd loadgen && go test ./...)
```

Format edited Go files with `gofmt` and run `go vet ./...` inside the affected
module. Changes to the ledger or vault also need the downstream node checks.
Documentation-only changes can be checked for working links, accurate commands,
and whitespace errors with `git diff --check`.

### Current security-test baseline

The node suite contains tests that assert desired safe behavior and currently
fail because the corresponding vulnerability is still reproducible. A failing
regression is evidence of the problem; it must not be described as a successful
security check.

At reviewed commit `12c4cd8c97367caf711fd7637bbe71d75ec209ee`, the failing tests
were:

| Test | Tracking discussion |
| --- | --- |
| `TestAttack_SplitWriteDoubleApplyAfterRestart` | [#5](https://github.com/HerodotusTree/grove-network/issues/5) |
| `TestAttack_CrossNodeReplay` | [#4](https://github.com/HerodotusTree/grove-network/issues/4) |
| `TestAttack_ReplayAfterSync_ContradictsKnownLimits` | [#4](https://github.com/HerodotusTree/grove-network/issues/4) |
| `TestAttack_SyncStepsFloorBackward_MetricsStaySilent` | [#1](https://github.com/HerodotusTree/grove-network/issues/1) |
| `TestAttack_StrangerRedeemsVictimsClaims` | [#3 follow-up](https://github.com/HerodotusTree/grove-network/issues/3#issuecomment-6244466761) |
| `TestAttack_VictimLockedOutAfterStrangerRedeem` | [#3 follow-up](https://github.com/HerodotusTree/grove-network/issues/3#issuecomment-6244466761) |
| `TestSyncAdoptsFabricatedPeerChain` | [#1](https://github.com/HerodotusTree/grove-network/issues/1) |

Recheck the baseline on the commit you are testing. Report any new failure
separately. When fixing one of these behaviors, keep the test asserting the
safe result, demonstrate that it fails before the fix and passes afterward,
and update this baseline. Do not invert an assertion or suppress a failure
just to obtain a passing run. A subset of passing checks does not establish
that the full suite passes.

## Report a bug

Search existing issues first. Add a new reproduction to the existing report
when it demonstrates the same underlying problem. Include:

- The tested commit, Go version, operating system, and relevant configuration.
- Minimal steps or a local automated test, including any prerequisites.
- Expected behavior, actual behavior, and the relevant output.
- The affected property: ownership, replay protection, accounting, durability,
  availability, or another concrete behavior.
- Whether the test used in-process handlers, local HTTP servers, or a simulated
  filesystem failure. Keep a simulated crash state distinct from a measured
  physical power-loss test.

For a sensitive vulnerability, use GitHub's private vulnerability reporting
feature if it is available, or follow any published security policy. If neither
provides a contact, ask for the preferred private reporting channel without
posting sensitive details. [GitHub's reporting guidance](https://docs.github.com/en/code-security/how-tos/report-and-fix-vulnerabilities/report-privately)
explains these options. Ordinary public bug reports can use the repository's
bug-report template.

## Propose a change

Discuss changes to the signing domain, ledger format, sequencing policy, or
accounting model in an issue before building a large implementation. Describe
the intended guarantee, migration or recovery behavior, and compatibility with
existing data. Small fixes and documentation improvements can be proposed
directly as focused PRs.

A PR description should explain the concrete trigger, the resulting behavior,
the tests run, and remaining limitations. Link the report it addresses. Keep
unrelated refactors, generated artifacts, and machine-specific configuration
out of the change. Review generated or tool-assisted code before submitting it;
the validation evidence should stand on its own.

For a security fix, distinguish the properties the patch repairs from those
that remain open. For example, valid request signatures alone do not establish
receipt ownership, and a hash-consistent history alone does not establish its
authority.

## Review and issue discussion

Keep discussion focused on code, reproducible evidence, and the claimed
property. Contributors can disagree about a design without making claims about
another person's motives. Maintainers decide project scope and whether to
accept a proposed change; contributions do not imply a response deadline.

When proposing closure as fixed, include the fixing commit or PR and the passing
regression. If a report is out of scope, a duplicate, or intentionally deferred,
record that reason and link the relevant decision rather than implying that
the behavior was repaired. Update user-facing claims when verified behavior or
known limitations change.

The repository's license is available in [LICENSE](LICENSE).
