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

## Changes

- One behavior per commit, with tests that fail if that behavior breaks.
- Put tests in `*_test.go` next to the package (`matcher/matcher_test.go`, etc.).
- Stack rules live in [siig-rules](https://github.com/yv3000/siig-rules), not ad-hoc strings in Go.

## Checks before a PR

```
go test ./...
go vet ./...
```

CI runs the same plus golangci-lint.
