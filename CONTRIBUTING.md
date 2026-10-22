# Contributing to sdk-go

Thank you for your interest in contributing. This document covers how to set up
the development environment, run tests, and submit changes.

---

## Prerequisites

| Tool | Minimum version |
|------|----------------|
| Go | 1.22 (the `go.mod` minimum; CI also tests the two latest Go releases) |
| Python | 3.12, for `scripts/` and the live contract test |

---

## Set up

```sh
git clone https://github.com/GenesisMeshLabs/sdk-go.git
cd sdk-go
go mod download
```

Verify the build and tests pass before making any changes:

```sh
go build ./...
go vet ./...
test -z "$(gofmt -l .)"
go test -race ./...
```

---

## Project structure

[docs/architecture.md](docs/architecture.md) maps every source file to its
layer, shows the direction layers may depend in (enforced by
`TestArchitectureLayers`), and has recipes for the common changes: a client
method, a response field, a record kind from a core release, an error
mapping, an API change. Each recipe lists the files to touch and the test
that fails if one is missed. [AGENT.md](AGENT.md) has the conventions.

Do not mix layers. A sub-client file must not contain crypto primitives, and
the verifiers and the auth module must not make HTTP calls.

---

## Making changes

### Branching

Branch from `main`. Use the pattern `{type}/{short-description}`:

```
feat/add-consensus-threshold-param
fix/datasource-required-fields
docs/update-raw-admin-examples
```

### Commit messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(attestation): add optional subject_public_key param
fix(errors): handle nested NA error format correctly
docs(readme): correct evidence verdict values
test(consensus): add negative test for threshold below vote count
chore(deps): update uuid to v1.7
```

Scope is the primary area: `auth`, `transport`, `types`, `agreement`,
`boundary`, `evidence`, `attestation`, `disclosure`, `consensus`,
`data-usage`, `sdk`, `ci`, `docs`.

### Code style

- All exported types use Go naming conventions (PascalCase fields with JSON snake_case tags)
- JSON struct tags must match the NA wire format exactly — never omit them
- No `fmt.Println` in `genesismesh/` source
- No global state

### Tests

Every public method needs tests in `genesismesh/client_test.go` (or a dedicated
`{domain}_test.go` if the file grows large):

1. Happy-path — `httptest.NewServer` returns the correct shape
2. URL — method calls the correct route path
3. Auth — admin methods return an error without a signing key ("signing key
   required for admin route")
4. Error — an NA error status → its typed SDK error with the correct `.Code`
   (use a status and code the NA really sends)

Use `httptest.NewServer` from the stdlib — no mocking library needed:

```go
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    json.NewEncoder(w).Encode(MyResponseType{Field: "value"})
}))
defer srv.Close()
```

Run the full suite with race detection:

```sh
go test -race ./...
```

A bug fix comes with the test that fails without it: run the test against the
code before your fix and see it fail. CI's `Regression test` check fails a
pull request titled `fix: ...` that changes source files without changing a
`*_test.go` file, unless the Maintainer labels it `no-test-needed`.

CI also holds these ratchets; each fails a pull request that makes things worse:

- `TestArchitectureLayers` (in `go test`): the layer table in
  [docs/architecture.md](docs/architecture.md). A new source file needs a
  layer, and the offline verifier may not use the HTTP client half.
- Coverage floors (`scripts/coverage_floor.sh`): the package and the offline
  verifier files may not fall below the floors in `.github/workflows/ci.yml`.
- `Fuzz`: every `Fuzz*` target in `genesismesh/fuzz_test.go` runs for a few
  seconds. A failing input the fuzzer writes to `genesismesh/testdata/fuzz/`
  is committed with the fix, as a regression test.
- `API compatibility`: an incompatible change to the Go API against the
  latest release tag needs the label `breaking` (the Maintainer's call) and an
  `### Upgrading` section in its changelog fragment. Run it locally with
  `scripts/api_compat.sh origin/main HEAD` (needs `apidiff`).
- `Vulnerabilities`: `govulncheck ./...`.

After copying a new `field_registry.json` from the core, run
`python scripts/sync_canonical_registry.py`; a test fails while the embedded
registry differs from the suite.

---

## NA protocol constraints

Before adding a new sub-client method, check the constraints table in
[AGENT.md](AGENT.md). Common traps:

- Evidence `verdict` must be `"allow" | "block" | "escalate" | "warn"`
- Roles must use a `role:` prefix (`role:client`, `role:anchor`, etc.)
- `DataSourceDescriptor` requires `source_id`, `source_type`, and `owner_sovereign_id`
- Agreement `accept` requires the NA to hold a prior recognition treaty

---

## Pull requests

- Keep PRs focused — one feature or fix per PR
- Include a test for every changed behaviour
- Ensure `go build ./...`, `go vet ./...`, `gofmt -l .` and `go test -race ./...`
  pass locally before opening the PR
- Add a changelog fragment `changelog.d/<version>/<short-name>.md` (see
  [changelog.d/README.md](changelog.d/README.md)); never edit `CHANGELOG.md`
- A change callers must act on gets an `### Upgrading` section in its fragment
- Fill in the PR template — the checklist exists for a reason

If your change adds a new NA constraint discovered during testing, add it to
the **Known constraints** table in `AGENT.md`.

---

## Live contract test against a real NA

The unit tests use `httptest.NewServer`. The live contract test starts a
disposable Network Authority (`scripts/contract_na.py`, which needs the
`genesis-mesh` core installed in the Python you name) and checks every client
method's typed result against the JSON the NA sent:

```sh
pip install -e ../genesismesh
GM_E2E_PYTHON=python go test -tags live -count=1 -run '^TestLiveContract$' ./genesismesh
```

Without `GM_E2E_PYTHON` the test skips; with `GM_E2E_REQUIRED=1` (as in CI) it
fails instead.

---

## Reporting issues

Use the GitHub issue templates:

- **Bug report** — unexpected behaviour, wrong types, broken build
- **Feature request** — new sub-client method, new NA route support

For security vulnerabilities, follow the process in [SECURITY.md](SECURITY.md).
