package genesismesh

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAgreementClient_Offer_HappyPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/agreements/offer" {
			http.NotFound(w, r)
			return
		}
		respondJSON(w, OfferRecord{OfferID: "offer-abc", CreatedAt: "2026-01-01T00:00:00Z"})
	})
	rec, err := c.Agreement.Offer(context.Background(), CapabilityOffer{
		OfferorSovereignID:   "NA-ALPHA",
		ResponderSovereignID: "NA-BETA",
		Capabilities:         []string{"read", "write"},
		Roles:                []string{"role:client"},
		ValidFrom:            "2026-01-01T00:00:00.000Z",
		ValidUntil:           "2027-01-01T00:00:00.000Z",
		ExpiresAt:            "2026-01-08T00:00:00.000Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.OfferID != "offer-abc" {
		t.Errorf("offer_id = %q, want offer-abc", rec.OfferID)
	}
}

func TestAgreementClient_Offer_AdminHeaderPresent(t *testing.T) {
	var gotKeyID string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotKeyID = r.Header.Get("X-Admin-Key-Id")
		respondJSON(w, OfferRecord{OfferID: "offer-hdr"})
	})
	_, err := c.Agreement.Offer(context.Background(), CapabilityOffer{
		ResponderSovereignID: "NA-B",
		Capabilities:         []string{"read"},
		Roles:                []string{"role:client"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotKeyID == "" {
		t.Error("X-Admin-Key-Id header not sent on admin route")
	}
}

func TestAgreementClient_Counter_HappyPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/agreements/counter" {
			http.NotFound(w, r)
			return
		}
		respondJSON(w, OfferRecord{OfferID: "counter-1"})
	})
	rec, err := c.Agreement.Counter(context.Background(), map[string]interface{}{
		"offer_id":     "offer-abc",
		"capabilities": []string{"read"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.OfferID != "counter-1" {
		t.Errorf("offer_id = %q, want counter-1", rec.OfferID)
	}
}

func TestAgreementClient_Accept_HappyPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/agreements/accept" {
			http.NotFound(w, r)
			return
		}
		respondJSON(w, AgreementRecord{AgreementID: "agr-1", Status: "active"})
	})
	agr, err := c.Agreement.Accept(context.Background(), &OfferRecord{OfferID: "offer-abc"})
	if err != nil {
		t.Fatal(err)
	}
	if agr.AgreementID != "agr-1" {
		t.Errorf("agreement_id = %q, want agr-1", agr.AgreementID)
	}
	if agr.Status != "active" {
		t.Errorf("status = %q, want active", agr.Status)
	}
}

func TestAgreementClient_Accept_WrapsOfferInBody(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad body", 400)
			return
		}
		if _, ok := body["offer"]; !ok {
			http.Error(w, "missing offer key", 422)
			return
		}
		respondJSON(w, AgreementRecord{AgreementID: "agr-2", Status: "active"})
	})
	_, err := c.Agreement.Accept(context.Background(), &OfferRecord{OfferID: "offer-xyz"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAgreementClient_Verify_PublicRouteNoSigningKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/agreements/verify" {
			http.NotFound(w, r)
			return
		}
		respondJSON(w, VerifyResult{Valid: true, Reason: "sig ok"})
	}))
	t.Cleanup(srv.Close)
	c, err := NewClient(ClientOptions{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.Agreement.Verify(context.Background(), map[string]interface{}{"agreement_id": "agr-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid {
		t.Error("expected valid=true")
	}
}

func TestAgreementClient_Verify_Returns404Error(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	_, err := c.Agreement.Verify(context.Background(), map[string]interface{}{"agreement_id": "agr-1"})
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if _, ok := err.(*NotFoundError); !ok {
		t.Errorf("want *NotFoundError, got %T: %v", err, err)
	}
}
