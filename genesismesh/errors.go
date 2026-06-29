package genesismesh

import (
	"encoding/json"
	"fmt"
)

// GenesisMeshError is the base error type for all NA API errors.
type GenesisMeshError struct {
	Status  int
	Code    string
	Message string
}

func (e *GenesisMeshError) Error() string {
	return fmt.Sprintf("genesismesh: HTTP %d %s: %s", e.Status, e.Code, e.Message)
}

// NetworkError wraps connection-level failures.
type NetworkError struct {
	Cause error
}

func (e *NetworkError) Error() string { return fmt.Sprintf("genesismesh: network error: %v", e.Cause) }
func (e *NetworkError) Unwrap() error { return e.Cause }

// Typed errors for specific HTTP status codes.
type (
	BadRequestError   struct{ GenesisMeshError }
	UnauthorizedError struct{ GenesisMeshError }
	NotFoundError     struct{ GenesisMeshError }
	ValidationError   struct{ GenesisMeshError }
	RateLimitError    struct{ GenesisMeshError }
	ServerError       struct{ GenesisMeshError }
)

func (e *BadRequestError) Error() string   { return e.GenesisMeshError.Error() }
func (e *UnauthorizedError) Error() string { return e.GenesisMeshError.Error() }
func (e *NotFoundError) Error() string     { return e.GenesisMeshError.Error() }
func (e *ValidationError) Error() string   { return e.GenesisMeshError.Error() }
func (e *RateLimitError) Error() string    { return e.GenesisMeshError.Error() }
func (e *ServerError) Error() string       { return e.GenesisMeshError.Error() }

type naErrorResponse struct {
	Error struct {
		Message   string `json:"message"`
		Code      string `json:"code"`
		RequestID string `json:"request_id"`
	} `json:"error"`
}

func parseErrorResponse(status int, body []byte) error {
	var resp naErrorResponse
	_ = json.Unmarshal(body, &resp)
	base := GenesisMeshError{
		Status:  status,
		Code:    resp.Error.Code,
		Message: resp.Error.Message,
	}
	if base.Message == "" {
		base.Message = string(body)
	}
	switch status {
	case 400:
		return &BadRequestError{base}
	case 401:
		return &UnauthorizedError{base}
	case 404:
		return &NotFoundError{base}
	case 422:
		return &ValidationError{base}
	case 429:
		return &RateLimitError{base}
	default:
		return &ServerError{base}
	}
}
