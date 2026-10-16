## What changed

<!-- What this pull request changes and why, in 2-4 sentences. Title it with a
conventional prefix (feat:, fix:, docs:, test:, ci:, chore:). -->

## How it was tested

<!-- Commands you ran and what they showed: go build ./..., go vet ./...,
gofmt -l ., go test -race ./..., and the live contract test if you changed a
client method (see CONTRIBUTING.md). -->

## Bug fix

Bug fix: the test that fails without this change is ___

<!-- Name the test and say how you saw it fail without the fix. A pull request
titled "fix: ..." that changes source files must change a test file, or carry
the label no-test-needed, which only the Maintainer adds. Write "not a bug
fix" otherwise. -->

## Changelog

- [ ] A fragment `changelog.d/<version>/<short-name>.md` describes the change
      for users (see `changelog.d/README.md`), or the change needs none
      (CI, tests, repository docs).
- [ ] `CHANGELOG.md` is not edited (only a `release/<version>` pull request
      folds the fragments into it).

## Breaking change

- [ ] No public API or behaviour changes for callers.
- [ ] Callers see a change: the fragment has an `### Upgrading` section that
      says what changes for them and what to do.

## Flagged paths

These need the Maintainer's review and merge. Tick any this pull request touches:

- [ ] Contract: the public API (`client.go`, `types.go`, `errors.go`) or the
      wire format of a request or response
- [ ] Vectors and registry: `genesismesh/testdata/`, `canonical_registry.json`
- [ ] Signing and verification: `auth.go`, `transport.go`, `strict*.go`,
      `canonical.go`, `verify.go`, `key_history.go`, `grants.go`
- [ ] Workflows: `.github/`
- [ ] `SECURITY.md`
- [ ] Packaging and release: `go.mod`, `go.sum`, `VERSION`

## Related issues

<!-- Closes #, Fixes #, or "None". -->
