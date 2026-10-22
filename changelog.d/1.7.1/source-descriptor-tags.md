### Fixed

- **A decoded data access intent verifies when sent back.**
  `DataSourceDescriptor` left out an empty `classification_tags` list when
  it was encoded, but the agent signs each source with the list. So an
  intent whose sources had no tags, decoded into a `DataAccessIntent` and
  sent back to `DataUsage.Verify`, failed its signature check
  (`intent_exceeds_license`, "Invalid intent signature"). An empty or nil
  `ClassificationTags` is now written as `[]`.
