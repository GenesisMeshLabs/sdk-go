# Changelog

All notable changes to `sdk-go` are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versions align with the [Genesis Mesh release sequence](https://github.com/GenesisMeshLabs/genesismesh/blob/main/CHANGELOG.md).

---

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
