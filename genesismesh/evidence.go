package genesismesh

import "context"

// EvidenceClient wraps the Trust Evidence domain.
type EvidenceClient struct{ t *transport }

// Build builds and signs trust evidence from a TrustDecision.
// POST /admin/trust-evidence
// verdict must be "allow" | "block" | "escalate" | "warn" — NOT "trusted".
func (c *EvidenceClient) Build(ctx context.Context, body TrustDecision) (*TrustEvidence, error) {
	var out TrustEvidence
	return &out, c.t.adminPost(ctx, "/admin/trust-evidence", body, &out)
}

// Verify verifies trust evidence signatures (public route).
// POST /trust-evidence/verify
func (c *EvidenceClient) Verify(ctx context.Context, body map[string]interface{}) (*VerifyResult, error) {
	var out VerifyResult
	return &out, c.t.publicPost(ctx, "/trust-evidence/verify", body, &out)
}
