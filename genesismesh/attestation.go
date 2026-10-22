package genesismesh

import "context"

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
// An attestationID that is not a single path segment (empty, ".", "..", or
// containing "/", "?", "#", "%" or a control character) returns a
// *PathSegmentError before anything is signed or sent (v1.7.1).
func (c *AttestationClient) Revoke(ctx context.Context, attestationID string, body map[string]interface{}) error {
	id, err := pathSegment("attestation_id", attestationID)
	if err != nil {
		return err
	}
	return c.t.adminPost(ctx, "/admin/attestations/"+id+"/revoke", body, nil)
}

// SavePolicy sets the active recognition policy.
// POST /admin/recognition-policy
// body.recognition_policy is the policy: a RecognitionPolicy, or a map with
// local_sovereign_id and recognized_issuers (each with sovereign_id and
// public_keys).
func (c *AttestationClient) SavePolicy(ctx context.Context, body map[string]interface{}) error {
	return c.t.adminPost(ctx, "/admin/recognition-policy", body, nil)
}
