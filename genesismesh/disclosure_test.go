package genesismesh

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDisclosureClient_Commit_HappyPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/disclosure/commit" {
			http.NotFound(w, r)
			return
		}
		respondJSON(w, CapabilityCommitment{
			CommitmentID: "commit-1",
			MerkleRoot:   "abc123",
		})
	})
	commit, err := c.Disclosure.Commit(context.Background(), map[string]interface{}{
		"capabilities": []string{"read", "write", "delete"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if commit.CommitmentID != "commit-1" {
		t.Errorf("commitment_id = %q, want commit-1", commit.CommitmentID)
	}
	if commit.MerkleRoot != "abc123" {
		t.Errorf("merkle_root = %q, want abc123", commit.MerkleRoot)
	}
}

func TestDisclosureClient_Commit_AdminHeaderPresent(t *testing.T) {
	var gotKeyID string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotKeyID = r.Header.Get("X-Admin-Key-Id")
		respondJSON(w, CapabilityCommitment{CommitmentID: "commit-hdr"})
	})
	_, err := c.Disclosure.Commit(context.Background(), map[string]interface{}{
		"capabilities": []string{"read"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotKeyID == "" {
		t.Error("X-Admin-Key-Id header not sent on admin route")
	}
}

func TestDisclosureClient_Nullifier_HappyPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/disclosure/nullifier" {
			http.NotFound(w, r)
			return
		}
		respondJSON(w, map[string]interface{}{
			"nullifier_id": "null-1",
			"commitment_id": "commit-1",
		})
	})
	out, err := c.Disclosure.Nullifier(context.Background(), map[string]interface{}{
		"commitment_id": "commit-1",
		"capability":    "read",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out["nullifier_id"] != "null-1" {
		t.Errorf("nullifier_id = %v, want null-1", out["nullifier_id"])
	}
}

func TestDisclosureClient_Prove_PublicRouteNoSigningKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/disclosure/prove" {
			http.NotFound(w, r)
			return
		}
		respondJSON(w, CapabilityMembershipProof{
			CommitmentID: "commit-1",
			Capability:   "read",
			LeafHash:     "deadbeef",
		})
	}))
	t.Cleanup(srv.Close)
	c, err := NewClient(ClientOptions{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	proof, err := c.Disclosure.Prove(context.Background(), map[string]interface{}{
		"commitment_id": "commit-1",
		"capability":    "read",
	})
	if err != nil {
		t.Fatal(err)
	}
	if proof.CommitmentID != "commit-1" {
		t.Errorf("commitment_id = %q, want commit-1", proof.CommitmentID)
	}
	if proof.Capability != "read" {
		t.Errorf("capability = %q, want read", proof.Capability)
	}
}

func TestDisclosureClient_Verify_PublicRouteNoSigningKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/disclosure/verify" {
			http.NotFound(w, r)
			return
		}
		respondJSON(w, VerifyResult{Valid: true, Reason: "proof valid"})
	}))
	t.Cleanup(srv.Close)
	c, err := NewClient(ClientOptions{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.Disclosure.Verify(context.Background(), map[string]interface{}{"commitment_id": "commit-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid {
		t.Error("expected valid=true")
	}
}

func TestDisclosureClient_Commit_WrongPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	_, err := c.Disclosure.Commit(context.Background(), map[string]interface{}{"capabilities": []string{"read"}})
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if _, ok := err.(*NotFoundError); !ok {
		t.Errorf("want *NotFoundError, got %T: %v", err, err)
	}
}
