### Changed

- The embedded field registry lists the Genesis Mesh 1.4.0 records
  (`RemediationRecord`, `ReviewRecord` with its `AdminApproval`), the entry
  kinds `remediation` and `review`, and the boundary policy sections
  `remediation` (`GateRemediation` by gate), `review_required` and
  `emergency` (`EmergencyCapability`). This SDK does not verify evidence
  exports, so it has nothing to refuse there; the TypeScript and Rust SDKs
  record and verify remediations and reviews.
- `VerifyBoundaryDecision` accepts an expected policy with the new sections:
  they are known fields, signed and bound by the policy digest. Before 1.4.0
  such a policy was refused as `unknown_field`.
- `PolicyDigest` leaves the new sections out when they are `null`, as the
  reference does, so a policy without them keeps the digest it had under
  1.3.
