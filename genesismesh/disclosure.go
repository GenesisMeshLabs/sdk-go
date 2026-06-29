package genesismesh

import "context"

// DisclosureClient wraps the Selective Disclosure domain.
type DisclosureClient struct{ t *transport }

// Commit commits to a capability set (Merkle root).
// POST /admin/disclosure/commit
func (c *DisclosureClient) Commit(ctx context.Context, body map[string]interface{}) (*CapabilityCommitment, error) {
	var out CapabilityCommitment
	return &out, c.t.adminPost(ctx, "/admin/disclosure/commit", body, &out)
}

// Nullifier issues a one-time nullifier for a proof.
// POST /admin/disclosure/nullifier
func (c *DisclosureClient) Nullifier(ctx context.Context, body map[string]interface{}) (map[string]interface{}, error) {
	var out map[string]interface{}
	return out, c.t.adminPost(ctx, "/admin/disclosure/nullifier", body, &out)
}

// Prove generates a Merkle membership proof (public route).
// POST /disclosure/prove
func (c *DisclosureClient) Prove(ctx context.Context, body map[string]interface{}) (*CapabilityMembershipProof, error) {
	var out CapabilityMembershipProof
	return &out, c.t.publicPost(ctx, "/disclosure/prove", body, &out)
}

// Verify verifies a capability membership proof (public route).
// POST /disclosure/verify
func (c *DisclosureClient) Verify(ctx context.Context, body map[string]interface{}) (*VerifyResult, error) {
	var out VerifyResult
	return &out, c.t.publicPost(ctx, "/disclosure/verify", body, &out)
}
