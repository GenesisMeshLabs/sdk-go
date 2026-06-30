# AGENT.md — sdk-go

Guidance for AI coding agents and human contributors working inside the
Genesis Mesh Go SDK.

This SDK is a standalone module. It does **not** import from the Python main
repo. It wraps the NA HTTP API surface documented in:

- `genesismesh/docs/sdk/go.md` — public reference
- `genesismesh/docs/api/trust-http.md` — NA HTTP routes

---

## Repo layout

```text
sdk-go/
  genesismesh/
    auth.go        # canonicalJSON, LoadPrivateKey, BuildAdminHeaders (Ed25519)
    transport.go   # HTTP transport — do, adminPost, publicPost, publicGet
    errors.go      # GenesisMeshError + typed subclasses, parseErrorResponse
    types.go       # Protocol structs (snake_case JSON tags, matching Pydantic models)
    client.go      # Client entry point — 7 sub-clients over shared transport
    agreement.go   # AgreementClient
    attestation.go # AttestationClient
    boundary.go    # BoundaryClient
    consensus.go   # ConsensusClient
    data_usage.go  # DataUsageClient
    disclosure.go  # DisclosureClient
    evidence.go    # EvidenceClient
    auth_test.go
    errors_test.go
    client_test.go
  go.mod
  go.sum
```

---

## Layer rule

Mirror the Python main repo's enforced layer separation:

```
genesismesh/auth.go       = Pure crypto: canonicalJSON, LoadPrivateKey, BuildAdminHeaders.
                            No HTTP. No domain knowledge.
                            Python equivalent: genesis_mesh/crypto/

genesismesh/transport.go  = HTTP transport only: do, adminPost, publicPost, publicGet.
                            No signing logic inline. No domain knowledge.
                            Python equivalent: na_service/ (transport layer)

genesismesh/errors.go     = Typed error types only.
                            No domain logic. No HTTP calls.

genesismesh/types.go      = Protocol structs only.
                            No methods beyond String(). No functions. Pure data.
                            Python equivalent: genesis_mesh/models/

genesismesh/{domain}.go   = Sub-client: thin wrapper over transport.
                            One file per domain. Methods call adminPost/publicPost/publicGet.
                            No signing logic inline. No URL construction beyond the path.
                            Python equivalent: na_service/routes/
```

**Do not mix layers.** If a sub-client needs to sign something directly, it
belongs in `auth.go`. If a sub-client is doing HTTP retry logic, it belongs in
`transport.go`.

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

Unknown HTTP status codes fall through to `GenesisMeshError` with the
raw status — never swallow them.

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

This SDK follows the same release process as the main Python repo. Every
version shipped must:

1. Pass `go build ./...`, `go vet ./...`, and `go test -race ./...`.
2. Have a CHANGELOG entry in the main repo's `CHANGELOG.md`.
3. Have its plan marked `[x]` in `genesismesh/ops/plan-vX.Y.Z.md`.
4. Be committed, tagged `vX.Y.Z`, pushed, and have a GitHub release created.
5. Have `genesismesh/docs/development/history.md` updated with a narrative entry.

Go modules are published by git tag on a public repo. Users install with:

```sh
go get github.com/GenesisMeshLabs/sdk-go@vX.Y.Z
```

The `pkg.go.dev` index auto-discovers the module within minutes of the tag push.

---

## Agent behavior rules

When acting as an AI coding agent in this repository:

1. Read this file before making changes.
2. Identify which Phase 2 goal the change supports (external operator onboarding,
   Atlas integration, RFC ratification, conformance validation).
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
10. To validate Go SDK output against the protocol reference, run the
    conformance test harness at `genesismesh/conformance/` in the main repo:
    `python conformance/runner.py --sdk go` after generating vectors with
    `python conformance/generate_vectors.py`. The vectors define the canonical
    wire format that all SDK implementations must match.
