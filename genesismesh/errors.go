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

// StatusCode returns the HTTP status the NA answered with (v1.7.1).
func (e *GenesisMeshError) StatusCode() int { return e.Status }

// ErrorCode returns the NA's error code, such as "insufficient_operator_tier"
// ("" when the response had no Genesis Mesh error body) (v1.7.1).
func (e *GenesisMeshError) ErrorCode() string { return e.Code }

// APIError is implemented by every error the client returns for an HTTP error
// response from the NA, whatever its type (v1.7.1): match it with errors.As to
// read the status and the code without naming each type. A NetworkError is
// not an APIError: no response came back.
type APIError interface {
	error
	StatusCode() int
	ErrorCode() string
}

// NetworkError wraps connection-level failures.
type NetworkError struct {
	Cause error
}

func (e *NetworkError) Error() string { return fmt.Sprintf("genesismesh: network error: %v", e.Cause) }
func (e *NetworkError) Unwrap() error { return e.Cause }

// PathSegmentError is a caller-supplied value refused as a segment of a
// request path (v1.7.1), such as an attestation ID containing "/" or "..":
// it would sign and call another route. It is returned before any request is
// signed or sent, so it is not an APIError.
type PathSegmentError struct {
	// Param names the value, such as "attestation_id".
	Param string
	// Value is the refused value.
	Value string
	// Reason says why, such as `contains "/"`.
	Reason string
}

func (e *PathSegmentError) Error() string {
	return fmt.Sprintf("genesismesh: %s %q is not a single path segment: %s", e.Param, e.Value, e.Reason)
}

// Typed errors for specific HTTP status codes. Every one is an APIError.
//
// A 4xx means the request cannot succeed as sent: only a RateLimitError is
// worth repeating, after a pause. A ServerError (5xx) may succeed later.
type (
	// BadRequestError is a 400, such as "invalid_json".
	BadRequestError struct{ GenesisMeshError }
	// UnauthorizedError is a 401, such as "admin_auth_failed".
	UnauthorizedError struct{ GenesisMeshError }
	// ForbiddenError is a 403 (v1.7.1): the request was authenticated, and
	// its key may not do this, such as "insufficient_operator_tier".
	ForbiddenError struct{ GenesisMeshError }
	// NotFoundError is a 404.
	NotFoundError struct{ GenesisMeshError }
	// ConflictError is a 409 (v1.7.1): the NA's state refuses the request,
	// such as "boundary_policy_required".
	ConflictError struct{ GenesisMeshError }
	// PayloadTooLargeError is a 413 (v1.7.1): the body is over the NA's
	// request size limit ("request_entity_too_large").
	PayloadTooLargeError struct{ GenesisMeshError }
	// ValidationError is a 422.
	ValidationError struct{ GenesisMeshError }
	// RateLimitError is a 429.
	RateLimitError struct{ GenesisMeshError }
	// ClientError is any other 4xx (v1.7.1), such as a 405.
	ClientError struct{ GenesisMeshError }
	// ServerError is a 5xx. Before v1.7.1 it was also every status without a
	// type of its own, so a 403, 409 or 413 read as a server error.
	ServerError struct{ GenesisMeshError }
)

func (e *BadRequestError) Error() string      { return e.GenesisMeshError.Error() }
func (e *UnauthorizedError) Error() string    { return e.GenesisMeshError.Error() }
func (e *ForbiddenError) Error() string       { return e.GenesisMeshError.Error() }
func (e *NotFoundError) Error() string        { return e.GenesisMeshError.Error() }
func (e *ConflictError) Error() string        { return e.GenesisMeshError.Error() }
func (e *PayloadTooLargeError) Error() string { return e.GenesisMeshError.Error() }
func (e *ValidationError) Error() string      { return e.GenesisMeshError.Error() }
func (e *RateLimitError) Error() string       { return e.GenesisMeshError.Error() }
func (e *ClientError) Error() string          { return e.GenesisMeshError.Error() }
func (e *ServerError) Error() string          { return e.GenesisMeshError.Error() }

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
	switch {
	case status == 400:
		return &BadRequestError{base}
	case status == 401:
		return &UnauthorizedError{base}
	case status == 403:
		return &ForbiddenError{base}
	case status == 404:
		return &NotFoundError{base}
	case status == 409:
		return &ConflictError{base}
	case status == 413:
		return &PayloadTooLargeError{base}
	case status == 422:
		return &ValidationError{base}
	case status == 429:
		return &RateLimitError{base}
	case status >= 400 && status < 500:
		return &ClientError{base}
	default:
		return &ServerError{base}
	}
}
