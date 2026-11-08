### Added

- **The operator key that asked is checked offline (1.10.0).** A Network
  Authority 1.10.0 names the key that asked for an attestation-basis decision
  in `attestation_binding.requester_key_id`, under the decision's signature.
  `VerifyBoundaryDecision` takes
  `DecisionVerifyOptions.ExpectedRequesterKeyID`: a different key fails with
  `attestation_binding_mismatch`, and a decision without the field (a 1.9.x
  NA) fails with `attestation_binding_missing` only when it is given. The
  canonical registry lists the field, omitted when absent, and the conformance
  vectors `bd-008` to `bd-010` cover it.

### Upgrading

- **Upgrade this SDK before, or with, a 1.10.0 Network Authority.** Earlier
  releases refuse a decision that names its requester as `unknown_field`.
