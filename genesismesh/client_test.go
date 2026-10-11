package genesismesh

import (
	"context"
	"encoding/json"
	"errors"
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
		// Admin signatures name the NA's public key; tests pass a fixed audience.
		Audience: "TEST",
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
		respondJSON(w, OfferRecord{OfferID: "offer-1"})
	})
	rec, err := c.Agreement.Offer(context.Background(), CapabilityOffer{
		OfferorSovereignID:   "NA-A",
		ResponderSovereignID: "NA-B",
		Capabilities:         []string{"read"},
		Roles:                []string{"role:client"},
		ValidFrom:            "2026-01-01T00:00:00.000Z",
		ValidUntil:           "2027-01-01T00:00:00.000Z",
		ExpiresAt:            "2026-01-08T00:00:00.000Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.OfferID != "offer-1" {
		t.Errorf("offer_id = %q", rec.OfferID)
	}
}

func TestBoundaryClient_Decide(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/boundary/decide" {
			http.NotFound(w, r)
			return
		}
		respondJSON(w, map[string]interface{}{"decision_id": "dec-1", "authorized": true})
	})
	dec, err := c.Boundary.Decide(context.Background(), map[string]interface{}{
		"requesting_agent_id": "agent-a",
		"capability":          "read",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !dec.Authorized || !dec.Allowed {
		t.Errorf("authorized = %v, allowed = %v; want both true", dec.Authorized, dec.Allowed)
	}
}

func TestEvidenceClient_Build(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, TrustEvidence{EvidenceID: "ev-1", Verdict: "allow"})
	})
	ev, err := c.Evidence.Build(context.Background(), TrustDecision{
		SourceSovereignID: "ALPHA-NA",
		TargetSovereignID: "BETA-NA",
		Verdict:           "allow",
		Reason:            "policy check passed",
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

func respondRaw(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

func TestClient_ReadsKeysInTheirOwnCaseOnly(t *testing.T) {
	// v1.3.1: encoding/json reads a key in another case into the same field,
	// so the first decision read as authorized; every other implementation
	// reads such a key as one it does not know.
	for _, body := range []string{
		`{"decision_id":"dec-1","authorized":false,"Authorized":true}`,
		`{"decision_id":"dec-1","Authorized":true,"authorized":false}`,
		`{"decision_id":"dec-1","AUTHORIZED":true,"Decision_Id":"dec-2"}`,
		`{"decision_id":"dec-1","deciſion_id":"dec-2"}`, // ſ folds to s
	} {
		c, _ := newTestClient(t, respondRaw(body))
		dec, err := c.Boundary.Decide(context.Background(), map[string]interface{}{})
		if err != nil {
			t.Fatal(err)
		}
		if dec.Authorized || dec.Allowed || dec.DecisionID != "dec-1" {
			t.Errorf("%s: authorized %v, decision_id %q", body, dec.Authorized, dec.DecisionID)
		}
	}
}

func TestClient_ReadsNestedKeysInTheirOwnCaseOnly(t *testing.T) {
	c, _ := newTestClient(t, respondRaw(
		`{"intent_id":"i-1","declared_sources":[{"source_id":"s-1","Source_Id":"s-2"}],"Estimated_Volume_Bytes":5}`))
	intent, err := c.DataUsage.CreateIntent(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if len(intent.DeclaredSources) != 1 || intent.DeclaredSources[0].SourceID != "s-1" || intent.EstimatedVolumeBytes != nil {
		t.Errorf("got %+v", intent)
	}

	c, _ = newTestClient(t, respondRaw(`{"Valid":true,"Accepted":true,"reason":"r"}`))
	verified, err := c.DataUsage.Verify(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if verified.Valid || verified.Accepted || verified.Reason != "r" {
		t.Errorf("got %+v", verified)
	}

	c, _ = newTestClient(t, respondRaw(
		`{"agreement_id":"a-1","agreed_terms":{"capabilities":["read"],"Capabilities":["write"]}}`))
	agreement, err := c.Agreement.Accept(context.Background(), &OfferRecord{})
	if err != nil {
		t.Fatal(err)
	}
	if len(agreement.Capabilities) != 1 || agreement.Capabilities[0] != "read" {
		t.Errorf("got %v", agreement.Capabilities)
	}
}

func TestClient_RefusesADuplicateKey(t *testing.T) {
	c, _ := newTestClient(t, respondRaw(`{"decision_id":"dec-1","authorized":false,"authorized":true}`))
	_, err := c.Boundary.Decide(context.Background(), map[string]interface{}{})
	var strict *StrictJSONError
	if !errors.As(err, &strict) || strict.Reason != "duplicate_key" {
		t.Fatalf("got %v", err)
	}
}
