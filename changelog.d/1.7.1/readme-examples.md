### Fixed

- **README examples that compile and that a governed NA accepts.** The
  quick start called `Boundary.Decide` with fields the NA does not read, and
  a Network Authority that requires boundary policies refuses `Decide` with
  409 `boundary_policy_required`. It now issues an attestation with the
  privileged key and asks for a policy-aware decision
  (`POST /admin/boundary/evaluate`) with the controller's standard key,
  through a signed raw admin call. The Agreement, Attestation, Disclosure and
  DataUsage examples passed typed structs where the methods take maps, or
  left out fields the NA requires; they now use the bodies the NA accepts.
  The README says which methods need a privileged key, that `Accept` takes an
  offer only, how to accept a counter-offer, and how to call `evaluate`.
- **Status codes in doc comments.** A bare role, a bad verdict and a bad data
  source are refused with 400 (`invalid_role`, `invalid_verdict`,
  `invalid_source`), not 422.
