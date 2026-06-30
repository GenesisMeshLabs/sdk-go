package genesismesh

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConsensusClient_Vote_HappyPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/consensus/vote" {
			http.NotFound(w, r)
			return
		}
		respondJSON(w, ConsensusVote{
			VoteID:     "vote-1",
			ProposalID: "prop-abc",
			Decision:   "accept",
		})
	})
	vote, err := c.Consensus.Vote(context.Background(), map[string]interface{}{
		"proposal_id": "prop-abc",
		"decision":    "accept",
	})
	if err != nil {
		t.Fatal(err)
	}
	if vote.VoteID != "vote-1" {
		t.Errorf("vote_id = %q, want vote-1", vote.VoteID)
	}
	if vote.Decision != "accept" {
		t.Errorf("decision = %q, want accept", vote.Decision)
	}
}

func TestConsensusClient_Vote_AdminHeaderPresent(t *testing.T) {
	var gotKeyID string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotKeyID = r.Header.Get("X-Admin-Key-Id")
		respondJSON(w, ConsensusVote{VoteID: "vote-hdr"})
	})
	_, err := c.Consensus.Vote(context.Background(), map[string]interface{}{
		"proposal_id": "prop-1",
		"decision":    "reject",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotKeyID == "" {
		t.Error("X-Admin-Key-Id header not sent on admin route")
	}
}

func TestConsensusClient_Proof_HappyPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/consensus/proof" {
			http.NotFound(w, r)
			return
		}
		respondJSON(w, ConsensusProof{
			ProofID:    "proof-1",
			ProposalID: "prop-abc",
			Threshold:  3,
			Votes: []ConsensusVote{
				{VoteID: "v1", Decision: "accept"},
				{VoteID: "v2", Decision: "accept"},
				{VoteID: "v3", Decision: "accept"},
			},
		})
	})
	proof, err := c.Consensus.Proof(context.Background(), map[string]interface{}{
		"proposal_id": "prop-abc",
		"vote_ids":    []string{"v1", "v2", "v3"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if proof.ProofID != "proof-1" {
		t.Errorf("proof_id = %q, want proof-1", proof.ProofID)
	}
	if proof.Threshold != 3 {
		t.Errorf("threshold = %d, want 3", proof.Threshold)
	}
	if len(proof.Votes) != 3 {
		t.Errorf("votes count = %d, want 3", len(proof.Votes))
	}
}

func TestConsensusClient_Proof_AdminHeaderPresent(t *testing.T) {
	var gotKeyID string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotKeyID = r.Header.Get("X-Admin-Key-Id")
		respondJSON(w, ConsensusProof{ProofID: "proof-hdr"})
	})
	_, err := c.Consensus.Proof(context.Background(), map[string]interface{}{
		"proposal_id": "prop-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotKeyID == "" {
		t.Error("X-Admin-Key-Id header not sent on admin route")
	}
}

func TestConsensusClient_Verify_PublicRouteNoSigningKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/consensus/verify" {
			http.NotFound(w, r)
			return
		}
		respondJSON(w, VerifyResult{Valid: true, Reason: "threshold met"})
	}))
	t.Cleanup(srv.Close)
	c, err := NewClient(ClientOptions{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.Consensus.Verify(context.Background(), map[string]interface{}{"proof_id": "proof-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid {
		t.Error("expected valid=true")
	}
}

func TestConsensusClient_Vote_WrongPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	_, err := c.Consensus.Vote(context.Background(), map[string]interface{}{"proposal_id": "p1"})
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if _, ok := err.(*NotFoundError); !ok {
		t.Errorf("want *NotFoundError, got %T: %v", err, err)
	}
}
