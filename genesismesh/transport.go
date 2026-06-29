package genesismesh

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ClientOptions configures a Client.
type ClientOptions struct {
	BaseURL    string
	SigningKey string // base64-encoded 32-byte Ed25519 seed (required for admin routes)
	KeyID      string // identifies the signing key in signatures
	Timeout    time.Duration
}

// transport handles HTTP communication with the NA.
type transport struct {
	baseURL    string
	httpClient *http.Client
	privateKey ed25519.PrivateKey
	keyID      string
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
	headers, err := BuildAdminHeaders(body, t.keyID, t.privateKey)
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
