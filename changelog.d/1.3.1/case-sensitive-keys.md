### Fixed

- Client responses are read with keys matched in their own case only.
  `encoding/json` reads a key in another case into a field, so
  `{"authorized":false,"Authorized":true}` read as an authorized decision,
  where every other implementation reads `Authorized` as a key it does not
  know. Such keys are now dropped, at any depth, before a response is
  decoded: decisions, agreements, evidence, attestations, policies and
  intents. `AgreementRecord.Capabilities` reads `capabilities` in the agreed
  terms in its own case only.
