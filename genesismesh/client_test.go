package genesismesh

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	// Zero seed for deterministic test key
	seedB64 := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" // 32 zero bytes base64
	c, err := NewClient(ClientOptions{
		BaseURL:    srv.URL,
		SigningKey: seedB64,
		KeyID:      "test-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	return c, srv
}

func respondJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func TestNewClient_NoSigningKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, VerifyResult{Valid: true})
	}))
	defer srv.Close()
	c, err := NewClient(ClientOptions{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	result, err := c.Boundary.Verify(context.Background(), map[string]interface{}{"foo": "bar"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid {
		t.Error("expected valid=true")
	}
}

func TestAgreementClient_Offer(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/agreements/offer" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("X-Admin-Key-Id") == "" {
			http.Error(w, "missing admin header", 401)
			return
		}
		respondJSON(w, AgreementRecord{AgreementID: "agr-1", Status: "offered"})
	})
	rec, err := c.Agreement.Offer(context.Background(), CapabilityOffer{
		OfferorSovereignID:   "NA-A",
		ResponderSovereignID: "NA-B",
		Capabilities:         []string{"read"},
		Roles:                []string{"role:client"},
		ValidityHours:        24,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.AgreementID != "agr-1" {
		t.Errorf("agreement_id = %q", rec.AgreementID)
	}
}

func TestBoundaryClient_Decide(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/boundary/decide" {
			http.NotFound(w, r)
			return
		}
		respondJSON(w, BoundaryDecision{DecisionID: "dec-1", Allowed: true})
	})
	dec, err := c.Boundary.Decide(context.Background(), map[string]interface{}{
		"requesting_agent_id": "agent-a",
		"capability":          "read",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !dec.Allowed {
		t.Error("expected allowed=true")
	}
}

func TestEvidenceClient_Build(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, TrustEvidence{EvidenceID: "ev-1", Verdict: "allow"})
	})
	ev, err := c.Evidence.Build(context.Background(), TrustDecision{
		DecisionID: "dec-1",
		SubjectID:  "agent-a",
		Verdict:    "allow",
		Reason:     "policy check passed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != "allow" {
		t.Errorf("verdict = %q", ev.Verdict)
	}
}

func TestClient_AdminRouteWithoutKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	c, _ := NewClient(ClientOptions{BaseURL: srv.URL})
	_, err := c.Agreement.Offer(context.Background(), CapabilityOffer{})
	if err == nil {
		t.Error("expected error when no signing key")
	}
}

func TestClient_ErrorResponse(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{"message": "bad verdict", "code": "VALIDATION_ERROR"},
		})
	})
	_, err := c.Evidence.Build(context.Background(), TrustDecision{Verdict: "trusted"})
	var ve *ValidationError
	if err == nil {
		t.Fatal("expected error")
	}
	if !isValidationError(err, &ve) {
		t.Errorf("want ValidationError, got %T: %v", err, err)
	}
}

func isValidationError(err error, target **ValidationError) bool {
	ve, ok := err.(*ValidationError)
	if ok {
		*target = ve
	}
	return ok
}
