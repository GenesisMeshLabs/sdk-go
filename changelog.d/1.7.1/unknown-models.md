### Fixed

- **An unknown model name fails closed.** `UnknownFields` returned no fields
  for a model the field registry does not define, so a misspelt or renamed
  model turned the strict check off. It now returns an error, and the
  verifiers' strict checks refuse a record of such a model. A test checks
  that every model name in the source is in the embedded registry, and that
  every model the registry nests is too.

### Upgrading

- `UnknownFields` returns an error for a model name the embedded registry
  does not define, where it returned an empty list. Pass a model name of the
  registry (such as `BoundaryDecision`); the verifiers are unaffected.
