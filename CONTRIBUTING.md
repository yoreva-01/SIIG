# Contributing

SIIG is a single-binary Go CLI. Keep it that way: no Docker, no extra services.

## Setup

Needs Go 1.21+.

```
git clone https://github.com/yv3000/SIIG.git
cd SIIG
go test ./...
go build -o siig .
```

## Checks before a PR

```
go test ./... -covermode=atomic -coverprofile=coverage.out
go tool cover -func=coverage.out | tail -1   # total must stay >= 80%
go vet ./...
golangci-lint run ./...                       # v2 config, standard linters + gofmt
```

CI runs the same set, on Go 1.21 and current stable, plus `govulncheck`.

## Commits

One behavior change per commit, together with the test that fails if that
behavior breaks. Formatting-only churn, refactors and features do not belong in
the same commit.

Message format is `type(scope): subject`, where type is one of `feat`, `fix`,
`test`, `refactor`, `docs`, `chore`, `ci` and scope is the package or area
(`scanner`, `writer`, `deps`, `release`…). The body says *why* — what was wrong
before and what the change guarantees now.

```
fix(scanner): stop detecting Express from a nested index.js

feat(cmd): add --check so CI can enforce current ignore files
```

Tests go next to the package they cover (`matcher/matcher_test.go`) and use
`t.TempDir()` or an injected `fs.FS` — never the real home directory or a real
project. Tests must not need the network: the fetch path is driven through
`httptest`, and the online fallback through a replaceable `fetchStack`.

## Stack rules

Rules live in [siig-rules](https://github.com/yv3000/siig-rules), not in ad-hoc
strings in Go, and are embedded into the binary from `rules/db/`.

Two detection pitfalls the scanner enforces, so a new rule cannot reintroduce
them:

- Entry-point names (`index.js`, `app.py`, `server.ts`, …) only count at the
  project root. A nested one is not evidence of a framework.
- Names SIIG itself writes (`.gitignore`, `.vercelignore`, `.dockerignore`,
  `.npmignore`) are not detection input when the file already carries the
  `# Added by SIIG` marker — otherwise a second run would detect its own output.

Remote rules are additionally capped and validated (`rules.Validate`): the id
must match the requested stack, and no entry may be empty, longer than 256
characters, or contain a newline.

## Reproducing CI without a container

The toolchain is three pinned tools and nothing else — no Dockerfile or dev
container is needed to work on SIIG, and adding one is out of scope for a
single-binary CLI.

| Tool | Version CI uses | Needed for |
|---|---|---|
| Go | 1.21 and current stable (matrix) | everything |
| golangci-lint | v2.15.0 | the `lint` job |
| actionlint | any recent release | checking `.github/workflows/*` locally |
| govulncheck | latest | the `audit` job |

```
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

Then run the commands under "Checks before a PR" above; a green local run means
a green CI run, because CI runs the same set.

## Dependencies

Two direct runtime dependencies (`cobra`, `fatih/color`) and nothing else.
Dependabot opens PRs weekly for `gomod` and `github-actions`, with
`chore(deps)` / `chore(ci)` commit prefixes; a bump lands as its own commit so
the history shows what was refreshed and when.

The module's `go` directive stays at **1.21** — that is the documented minimum
for building from source. A dependency update that would raise it is declined
rather than merged (this is why `fatih/color` v1.19.0 was not taken). Raising
the floor is a deliberate decision, not a side effect of a version bump.

To refresh by hand:

```
go get -u ./... && go mod tidy
```

then run the full gate above and note the coverage figure in the pull request.

## Review, and the bus factor

This project has **one active maintainer**, who today writes, reviews and merges
almost everything. That is the largest maintenance risk here: reviews, releases
and the release tag all sit behind one person, and a long absence stops the
project.

What reduces it, and what does not:

- **CI is the review floor.** Coverage, `go vet`, golangci-lint and govulncheck
  gate every pull request, so a change cannot merge on approval alone. A second
  reviewer would be a human nicety, not a safety requirement.
- **Rules are a separate repo.** [siig-rules](https://github.com/yv3000/siig-rules)
  takes a stack contribution as one JSON file plus `go run ./cmd/validate` — no
  Go knowledge and no build access required, and it has its own CI. That is the
  lowest-friction way to contribute, and the easiest place to become a regular
  reviewer.
- **Nothing merges itself.** The weekly rules sync opens a pull request; it does
  not push to `main` or cut a tag. A release happens when a maintainer pushes a
  tag. An unattended repo therefore stalls, it does not drift silently.

### How a pull request is handled

1. CI must be green — that is checked before a human looks.
2. The commit message is read first: if it does not say *why*, the change goes
   back rather than being described for its author.
3. Merges use a merge commit, not a squash, so a change and the test that pins
   it stay attributable.
4. If a pull request sits unreviewed for two weeks, ping it on the thread.
   Stale is not the same as rejected.

### Good first contributions

- A stack rule in siig-rules — one JSON file, validated by CI, no Go required.
- A failing test for a detection false positive you hit in a real project.
- Docs that describe behaviour the code does not have. Those are bug reports, and
  they get fixed before new prose is added.
