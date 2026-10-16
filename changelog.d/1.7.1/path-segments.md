### Security

- **An attestation ID is one path segment.** `Attestation.Revoke` put the
  ID into the request path as given, so an ID containing `/`, `..`, `?` or
  `#` (for example `../../recognition-policy`) signed and sent a request to
  another admin route or with another path. An ID that is empty, `.` or `..`,
  or contains `/`, `?`, `#`, `%` or a control character is now refused with a
  `*PathSegmentError` before anything is signed or sent. No other client
  method puts a caller-supplied value into a request path.

### Added

- **`PathSegmentError`** (`Param`, `Value`, `Reason`): a caller-supplied
  value refused as a request path segment. It is not an `APIError`, since no
  request was sent.

### Upgrading

- `Attestation.Revoke` returns a `*PathSegmentError` for an ID it used to
  send. The NA's attestation IDs are UUIDs and are unaffected; only an ID
  taken from untrusted input could now fail, and it would have reached
  another route before.
