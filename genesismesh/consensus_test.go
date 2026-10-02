package genesismesh

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// A real Python-signed ConsensusProof decodes into the typed structs and
// encodes back to the same canonical JSON: no field is lost or renamed, so a
// vote or proof can be passed back to the NA and its signatures still verify.
func TestConsensusTypesRoundTripPythonProof(t *testing.T) {
	raw, err := os.ReadFile("testdata/conformance/consensus.json")
	if err != nil {
		t.Fatal(err)
	}
	var suite struct {
		Vectors []struct {
			Input struct {
				Proof json.RawMessage `json:"proof"`
			} `json:"input"`
		} `json:"vectors"`
	}
	if err := json.Unmarshal(raw, &suite); err != nil {
		t.Fatal(err)
	}
	original := suite.Vectors[0].Input.Proof
	var proof ConsensusProof
	if err := json.Unmarshal(original, &proof); err != nil {
		t.Fatal(err)
	}
	if len(proof.Votes) == 0 || proof.Votes[0].ContextDigest == nil || len(proof.Votes[0].Signature) == 0 {
		t.Fatalf("vote fields lost: %+v", proof.Votes)
	}
	encoded, err := json.Marshal(proof)
	if err != nil {
		t.Fatal(err)
	}
	want, err := CanonicalJSON(original)
	if err != nil {
		t.Fatal(err)
	}
	got, err := CanonicalJSON(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("round trip changed the proof\n got: %s\nwant: %s", got, want)
	}
}

func TestConsensusClient_Vote_HappyPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/consensus/vote" {
			http.NotFound(w, r)
			return
		}
		respondJSON(w, ConsensusVote{
			VoteID:               "vote-1",
			ProofID:              "jp-1",
			ValidatorSovereignID: "ALPHA",
			Vote:                 true,
		})
	})
	vote, err := c.Consensus.Vote(context.Background(), map[string]interface{}{
		"justification_proof": map[string]interface{}{"proof_id": "jp-1"},
		"vote":                true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if vote.VoteID != "vote-1" {
		t.Errorf("vote_id = %q, want vote-1", vote.VoteID)
	}
	if !vote.Vote || vote.ProofID != "jp-1" {
		t.Errorf("vote = %+v, want an approve vote on jp-1", vote)
	}
}

func TestConsensusClient_Vote_AdminHeaderPresent(t *testing.T) {
	var gotKeyID string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotKeyID = r.Header.Get("X-Admin-Key-Id")
		respondJSON(w, ConsensusVote{VoteID: "vote-hdr"})
	})
	_, err := c.Consensus.Vote(context.Background(), map[string]interface{}{
		"justification_proof": map[string]interface{}{"proof_id": "jp-1"},
		"vote":                false,
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
			ConsensusID:       "con-1",
			ProofID:           "proof-1",
			RequiredThreshold: 3,
			Votes: []ConsensusVote{
				{VoteID: "v1", Vote: true},
				{VoteID: "v2", Vote: true},
				{VoteID: "v3", Vote: true},
			},
		})
	})
	proof, err := c.Consensus.Proof(context.Background(), map[string]interface{}{
		"justification_proof":     map[string]interface{}{"proof_id": "proof-1"},
		"votes":                   []interface{}{},
		"required_threshold":      3,
		"validator_sovereign_ids": []string{"A", "B", "C"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if proof.ProofID != "proof-1" {
		t.Errorf("proof_id = %q, want proof-1", proof.ProofID)
	}
	if proof.RequiredThreshold != 3 {
		t.Errorf("required_threshold = %d, want 3", proof.RequiredThreshold)
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
		"justification_proof": map[string]interface{}{"proof_id": "proof-1"},
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
		respondJSON(w, map[string]interface{}{"valid": true, "reason": "valid", "consensus_id": "con-1"})
	}))
	t.Cleanup(srv.Close)
	c, err := NewClient(ClientOptions{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.Consensus.Verify(context.Background(), map[string]interface{}{"proof": map[string]interface{}{}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid || res.Reason != "valid" || res.ConsensusID == nil || *res.ConsensusID != "con-1" {
		t.Errorf("verification = %+v", res)
	}
}

func TestConsensusClient_Vote_WrongPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	_, err := c.Consensus.Vote(context.Background(), map[string]interface{}{"vote": true})
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if _, ok := err.(*NotFoundError); !ok {
		t.Errorf("want *NotFoundError, got %T: %v", err, err)
	}
}
