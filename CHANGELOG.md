# Changelog

All notable changes to `sdk-go` are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versions align with the [Genesis Mesh release sequence](https://github.com/GenesisMeshLabs/genesismesh/blob/main/CHANGELOG.md).

---

## [1.0.1] - 2026-10-04

Coordinated Genesis Mesh v1.0.1 release: gateway console fixes. No changes in
this SDK.

## [1.0.0] - 2026-10-04

Coordinated Genesis Mesh v1.0.0 release: the public contract is stable for
the 1.x line and Genesis Mesh is ready for an independently operated pilot.
No functional changes in this SDK; its documented stable surface follows the
1.x compatibility rules.

## [0.65.0] - 2026-10-04

Coordinated Genesis Mesh v0.65.0 release: demo access and a guided tour in the
gateway, and the public reference federated with the live NA. No changes in
this SDK.

## [0.64.1] - 2026-10-03

Coordinated Genesis Mesh v0.64.1 release: the Network Authority republishes an
expiring CRL. No changes in this SDK.

## [0.64.0] - 2026-10-03

Coordinated Genesis Mesh v0.64.0 release: the Rust SDK and the Rust gateway
reach governed-action parity (boundary policies, evidence store, offline
verification). No changes in this SDK.

## [0.63.1] - 2026-10-02

Coordinated Genesis Mesh v0.63.1 release: configurable Network Authority rate
limits. No changes in this SDK.

## [0.63.0] - 2026-10-02

Coordinated Genesis Mesh v0.63.0 release: pilot readiness. No changes in
this SDK.

## [0.62.0] - 2026-10-02

Coordinated Genesis Mesh v0.62.0 release: the v1 public contract and security
review. No changes in this SDK. On the Network Authority, accepting an
agreement (`/admin/agreements/accept`) and creating a data license policy
(`/admin/data-usage/policy`) now require a privileged operator key; a
standard key gets `403 insufficient_operator_tier`.

## [0.61.1] - 2026-10-02

Coordinated Genesis Mesh v0.61.1 release.

### Fixed

- `ConsensusVote` and `ConsensusProof` now match the wire format (Python
  `ValidatorVote` and `ConsensusProof`). They previously used fields the NA
  never sends (`proposal_id`, `decision`, `threshold`, `assembled_at`), so
  decoded votes and proofs were empty and could not be passed back. A
  Python-signed proof from the `consensus` conformance vectors now round-trips
  to the same canonical JSON.
- `Consensus.Verify` returns `ConsensusVerification` (`valid`, `reason`,
  `consensus_id`).
- README consensus example uses the real request fields.

## [0.61.0] - 2026-10-02

Coordinated Genesis Mesh v0.61.0 release: the cross-language interoperability
proof. The core's interop scenario uses this SDK to verify Python Network
Authority records.

### Added

- Offline verifiers: `VerifyAgreement`, `VerifyBoundaryDecision` (signature,
  expiry, freshness proof, policy and attestation bindings),
  `VerifyDataLicensePolicySignature` and `VerifyDataAccessIntent`, with the
  reason codes of the Python reference. Helpers `CanonicalJSON`,
  `ParseTimestamp`, `PolicyDigest`, `AttestationDigest` and `PolicySetDigest`.
- The shared `interop` conformance vectors (25) in
  `genesismesh/testdata/conformance/interop.json`, all passing.

### Fixed

- Canonical JSON now matches Python's: non-ASCII text and DEL are escaped as
  `ensure_ascii` does, astral characters as surrogate pairs, floats use Python's
  repr, and integer literals are kept exactly at any size. Admin requests whose
  body held such values were rejected by the NA.

## [0.60.0] - 2026-10-01

Coordinated Genesis Mesh v0.60.0 release. No functional changes; the core adds
optional high availability for the Network Authority (PostgreSQL, several
instances behind a load balancer). This SDK keeps using one NA URL, which can
be the load balancer.

## [0.59.1] - 2026-10-01

Coordinated Genesis Mesh v0.59.1 release. No functional changes; the TypeScript
SDK adds attestation-backed evaluation, the boundary policy lifecycle and the
evidence store client. This SDK does not wrap them yet.

## [0.59.0] - 2026-10-01

Coordinated Genesis Mesh v0.59.0 release. No functional changes; the core adds
the Network Authority evidence store, which this SDK does not wrap yet.

## [0.58.1] - 2026-09-29

Coordinated Genesis Mesh v0.58.1 release. No functional changes; the core adds
attestation-backed boundary evaluation, which this SDK does not wrap yet.

## [0.58.0] - 2026-09-29

Coordinated Genesis Mesh v0.58.0 release. No functional changes; the SDK passes its
compatibility tests against the v0.58.0 Network Authority. Version 0.57 was skipped
across the train (see the core `docs/development/versioning.md`).

### Changed

- CI and publishing now fail if this component's version is ahead of the Genesis Mesh core version.

## [0.56.0] - 2026-09-01

### Changed

- Joined the coordinated Genesis Mesh v0.56.0 release train.
- Aligned canonical JSON, timestamps, signatures, request envelopes, and
  response types with the live Network Authority wire format.
- Updated agreement offer and acceptance types to match the current protocol.
- Added per-domain test coverage for all seven SDK clients.
- Added a shared `VERSION` declaration and publishing guard that rejects tags
  which do not match the declared module release.
- Updated the supported security line to `0.56.x`.

---

## [0.54.0] — 2026-06-29

### Added

- `Client` — unified entry point with 7 domain sub-clients over shared transport
- `AgreementClient` — capability offer, counter, accept, verify
- `BoundaryClient` — boundary decision and verification
- `EvidenceClient` — trust evidence build
- `AttestationClient` — membership attestation issue, revoke, recognition policy
- `DisclosureClient` — selective Merkle capability disclosure, nullifier, verify
- `ConsensusClient` — validator vote, consensus proof assembly and verify
- `DataUsageClient` — data license policy, access intent, get policy, verify
- `genesismesh/auth.go` — `canonicalJSON`, `LoadPrivateKey`, `BuildAdminHeaders` (Ed25519)
- `genesismesh/transport.go` — `adminPost`, `publicPost`, `publicGet`, typed error mapping
- `genesismesh/errors.go` — `GenesisMeshError` and typed subclasses for all NA error codes
- `genesismesh/types.go` — 30+ protocol structs matching the NA JSON wire format
- 19 tests across auth, errors, and all sub-client paths (`-race` clean)
- CI matrix: Go 1.22 and 1.23

[0.56.0]: https://github.com/GenesisMeshLabs/sdk-go/compare/v0.54.0...v0.56.0
[0.54.0]: https://github.com/GenesisMeshLabs/sdk-go/releases/tag/v0.54.0
