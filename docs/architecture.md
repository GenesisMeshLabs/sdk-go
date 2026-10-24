# Architecture

The Go SDK is one package, `genesismesh` (module
`github.com/GenesisMeshLabs/sdk-go`), with two halves:

- an **offline verifier** of the records a Network Authority (NA) signs:
  decisions, agreements, policies, intents, the NA key history and grant
  signatures. It is about two thirds of the code and makes no network calls.
- a **thin HTTP client** for the NA routes listed in the README.

It is the "verifier plus thin client" tier of the core's decision 0002 (SDK
capability tiers): the governed lifecycle (outbox, grants holder, remediation,
audit packs) lives in the TypeScript and Rust SDKs. The README's Scope section
says what is and is not covered.

## Package map

| File | Layer | What it holds |
|---|---|---|
| `strict_json.go` | json | `CheckStrictJSON`: refuses JSON parsers read differently (duplicate keys, lone surrogates, -0, out-of-range numbers) |
| `canonical.go` | json | `CanonicalJSON` and `decodeJSON`: the canonical form, byte-identical to the Python reference |
| `strict.go` | registry | the embedded field registry (`canonical_registry.json`), `UnknownFields`, canonical timestamps and forms |
| `verify.go` | verifiers | `VerifyBoundaryDecision`, `VerifyAgreement`, data-usage policy and intent checks, shared record helpers |
| `key_history.go` | verifiers | `VerifyKeyHistory`, `NAKeyHistory`, `NARecordTimeFields` |
| `grants.go` | verifiers | `VerifyCapabilityGrant`, `VerifyCapabilityGrantAt` (1.8.0) |
| `vouching.go` | verifiers | `NARecordRefusal`, `NAVouchingTimeFields` (1.8.0) |
| `auth.go` | signing | `LoadPrivateKey`, `AdminSigningPayload`, `BuildAdminHeaders` (Ed25519) |
| `types.go` | protocol | wire structs (snake_case JSON tags) and their `UnmarshalJSON` methods |
| `errors.go` | protocol | typed API errors (`APIError`, one type per status class), `parseErrorResponse` |
| `transport.go` | transport | `ClientOptions`, HTTP, strict response decoding, admin headers, error mapping |
| `agreement.go`, `attestation.go`, `boundary.go`, `consensus.go`, `data_usage.go`, `disclosure.go`, `evidence.go` | sub-clients | one thin wrapper per NA route domain |
| `client.go` | entry | `Client` and `NewClient` |
| `testdata/conformance/` | (data) | vector suites copied unchanged from the core's `conformance/vectors/` |
| `testdata/contract/na_responses.json` | (data) | NA responses captured by the live contract test |

`scripts/` holds `sync_canonical_registry.py` (regenerates the embedded
registry), `contract_na.py` (a disposable NA for the live test),
`changelog.py` (fragments), `api_compat.sh` and `coverage_floor.sh` (CI
ratchets).

## Layers

A file may use what its own layer declares and what the layers in its "may
use" column declare, nothing else. `TestArchitectureLayers`
(`genesismesh/architecture_test.go`) type-checks the package and enforces
this table, which it holds in the same form; change both together. Every
source file must have a layer, and the offline layers (json, registry,
verifiers, signing) import no `net` package.

| Layer | Files | May use |
|---|---|---|
| json | `strict_json.go`, `canonical.go` | (nothing) |
| registry | `strict.go` | json |
| verifiers | `verify.go`, `key_history.go`, `grants.go`, `vouching.go` | json, registry |
| signing | `auth.go` | json |
| protocol | `types.go`, `errors.go` | (nothing) |
| transport | `transport.go` | json, signing, protocol |
| sub-clients | `agreement.go`, `attestation.go`, `boundary.go`, `consensus.go`, `data_usage.go`, `disclosure.go`, `evidence.go` | protocol, transport |
| entry | `client.go` | transport, sub-clients |

So the verifier half (json, registry, verifiers) never reaches the client
half (protocol, transport, sub-clients, entry), and the client reuses only
the json layer (strict decoding) and signing.

Today's one exception is allowlisted in the test: `strict.go` uses the
`object` type alias declared in `verify.go`. The allowlist only shrinks: a
new violation fails, and so does an entry that no longer occurs.

## Data flow

**A client call.** `client.Boundary.Decide(ctx, body)` calls
`transport.adminPost`, which reads the NA's public key once from
`/sovereign.json` (the signature's audience), signs method, path, body,
audience, timestamp and nonce with `BuildAdminHeaders`, and sends the request.
Public routes go through `publicPost` or `publicGet` without headers. A
status of 400 or more becomes a typed error (`parseErrorResponse`: one type
for 400, 401, 403, 404, 409, 413, 422 and 429, `ClientError` for any other
4xx, `ServerError` only for 5xx). A success body must be strict JSON; keys
that `encoding/json` would match to a field only by ignoring case are dropped;
then it is decoded into the typed result.

**An offline verification.** `VerifyBoundaryDecision(raw, opts)` decodes the
record with `decodeObject` (strict JSON, number literals kept), then checks in
the reference's order: signature present, not expired, Ed25519 signature over
the canonical form of the signed projection (by the operator keys, or by the
key history's keys), no signed field the registry does not define
(`unknown_field`), the form the reference writes (`non_canonical_form`), the
key valid at the decision's time, an embedded freshness proof, and the
expected policy and attestation bindings. The result is `Accepted` with a `Reason`; the other verifiers follow
the same pattern.

## Recipes: where a change goes

Each recipe lists the files to touch and the test that fails if one is
missed. Every change also adds a fragment `changelog.d/<version>/<name>.md`
(the `Changelog fragments` check), and a change callers must act on gets an
`### Upgrading` section.

### A client method for an NA route

1. `genesismesh/<domain>.go`: the method, calling `c.t.adminPost`,
   `publicPost` or `publicGet` with the route's path. A new domain is a new
   file in the sub-clients layer, a field on `Client` and a line in
   `NewClient` (`client.go`), and a row in the layer table here and in
   `architecture_test.go` (`TestArchitectureLayers` fails without it).
2. `genesismesh/types.go`: the response struct, with the NA's snake_case keys.
3. `genesismesh/<domain>_test.go`: an `httptest` test for the path, the admin
   headers and a typed error.
4. `genesismesh/live_contract_test.go`: a call and `check(...)`, so
   `TestLiveContract` compares the typed result with the JSON a real NA sends
   (it fails on any key the struct does not model or any field the NA does not
   send). Nothing fails if this step is missed: reviewers check it.
5. README: the method under its sub-client.

### A field in an NA response

1. `genesismesh/types.go`: the field.
2. `genesismesh/testdata/contract/na_responses.json`: refresh it with
   `GM_CONTRACT_FIXTURE=<path>` on the live test.
   `TestResponseTypesModelTheNAResponses` fails while the fixture and the
   struct disagree, and `TestLiveContract` fails against a live NA.

### A record kind or verifier from a core release

1. Copy the core's `conformance/vectors/<suite>.json` and
   `field_registry.json` into `genesismesh/testdata/conformance/` unchanged,
   then run `python scripts/sync_canonical_registry.py`.
   `TestConformanceFieldRegistryIsEmbedded` fails until the embedded registry
   matches the suite.
2. The verifier in `verify.go`, `key_history.go`, `grants.go`, `vouching.go` or a new file of
   the verifiers layer (and `TestArchitectureLayers`' table). Model names are
   string literals: `TestEveryModelNameInTheSourceIsInTheRegistry` fails for a
   name the registry lacks. Hand tables copied from the registry are checked
   by `TestCanonicalRulesEqualTheRegistrys`. Add a new record time field to
   `NARecordTimeFields` (`TestRecordTimeFieldsAreTimestamps` checks that each
   entry is a timestamp of the registry; nothing fails if one is missing).
3. The suite's test: run every vector kind, or list it in the suite's
   `...KindsNotRun` map. A kind neither run nor listed fails
   (`TestConformanceGrantsVectors`, `TestConformanceAuditVectors`).
4. README: the Scope section and a section for the verifier.
5. The same change in sdk-dotnet, and the core's `interop.yml` compares the
   copied suite with the core's.

### A typed error or status mapping

`genesismesh/errors.go` (`parseErrorResponse`) and `errors_test.go` with a
status and code the NA really sends. `TestOnlyA5xxIsAServerError` and
`TestEveryNAErrorIsAnAPIError` fail if a 4xx becomes a `ServerError` or a
type does not implement `APIError`.

### A change to the public API

Additions are free. A removal or a changed signature, field type or method
set fails the `API compatibility` check (`scripts/api_compat.sh`, apidiff
against the latest release tag) unless the Maintainer labels the pull request
`breaking` and its fragment has an `### Upgrading` section. A Go module's
major version is part of its import path, so a break in 1.x is a deliberate
decision.

### A change to strict JSON or the canonical form

`strict_json.go` or `canonical.go`, checked by the canonical suite
(`TestConformanceCanonicalVectors`) and the fuzz targets in `fuzz_test.go`
(`go test` runs their seeds; CI's `Fuzz` job fuzzes each for a few seconds).
A failing input the fuzzer writes to `genesismesh/testdata/fuzz/` is committed
with the fix.
