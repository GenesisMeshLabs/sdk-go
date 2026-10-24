### Added

- **The vouching horizon (Genesis Mesh 1.8.0).** Judged for the present, a
  record signed by an NA key that has ended vouches only until that key's end,
  the 300-second hand-over and a grace of at most 24 hours. Judged at its own
  time, it is unchanged.
  - `NAKey.End`, and on `NAKeyHistory`: `VouchingGrace`, `WithVouchingGrace`
    (only shortens), `VouchesUntil`, `RecordOutlivesKey` and
    `KeysVouchingAt`. `SuccessionVouchingGrace` is 24 hours.
  - `VerifyCapabilityGrantAt(grantJSON, history, now)`: `VerifyCapabilityGrant`
    judged for the present, refusing a grant past its key's horizon as
    `outlives_key`.
  - `NARecordRefusal(model, recordJSON, history, now)`: the reference's
    verdict on an attestation, treaty, revocation feed or grant, with
    `record_malformed` for a record its kind's time rules refuse (such as a
    grant that starts more than 24 hours after its issue).
  - `VerifyKeyHistoryResponse` takes the NA's `vouching_grace_seconds` only
    to shorten the grace.
  - The shared `vouching` conformance suite (32 vectors) runs on every test
    run.

### Upgrading

- **Upgrade the Network Authority to 1.8.0 first** and let it re-issue its
  records (`reissue_pending: false` on its `/readyz`) before judging records
  for the present. `VerifyCapabilityGrant` and every other verifier keep
  judging at the record's own time.
