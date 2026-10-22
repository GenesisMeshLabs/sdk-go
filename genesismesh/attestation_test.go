package genesismesh

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestAttestationClient_Issue_HappyPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/attestations" {
			http.NotFound(w, r)
			return
		}
		respondJSON(w, MembershipAttestation{
			AttestationID: "attest-1",
			Roles:         []string{"role:client"},
			IssuedAt:      "2026-01-01T00:00:00Z",
		})
	})
	att, err := c.Attestation.Issue(context.Background(), map[string]interface{}{
		"subject_sovereign_id": "NA-BETA",
		"roles":                []string{"role:client"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if att.AttestationID != "attest-1" {
		t.Errorf("attestation_id = %q, want attest-1", att.AttestationID)
	}
	if len(att.Roles) != 1 || att.Roles[0] != "role:client" {
		t.Errorf("roles = %v, want [role:client]", att.Roles)
	}
}

func TestAttestationClient_Issue_AdminHeaderPresent(t *testing.T) {
	var gotKeyID string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotKeyID = r.Header.Get("X-Admin-Key-Id")
		respondJSON(w, MembershipAttestation{AttestationID: "attest-hdr"})
	})
	_, err := c.Attestation.Issue(context.Background(), map[string]interface{}{
		"subject_sovereign_id": "NA-BETA",
		"roles":                []string{"role:anchor"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotKeyID == "" {
		t.Error("X-Admin-Key-Id header not sent on admin route")
	}
}

func TestAttestationClient_Revoke_UsesIDInPath(t *testing.T) {
	var capturedPath string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(200)
	})
	err := c.Attestation.Revoke(context.Background(), "attest-xyz", map[string]interface{}{
		"reason": "key compromise",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(capturedPath, "attest-xyz") {
		t.Errorf("path %q does not contain attestation ID", capturedPath)
	}
	if capturedPath != "/admin/attestations/attest-xyz/revoke" {
		t.Errorf("path = %q, want /admin/attestations/attest-xyz/revoke", capturedPath)
	}
}

func TestAttestationClient_Revoke_AdminHeaderPresent(t *testing.T) {
	var gotKeyID string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotKeyID = r.Header.Get("X-Admin-Key-Id")
		w.WriteHeader(200)
	})
	err := c.Attestation.Revoke(context.Background(), "attest-abc", map[string]interface{}{
		"reason": "expired",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotKeyID == "" {
		t.Error("X-Admin-Key-Id header not sent on revoke route")
	}
}

func TestAttestationClient_SavePolicy_HappyPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/recognition-policy" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(200)
	})
	err := c.Attestation.SavePolicy(context.Background(), map[string]interface{}{
		"recognition_policy": map[string]interface{}{
			"local_sovereign_id": "NA-LOCAL",
			"recognized_issuers": []map[string]interface{}{
				{"sovereign_id": "NA-REMOTE", "public_keys": []string{"cHVibGljLWtleQ=="}, "allowed_roles": []string{"role:client"}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAttestationClient_SavePolicy_WrongPath(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	err := c.Attestation.SavePolicy(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if _, ok := err.(*NotFoundError); !ok {
		t.Errorf("want *NotFoundError, got %T: %v", err, err)
	}
}

// A RecognitionPolicy is sent as the NA's RecognitionPolicy: each issuer as
// sovereign_id with its public_keys. Before v1.7.1 the type wrote
// issuer_sovereign_id and had no public keys, so the NA refused every policy
// built from it (422 invalid_recognition_policy).
func TestAttestationClient_SavePolicy_RecognitionPolicyWireShape(t *testing.T) {
	var body []byte
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		respondJSON(w, map[string]interface{}{"policy_id": "recognition-NA-LOCAL", "active": true})
	})
	policy := RecognitionPolicy{
		LocalSovereignID: "NA-LOCAL",
		RecognizedIssuers: []RecognizedIssuer{
			{SovereignID: "NA-REMOTE", PublicKeys: []string{"cHVibGljLWtleQ=="}, AllowedRoles: []string{"role:client"}},
			{IssuerSovereignID: "NA-OLD", PublicKeys: []string{"b2xkLWtleQ=="}},
		},
	}
	if err := c.Attestation.SavePolicy(context.Background(), map[string]interface{}{"recognition_policy": policy}); err != nil {
		t.Fatal(err)
	}
	var sent struct {
		RecognitionPolicy map[string]json.RawMessage `json:"recognition_policy"`
	}
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatal(err)
	}
	var issuers []map[string]interface{}
	if err := json.Unmarshal(sent.RecognitionPolicy["recognized_issuers"], &issuers); err != nil {
		t.Fatal(err)
	}
	want := []map[string]interface{}{
		{"sovereign_id": "NA-REMOTE", "public_keys": []interface{}{"cHVibGljLWtleQ=="}, "allowed_roles": []interface{}{"role:client"}},
		{"sovereign_id": "NA-OLD", "public_keys": []interface{}{"b2xkLWtleQ=="}},
	}
	if !reflect.DeepEqual(issuers, want) {
		t.Errorf("recognized_issuers = %v, want %v", issuers, want)
	}
	if _, ok := sent.RecognitionPolicy["revoked_attestation_ids"]; ok {
		t.Error("an empty revoked_attestation_ids is sent; the NA's default is an empty set")
	}
}

// An empty policy sends no null lists, which the NA would refuse.
func TestRecognitionPolicy_EmptyListsAreNotNull(t *testing.T) {
	data, err := json.Marshal(RecognitionPolicy{LocalSovereignID: "NA-LOCAL"})
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"local_sovereign_id":"NA-LOCAL"}` {
		t.Errorf("RecognitionPolicy{} = %s", data)
	}
	var issuer RecognizedIssuer
	if err := json.Unmarshal([]byte(`{"sovereign_id":"NA-REMOTE","public_keys":["a2V5"]}`), &issuer); err != nil {
		t.Fatal(err)
	}
	if issuer.SovereignID != "NA-REMOTE" || issuer.IssuerSovereignID != "NA-REMOTE" {
		t.Errorf("decoded issuer = %+v", issuer)
	}
}
