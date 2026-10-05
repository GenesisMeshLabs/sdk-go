package genesismesh

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// The shared reference vectors (genesismesh conformance/vectors/admin_auth.json),
// copied unchanged. Seed "a" of the reference suite is bytes 0..31.
func TestAdminSignatureConformanceVectors(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "conformance", "admin_auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	var suite struct {
		Vectors []struct {
			ID    string `json:"id"`
			Input struct {
				Method    string              `json:"method"`
				Path      string              `json:"path"`
				Query     map[string][]string `json:"query"`
				Audience  string              `json:"audience"`
				Body      json.RawMessage     `json:"body"`
				KeyID     string              `json:"key_id"`
				Timestamp string              `json:"timestamp"`
				Nonce     string              `json:"nonce"`
			} `json:"input"`
			Expected struct {
				Payload   string `json:"payload"`
				Signature string `json:"signature_b64"`
			} `json:"expected"`
		} `json:"vectors"`
	}
	if err := json.Unmarshal(raw, &suite); err != nil {
		t.Fatal(err)
	}
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	for _, v := range suite.Vectors {
		t.Run(v.ID, func(t *testing.T) {
			body, err := decodeJSON(v.Input.Body)
			if err != nil {
				t.Fatal(err)
			}
			req := AdminRequest{Method: v.Input.Method, Path: v.Input.Path, Query: v.Input.Query, Audience: v.Input.Audience, Body: body}
			payload, err := AdminSigningPayload(req, v.Input.KeyID, v.Input.Timestamp, v.Input.Nonce)
			if err != nil {
				t.Fatal(err)
			}
			if string(payload) != v.Expected.Payload {
				t.Fatalf("payload\n got %s\nwant %s", payload, v.Expected.Payload)
			}
			headers, err := BuildAdminHeadersAt(req, v.Input.KeyID, priv, v.Input.Timestamp, v.Input.Nonce)
			if err != nil {
				t.Fatal(err)
			}
			if headers.Signature != v.Expected.Signature {
				t.Fatalf("signature = %s, want %s", headers.Signature, v.Expected.Signature)
			}
		})
	}
}

func TestAdminSignatureBindsTheRequest(t *testing.T) {
	priv := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	req := AdminRequest{Method: "POST", Path: "/admin/invite", Audience: "TEST", Body: map[string]interface{}{}}
	headers, err := BuildAdminHeaders(req, "k", priv)
	if err != nil {
		t.Fatal(err)
	}
	sig := mustDecodeB64(t, headers.Signature)
	for name, other := range map[string]AdminRequest{
		"method":   {Method: "PUT", Path: req.Path, Audience: req.Audience},
		"path":     {Method: req.Method, Path: "/admin/revoke", Audience: req.Audience},
		"query":    {Method: req.Method, Path: req.Path, Audience: req.Audience, Query: map[string][]string{"limit": {"1"}}},
		"audience": {Method: req.Method, Path: req.Path, Audience: "OTHER"},
	} {
		payload, err := AdminSigningPayload(other, "k", headers.Timestamp, headers.Nonce)
		if err != nil {
			t.Fatal(err)
		}
		if ed25519.Verify(priv.Public().(ed25519.PublicKey), payload, sig) {
			t.Errorf("signature still verifies with a different %s", name)
		}
	}
	if _, err := AdminSigningPayload(AdminRequest{Method: "GET", Path: "nodes", Audience: "TEST"}, "k", "t", "n"); err == nil {
		t.Error("a relative path must be refused")
	}
	if _, err := AdminSigningPayload(AdminRequest{Method: "POST", Path: "/admin/attestations/a?b/revoke", Audience: "TEST"}, "k", "t", "n"); err != nil {
		t.Errorf("a decoded '?' inside a path must be signed: %v", err)
	}
}

func TestAdminAudienceIsReadOnceFromSovereignJSON(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/sovereign.json" {
			respondJSON(w, map[string]interface{}{"network_authority": map[string]string{"public_key": "NA-PUBLIC-KEY"}})
			return
		}
		w.WriteHeader(http.StatusCreated)
		respondJSON(w, map[string]string{"commitment_id": "c1"})
	}))
	t.Cleanup(srv.Close)
	c, err := NewClient(ClientOptions{BaseURL: srv.URL, SigningKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", KeyID: "k"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := c.Disclosure.Commit(context.Background(), map[string]interface{}{"capability": "read"}); err != nil {
			t.Fatal(err)
		}
	}
	if len(paths) != 3 || paths[0] != "/sovereign.json" || paths[1] != "/admin/disclosure/commit" {
		t.Fatalf("requests = %v", paths)
	}
}

func mustDecodeB64(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
