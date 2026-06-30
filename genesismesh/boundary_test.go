package genesismesh

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBoundaryClient_Decide_HappyPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/boundary/decide" {
			http.NotFound(w, r)
			return
		}
		respondJSON(w, BoundaryDecision{
			DecisionID:        "dec-1",
			Allowed:           true,
			Reason:            "policy matched",
			RequestingAgentID: "agent-a",
		})
	})
	dec, err := c.Boundary.Decide(context.Background(), map[string]interface{}{
		"requesting_agent_id": "agent-a",
		"target_agent_id":     "agent-b",
		"capability":          "read",
		"agreement_id":        "agr-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if dec.DecisionID != "dec-1" {
		t.Errorf("decision_id = %q, want dec-1", dec.DecisionID)
	}
	if !dec.Allowed {
		t.Error("expected allowed=true")
	}
}

func TestBoundaryClient_Decide_AdminHeaderPresent(t *testing.T) {
	var gotKeyID string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotKeyID = r.Header.Get("X-Admin-Key-Id")
		respondJSON(w, BoundaryDecision{DecisionID: "dec-hdr", Allowed: false})
	})
	_, err := c.Boundary.Decide(context.Background(), map[string]interface{}{
		"requesting_agent_id": "agent-x",
		"capability":          "write",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotKeyID == "" {
		t.Error("X-Admin-Key-Id header not sent on admin route")
	}
}

func TestBoundaryClient_Decide_AllowedFalseDeserializes(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, BoundaryDecision{DecisionID: "dec-deny", Allowed: false, Reason: "no agreement"})
	})
	dec, err := c.Boundary.Decide(context.Background(), map[string]interface{}{
		"requesting_agent_id": "agent-z",
		"capability":          "delete",
	})
	if err != nil {
		t.Fatal(err)
	}
	if dec.Allowed {
		t.Error("expected allowed=false")
	}
	if dec.Reason != "no agreement" {
		t.Errorf("reason = %q, want 'no agreement'", dec.Reason)
	}
}

func TestBoundaryClient_Verify_PublicRouteNoSigningKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/boundary/verify" {
			http.NotFound(w, r)
			return
		}
		respondJSON(w, VerifyResult{Valid: true, Reason: "signature valid"})
	}))
	t.Cleanup(srv.Close)
	c, err := NewClient(ClientOptions{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.Boundary.Verify(context.Background(), map[string]interface{}{"decision_id": "dec-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid {
		t.Error("expected valid=true")
	}
}

func TestBoundaryClient_Verify_InvalidSignatureResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, VerifyResult{Valid: false, Reason: "bad signature"})
	}))
	t.Cleanup(srv.Close)
	c, err := NewClient(ClientOptions{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.Boundary.Verify(context.Background(), map[string]interface{}{"decision_id": "dec-bad"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Valid {
		t.Error("expected valid=false")
	}
	if res.Reason != "bad signature" {
		t.Errorf("reason = %q, want 'bad signature'", res.Reason)
	}
}

func TestBoundaryClient_Decide_WrongPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	_, err := c.Boundary.Decide(context.Background(), map[string]interface{}{"capability": "read"})
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if _, ok := err.(*NotFoundError); !ok {
		t.Errorf("want *NotFoundError, got %T: %v", err, err)
	}
}
