package genesismesh

import "context"

// AgreementClient wraps the Agreement domain.
type AgreementClient struct{ t *transport }

// Offer creates and signs a capability offer.
// POST /admin/agreements/offer
func (c *AgreementClient) Offer(ctx context.Context, body CapabilityOffer) (*AgreementRecord, error) {
	var out AgreementRecord
	return &out, c.t.adminPost(ctx, "/admin/agreements/offer", body, &out)
}

// Counter creates a counter-offer against an existing agreement.
// POST /admin/agreements/counter
func (c *AgreementClient) Counter(ctx context.Context, body map[string]interface{}) (*AgreementRecord, error) {
	var out AgreementRecord
	return &out, c.t.adminPost(ctx, "/admin/agreements/counter", body, &out)
}

// Accept accepts an offer or counter-offer.
// POST /admin/agreements/accept
// Requires an active recognition treaty for responder_sovereign_id.
func (c *AgreementClient) Accept(ctx context.Context, body map[string]interface{}) (*AgreementRecord, error) {
	var out AgreementRecord
	return &out, c.t.adminPost(ctx, "/admin/agreements/accept", body, &out)
}

// Verify verifies agreement signatures (public route).
// POST /agreements/verify
func (c *AgreementClient) Verify(ctx context.Context, body map[string]interface{}) (*VerifyResult, error) {
	var out VerifyResult
	return &out, c.t.publicPost(ctx, "/agreements/verify", body, &out)
}
