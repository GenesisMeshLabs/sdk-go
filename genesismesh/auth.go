package genesismesh

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
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

// AdminSignatureVersion is the admin signature format this SDK produces
// (Genesis Mesh 1.0.2).
const AdminSignatureVersion = 2

// AdminRequest is what an admin signature binds (signature version 2): the
// HTTP method, the path the NA serves (decoded, without the query string),
// the query parameters as sent, the target NA's public key
// (network_authority.public_key in its /sovereign.json) and the JSON body
// (nil signs {}).
type AdminRequest struct {
	Method   string
	Path     string
	Query    map[string][]string
	Audience string
	Body     interface{}
}

// AdminSigningPayload returns the canonical bytes an operator signs for req.
func AdminSigningPayload(req AdminRequest, keyID, timestamp, nonce string) ([]byte, error) {
	// A decoded path may itself contain '?' (from %3F); query parameters go in Query.
	if !strings.HasPrefix(req.Path, "/") {
		return nil, fmt.Errorf("genesismesh: admin request path must start with /")
	}
	body := req.Body
	if body == nil {
		body = map[string]interface{}{}
	}
	query := map[string][]string{}
	for name, values := range req.Query {
		query[name] = append([]string{}, values...)
	}
	return canonicalJSON(map[string]interface{}{
		"v":         AdminSignatureVersion,
		"method":    strings.ToUpper(req.Method),
		"path":      req.Path,
		"query":     query,
		"audience":  req.Audience,
		"body":      body,
		"key_id":    keyID,
		"timestamp": timestamp,
		"nonce":     nonce,
	})
}

// BuildAdminHeaders computes the four X-Admin-* headers for one admin request
// (signature version 2).
func BuildAdminHeaders(req AdminRequest, keyID string, privateKey ed25519.PrivateKey) (AdminHeaders, error) {
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000") + "Z"
	return BuildAdminHeadersAt(req, keyID, privateKey, timestamp, uuid.New().String())
}

// BuildAdminHeadersAt is BuildAdminHeaders with a fixed timestamp and nonce,
// for reproducing a signature (tests and conformance vectors).
func BuildAdminHeadersAt(req AdminRequest, keyID string, privateKey ed25519.PrivateKey, timestamp, nonce string) (AdminHeaders, error) {
	canonical, err := AdminSigningPayload(req, keyID, timestamp, nonce)
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
