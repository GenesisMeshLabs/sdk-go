package genesismesh

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// ClientOptions configures a Client.
type ClientOptions struct {
	BaseURL    string
	SigningKey string // base64-encoded 32-byte Ed25519 seed (required for admin routes)
	KeyID      string // identifies the signing key in signatures
	Timeout    time.Duration
	// Audience is the NA's public key, which admin signatures name (signature
	// version 2). When empty it is read once from the NA's /sovereign.json
	// (network_authority.public_key).
	Audience string
}

// transport handles HTTP communication with the NA.
type transport struct {
	baseURL    string
	httpClient *http.Client
	privateKey ed25519.PrivateKey
	keyID      string

	audienceMu sync.Mutex
	audience   string
}

func newTransport(opts ClientOptions) (*transport, error) {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	t := &transport{
		baseURL:    opts.BaseURL,
		httpClient: &http.Client{Timeout: timeout},
		keyID:      opts.KeyID,
		audience:   opts.Audience,
	}
	if opts.SigningKey != "" {
		priv, _, err := LoadPrivateKey(opts.SigningKey)
		if err != nil {
			return nil, err
		}
		t.privateKey = priv
	}
	return t, nil
}

func (t *transport) adminPost(ctx context.Context, path string, body interface{}, out interface{}) error {
	if t.privateKey == nil {
		return fmt.Errorf("genesismesh: signing key required for admin route %s", path)
	}
	audience, err := t.adminAudience(ctx)
	if err != nil {
		return err
	}
	headers, err := BuildAdminHeaders(AdminRequest{
		Method: http.MethodPost, Path: path, Audience: audience, Body: body,
	}, t.keyID, t.privateKey)
	if err != nil {
		return err
	}
	return t.do(ctx, http.MethodPost, path, body, map[string]string{
		"X-Admin-Key-Id":    headers.KeyID,
		"X-Admin-Signature": headers.Signature,
		"X-Admin-Timestamp": headers.Timestamp,
		"X-Admin-Nonce":     headers.Nonce,
	}, out)
}

// adminAudience returns the NA's public key for admin signatures, read once
// from /sovereign.json; a failed lookup is retried by the next request.
func (t *transport) adminAudience(ctx context.Context) (string, error) {
	t.audienceMu.Lock()
	defer t.audienceMu.Unlock()
	if t.audience != "" {
		return t.audience, nil
	}
	var meta struct {
		NetworkAuthority struct {
			PublicKey string `json:"public_key"`
		} `json:"network_authority"`
	}
	if err := t.do(ctx, http.MethodGet, "/sovereign.json", nil, nil, &meta); err != nil {
		return "", fmt.Errorf("genesismesh: read NA public key from /sovereign.json: %w", err)
	}
	if meta.NetworkAuthority.PublicKey == "" {
		return "", fmt.Errorf("genesismesh: /sovereign.json has no network_authority.public_key")
	}
	t.audience = meta.NetworkAuthority.PublicKey
	return t.audience, nil
}

func (t *transport) publicPost(ctx context.Context, path string, body interface{}, out interface{}) error {
	return t.do(ctx, http.MethodPost, path, body, nil, out)
}

func (t *transport) publicGet(ctx context.Context, path string, out interface{}) error {
	return t.do(ctx, http.MethodGet, path, nil, nil, out)
}

func (t *transport) do(ctx context.Context, method, path string, body interface{}, extraHeaders map[string]string, out interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("genesismesh: marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, t.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("genesismesh: build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return &NetworkError{Cause: err}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return &NetworkError{Cause: err}
	}

	if resp.StatusCode >= 400 {
		return parseErrorResponse(resp.StatusCode, raw)
	}

	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("genesismesh: decode response: %w", err)
		}
	}
	return nil
}
