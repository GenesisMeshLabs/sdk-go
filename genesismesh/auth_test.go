package genesismesh

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
)

func TestCanonicalJSON_SortsKeys(t *testing.T) {
	input := map[string]interface{}{"z": 1, "a": 2, "m": 3}
	got, err := canonicalJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a":2,"m":3,"z":1}`
	if string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestCanonicalJSON_Nested(t *testing.T) {
	input := map[string]interface{}{
		"body":      map[string]interface{}{"foo": "bar"},
		"key_id":    "k1",
		"nonce":     "n1",
		"timestamp": "2026-01-01T00:00:00Z",
	}
	got, err := canonicalJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"body":{"foo":"bar"},"key_id":"k1","nonce":"n1","timestamp":"2026-01-01T00:00:00Z"}`
	if string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestLoadPrivateKey(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	encoded := base64.StdEncoding.EncodeToString(seed)
	priv, pub, err := LoadPrivateKey(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(priv) != ed25519.PrivateKeySize {
		t.Errorf("private key size %d, want %d", len(priv), ed25519.PrivateKeySize)
	}
	if len(pub) != ed25519.PublicKeySize {
		t.Errorf("public key size %d, want %d", len(pub), ed25519.PublicKeySize)
	}
}

func TestLoadPrivateKey_BadBase64(t *testing.T) {
	_, _, err := LoadPrivateKey("not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for bad base64")
	}
}

func TestLoadPrivateKey_WrongLength(t *testing.T) {
	bad := base64.StdEncoding.EncodeToString([]byte("tooshort"))
	_, _, err := LoadPrivateKey(bad)
	if err == nil {
		t.Error("expected error for wrong seed length")
	}
}

func TestBuildAdminHeaders(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	priv := ed25519.NewKeyFromSeed(seed)
	headers, err := BuildAdminHeaders(map[string]string{"foo": "bar"}, "test-key", priv)
	if err != nil {
		t.Fatal(err)
	}
	if headers.KeyID != "test-key" {
		t.Errorf("key_id = %q, want %q", headers.KeyID, "test-key")
	}
	if headers.Signature == "" {
		t.Error("signature is empty")
	}
	if headers.Timestamp == "" {
		t.Error("timestamp is empty")
	}
	if headers.Nonce == "" {
		t.Error("nonce is empty")
	}
}
