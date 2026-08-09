// Package errors provides a single structured application error type used
// across the SDK: every package (auth, database, messaging, server) wraps
// failures in *errors.Error so HTTP/gRPC layers can map them to a status
// code and clients get a stable machine-readable "code" field instead of
// parsing free-text messages.
package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// Code is a stable, machine-readable error category. Add new ones here
// rather than inventing ad-hoc strings at call sites.
type Code string

const (
	CodeInvalidArgument Code = "invalid_argument"
	CodeUnauthorized    Code = "unauthorized"
	CodeForbidden       Code = "forbidden"
	CodeNotFound        Code = "not_found"
	CodeConflict        Code = "conflict"
	CodeRateLimited     Code = "rate_limited"
	CodeUnavailable     Code = "unavailable"
	CodeInternal        Code = "internal"
)

var codeToStatus = map[Code]int{
	CodeInvalidArgument: http.StatusBadRequest,
	CodeUnauthorized:    http.StatusUnauthorized,
	CodeForbidden:       http.StatusForbidden,
	CodeNotFound:        http.StatusNotFound,
	CodeConflict:        http.StatusConflict,
	CodeRateLimited:     http.StatusTooManyRequests,
	CodeUnavailable:     http.StatusServiceUnavailable,
	CodeInternal:        http.StatusInternalServerError,
}

// Error is the SDK's structured application error. Message is safe to
// show to a caller; the wrapped cause (Err) is for logs/traces only -
// server middleware must never serialize err.Error() straight into an
// HTTP response body, only Message
type Error struct {
	Code    Code
	Message string
	Err     error
	Details map[string]any
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

// HTTPStatus maps Code to a HTTP status code. Unknown codes map to 500
func (e *Error) HTTPStatus() int {
	if s, ok := codeToStatus[e.Code]; ok {
		return s
	}
	return http.StatusInternalServerError
}

// New creates an Error with no wrapped cause
func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Wrap creates an Error that wraps cause. Use this at the boundary where
// an internal error (DB driver, HTTP client, ...) becomess a domain error
func Wrap(code Code, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Err: cause}
}

// WithDetails attaches structured context (field-level validation errors,
// the resources ID that conflicted, etc.) and returns e for chaining
func (e *Error) WithDetails(details map[string]any) *Error {
	e.Details = details
	return e
}

// As pulls an *Error out of an error chain, e.g. in HTTP middleware
// deciding how to respond
func As(err error) (*Error, bool) {
	return errors.AsType[*Error](err)
}

// Is reports whether err has the given Code, walking wrapped errors
func Is(err error, code Code) bool {
	e, ok := As(err)
	return ok && e.Code == code
}
