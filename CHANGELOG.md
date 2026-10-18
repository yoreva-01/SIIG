# Changelog

## Unreleased

- Detect npm libraries from root `package.json` `main` / `exports` so `.npmignore` can apply.
- Reject unknown `--targets` values and unsafe remote stack IDs.
- Cobra commands return errors instead of calling `os.Exit` from `Run`.
- Automated tests for matcher, writer, scanner, cache, rules, and fetcher validation.
- CI runs `go test`, `go vet`, and golangci-lint on push and pull requests.

## 2.0.0

- Embedded 110-stack rule DB, online fetch fallback, local cache, and ignore-file writer.
