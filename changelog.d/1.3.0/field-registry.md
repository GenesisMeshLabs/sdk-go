### Changed

- The embedded field registry lists the Genesis Mesh 1.3.0 records
  (`ObservationRecord`, `BreakGlassRecord`, `JudgementRecord`,
  `QuarantineRecord`, `RegistryRecord`), the entry kinds `observation`,
  `break_glass`, `judgement`, `quarantine` and `registry`, the envelope
  fields `record_id`, `subject_id`, `matched_evidence_id` and
  `observation_sequence`, and the retention checkpoint's
  `observation_heads`. This SDK does not verify evidence exports, so it has
  nothing to refuse; the TypeScript and Rust SDKs verify the new records.

### Fixed

- Verifiers refuse a record that leaves out a field the reference always
  writes (`non_canonical_form`), as the reference does: a decision signed
  without `denial_reason` verified here. A field the reference leaves out
  when absent reads the same whether absent or `null`.
- `UnknownFields` reports an empty key on records without a signature field
  (`ContextRecord`, `EvidenceStoreEntry`); it was taken for the signature.
- `CheckStrictJSON` refuses text that is not UTF-8 before any other fault, as
  the other implementations do (`[-0,"ÿ"]` was `negative_zero`).
