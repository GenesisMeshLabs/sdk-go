package genesismesh

import (
	"context"
	"net/http"
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
				{"issuer_sovereign_id": "NA-REMOTE", "allowed_roles": []string{"role:client"}},
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
