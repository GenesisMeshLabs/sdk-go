package genesismesh

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
)

// v1.7.1: a caller-supplied value interpolated into a request path is one
// path segment, or the call fails before anything is signed or sent. Before,
// Revoke("../../recognition-policy") signed and sent a request to another
// admin route, and "a/b" or "x?y" changed the path the NA routed on.
func TestAttestationRevokeRefusesAnIDThatIsNotOnePathSegment(t *testing.T) {
	for _, id := range []string{
		"", ".", "..", "a/b", "../../recognition-policy", "x/../../recognition-policy",
		"x?y", "x#y", "x%2Fy", "%2e%2e", "a\nb", "a\tb", "a\x00b", "a\x7fb",
	} {
		var mu sync.Mutex
		var requests []string
		c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			requests = append(requests, r.Method+" "+r.URL.RequestURI())
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		})
		err := c.Attestation.Revoke(context.Background(), id, map[string]interface{}{"reason": "test"})
		mu.Lock()
		sent := append([]string(nil), requests...)
		mu.Unlock()
		if len(sent) > 0 {
			t.Errorf("Revoke(%q) sent %v; want no request", id, sent)
		}
		if err == nil {
			t.Errorf("Revoke(%q) returned no error", id)
			continue
		}
		var refused *PathSegmentError
		if !errors.As(err, &refused) || refused.Param != "attestation_id" || refused.Value != id {
			t.Errorf("Revoke(%q) = %v (%T); want a *PathSegmentError for attestation_id", id, err, err)
		}
		var api APIError
		if errors.As(err, &api) {
			t.Errorf("Revoke(%q): a refused path segment is not an APIError (no response came back)", id)
		}
	}
}

func TestAttestationRevokeAcceptsTheNAsAttestationIDs(t *testing.T) {
	for _, id := range []string{"7d9f2c1e-3b4a-4f6e-9a8b-0c1d2e3f4a5b", "attest-xyz", "a.b", "...", "a b", "é"} {
		var mu sync.Mutex
		var path string
		c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			path = r.URL.Path
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		})
		if err := c.Attestation.Revoke(context.Background(), id, map[string]interface{}{"reason": "test"}); err != nil {
			t.Errorf("Revoke(%q): %v", id, err)
			continue
		}
		mu.Lock()
		got := path
		mu.Unlock()
		if want := "/admin/attestations/" + id + "/revoke"; got != want {
			t.Errorf("Revoke(%q) called %q, want %q", id, got, want)
		}
	}
}
