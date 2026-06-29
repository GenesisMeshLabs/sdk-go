package genesismesh

import (
	"errors"
	"testing"
)

func TestParseErrorResponse_400(t *testing.T) {
	body := []byte(`{"error":{"message":"bad input","code":"INVALID_REQUEST"}}`)
	err := parseErrorResponse(400, body)
	var target *BadRequestError
	if !errors.As(err, &target) {
		t.Errorf("want BadRequestError, got %T", err)
	}
	if target.Code != "INVALID_REQUEST" {
		t.Errorf("code = %q", target.Code)
	}
}

func TestParseErrorResponse_401(t *testing.T) {
	err := parseErrorResponse(401, []byte(`{"error":{"message":"unauth","code":"UNAUTHORIZED"}}`))
	var target *UnauthorizedError
	if !errors.As(err, &target) {
		t.Errorf("want UnauthorizedError, got %T", err)
	}
}

func TestParseErrorResponse_404(t *testing.T) {
	err := parseErrorResponse(404, []byte(`{"error":{"message":"not found","code":"NOT_FOUND"}}`))
	var target *NotFoundError
	if !errors.As(err, &target) {
		t.Errorf("want NotFoundError, got %T", err)
	}
}

func TestParseErrorResponse_422(t *testing.T) {
	err := parseErrorResponse(422, []byte(`{"error":{"message":"invalid verdict","code":"VALIDATION_ERROR"}}`))
	var target *ValidationError
	if !errors.As(err, &target) {
		t.Errorf("want ValidationError, got %T", err)
	}
}

func TestParseErrorResponse_429(t *testing.T) {
	err := parseErrorResponse(429, []byte(`{"error":{"message":"slow down","code":"RATE_LIMITED"}}`))
	var target *RateLimitError
	if !errors.As(err, &target) {
		t.Errorf("want RateLimitError, got %T", err)
	}
}

func TestParseErrorResponse_500(t *testing.T) {
	err := parseErrorResponse(500, []byte(`{"error":{"message":"oops","code":"SERVER_ERROR"}}`))
	var target *ServerError
	if !errors.As(err, &target) {
		t.Errorf("want ServerError, got %T", err)
	}
}

func TestNetworkError(t *testing.T) {
	inner := errors.New("connection refused")
	err := &NetworkError{Cause: inner}
	if !errors.Is(err, inner) {
		t.Error("expected Unwrap to reach inner error")
	}
}
