### Added

- **A pre-issued grant's NA signature, checked offline.**
  `VerifyCapabilityGrant` checks a Genesis Mesh 1.6.0 `CapabilityGrant`
  against the NA's key history (or `NAKeysValidAlways` for a plain list of
  NA keys): signed by the key valid at the grant's `issued_at`, with every
  signed field known and in canonical form. Its reasons are the existing
  `missing_signature`, `invalid_signature`, `unknown_field`,
  `non_canonical_form` and `na_key_not_valid_at_time`. It does not check the
  grant's validity, budget, parameter envelope or revocation.
- `NARecordTimeFields` places a grant and a time token at their `issued_at`
  and a grant revocation at its `revoked_at`.
- The SDK runs the signed-form and signature vectors of the shared `grants`
  suite, and the new record-time vectors of the `audit` suite.

### Changed

- The embedded field registry lists the Genesis Mesh 1.6.0 records
  (`CapabilityGrant`, `TimeToken`, `TimeTokenRequest`,
  `GrantExecutionEvidence`, `GrantRevocationRecord` and the models they
  nest) and the entry kinds `grant`, `grant_execution` and
  `grant_revocation`.
- The revocation feed's `revoked_grant_ids`, which the NA writes only while
  grants are on, is a known signed field: `UnknownFields` no longer reports
  it for a feed that lists revoked grants.
- Requesting grants, holding time tokens, acting within a grant offline,
  recording its uses and verifying grants in an evidence export are not part
  of this SDK; they live in the TypeScript and Rust SDKs.
