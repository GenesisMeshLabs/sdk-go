package genesismesh

import "context"

// AgreementClient wraps the Agreement domain.
type AgreementClient struct{ t *transport }

// Offer creates and signs a capability offer.
// POST /admin/agreements/offer
// Returns an OfferRecord — pass it to Accept to complete the handshake.
func (c *AgreementClient) Offer(ctx context.Context, body CapabilityOffer) (*OfferRecord, error) {
	var out OfferRecord
	return &out, c.t.adminPost(ctx, "/admin/agreements/offer", body, &out)
}

// Counter creates a counter-offer against an existing agreement.
// POST /admin/agreements/counter
func (c *AgreementClient) Counter(ctx context.Context, body map[string]interface{}) (*OfferRecord, error) {
	var out OfferRecord
	return &out, c.t.adminPost(ctx, "/admin/agreements/counter", body, &out)
}

// Accept accepts an offer.
// POST /admin/agreements/accept with {"offer": ...}
// Requires an active recognition treaty for the offer's responder_sovereign_id.
// To accept a counter-offer, use AcceptCounter.
func (c *AgreementClient) Accept(ctx context.Context, offer *OfferRecord) (*AgreementRecord, error) {
	var out AgreementRecord
	return &out, c.t.adminPost(ctx, "/admin/agreements/accept", map[string]interface{}{"offer": offer}, &out)
}

// AcceptCounter accepts a counter-offer (v1.8.0): counter is what Counter
// returned, originalOffer the offer it counters.
// POST /admin/agreements/accept with {"counter": ..., "original_offer": ...}
// The NA refuses a counter sent to Accept as an offer (422 accept_rejected).
func (c *AgreementClient) AcceptCounter(ctx context.Context, counter, originalOffer *OfferRecord) (*AgreementRecord, error) {
	var out AgreementRecord
	return &out, c.t.adminPost(ctx, "/admin/agreements/accept",
		map[string]interface{}{"counter": counter, "original_offer": originalOffer}, &out)
}

// Verify verifies agreement signatures (public route).
// POST /agreements/verify
func (c *AgreementClient) Verify(ctx context.Context, body map[string]interface{}) (*VerifyResult, error) {
	var out VerifyResult
	return &out, c.t.publicPost(ctx, "/agreements/verify", body, &out)
}
