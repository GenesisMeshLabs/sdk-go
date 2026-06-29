package genesismesh

import (
	"context"
	"fmt"
)

// AttestationClient wraps the Attestation domain.
type AttestationClient struct{ t *transport }

// Issue issues a signed membership attestation.
// POST /admin/attestations
// roles must use recognised prefixes: role:anchor, role:bridge, role:client,
// role:operator, role:service:<name>. Bare names return HTTP 422.
func (c *AttestationClient) Issue(ctx context.Context, body map[string]interface{}) (*MembershipAttestation, error) {
	var out MembershipAttestation
	return &out, c.t.adminPost(ctx, "/admin/attestations", body, &out)
}

// Revoke revokes an attestation by ID.
// POST /admin/attestations/{id}/revoke
func (c *AttestationClient) Revoke(ctx context.Context, attestationID string, body map[string]interface{}) error {
	path := fmt.Sprintf("/admin/attestations/%s/revoke", attestationID)
	return c.t.adminPost(ctx, path, body, nil)
}

// SavePolicy sets the active recognition policy.
// POST /admin/recognition-policy
// body.recognition_policy must have local_sovereign_id and recognized_issuers.
func (c *AttestationClient) SavePolicy(ctx context.Context, body map[string]interface{}) error {
	return c.t.adminPost(ctx, "/admin/recognition-policy", body, nil)
}
