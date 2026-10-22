package genesismesh

import (
	"context"
	"encoding/json"
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
		respondJSON(w, map[string]interface{}{
			"policy_id":             "pol-1",
			"licensor_sovereign_id": "NA-LOCAL",
			"licensee_sovereign_id": "NA-PARTNER",
			"allowed_access_types":  []string{"read", "aggregate"},
			"valid_from":            "2026-07-01T00:00:00+00:00",
		})
	})
	pol, err := c.DataUsage.CreatePolicy(context.Background(), map[string]interface{}{
		"licensee_sovereign_id": "NA-PARTNER",
		"allowed_source_ids":    []string{"src-1"},
		"allowed_access_types":  []string{"read", "aggregate"},
		"valid_from":            "2026-07-01T00:00:00+00:00",
		"valid_until":           "2026-08-01T00:00:00+00:00",
	})
	if err != nil {
		t.Fatal(err)
	}
	if pol.PolicyID != "pol-1" {
		t.Errorf("policy_id = %q, want pol-1", pol.PolicyID)
	}
	if len(pol.AllowedAccessTypes) != 2 {
		t.Errorf("allowed_access_types count = %d, want 2", len(pol.AllowedAccessTypes))
	}
	if pol.LicensorSovereignID != "NA-LOCAL" || pol.LocalSovereignID != "NA-LOCAL" {
		t.Errorf("licensor = %q, local = %q, want NA-LOCAL", pol.LicensorSovereignID, pol.LocalSovereignID)
	}
	if pol.IssuedAt != pol.ValidFrom {
		t.Errorf("issued_at = %q, want valid_from %q", pol.IssuedAt, pol.ValidFrom)
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
		respondJSON(w, map[string]interface{}{
			"intent_id": "intent-1",
			"declared_sources": []map[string]interface{}{
				{"source_id": "src-1", "source_type": "personal", "owner_sovereign_id": "NA-OWNER"},
			},
			"declared_access_types": []string{"read"},
			"declared_at":           "2026-07-01T12:00:00+00:00",
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
	if len(intent.DeclaredSources) != 1 || len(intent.Sources) != 1 {
		t.Errorf("declared_sources = %d, sources = %d, want 1", len(intent.DeclaredSources), len(intent.Sources))
	}
	if len(intent.AccessTypes) != 1 || intent.AccessTypes[0] != "read" {
		t.Errorf("access_types = %v, want [read] from declared_access_types", intent.AccessTypes)
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

// A signed intent whose source has no classification tags still verifies
// after it is decoded into a DataAccessIntent and sent back. Before v1.7.1
// DataSourceDescriptor left out the empty classification_tags list, so the
// intent sent back was no longer the one the agent signed, and the NA
// answered intent_exceeds_license ("Invalid intent signature").
func TestDataUsageClient_Verify_DecodedIntentWithEmptyTagsStillVerifies(t *testing.T) {
	v := interopVector(t, "int-001") // a valid signed intent; its one source has classification_tags []
	var intent DataAccessIntent
	if err := json.Unmarshal(v.Input["intent"], &intent); err != nil {
		t.Fatal(err)
	}
	if len(intent.DeclaredSources) != 1 || len(intent.DeclaredSources[0].ClassificationTags) != 0 {
		t.Fatalf("vector int-001 changed: sources %+v", intent.DeclaredSources)
	}
	at, err := ParseTimestamp(text(t, v.Input["at"]))
	if err != nil {
		t.Fatal(err)
	}
	agentKeys := strings_(t, v.Input["agent_public_keys"])

	// The server checks the intent it receives as the NA does: against the
	// agent's signature and the policy.
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Intent json.RawMessage `json:"intent"`
			Policy json.RawMessage `json:"policy"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		res, err := VerifyDataAccessIntent(body.Intent, body.Policy, agentKeys, at)
		if err != nil {
			t.Error(err)
		}
		respondJSON(w, map[string]interface{}{"valid": res.Valid, "violation_reason": res.ViolationReason})
	})
	result, err := c.DataUsage.Verify(context.Background(), map[string]interface{}{
		"intent": intent, "policy": v.Input["policy"],
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid {
		t.Errorf("the decoded intent sent back does not verify: %+v", result)
	}
}

// classification_tags is always a list: the NA refuses null for it.
func TestDataSourceDescriptor_EmptyTagsAreAList(t *testing.T) {
	data, err := json.Marshal(DataSourceDescriptor{SourceID: "src-a", SourceType: "public", OwnerSovereignID: "NA"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"source_id":"src-a","source_type":"public","owner_sovereign_id":"NA","classification_tags":[]}`
	if string(data) != want {
		t.Errorf("DataSourceDescriptor = %s, want %s", data, want)
	}
}
