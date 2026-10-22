### Fixed

- **`RecognitionPolicy` and `RecognizedIssuer` are the NA's wire format.**
  `RecognizedIssuer` wrote `issuer_sovereign_id` and had no public keys, so
  the NA refused every recognition policy built from these types with 422
  `invalid_recognition_policy`. It now writes `sovereign_id` and
  `public_keys` (required, at least one), with `allowed_roles` and
  `accepted_statuses`; `RecognitionPolicy` gains `revoked_attestation_ids`.
  Its doc comment no longer calls it the request body: pass it to
  `Attestation.SavePolicy` as `{"recognition_policy": policy}`.

### Upgrading

- `RecognizedIssuer.IssuerSovereignID` is deprecated: set `SovereignID` and
  `PublicKeys`. A value left in `IssuerSovereignID` is still sent, as
  `sovereign_id`, when `SovereignID` is empty.
