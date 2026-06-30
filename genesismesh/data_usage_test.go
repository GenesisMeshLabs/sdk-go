package genesismesh

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDataUsageClient_CreatePolicy_HappyPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/data-usage/policy" {
			http.NotFound(w, r)
			return
		}
		respondJSON(w, DataLicensePolicy{
			PolicyID:        "pol-1",
			AllowedPurposes: []string{"analytics", "training"},
		})
	})
	pol, err := c.DataUsage.CreatePolicy(context.Background(), map[string]interface{}{
		"local_sovereign_id": "NA-LOCAL",
		"allowed_purposes":   []string{"analytics", "training"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if pol.PolicyID != "pol-1" {
		t.Errorf("policy_id = %q, want pol-1", pol.PolicyID)
	}
	if len(pol.AllowedPurposes) != 2 {
		t.Errorf("allowed_purposes count = %d, want 2", len(pol.AllowedPurposes))
	}
}

func TestDataUsageClient_CreatePolicy_AdminHeaderPresent(t *testing.T) {
	var gotKeyID string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotKeyID = r.Header.Get("X-Admin-Key-Id")
		respondJSON(w, DataLicensePolicy{PolicyID: "pol-hdr"})
	})
	_, err := c.DataUsage.CreatePolicy(context.Background(), map[string]interface{}{
		"allowed_purposes": []string{"analytics"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotKeyID == "" {
		t.Error("X-Admin-Key-Id header not sent on admin route")
	}
}

func TestDataUsageClient_CreateIntent_HappyPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/data-usage/intent" {
			http.NotFound(w, r)
			return
		}
		respondJSON(w, DataAccessIntent{
			IntentID: "intent-1",
			Sources: []DataSourceDescriptor{
				{SourceID: "src-1", SourceType: "personal", OwnerSovereignID: "NA-OWNER"},
			},
			AccessTypes: []string{"read"},
		})
	})
	intent, err := c.DataUsage.CreateIntent(context.Background(), map[string]interface{}{
		"agent_sovereign_id": "NA-AGENT",
		"decision_id":        "dec-1",
		"sources": []map[string]interface{}{
			{"source_id": "src-1", "source_type": "personal", "owner_sovereign_id": "NA-OWNER"},
		},
		"access_types": []string{"read"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if intent.IntentID != "intent-1" {
		t.Errorf("intent_id = %q, want intent-1", intent.IntentID)
	}
	if len(intent.Sources) != 1 {
		t.Errorf("sources count = %d, want 1", len(intent.Sources))
	}
}

func TestDataUsageClient_GetPolicy_PublicRouteNoSigningKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/data-usage/policy" {
			http.NotFound(w, r)
			return
		}
		if r.Method != "GET" {
			http.Error(w, "method not allowed", 405)
			return
		}
		respondJSON(w, DataLicensePolicy{PolicyID: "pol-public", AllowedPurposes: []string{"analytics"}})
	}))
	t.Cleanup(srv.Close)
	c, err := NewClient(ClientOptions{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	pol, err := c.DataUsage.GetPolicy(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if pol.PolicyID != "pol-public" {
		t.Errorf("policy_id = %q, want pol-public", pol.PolicyID)
	}
}

func TestDataUsageClient_Verify_PublicRouteNoSigningKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/data-usage/verify" {
			http.NotFound(w, r)
			return
		}
		respondJSON(w, VerifyResult{Valid: true, Reason: "intent matches policy"})
	}))
	t.Cleanup(srv.Close)
	c, err := NewClient(ClientOptions{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.DataUsage.Verify(context.Background(), map[string]interface{}{"intent_id": "intent-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid {
		t.Error("expected valid=true")
	}
}

func TestDataUsageClient_CreateIntent_WrongPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	_, err := c.DataUsage.CreateIntent(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if _, ok := err.(*NotFoundError); !ok {
		t.Errorf("want *NotFoundError, got %T: %v", err, err)
	}
}
