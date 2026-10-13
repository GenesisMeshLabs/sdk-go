### Added

- **The NA key history.** `VerifyKeyHistory` and `VerifyKeyHistoryResponse`
  (the body of `GET /na/key-history`) check a genesis block and its key
  successions from the root public key you hold out of band, with the
  reference's reasons in its order. The genesis block is strict like every
  signed root: `genesis_invalid`, `genesis_root_mismatch`, `unknown_field`,
  `non_canonical_form`, `genesis_invalid_signature`. Each succession carries
  three signatures (the root key's, the new key's as proof that it is held,
  and the outgoing key's) and is checked for `payload_invalid`,
  `unknown_field` or `invalid_signature`, `non_canonical_form`,
  `succession_sovereign_mismatch`, `succession_chain_break`,
  `succession_key_reused` (the root key included), `succession_backdated`
  (issued after it takes effect), `succession_not_increasing`,
  `succession_after_expiry`, `succession_invalid_root_signature`,
  `succession_invalid_new_key_signature` and
  `succession_invalid_previous_signature`. The result's `NAKeyHistory` gives
  the keys valid at a time (`KeysAt`), with the 300-second hand-over after a
  succession (`SuccessionHandover`). `NARecordTimeFields` names the time each
  NA-signed record read alone is placed at.
- **Decisions across a key succession.** `DecisionVerifyOptions.KeyHistory`,
  instead of `OperatorPublicKeys`, requires the signature of the key valid at
  the decision's `decision_made_at` (`na_key_not_valid_at_time` for another
  key of the history, a new reason), and refuses a decision that stays valid
  more than `MaxDecisionLifetime` (300 seconds) past the end of the key that
  signed it and the hand-over (`decision_outlives_key`, a new reason).
  `DecisionVerifyOptions.AtRecordTime` judges an archived decision as of
  `decision_made_at` rather than now.
- **Agreements across a key succession.** `VerifyAgreementWithKeyHistory`
  checks each party's signature with the keys valid at the agreement's
  `agreed_terms.valid_from` (signed, unlike `established_at`) and still
  accepted now: an agreement a succeeded key signed stops vouching when that
  key's hand-over ends. `NAKeysValidAlways` stands for a party whose history
  you do not hold.
- The SDK runs the signed-form, key history, keys-at-a-time, record-time and
  decision vectors of the shared `audit` suite.

### Changed

- The embedded field registry lists the Genesis Mesh 1.5.0 records
  (`KeySuccessionRecord`, `ArchiveRecord`, `AuditPackManifest`, the
  `GenesisBlock` and `RecognitionTreaty`) and the entry kinds
  `key_succession` and `archive`.
- Without a key history every verifier behaves as before: a list of NA keys
  is valid at every time. Audit packs and their verification are not part of
  this SDK; they live in the core and in the Rust SDK's
  `genesis-mesh-verify`.
