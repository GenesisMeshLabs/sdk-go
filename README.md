# sdk-go

Go SDK for the Genesis Mesh Network Authority HTTP API.

**Go ≥ 1.22 required. Zero runtime dependencies (stdlib only + `github.com/google/uuid`).**

## Install

```sh
go get github.com/GenesisMeshLabs/sdk-go@latest
```

## Quick start

```go
package main

import (
    "context"
    "os"

    "github.com/GenesisMeshLabs/sdk-go/genesismesh"
)

func main() {
    client, err := genesismesh.NewClient(genesismesh.ClientOptions{
        BaseURL:    "http://127.0.0.1:9443",
        SigningKey: os.Getenv("OPERATOR_KEY"), // base64-encoded 32-byte Ed25519 seed
        KeyID:      "operator-local",          // must match the NA's registered key
    })
    if err != nil {
        panic(err)
    }

    decision, err := client.Boundary.Decide(context.Background(), map[string]interface{}{
        "requesting_agent_id": "agent-a",
        "capability":          "transactions.read",
    })
    if err != nil {
        panic(err)
    }
    _ = decision
}
```

## Sub-clients

### Agreement

```go
// Create a capability offer (admin — requires signing key)
offer, err := client.Agreement.Offer(ctx, genesismesh.CapabilityOffer{
    OfferorSovereignID:   "ALPHA-NA",
    ResponderSovereignID: "BETA-NA",
    Capabilities:         []string{"read:data", "write:log"},
    Roles:                []string{"role:client"},
    ValidityHours:        8760,
})

// Counter an offer
counter, err := client.Agreement.Counter(ctx, map[string]interface{}{
    "offer":         offer,
    "capabilities":  []string{"read:data"},
    "validity_hours": 4380,
})

// Accept an offer (requires the NA to hold a recognition treaty for the responder)
agreement, err := client.Agreement.Accept(ctx, map[string]interface{}{"offer": offer})

// Verify agreement signatures (no auth required)
result, err := client.Agreement.Verify(ctx, map[string]interface{}{"agreement": agreement})
```

### Boundary

```go
// Issue a boundary decision (admin)
decision, err := client.Boundary.Decide(ctx, map[string]interface{}{
    "agreement":            agreement,
    "requested_capability": "read:data",
})

// Verify a boundary decision (no auth required)
v, err := client.Boundary.Verify(ctx, map[string]interface{}{"decision": decision})
```

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
```

### Attestation

```go
// Issue a membership attestation (admin)
// roles must use a recognized prefix: role:anchor | role:bridge | role:client | role:operator | role:service:<name>
att, err := client.Attestation.Issue(ctx, genesismesh.MembershipAttestation{
    SubjectID:      "node-xyz",
    Roles:          []string{"role:client"},
    ValidityHours:  8760,
})

// Revoke an attestation (admin)
err = client.Attestation.Revoke(ctx, att.AttestationID, map[string]string{"reason": "key compromised"})

// Set recognition policy (admin)
err = client.Attestation.SavePolicy(ctx, genesismesh.RecognitionPolicy{
    LocalSovereignID:  "MY-NA",
    RecognizedIssuers: []genesismesh.RecognizedIssuer{},
})
```

### Disclosure

```go
// Commit to a set of capabilities (admin)
commitment, err := client.Disclosure.Commit(ctx, genesismesh.CapabilityCommitment{
    Capabilities: []string{"read:data", "write:log"},
})

// Generate a Merkle membership proof (no auth required)
proof, err := client.Disclosure.Prove(ctx, genesismesh.CapabilityMembershipProof{
    Capability:        "read:data",
    ProverSovereignID: "BETA",
})

// Issue a one-time nullifier (admin)
nullifier, err := client.Disclosure.Nullifier(ctx, map[string]interface{}{"proof": proof})

// Verify the proof (no auth required)
dv, err := client.Disclosure.Verify(ctx, map[string]interface{}{"proof": proof})
```

### Consensus

```go
// Cast a validator vote (admin)
vote, err := client.Consensus.Vote(ctx, genesismesh.ConsensusVote{
    Vote:   true,
    Reason: "evidence satisfactory",
})

// Assemble a consensus proof (admin)
cp, err := client.Consensus.Proof(ctx, genesismesh.ConsensusProof{
    RequiredThreshold:    1,
    ValidatorSovereignIDs: []string{"ALPHA"},
})

// Verify the consensus proof (no auth required)
cv, err := client.Consensus.Verify(ctx, map[string]interface{}{"proof": cp})
```

### DataUsage

```go
// Create a data license policy (admin)
pol, err := client.DataUsage.CreatePolicy(ctx, genesismesh.DataLicensePolicy{
    LicenseeSovereignID:   "BETA",
    AllowedSourceIDs:      []string{"src-a"},
    AllowedAccessTypes:    []string{"read", "aggregate"},
    ValidFrom:             "2026-01-01T00:00:00Z",
    ValidUntil:            "2026-12-31T00:00:00Z",
})

// Create a data access intent (admin)
// source_type: "personal" | "proprietary" | "public" | "synthetic"
intent, err := client.DataUsage.CreateIntent(ctx, genesismesh.DataAccessIntent{
    Sources: []genesismesh.DataSourceDescriptor{{
        SourceID:          "src-a",
        SourceType:        "public",
        OwnerSovereignID:  "MY-NA",
    }},
    AccessTypes: []string{"read"},
})

// Get the active policy (no auth required)
activePol, err := client.DataUsage.GetPolicy(ctx)

// Verify intent against policy (no auth required)
dv, err := client.DataUsage.Verify(ctx, map[string]interface{}{"intent": intent, "policy": pol})
```

## Raw admin calls

For NA routes not yet covered by a sub-client (e.g. `/admin/recognition-treaties`),
use `BuildAdminHeaders` directly:

```go
import (
    "bytes"
    "encoding/json"
    "net/http"

    "github.com/GenesisMeshLabs/sdk-go/genesismesh"
)

priv, _, _ := genesismesh.LoadPrivateKey(os.Getenv("OPERATOR_KEY"))

body := map[string]interface{}{
    "subject_sovereign_id":  "BETA-NA",
    "subject_public_keys":   []string{"<base64-ed25519-pubkey>"},
    "scope":                 map[string]interface{}{"allowed_roles": []string{"role:client"}},
    "validity_hours":        24,
}

headers, err := genesismesh.BuildAdminHeaders(body, "operator-local", priv)

b, _ := json.Marshal(body)
req, _ := http.NewRequest("POST", baseURL+"/admin/recognition-treaties", bytes.NewReader(b))
req.Header.Set("Content-Type", "application/json")
req.Header.Set("X-Admin-Key-Id", headers.KeyID)
req.Header.Set("X-Admin-Signature", headers.Signature)
req.Header.Set("X-Admin-Timestamp", headers.Timestamp)
req.Header.Set("X-Admin-Nonce", headers.Nonce)
```

## Error handling

```go
import "errors"

_, err := client.Agreement.Offer(ctx, genesismesh.CapabilityOffer{})
switch {
case err == nil:
    // success
default:
    var unauthorized *genesismesh.UnauthorizedError
    var rateLimit    *genesismesh.RateLimitError
    var validation  *genesismesh.ValidationError
    var network     *genesismesh.NetworkError

    switch {
    case errors.As(err, &unauthorized):
        // bad signing key or stale timestamp
    case errors.As(err, &rateLimit):
        // back off and retry
    case errors.As(err, &validation):
        // inspect err.Code and err.Message
    case errors.As(err, &network):
        // connection refused or timeout — inspect err.Cause
    }
}
```

## Admin authentication

Admin routes are authenticated with four HTTP headers built from an Ed25519 operator key:

| Header | Description |
|---|---|
| `X-Admin-Key-Id` | Key identifier registered with the NA |
| `X-Admin-Signature` | Ed25519 signature over `canonicalJSON({body, key_id, nonce, timestamp})` |
| `X-Admin-Timestamp` | ISO 8601 UTC timestamp (must be within NA's nonce window) |
| `X-Admin-Nonce` | UUID v4 replay-protection token (single use) |

The SDK handles all of this automatically when `SigningKey` is provided.

## Build and test

```sh
go build ./...
go test -race ./...
go vet ./...
```

## License

MIT
