# Contributing to sdk-go

Thank you for your interest in contributing. This document covers how to set up
the development environment, run tests, and submit changes.

---

## Prerequisites

| Tool | Minimum version |
|------|----------------|
| Go | 1.22 |

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
go test -race ./...
```

---

## Project structure

Read [AGENT.md](AGENT.md) for the enforced layer rule before adding or changing
source files. The short version:

| File | What goes here |
|------|---------------|
| `genesismesh/auth.go` | Crypto only — canonical JSON, Ed25519 signing, admin headers |
| `genesismesh/transport.go` | HTTP transport only — do, adminPost, publicPost, publicGet |
| `genesismesh/errors.go` | Typed error types only |
| `genesismesh/types.go` | Protocol structs only — no methods, no functions |
| `genesismesh/{domain}.go` | Sub-client — thin wrapper over transport |

Do not mix layers. A sub-client file must not contain crypto primitives.
The auth module must not make HTTP calls.

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
3. Auth — admin methods require a signing key (`ErrNoSigningKey` without one)
4. Error — NA 4xx → typed SDK error with correct `.Code`

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
- Ensure `go build ./...` and `go test -race ./...` pass locally before opening the PR
- Fill in the PR template — the checklist exists for a reason

If your change adds a new NA constraint discovered during testing, add it to
the **Known constraints** table in `AGENT.md`.

---

## Smoke testing against a live NA

The unit tests use `httptest.NewServer`. To run against a real Network Authority:

```sh
cd ../sandbox/sdk-smoke-go
go run ./smoke.go   # requires NA on http://127.0.0.1:9443
```

---

## Reporting issues

Use the GitHub issue templates:

- **Bug report** — unexpected behaviour, wrong types, broken build
- **Feature request** — new sub-client method, new NA route support

For security vulnerabilities, follow the process in [SECURITY.md](SECURITY.md).
