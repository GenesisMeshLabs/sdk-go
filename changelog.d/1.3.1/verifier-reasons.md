### Fixed

- `VerifyBoundaryDecision` answers `missing_signature` only when the
  signature is absent or `null`, as the reference does. A signature of
  another shape (`{"sig": ""}`, a string, a list) is `invalid_signature`,
  and an expired decision with one is `decision_expired`. Likewise
  `VerifyDataAccessIntent` reports `Missing intent signature` only for an
  absent or `null` signature, and `Invalid intent signature` otherwise.
- A decision whose `decision_valid_until` does not parse is no longer an
  error before its signature and form are checked. It is refused as
  `invalid_signature`, `unknown_field` or `non_canonical_form` when one of
  them applies, as the reference refuses it; the parse error is returned
  only when none does, so such a decision is never accepted.
