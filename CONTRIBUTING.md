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
