package genesismesh

import "context"

// ConsensusClient wraps the Consensus domain.
type ConsensusClient struct{ t *transport }

// Vote casts a validator vote signed by the NA.
// POST /admin/consensus/vote
func (c *ConsensusClient) Vote(ctx context.Context, body map[string]interface{}) (*ConsensusVote, error) {
	var out ConsensusVote
	return &out, c.t.adminPost(ctx, "/admin/consensus/vote", body, &out)
}

// Proof assembles a consensus proof from validator votes.
// POST /admin/consensus/proof
func (c *ConsensusClient) Proof(ctx context.Context, body map[string]interface{}) (*ConsensusProof, error) {
	var out ConsensusProof
	return &out, c.t.adminPost(ctx, "/admin/consensus/proof", body, &out)
}

// Verify verifies a consensus proof and threshold (public route).
// POST /consensus/verify
func (c *ConsensusClient) Verify(ctx context.Context, body map[string]interface{}) (*VerifyResult, error) {
	var out VerifyResult
	return &out, c.t.publicPost(ctx, "/consensus/verify", body, &out)
}
