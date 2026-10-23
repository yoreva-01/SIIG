# Score roadmap (63 → 85+)

DataFactor scan of 2026-10-23: **63 / C**, `SOURCE LOC 1,871`, `VOLUME 14.2`,
`MINING RANK 0`, `TASK CAPACITY ~0`. Buyer fit passed on *builds & tests at
HEAD*, *genuine history*, *self-contained*, *not a demo* — and failed only on
**task capacity**: "almost no mineable commits. Value scales with task capacity,
and quality times zero tasks is zero."

So the score is not held down by code quality any more. It is held down by how
much of the history arrives as a self-contained change with the test that proves
it. That is what this file tracks.

Skipped on purpose (CLI, user rule): Docker, Kubernetes, container tooling,
structured logging stack, `.env.example`.

## Levers, as the report ranks them

| # | Lever | Report impact | Status |
|---|---|---|---|
| 01 | Ship features with their tests, in small focused commits | payout + high | **in progress** — every item below is its own commit |
| 02 | Enforced coverage threshold in CI | medium | done — 80% gate, HEAD measured at 86.5% |
| 03 | Stop silently discarding filesystem walk errors | medium | done — `ScanResult.ScanErrors`, reported by the CLI |
| 04 | Dependency update automation + audit | low | done — Dependabot (gomod + actions) and `govulncheck` job |
| 05 | Commit cadence + conventional commit discipline | medium | **in progress** — `type(scope): subject` documented in CONTRIBUTING, applied throughout |
| 06 | Input validation at the network fetch boundary | low | done — size cap, strict JSON, `rules.Validate` on fetch and cache read |

Category scores that moved as a side effect: Test Coverage 55 → gate + 86.5%,
Security Hygiene 68 → boundary validation, CI/CD Maturity 62 → gates, release
smoke test, PR-based rule sync, Dependency Health 65 → refreshed deps + bot.

## Landed in this batch

1. `refactor(version)` — release version resolved from one package, stampable with `-ldflags`.
2. `fix(scanner)` — walk errors surfaced; nested `index.js`, bare `.vscode` and the project folder name no longer detect stacks; SIIG-authored ignore files no longer feed detection (idempotency).
3. `feat(output)` / `fix(output)` — unreadable paths reported; verbose order stable.
4. `feat(rules)` + `fix(fetcher)` + `fix(cache)` — `rules.Validate`, 1 MiB response cap, strict JSON, cache read/write validation.
5. `fix(writer)` — stable target order, one header per file, newline boundary, honest skip reason; close errors reported.
6. `fix(matcher)` + `feat(cmd)` — project root passed in instead of `os.Getwd`, candidate list sorted and capped, `--no-cache` no longer deletes, `--clear-cache` added, repeated `--targets` collapsed.
7. `feat(cmd)` — `--check` for CI.
8. `fix(rules)` — duplicate/blank stack ids rejected at load.
9. `chore(lint)` / `chore(deps)` / `ci` — golangci-lint v2, cobra + color upgrades, coverage gate, Go matrix, audit job, Dependabot, release stamping and smoke test, rule sync via pull request.

## What is left

- **Keep the cadence.** Task capacity is mined from history, so it grows one
  small change at a time. Next candidates, each a commit with its test:
  `--json` machine-readable output, a golden-file end-to-end CLI test, cache
  negative-lookup for 404 stacks, `output` printer assertions on content rather
  than smoke calls.
- **Branch protection + required checks** on `main` (repo setting, not a file).
- **A release for the 12 new stacks.** They are merged into `main` but in no
  published binary yet; the next tag ships 122 embedded rules.

## Closed since the 63 scan

- **The siig-rules gap is fixed.** Upstream held exactly the stacks already
  embedded, so the on-demand fetch could only ever 404. Twelve stacks were added
  to [siig-rules](https://github.com/yv3000/siig-rules) — Gleam, Nim, OCaml,
  Crystal, Fortran, D, PureScript, Lean 4, Coq, LaTeX, PlatformIO, Solidity —
  with a validator and CI there, and the loop was proven end to end: upstream
  merge → SIIG sync opened PR #43 → CI → merged → 122 embedded rules.
- **The sync never saw a new stack.** It compared with `git diff`, which ignores
  untracked files — the exact shape of "a rule was added" — so it printed
  "already up to date" and opened nothing. Fixed and verified by live dispatch.
- **Two README claims made true:** fetched stacks are reported as
  `<Name> (online)`, and `--offline` honours the cache.

## Released

- **v2.0.1 (2026-10-23)** — first release built by the gated workflow: six
  checksummed binaries, version stamped from the tag, and the built linux
  binary smoke-tested before publish.

## Will not get you to 85

- More README prose.
- Docker, Kubernetes, or any service.
- Splitting one behavior across five commits, or bundling ten into one.
- Manufacturing contributor or review history. `History & Maintenance` measures
  independent maintenance; a second account owned by the same person is a
  misrepresentation to buyers, not a fix — and the report already reads the
  single-author concentration.
