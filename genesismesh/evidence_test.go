package genesismesh

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEvidenceClient_Build_HappyPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/trust-evidence" {
			http.NotFound(w, r)
			return
		}
		respondJSON(w, TrustEvidence{EvidenceID: "ev-1", Verdict: "allow"})
	})
	ev, err := c.Evidence.Build(context.Background(), TrustDecision{
		SourceSovereignID: "NA-ALPHA",
		TargetSovereignID: "NA-BETA",
		Verdict:           "allow",
		Reason:            "policy check passed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ev.EvidenceID != "ev-1" {
		t.Errorf("evidence_id = %q, want ev-1", ev.EvidenceID)
	}
	if ev.Verdict != "allow" {
		t.Errorf("verdict = %q, want allow", ev.Verdict)
	}
}

func TestEvidenceClient_Build_WrapsDecisionInBody(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad body", 400)
			return
		}
		if _, ok := body["decision"]; !ok {
			http.Error(w, "missing decision key", 422)
			return
		}
		respondJSON(w, TrustEvidence{EvidenceID: "ev-wrap", Verdict: "block"})
	})
	ev, err := c.Evidence.Build(context.Background(), TrustDecision{
		SourceSovereignID: "NA-A",
		TargetSovereignID: "NA-B",
		Verdict:           "block",
		Reason:            "blocked by policy",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ev.EvidenceID != "ev-wrap" {
		t.Errorf("evidence_id = %q, want ev-wrap", ev.EvidenceID)
	}
}

func TestEvidenceClient_Build_AdminHeaderPresent(t *testing.T) {
	var gotKeyID string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotKeyID = r.Header.Get("X-Admin-Key-Id")
		respondJSON(w, TrustEvidence{EvidenceID: "ev-hdr", Verdict: "warn"})
	})
	_, err := c.Evidence.Build(context.Background(), TrustDecision{
		Verdict: "warn",
		Reason:  "elevated risk",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotKeyID == "" {
		t.Error("X-Admin-Key-Id header not sent on admin route")
	}
}

func TestEvidenceClient_Build_DecisionFieldsPreserved(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Decision TrustDecision `json:"decision"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad body", 400)
			return
		}
		if body.Decision.Verdict != "escalate" {
			http.Error(w, "wrong verdict in decision", 422)
			return
		}
		respondJSON(w, TrustEvidence{EvidenceID: "ev-fields", Verdict: "escalate"})
	})
	ev, err := c.Evidence.Build(context.Background(), TrustDecision{
		SourceSovereignID: "NA-SRC",
		TargetSovereignID: "NA-TGT",
		Verdict:           "escalate",
		Reason:            "requires human review",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ev.EvidenceID != "ev-fields" {
		t.Errorf("evidence_id = %q, want ev-fields", ev.EvidenceID)
	}
}

func TestEvidenceClient_Verify_PublicRouteNoSigningKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/trust-evidence/verify" {
			http.NotFound(w, r)
			return
		}
		respondJSON(w, VerifyResult{Valid: true, Reason: "evidence authentic"})
	}))
	t.Cleanup(srv.Close)
	c, err := NewClient(ClientOptions{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.Evidence.Verify(context.Background(), map[string]interface{}{"evidence_id": "ev-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid {
		t.Error("expected valid=true")
	}
}

func TestEvidenceClient_Build_WrongPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	_, err := c.Evidence.Build(context.Background(), TrustDecision{Verdict: "allow", Reason: "ok"})
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if _, ok := err.(*NotFoundError); !ok {
		t.Errorf("want *NotFoundError, got %T: %v", err, err)
	}
}
