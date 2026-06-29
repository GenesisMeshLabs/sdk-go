package genesismesh

import "context"

// DataUsageClient wraps the Data Usage domain.
type DataUsageClient struct{ t *transport }

// CreatePolicy creates and signs a data license policy.
// POST /admin/data-usage/policy
func (c *DataUsageClient) CreatePolicy(ctx context.Context, body map[string]interface{}) (*DataLicensePolicy, error) {
	var out DataLicensePolicy
	return &out, c.t.adminPost(ctx, "/admin/data-usage/policy", body, &out)
}

// CreateIntent creates and signs a data access intent.
// POST /admin/data-usage/intent
// Each source in sources requires source_id, source_type, and owner_sovereign_id.
// source_type must be "personal" | "proprietary" | "public" | "synthetic".
func (c *DataUsageClient) CreateIntent(ctx context.Context, body map[string]interface{}) (*DataAccessIntent, error) {
	var out DataAccessIntent
	return &out, c.t.adminPost(ctx, "/admin/data-usage/intent", body, &out)
}

// GetPolicy returns the currently active data license policy (public route).
// GET /data-usage/policy
func (c *DataUsageClient) GetPolicy(ctx context.Context) (*DataLicensePolicy, error) {
	var out DataLicensePolicy
	return &out, c.t.publicGet(ctx, "/data-usage/policy", &out)
}

// Verify verifies an intent against a policy (public route).
// POST /data-usage/verify
func (c *DataUsageClient) Verify(ctx context.Context, body map[string]interface{}) (*VerifyResult, error) {
	var out VerifyResult
	return &out, c.t.publicPost(ctx, "/data-usage/verify", body, &out)
}
