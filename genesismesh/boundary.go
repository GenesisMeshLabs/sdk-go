package genesismesh

import "context"

// BoundaryClient wraps the Boundary domain.
type BoundaryClient struct{ t *transport }

// Decide issues a signed boundary decision for a capability request.
// POST /admin/boundary/decide
func (c *BoundaryClient) Decide(ctx context.Context, body map[string]interface{}) (*BoundaryDecision, error) {
	var out BoundaryDecision
	return &out, c.t.adminPost(ctx, "/admin/boundary/decide", body, &out)
}

// Verify verifies a boundary decision signature (public route).
// POST /boundary/verify
func (c *BoundaryClient) Verify(ctx context.Context, body map[string]interface{}) (*VerifyResult, error) {
	var out VerifyResult
	return &out, c.t.publicPost(ctx, "/boundary/verify", body, &out)
}
