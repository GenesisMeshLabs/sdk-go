# AGENT.md — sdk-go

Guidance for AI coding agents and human contributors working inside the
Genesis Mesh Go SDK.

This SDK is a standalone module. It does **not** import from the Python main
repo. It has two halves:

- an **offline verifier** (about two thirds of the code) of the records an NA
  signs (decisions, agreements, policies, intents, the NA key history, grant
  signatures), checked against conformance vectors copied from the core;
- a **thin client** for the NA HTTP routes listed in the README.

References in the core repository (`genesismesh`):

- `README.md` of this repo — the SDK's reference and its scope
- `docs/api/trust-http.md` — NA HTTP routes
- `contract/public-surface.json` — routes, error codes and stability

---

## Repo layout

```text
sdk-go/
  genesismesh/                # one package: the offline verifier and the HTTP client
    testdata/conformance/     # vector suites copied from the core's conformance/vectors
    testdata/contract/        # NA response fixture for contract_types_test.go
  docs/architecture.md        # file map, layers, data flow, "where a change goes"
  scripts/                    # registry sync, live-test NA, changelog fragments, CI ratchets
  changelog.d/                # fragments for the next version
  go.mod  go.sum  VERSION
```

[docs/architecture.md](docs/architecture.md) is the map: one line per file,
the layers and the direction they may depend in, how a call and a
verification flow, and recipes listing the files a change touches and the
test that fails if one is missed. Read it before adding or moving code.

---

## Layer rule

The layer table in [docs/architecture.md](docs/architecture.md) is enforced
by `TestArchitectureLayers` (`genesismesh/architecture_test.go`): a file may
use only its own layer and the layers listed for it, every source file must
have a layer, and the offline verifier (json, registry, verifiers) never
uses the HTTP client half and imports no `net` package.

**Do not mix layers.** If a sub-client needs to sign something directly, it
belongs in `auth.go`. If a sub-client is doing HTTP retry logic, it belongs in
`transport.go`. The verifier files make no HTTP calls, and the sub-clients do
no verification.

---

## Architectural principles

### 1. Near-zero dependencies

The SDK uses only the Go stdlib plus `github.com/google/uuid` (for nonce
generation). Do not introduce HTTP client libraries, JSON libraries, or utility
packages.

### 2. Field names follow the wire format exactly

All `types.go` structs have JSON struct tags using snake_case, matching the
NA's JSON API exactly. Go field names follow PascalCase convention but the
wire representation is snake_case.

The reason: the same protocol models ship in Python, TypeScript, and C#. A
shared wire naming convention makes cross-language debugging tractable.

### 3. Security-sensitive code stays boring

`auth.go` is the most security-critical file. Keep it minimal and explicit:

- Do not add caching or memoization to key operations.
- Do not add fallbacks for unsupported key formats.
- Do not silently swallow signing errors.
- `canonicalJSON` must produce output identical to Python's
  `json.dumps(sort_keys=True, separators=(",",":"))`.

### 4. Errors fail closed

`parseErrorResponse` must handle the NA's nested error format:
`{ "error": { "message": "...", "code": "..." } }`. If the format changes,
**return an error**, do not silently produce a misleading result.

Every status maps to a typed error that is an `APIError` (v1.7.1): 400, 401,
403, 404, 409, 413, 422 and 429 have their own types, any other 4xx is a
`ClientError`, and only a 5xx is a `ServerError` (a caller may retry it).
Never swallow a status, and never map a 4xx to `ServerError`.

A model name the field registry does not define fails closed (v1.7.1):
`UnknownFields` returns an error and the strict checks refuse the record.
`TestEveryModelNameInTheSourceIsInTheRegistry` checks every model literal.

### 5. Admin route invariant

The NA constructs and signs protocol artifacts from declared intent.
The SDK must not pre-build a model client-side and ask the NA to sign it.
Sub-client methods send parameters, not pre-built signed models.

---

## Known constraints (learned from smoke testing)

These are non-obvious and not in the HTTP reference. Tests must cover them.

| Constraint | Detail |
|-----------|--------|
| Evidence verdict | Must be `"allow"` \| `"block"` \| `"escalate"` \| `"warn"`. The value `"trusted"` is invalid. |
| Role prefixes | Roles must start with `role:anchor`, `role:bridge`, `role:client`, `role:operator`, or `role:service:<name>`. Bare names return 422. |
| Agreement accept | Requires the NA to hold an active recognition treaty for the `responder_sovereign_id`. Issue it via `POST /admin/recognition-treaties` first. |
| `DataSourceDescriptor` | `source_id`, `source_type` (`"personal"` \| `"proprietary"` \| `"public"` \| `"synthetic"`), and `owner_sovereign_id` are all required. Missing any returns HTTP 422. |

---

## Development environment

**This is a Windows project.** Development is on Windows 11 / PowerShell.
CI runs on Linux. Code must work on both.

- Use `go test -race ./...` to check for data races.
- Use `go vet ./...` for static analysis before committing.
- `go mod tidy` after any dependency change.

---

## Pre-commit equivalent

There is no pre-commit framework in this repo. Run these manually before
every commit:

```sh
go build ./...  # must exit 0
go vet ./...    # must exit 0
go test -race ./...  # all tests must pass
```

---

## Testing requirements

Use `httptest.NewServer` from the stdlib. No mocking library.

Every public method must have:

- A happy-path test that returns the expected struct shape.
- A test that admin methods require a signing key (error without one).
- A negative test: the method maps a 4xx response to the correct typed error.

---

## Coding standards

- Go 1.22+ — use `errors.As` and `errors.Is` for error unwrapping.
- No `fmt.Println` in `genesismesh/` source. Return errors, never log.
- Do not add comments explaining what the code does. Only add comments for
  non-obvious invariants.
- All exported symbols must have godoc comments.

---

## Release process

The core and the official SDKs use one coordinated version, and the core
releases first (CI's `release-train` check). The `VERSION` file and the
release tag must match.

1. Every pull request adds a fragment `changelog.d/<version>/<name>.md`
   (see `changelog.d/README.md`); none edits `CHANGELOG.md`.
2. The release pull request (branch `release/<version>`) sets `VERSION` and
   folds the fragments with `python scripts/changelog.py release <version>
   --heading "## [<version>] - <date>"`.
3. After the core's tag exists, the Maintainer tags `v<version>`; the
   `Publish` workflow checks the tag and creates the GitHub release from the
   version's `CHANGELOG.md` section.

Go modules are published by git tag on a public repo. Users install with:

```sh
go get github.com/GenesisMeshLabs/sdk-go@vX.Y.Z
```

The `pkg.go.dev` index auto-discovers the module within minutes of the tag push.

---

## Agent behavior rules

When acting as an AI coding agent in this repository:

1. Read this file before making changes.
2. Say which part the change touches: the offline verifier or the HTTP client.
3. Keep changes small. One method → one test. One sub-client → one test file.
4. Preserve layer boundaries. No signing logic in sub-clients. No domain
   knowledge in `transport.go`.
5. Match the NA's wire format exactly — JSON tags must be snake_case.
6. Do not introduce runtime dependencies.
7. Do not add methods that the NA HTTP API does not expose.
8. When a NA constraint is discovered (invalid enum value, missing required
   field, prerequisite call), add it to the "Known constraints" table in this
   file AND cover it with a negative test.
9. Confirm before destructive operations. Approval once does not generalize.
10. The conformance vectors define the canonical wire format every
    implementation must match. They are copied from the core's
    `conformance/vectors/` into `genesismesh/testdata/conformance/` and run by
    `go test ./...`; after copying `field_registry.json`, run
    `python scripts/sync_canonical_registry.py`. A vector kind the tests
    neither run nor list as not run fails a test.
