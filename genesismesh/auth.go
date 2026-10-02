package genesismesh

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// canonicalJSON produces deterministic JSON matching Python's
// json.dumps(value, sort_keys=True, separators=(",",":")).
func canonicalJSON(v interface{}) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var generic interface{}
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, err
	}
	return marshalCanonical(generic)
}

// LoadPrivateKey decodes a base64-encoded 32-byte Ed25519 seed.
func LoadPrivateKey(seedBase64 string) (ed25519.PrivateKey, ed25519.PublicKey, error) {
	seed, err := base64.StdEncoding.DecodeString(seedBase64)
	if err != nil {
		seed, err = base64.RawStdEncoding.DecodeString(seedBase64)
		if err != nil {
			return nil, nil, fmt.Errorf("genesismesh: invalid signing key base64: %w", err)
		}
	}
	if len(seed) != ed25519.SeedSize {
		return nil, nil, fmt.Errorf("genesismesh: signing key must be %d bytes, got %d", ed25519.SeedSize, len(seed))
	}
	priv := ed25519.NewKeyFromSeed(seed)
	return priv, priv.Public().(ed25519.PublicKey), nil
}

// AdminHeaders holds the four headers required by NA admin routes.
type AdminHeaders struct {
	KeyID     string
	Signature string
	Timestamp string
	Nonce     string
}

// BuildAdminHeaders computes the four X-Admin-* headers for an admin request.
// body must be JSON-serialisable. The signature covers
// canonicalJSON({body, key_id, nonce, timestamp}).
func BuildAdminHeaders(body interface{}, keyID string, privateKey ed25519.PrivateKey) (AdminHeaders, error) {
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000") + "Z"
	nonce := uuid.New().String()

	payload := map[string]interface{}{
		"body":      body,
		"key_id":    keyID,
		"nonce":     nonce,
		"timestamp": timestamp,
	}
	canonical, err := canonicalJSON(payload)
	if err != nil {
		return AdminHeaders{}, fmt.Errorf("genesismesh: canonical JSON failed: %w", err)
	}
	sig := ed25519.Sign(privateKey, canonical)
	return AdminHeaders{
		KeyID:     keyID,
		Signature: base64.StdEncoding.EncodeToString(sig),
		Timestamp: timestamp,
		Nonce:     nonce,
	}, nil
}
