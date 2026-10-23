package genesismesh

import (
	"errors"
	"fmt"
	"testing"
)

// naError is the error body a Genesis Mesh NA sends (genesis_mesh/na_service/errors.py).
func naError(code, message string) []byte {
	return []byte(fmt.Sprintf(`{"error":{"code":%q,"message":%q,"details":{},"request_id":"req-1"}}`, code, message))
}

// The status and code pairs below are ones a Genesis Mesh NA sends.
func TestParseErrorResponse_400(t *testing.T) {
	err := parseErrorResponse(400, naError("invalid_json", "request body is not accepted JSON (duplicate_key)"))
	var target *BadRequestError
	if !errors.As(err, &target) {
		t.Fatalf("want BadRequestError, got %T", err)
	}
	if target.Code != "invalid_json" {
		t.Errorf("code = %q", target.Code)
	}
}

func TestParseErrorResponse_401(t *testing.T) {
	err := parseErrorResponse(401, naError("admin_auth_failed", "Unauthorized"))
	var target *UnauthorizedError
	if !errors.As(err, &target) {
		t.Errorf("want UnauthorizedError, got %T", err)
	}
}

func TestParseErrorResponse_403(t *testing.T) {
	err := parseErrorResponse(403, naError("insufficient_operator_tier", "This operation requires the security operator tier."))
	var target *ForbiddenError
	if !errors.As(err, &target) {
		t.Fatalf("want ForbiddenError, got %T", err)
	}
	if target.Status != 403 || target.Code != "insufficient_operator_tier" {
		t.Errorf("got %d %q", target.Status, target.Code)
	}
}

func TestParseErrorResponse_404(t *testing.T) {
	err := parseErrorResponse(404, naError("not_found", "Not found"))
	var target *NotFoundError
	if !errors.As(err, &target) {
		t.Errorf("want NotFoundError, got %T", err)
	}
}

func TestParseErrorResponse_409(t *testing.T) {
	err := parseErrorResponse(409, naError("boundary_policy_required", "Boundary policy enforcement is required; use /admin/boundary/evaluate"))
	var target *ConflictError
	if !errors.As(err, &target) {
		t.Fatalf("want ConflictError, got %T", err)
	}
	if target.Code != "boundary_policy_required" {
		t.Errorf("code = %q", target.Code)
	}
}

func TestParseErrorResponse_413(t *testing.T) {
	err := parseErrorResponse(413, naError("request_entity_too_large", "The data value transmitted exceeds the capacity limit."))
	var target *PayloadTooLargeError
	if !errors.As(err, &target) {
		t.Fatalf("want PayloadTooLargeError, got %T", err)
	}
	if target.Code != "request_entity_too_large" {
		t.Errorf("code = %q", target.Code)
	}
}

func TestParseErrorResponse_422(t *testing.T) {
	err := parseErrorResponse(422, naError("request_validation_failed", "Request validation failed"))
	var target *ValidationError
	if !errors.As(err, &target) {
		t.Errorf("want ValidationError, got %T", err)
	}
}

func TestParseErrorResponse_429(t *testing.T) {
	err := parseErrorResponse(429, naError("rate_limit_exceeded", "Rate limit exceeded"))
	var target *RateLimitError
	if !errors.As(err, &target) {
		t.Errorf("want RateLimitError, got %T", err)
	}
}

// Another 4xx (here the NA's 405 for a method a route does not serve) is a
// ClientError: the request was wrong, so repeating it cannot help.
func TestParseErrorResponse_Other4xx(t *testing.T) {
	err := parseErrorResponse(405, naError("method_not_allowed", "The method is not allowed for the requested URL."))
	var target *ClientError
	if !errors.As(err, &target) {
		t.Fatalf("want ClientError, got %T", err)
	}
	if target.Status != 405 || target.Code != "method_not_allowed" {
		t.Errorf("got %d %q", target.Status, target.Code)
	}
}

func TestParseErrorResponse_500(t *testing.T) {
	err := parseErrorResponse(500, naError("internal_server_error", "Internal server error"))
	var target *ServerError
	if !errors.As(err, &target) {
		t.Errorf("want ServerError, got %T", err)
	}
}

func TestParseErrorResponse_503(t *testing.T) {
	err := parseErrorResponse(503, naError("service_unavailable", "Service unavailable"))
	var target *ServerError
	if !errors.As(err, &target) {
		t.Errorf("want ServerError, got %T", err)
	}
}

// v1.8.0: a 4xx is never a ServerError. Before, a 403 (insufficient_operator_tier),
// a 409 (boundary_policy_required) or a 413 came back as ServerError, so a
// policy retrying server errors retried a request that could not succeed.
func TestOnlyA5xxIsAServerError(t *testing.T) {
	cases := []struct {
		status int
		code   string
	}{
		{400, "bad_request"}, {401, "admin_auth_failed"}, {403, "insufficient_operator_tier"},
		{404, "not_found"}, {405, "method_not_allowed"}, {409, "boundary_policy_required"},
		{413, "request_entity_too_large"}, {422, "validation_failed"}, {429, "rate_limit_exceeded"},
		{500, "internal_server_error"}, {503, "service_unavailable"},
	}
	for _, c := range cases {
		err := parseErrorResponse(c.status, naError(c.code, "message"))
		var server *ServerError
		if got, want := errors.As(err, &server), c.status >= 500; got != want {
			t.Errorf("%d %s: ServerError %v, want %v (%T)", c.status, c.code, got, want, err)
		}
	}
}

// Every error for an NA error response is an APIError, whatever its type, so
// a caller can read the status and code without naming each type.
func TestEveryNAErrorIsAnAPIError(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404, 405, 409, 413, 422, 429, 500, 502, 503} {
		err := parseErrorResponse(status, naError("some_code", "message"))
		var apiErr APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("%d: %T is not an APIError", status, err)
		}
		if apiErr.StatusCode() != status || apiErr.ErrorCode() != "some_code" {
			t.Errorf("%d: got %d %q", status, apiErr.StatusCode(), apiErr.ErrorCode())
		}
		if apiErr.Error() != fmt.Sprintf("genesismesh: HTTP %d some_code: message", status) {
			t.Errorf("%d: message %q", status, apiErr.Error())
		}
	}
	var network APIError
	if errors.As(&NetworkError{Cause: errors.New("connection refused")}, &network) {
		t.Error("a NetworkError is not an NA error response")
	}
}

func TestParseErrorResponse_NotJSON(t *testing.T) {
	err := parseErrorResponse(502, []byte("<html>Bad Gateway</html>"))
	var target *ServerError
	if !errors.As(err, &target) {
		t.Fatalf("want ServerError, got %T", err)
	}
	if target.Message != "<html>Bad Gateway</html>" || target.Code != "" {
		t.Errorf("got %q %q", target.Code, target.Message)
	}
}

func TestNetworkError(t *testing.T) {
	inner := errors.New("connection refused")
	err := &NetworkError{Cause: inner}
	if !errors.Is(err, inner) {
		t.Error("expected Unwrap to reach inner error")
	}
}
