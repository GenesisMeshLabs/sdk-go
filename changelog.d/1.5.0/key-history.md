### Added

- **The NA key history.** `VerifyKeyHistory` and `VerifyKeyHistoryResponse`
  (the body of `GET /na/key-history`) check a genesis block and its key
  successions from the root public key you hold out of band, with the
  reference's reasons in its order (`genesis_invalid`,
  `genesis_root_mismatch`, `genesis_invalid_signature`, then for each
  succession `payload_invalid`, `invalid_signature`, `unknown_field`,
  `non_canonical_form`, `succession_sovereign_mismatch`,
  `succession_chain_break`, `succession_key_reused`,
  `succession_not_increasing`, `succession_after_expiry`,
  `succession_invalid_root_signature`,
  `succession_invalid_previous_signature`). The result's `NAKeyHistory`
  gives the keys valid at a time (`KeysAt`), with the 300-second hand-over
  after a succession (`SuccessionHandover`).
- **Decisions and agreements across a key succession.**
  `DecisionVerifyOptions.KeyHistory`, instead of `OperatorPublicKeys`,
  requires the signature of the key valid at the decision's
  `decision_made_at`; a decision signed by another key of the history is
  refused as `na_key_not_valid_at_time`, a new reason.
  `DecisionVerifyOptions.AtRecordTime` judges an archived decision as of
  `decision_made_at` rather than now. `VerifyAgreementWithKeyHistory` checks
  each party's signature with the keys valid at the agreement's
  `established_at`; `NAKeysValidAlways` stands for a party whose history you
  do not hold.
- The SDK runs the key history, keys-at-a-time, decision-at-record-time and
  signed-form vectors of the shared `audit` suite.

### Changed

- The embedded field registry lists the Genesis Mesh 1.5.0 records
  (`KeySuccessionRecord`, `ArchiveRecord`, `AuditPackManifest` and the
  `GenesisBlock`) and the entry kinds `key_succession` and `archive`.
- Without a key history every verifier behaves as before: a list of NA keys
  is valid at every time. Audit packs and their verification are not part of
  this SDK; they live in the core and in the Rust SDK's
  `genesis-mesh-verify`.
