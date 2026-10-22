# sdk-go

Go SDK for the Genesis Mesh Network Authority HTTP API.

**Go ≥ 1.22 required. One runtime dependency: `github.com/google/uuid`, for nonces.**

## Scope

This SDK (1.7) covers the Network Authority's trust API: the sub-clients
below (agreements, boundary decisions, trust evidence, attestations,
selective disclosure, consensus and data usage), offline verification of
their signed records, and signed admin calls to any other route. It has no
governed lifecycle API: no evidence store client or evidence outbox, none
of the 1.3.0 changes outside the controlled path (observations, break-glass
records, the record outbox), and none of the 1.4.0 remediation and review
APIs (remediation records, the due queue, reviews). Those live in the
[TypeScript SDK](https://github.com/GenesisMeshLabs/sdk-typescript) and the
[Rust SDK](https://github.com/GenesisMeshLabs/sdk-rust). The embedded field
registry lists their records, but this SDK does not verify evidence exports.
A boundary policy with the 1.4.0 sections (`remediation`, `review_required`,
`emergency`) verifies here like any other.

From 1.5 it reads the NA's key history (see
[NA key history](#na-key-history)), so decisions and agreements signed before
and after a key succession keep verifying. Audit packs, archives and their
verification are not part of this SDK: they live in the core
(`genesis-mesh audit pack`, `genesis-mesh audit verify`) and in the Rust
SDK's standalone verifier `genesis-mesh-verify`.

From 1.6 it checks a pre-issued grant's NA signature (see
[Pre-issued grants](#pre-issued-grants)). Requesting grants, holding time
tokens, acting within a grant offline, recording its uses and falling back to
break-glass live in the
[TypeScript SDK](https://github.com/GenesisMeshLabs/sdk-typescript) and the
[Rust SDK](https://github.com/GenesisMeshLabs/sdk-rust), as does verifying
grants, their uses and revocations in an evidence export.

## Install

```sh
go get github.com/GenesisMeshLabs/sdk-go@latest
```

## Quick start

This program runs against a governed Network Authority on your machine, set
up as in [Develop Against a Local Network Authority](https://docs.genesismesh.org/sdk/local-network-authority.html).
It issues an attestation with the privileged setup key, asks the NA for a
policy-aware decision with the controller's standard key, and checks the
NA's signature on the decision.

A governed NA refuses `Boundary.Decide` (see [Boundary](#boundary)), and this
SDK has no method for `POST /admin/boundary/evaluate`, so the program signs
that call itself with `adminPost`, the helper from
[Raw admin calls](#raw-admin-calls).

```go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/GenesisMeshLabs/sdk-go/genesismesh"
)

const baseURL = "http://127.0.0.1:9443"

func main() {
	ctx := context.Background()

	// Setup, with the privileged key from `genesis-mesh init`: an attestation
	// for the agent, with the capabilities it may be granted.
	setup, err := genesismesh.NewClient(genesismesh.ClientOptions{
		BaseURL:    baseURL,
		SigningKey: os.Getenv("OPERATOR_KEY"), // base64-encoded 32-byte Ed25519 seed
		KeyID:      "operator-local",          // the key ID the NA has registered
	})
	if err != nil {
		panic(err)
	}
	att, err := setup.Attestation.Issue(ctx, map[string]interface{}{
		"subject_id": "agent-a",
		"roles":      []string{"role:client"},
		"claims":     map[string]interface{}{"capabilities": []string{"transactions.read"}},
	})
	if err != nil {
		panic(err)
	}

	// At run time, with the controller's standard key: a policy-aware decision.
	var result struct {
		Decision           json.RawMessage `json:"decision"`
		JustificationProof json.RawMessage `json:"justification_proof"`
	}
	err = adminPost(ctx, os.Getenv("CONTROLLER_KEY"), "controller", "/admin/boundary/evaluate",
		map[string]interface{}{
			"attestation_id":       att.AttestationID,
			"requested_capability": "transactions.read",
		}, &result)
	if err != nil {
		panic(err)
	}

	// Check the NA's signature on the decision before acting on it.
	client, err := genesismesh.NewClient(genesismesh.ClientOptions{BaseURL: baseURL})
	if err != nil {
		panic(err)
	}
	check, err := client.Boundary.Verify(ctx, map[string]interface{}{"decision": result.Decision})
	if err != nil {
		panic(err)
	}
	var decision genesismesh.BoundaryDecision
	if err := json.Unmarshal(result.Decision, &decision); err != nil {
		panic(err)
	}
	fmt.Println("signature accepted:", check.Accepted)
	fmt.Println("authorized:", decision.Authorized)
}
```

Add `adminPost` from [Raw admin calls](#raw-admin-calls) to the same file,
then run it with the two keys from the local NA's key files:

```sh
export OPERATOR_KEY=$(grep -v '^#' local/.genesis-mesh/keys/operator.key)
export CONTROLLER_KEY=$(grep -v '^#' local/.genesis-mesh/keys/controller.key)
go run .
```

It prints `signature accepted: true` and `authorized: true`. The decision is
an ALLOW because the attestation's `capabilities` claim names the capability
and no active boundary policy denies it. Publish and activate policies with
the privileged key to govern it further.

## Local Network Authority

To develop against a governed Network Authority on your machine (policies
required, a privileged key for setup and a standard key for your controller),
see [Develop Against a Local Network Authority](https://docs.genesismesh.org/sdk/local-network-authority.html).
It needs `genesis-mesh` 1.1.0 or later from PyPI.

## Sub-clients

The examples below share `ctx` and a `client` created with a signing key.
Methods marked "admin" need one; the others are public routes.

Each admin method needs an operator key of a tier. `Agreement.Counter`,
`Agreement.Accept`, `Attestation.Issue`, `Attestation.Revoke`,
`Attestation.SavePolicy` and `DataUsage.CreatePolicy` need a privileged key;
with a standard key the NA answers 403 `insufficient_operator_tier`. The
other admin methods accept a standard key.

### Agreement

```go
now := time.Now().UTC()

// Create a capability offer (admin). The NA names itself as the offerer and
// ignores OfferorSovereignID and Roles.
offer, err := client.Agreement.Offer(ctx, genesismesh.CapabilityOffer{
    ResponderSovereignID: "BETA-NA",
    Capabilities:         []string{"read:data", "write:log"},
    ValidFrom:            now.Format(time.RFC3339),
    ValidUntil:           now.Add(365 * 24 * time.Hour).Format(time.RFC3339),
    ExpiresAt:            now.Add(time.Hour).Format(time.RFC3339), // the offer's own expiry
})

// Counter an offer (admin, privileged)
counter, err := client.Agreement.Counter(ctx, map[string]interface{}{
    "offer":        offer,
    "capabilities": []string{"read:data"},
    "valid_from":   now.Format(time.RFC3339),
    "valid_until":  now.Add(180 * 24 * time.Hour).Format(time.RFC3339),
})

// Accept an offer (admin, privileged). The NA must hold a recognition treaty
// for the responder: issue one first (see Raw admin calls).
agreement, err := client.Agreement.Accept(ctx, offer)

// Verify agreement signatures (public)
result, err := client.Agreement.Verify(ctx, map[string]interface{}{"agreement": agreement})
// result.Accepted, result.Reason
```

`Accept` sends `{"offer": ...}` and accepts an offer only. To accept a
counter-offer, use `AcceptCounter` (from 1.7.1) with the counter and the offer
it answers; it needs a privileged key:

```go
agreement, err := client.Agreement.AcceptCounter(ctx, counter, offer)
```

With 1.7.0, send the same body with a raw admin call (see
[Raw admin calls](#raw-admin-calls)).

### Boundary

```go
// Issue a boundary decision under an agreement (admin)
decision, err := client.Boundary.Decide(ctx, map[string]interface{}{
    "agreement":            agreement,
    "requested_capability": "read:data",
})

// Verify a boundary decision (public)
v, err := client.Boundary.Verify(ctx, map[string]interface{}{"decision": decision})
```

A Network Authority that requires boundary policies
(`BOUNDARY_POLICY_ENFORCEMENT=required`, which `genesis-mesh init` sets)
refuses `Decide` with 409 `boundary_policy_required`. Call
`POST /admin/boundary/evaluate` with a raw admin call instead (see
[Raw admin calls](#raw-admin-calls)); this SDK has no method for it.

### Evidence

```go
// Build signed trust evidence (admin)
// verdict must be one of: "allow" | "block" | "escalate" | "warn"
evidence, err := client.Evidence.Build(ctx, genesismesh.TrustDecision{
    SourceSovereignID: "ALPHA",
    TargetSovereignID: "BETA",
    Verdict:           "allow",
    Reason:            "long-standing member",
})

// Verify trust evidence (public)
ev, err := client.Evidence.Verify(ctx, map[string]interface{}{"evidence": evidence})
```

Another verdict is refused with 400 `invalid_verdict` (a `BadRequestError`).

### Attestation

```go
// Issue a membership attestation (admin, privileged)
// roles must use a recognized prefix: role:anchor | role:bridge | role:client | role:operator | role:service:<name>
att, err := client.Attestation.Issue(ctx, map[string]interface{}{
    "subject_id":     "node-xyz",
    "roles":          []string{"role:client"},
    "validity_hours": 8760,
})

// Revoke an attestation (admin, privileged)
err = client.Attestation.Revoke(ctx, att.AttestationID, map[string]interface{}{"reason": "key compromised"})

// Set the recognition policy (admin, privileged): the issuers whose
// attestations this NA accepts, each with its public keys.
err = client.Attestation.SavePolicy(ctx, map[string]interface{}{
    "recognition_policy": map[string]interface{}{
        "local_sovereign_id": "MY-NA",
        "recognized_issuers": []map[string]interface{}{{
            "sovereign_id":  "PARTNER-NA",
            "public_keys":   []string{"<base64-ed25519-pubkey>"},
            "allowed_roles": []string{"role:client"},
        }},
    },
})
```

A role without a recognized prefix is refused with 400 `invalid_role`.

From 1.7.1, `Revoke` refuses an ID that is not one path segment (empty, `.`,
`..`, or containing `/`, `?`, `#`, `%` or a control character) with a
`*PathSegmentError`, before anything is signed or sent.

### Disclosure

```go
// Commit to the capabilities of an agreement (admin): needs agreement and capabilities
commitment, err := client.Disclosure.Commit(ctx, map[string]interface{}{
    "agreement":    agreement,
    "capabilities": []string{"read:data", "write:log"},
})

// Generate a Merkle membership proof (public): needs capability, capabilities,
// commitment and prover_sovereign_id
proof, err := client.Disclosure.Prove(ctx, map[string]interface{}{
    "capability":          "read:data",
    "capabilities":        []string{"read:data", "write:log"},
    "commitment":          commitment,
    "prover_sovereign_id": "BETA-NA",
})

// Verify the proof (public): needs proof and commitment
dv, err := client.Disclosure.Verify(ctx, map[string]interface{}{"proof": proof, "commitment": commitment})

// Issue a one-time nullifier (admin)
nullifier, err := client.Disclosure.Nullifier(ctx, map[string]interface{}{"proof": proof})
```

### Consensus

Validators vote on a justification proof: the `justification_proof` returned
with a decision by `POST /admin/boundary/evaluate` (see Raw admin calls). Pass
votes and proofs back unchanged; every field is covered by a signature.

```go
// Cast a validator vote, signed by the NA as validator (admin)
vote, err := client.Consensus.Vote(ctx, map[string]interface{}{
    "justification_proof": justificationProof,
    "vote":                true,
    "reason":              "evidence satisfactory",
})

// Assemble a K-of-N consensus proof (admin)
cp, err := client.Consensus.Proof(ctx, map[string]interface{}{
    "justification_proof":     justificationProof,
    "votes":                   []genesismesh.ConsensusVote{*vote},
    "required_threshold":      1,
    "validator_sovereign_ids": []string{vote.ValidatorSovereignID},
})

// Verify against the validators' keys (public)
cv, err := client.Consensus.Verify(ctx, map[string]interface{}{
    "proof":                 cp,
    "validator_public_keys": map[string]string{vote.ValidatorSovereignID: validatorPublicKey},
})
// cv.Valid, cv.Reason ("valid", "threshold_not_met", "invalid_vote_signature", ...)
```

### DataUsage

```go
// Create a data license policy (admin, privileged)
pol, err := client.DataUsage.CreatePolicy(ctx, map[string]interface{}{
    "licensee_sovereign_id": "BETA",
    "allowed_source_ids":    []string{"src-a"},
    "allowed_access_types":  []string{"read", "aggregate"},
    "valid_from":            "2026-01-01T00:00:00Z",
    "valid_until":           "2030-01-01T00:00:00Z",
})

// Create a data access intent (admin)
// each source needs source_id, source_type and owner_sovereign_id;
// source_type: "personal" | "proprietary" | "public" | "synthetic"
intent, err := client.DataUsage.CreateIntent(ctx, map[string]interface{}{
    "sources": []map[string]interface{}{{
        "source_id":           "src-a",
        "source_type":         "public",
        "owner_sovereign_id":  "MY-NA",
        "classification_tags": []string{"internal"},
    }},
    "access_types": []string{"read"},
})

// Get the active policy (public)
activePol, err := client.DataUsage.GetPolicy(ctx)

// Verify intent against policy (public)
dv, err := client.DataUsage.Verify(ctx, map[string]interface{}{"intent": intent, "policy": pol})
```

A source without those fields, or with another `source_type`, is refused with
400 `invalid_source`.

`DataUsage.Verify` checks the signatures over the records as they are sent
back. `DataSourceDescriptor` leaves out an empty `classification_tags` list
when it is encoded, so an intent with a source that has no tags no longer
verifies once it has been decoded into a `DataAccessIntent` (the NA reports
`intent_exceeds_license`, "Invalid intent signature"). Give each source at
least one tag, as above.

## Offline verification

From 0.61.0 the SDK verifies Network Authority records locally, using
canonical JSON and Ed25519 only, without calling the NA. The reason codes are
the Python reference's. Pass the JSON as received.

```go
a, err := genesismesh.VerifyAgreement(agreementJSON, []string{offererKey}, []string{responderKey}, "")
// a.Accepted, a.Reason: "accepted", "invalid_offerer_signature", "graph_digest_mismatch", ...

d, err := genesismesh.VerifyBoundaryDecision(decisionJSON, genesismesh.DecisionVerifyOptions{
	OperatorPublicKeys:  []string{naPublicKey},
	ExpectedPolicies:    [][]byte{boundaryPolicyJSON}, // the decision must bind exactly these versions
	ExpectedAttestation: attestationJSON,              // optional
})
// d.Accepted (verified), d.Authorized (ALLOW or DENY), d.Reason

ok, err := genesismesh.VerifyDataLicensePolicySignature(licensePolicyJSON, []string{licensorKey})
v, err := genesismesh.VerifyDataAccessIntent(intentJSON, licensePolicyJSON, []string{agentKey}, time.Now())
// v.Valid, v.ViolationReason, v.Violations
```

An accepted DENY has `Accepted` true and `Authorized` false: the decision
verified, and it denies. Revocation is not checked offline.

### NA key history

From 1.5.0 a Network Authority can move to a new signing key (a key
succession, signed by its offline root key and by the outgoing key). Pass the
NA's key history and a record must carry the signature of the key valid at
the record's own time: a decision's `decision_made_at`, an agreement's
`agreed_terms.valid_from` (`NARecordTimeFields` lists the time of each
NA-signed record). A succeeded key is still accepted for 300 seconds after
the next key takes effect (`SuccessionHandover`). A decision may not stay
valid more than `MaxDecisionLifetime` (300 seconds) past the end of the key
that signed it, and an agreement vouches only while its key is still
accepted. Without a history nothing changes: a list of keys is valid at every
time.

```go
// The body of GET /na/key-history, checked from the root key you hold out of band.
h, err := genesismesh.VerifyKeyHistoryResponse(body, rootPublicKey)
// h.Verified; otherwise h.Reason ("genesis_root_mismatch", "succession_chain_break", ...)
// and h.KeySequence, the refused record's position.

d, err := genesismesh.VerifyBoundaryDecision(decisionJSON, genesismesh.DecisionVerifyOptions{
	KeyHistory:   h.History, // instead of OperatorPublicKeys
	AtRecordTime: true,      // optional: an archived decision, judged as of decision_made_at
})
// d.Reason "na_key_not_valid_at_time": signed by a key of the history, but not the one valid then;
// "decision_outlives_key": valid longer than its key may vouch for.

a, err := genesismesh.VerifyAgreementWithKeyHistory(agreementJSON, h.History,
	genesismesh.NAKeysValidAlways([]string{responderKey}), "", time.Now())
```

`VerifyKeyHistory(genesisJSON, successionsJSON, rootPublicKey)` takes the
genesis block and the succession records directly. The refusal reasons and
their order are the reference's: `genesis_invalid`, `genesis_root_mismatch`,
`unknown_field`, `non_canonical_form`, `genesis_invalid_signature`, then for
each succession `payload_invalid`, `unknown_field` or `invalid_signature`,
`non_canonical_form` and the `succession_*` reasons. A succession carries
three signatures: the root key's, the new key's (proof that it is held) and
the outgoing key's. The SDK runs the key history vectors of the shared
`audit` suite (`genesismesh/testdata/conformance/audit.json`).
`CanonicalJSON`, `PolicyDigest`, `AttestationDigest` and `PolicySetDigest`
reproduce the reference forms. The verifiers pass the shared `interop`
conformance vectors (`genesismesh/testdata/conformance/interop.json`). The
core's cross-language scenario (`interop/`) also has them verify agreements,
decisions, a license policy and an intent that a live NA signs, on every pull
request and push to the core's `main`.

### Pre-issued grants

From 1.6.0 the NA can issue a `CapabilityGrant` before the bearer acts, so
the bearer can act within it while the NA is unreachable. Grants are off
unless the NA runs with `EVIDENCE_GRANTS=on`. `VerifyCapabilityGrant` checks
that a grant is the NA's: signed by the key of its key history valid at the
grant's `issued_at`, with every signed field known and in canonical form.

```go
g, err := genesismesh.VerifyCapabilityGrant(grantJSON, h.History) // or NAKeysValidAlways([]string{naKey})
// g.Valid; otherwise g.Reason: "missing_signature", "invalid_signature", "unknown_field",
// "non_canonical_form" or "na_key_not_valid_at_time".
```

It does not check the grant's validity, budget, parameter envelope or
revocation. The NA's revocation feed lists revoked grants in
`revoked_grant_ids` while grants are on (left out while they are off); the
embedded field registry knows both forms. The SDK runs the signed-form and
signature vectors of the shared `grants` suite
(`genesismesh/testdata/conformance/grants.json`).

## Raw admin calls

For NA routes this SDK has no method for (such as
`/admin/recognition-treaties` and `/admin/boundary/evaluate`), sign the
request with `BuildAdminHeaders` and send it yourself. `adminPost` signs one
POST with an operator key and decodes the answer:

```go
// adminPost signs one admin request with the operator key seed (base64) and
// key ID, sends it, and decodes the answer into out.
func adminPost(ctx context.Context, seed, keyID, path string, body, out interface{}) error {
	priv, _, err := genesismesh.LoadPrivateKey(seed)
	if err != nil {
		return err
	}
	// The signature names the NA's public key: network_authority.public_key in
	// its /sovereign.json. Outside development, pin the key you trust instead.
	res, err := http.Get(baseURL + "/sovereign.json")
	if err != nil {
		return err
	}
	var meta struct {
		NetworkAuthority struct {
			PublicKey string `json:"public_key"`
		} `json:"network_authority"`
	}
	err = json.NewDecoder(res.Body).Decode(&meta)
	res.Body.Close()
	if err != nil {
		return err
	}

	headers, err := genesismesh.BuildAdminHeaders(genesismesh.AdminRequest{
		Method: "POST", Path: path, Audience: meta.NetworkAuthority.PublicKey, Body: body,
	}, keyID, priv)
	if err != nil {
		return err
	}
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Key-Id", headers.KeyID)
	req.Header.Set("X-Admin-Signature", headers.Signature)
	req.Header.Set("X-Admin-Timestamp", headers.Timestamp)
	req.Header.Set("X-Admin-Nonce", headers.Nonce)

	res, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		msg, _ := io.ReadAll(res.Body)
		return fmt.Errorf("%s: HTTP %d: %s", path, res.StatusCode, msg)
	}
	return json.NewDecoder(res.Body).Decode(out)
}
```

It needs the imports `bytes`, `context`, `encoding/json`, `fmt`, `io` and
`net/http`, and a `baseURL` constant, as in the quick start.

Issue a recognition treaty, which `Agreement.Accept` needs for the responder
(privileged):

```go
var treaty map[string]interface{}
err = adminPost(ctx, os.Getenv("OPERATOR_KEY"), "operator-local", "/admin/recognition-treaties",
	map[string]interface{}{
		"subject_sovereign_id": "BETA-NA",
		"subject_public_keys":  []string{"<base64-ed25519-pubkey>"},
		"scope":                map[string]interface{}{"allowed_roles": []string{"role:client"}},
		"validity_hours":       24,
	}, &treaty)
```

Ask for a policy-aware decision under an agreement (standard). Under an
attestation, send `attestation_id` instead of `agreement`, as in the quick
start. The answer holds the signed `decision` and its `justification_proof`:

```go
var evaluated struct {
	Decision           json.RawMessage `json:"decision"`
	JustificationProof json.RawMessage `json:"justification_proof"`
}
err = adminPost(ctx, os.Getenv("CONTROLLER_KEY"), "controller", "/admin/boundary/evaluate",
	map[string]interface{}{"agreement": agreement, "requested_capability": "read:data"}, &evaluated)
```

Accept a counter-offer on 1.7.0, before `AcceptCounter` (privileged).
`counter` is what `Agreement.Counter` returned and `offer` the offer it
counters:

```go
var fromCounter genesismesh.AgreementRecord
err = adminPost(ctx, os.Getenv("OPERATOR_KEY"), "operator-local", "/admin/agreements/accept",
	map[string]interface{}{"counter": counter, "original_offer": offer}, &fromCounter)
```

## Error handling

Every HTTP error response from the NA comes back as a typed error, and every
one of them is a `genesismesh.APIError` (its `StatusCode()` and `ErrorCode()`).

From 1.7.1. In 1.7.0 and earlier, 403, 409, 413 and any other status without
its own type are `ServerError`, and there is no `APIError`, `ForbiddenError`,
`ConflictError`, `PayloadTooLargeError` or `ClientError`.

| Status | Type |
|---|---|
| 400 | `BadRequestError` |
| 401 | `UnauthorizedError` |
| 403 | `ForbiddenError` (such as `insufficient_operator_tier`) |
| 404 | `NotFoundError` |
| 409 | `ConflictError` (such as `boundary_policy_required`) |
| 413 | `PayloadTooLargeError` |
| 422 | `ValidationError` |
| 429 | `RateLimitError` |
| other 4xx | `ClientError` |
| 5xx | `ServerError` |

A 4xx cannot succeed as sent, so retry only a `RateLimitError` (after a pause),
a `ServerError` or a `NetworkError`.

A successful response that is not strict JSON (duplicate keys, `-0`, lone
surrogates, ...) is refused with a "decode response" error, which is neither
an `APIError` nor a `NetworkError`. An admin method called on a client without
a signing key returns an error ("signing key required for admin route")
before sending anything.

```go
import "errors"

_, err := client.Agreement.Offer(ctx, genesismesh.CapabilityOffer{})
if err != nil {
    var unauthorized *genesismesh.UnauthorizedError
    var forbidden    *genesismesh.ForbiddenError
    var rateLimit    *genesismesh.RateLimitError
    var server       *genesismesh.ServerError
    var network      *genesismesh.NetworkError
    var apiErr       genesismesh.APIError

    switch {
    case errors.As(err, &unauthorized):
        // bad signing key or stale timestamp
    case errors.As(err, &forbidden):
        // the key's operator tier may not do this: do not retry
    case errors.As(err, &rateLimit), errors.As(err, &server):
        // back off and retry
    case errors.As(err, &network):
        // connection refused or timeout: inspect network.Cause
    case errors.As(err, &apiErr):
        // any other NA error: apiErr.StatusCode(), apiErr.ErrorCode()
    }
}
```

## Admin authentication

Admin routes are authenticated with four HTTP headers built from an Ed25519 operator key:

| Header | Description |
|---|---|
| `X-Admin-Key-Id` | Key identifier registered with the NA |
| `X-Admin-Signature` | Ed25519 over the canonical admin payload: `{v: 2, method, path, query, audience, body, key_id, timestamp, nonce}` |
| `X-Admin-Timestamp` | ISO 8601 UTC timestamp (must be within NA's nonce window) |
| `X-Admin-Nonce` | UUID v4 replay-protection token (single use) |

Signature version 2 (1.0.2) binds each signature to the HTTP method, the request
path, the query parameters and the target NA's public key (`Audience`). The
client reads it (`network_authority.public_key`) once from the NA's public
`/sovereign.json`, or
uses `ClientOptions.Audience`. `AdminSigningPayload` returns the signed bytes.


The SDK handles all of this automatically when `SigningKey` is provided.

## Build and test

```sh
go build ./...
go test -race ./...
go vet ./...
```

How the package is laid out, and where a change goes:
[docs/architecture.md](docs/architecture.md). How to contribute:
[CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT
