### Added

- **`VerifyBoundaryDecision` checks an expected attestation or subject.**
  `DecisionVerifyOptions.ExpectedAttestationID` and `ExpectedSubjectID`, when
  not empty, must equal the decision's signed `attestation_binding`. Another
  attestation or subject fails with `attestation_binding_mismatch`, and a
  decision without an attestation binding with `attestation_binding_missing`.
  A relying party that does not hold the attestation can now check that a
  decision was made for the subject it expects. From 1.9.2 the Network
  Authority makes attestation-basis decisions only for the attestation's
  subject (core 1.9.2).
