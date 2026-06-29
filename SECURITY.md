# Security Policy

## Supported versions

| Version | Supported |
|---------|-----------|
| 0.54.x  | Yes       |
| < 0.54  | No        |

## Scope

This policy covers the `github.com/GenesisMeshLabs/sdk-go` Go module. It does **not**
cover the Genesis Mesh Network Authority server or the Genesis Mesh protocol
itself — those are tracked in the
[main repository](https://github.com/GenesisMeshLabs/genesismesh/blob/main/SECURITY.md).

Vulnerabilities in scope:

- Incorrect Ed25519 signature construction or verification
- `canonicalJSON` producing incorrect output that could allow signature bypass
- Admin header building that leaks the signing key or produces forgeable headers
- Error handling that leaks internal server details to callers
- Dependency vulnerabilities in the published module

Out of scope (report to the main repo instead):

- NA server-side vulnerabilities
- Protocol design issues
- Issues requiring physical access to the signing key material

## Reporting a vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Use GitHub's private vulnerability reporting:

1. Go to the [Security tab](https://github.com/GenesisMeshLabs/sdk-go/security)
2. Click **Report a vulnerability**
3. Fill in the report form

You will receive a response within 72 hours. We aim to release a fix within
14 days for critical issues and 30 days for moderate issues.

Please include:

- A description of the vulnerability and its impact
- Steps to reproduce or a minimal code sample
- The module version affected
- Any proposed fix if you have one

## Disclosure

We follow coordinated disclosure. We ask that you keep the vulnerability private
until a fix is released. We will credit you in the release notes unless you
prefer to remain anonymous.
