### Added

- **`ForbiddenError`, `ConflictError`, `PayloadTooLargeError` and
  `ClientError`.** A 403 (such as `insufficient_operator_tier`), a 409 (such
  as `boundary_policy_required`) and a 413 (`request_entity_too_large`) have
  their own types, and any other 4xx is a `ClientError`.
- **`APIError`**, an interface every error for an NA error response
  implements (`StatusCode()`, `ErrorCode()`), so a caller can read the status
  and code with one `errors.As`, whatever the type.

### Fixed

- **A 4xx is no longer a `ServerError`.** Every status without a type of its
  own came back as `ServerError`, so a policy that retries server errors
  retried a 403, 409 or 413 that could never succeed. `ServerError` is now
  only a 5xx.

### Upgrading

- Code that matched `*ServerError` to catch a 403, 409, 413 or another 4xx no
  longer catches it: match `*ForbiddenError`, `*ConflictError`,
  `*PayloadTooLargeError` or `*ClientError`, or match `APIError` and read
  `StatusCode()`. Code that retries on `*ServerError` now retries only 5xx
  responses, as intended. The typed errors for 400, 401, 404, 422 and 429,
  their fields and their messages are unchanged.
