package middleware

import "fmt"

// Kind classifies an application error. HTTP status is mapped from Kind in one place.
type Kind int

const (
	KindInternal Kind = iota
	KindInvalid
	KindUnprocessable
	KindConflict
	KindNotFound
	KindUnauthorized
	KindForbidden
	KindUnavailable
)

// AppError is a typed error returned by services.
type AppError struct {
	Kind    Kind
	Code    string
	Message string
	Details any
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// APIError is one entry in the JSON error envelope.
type APIError struct {
	Code    string `json:"code" example:"invalid_request"`
	Message string `json:"message" example:"request body is invalid"`
	Details any    `json:"details"`
}

// ErrorResponse is the JSON body for every failed request.
type ErrorResponse struct {
	Error APIError `json:"error"`
}

// Invalid reports a malformed request.
func Invalid(code, message string) *AppError {
	return &AppError{Kind: KindInvalid, Code: code, Message: message}
}

// Unprocessable reports a request that is well formed but cannot be applied.
func Unprocessable(code, message string) *AppError {
	return &AppError{Kind: KindUnprocessable, Code: code, Message: message}
}

// Conflict reports a colliding request.
func Conflict(code, message string) *AppError {
	return &AppError{Kind: KindConflict, Code: code, Message: message}
}

// Unauthorized reports missing or invalid credentials.
func Unauthorized(code, message string) *AppError {
	return &AppError{Kind: KindUnauthorized, Code: code, Message: message}
}

// Forbidden reports a caller who is authenticated but not allowed.
func Forbidden(code, message string) *AppError {
	return &AppError{Kind: KindForbidden, Code: code, Message: message}
}

// NotFound reports a missing record.
func NotFound(code, message string) *AppError {
	return &AppError{Kind: KindNotFound, Code: code, Message: message}
}

// Unavailable reports a dependency that is not ready.
func Unavailable(code, message string) *AppError {
	return &AppError{Kind: KindUnavailable, Code: code, Message: message}
}

// Internal reports an unexpected failure.
func Internal(code, message string) *AppError {
	return &AppError{Kind: KindInternal, Code: code, Message: message}
}
