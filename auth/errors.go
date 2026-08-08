package auth

import "errors"

var (
	// ErrMissingToken is returned when no bearer token was found on the request
	ErrMissingToken = errors.New("auth: missing bearer token")

	// ErrInvalidToken covers malformed tokens, bad signatures, wrong issuers, etc.
	ErrInvalidToken = errors.New("auth: invalid token")

	// ErrTokenExpired is returned when the token's exp claim is in the past.
	ErrTokenExpired = errors.New("auth: token expired")

	// ErrTokenInactive is returned by the introspection validator when the
	// IdP reports the token as no longer active (revoked, logged out, etc.)
	ErrTokenInactive = errors.New("auth: token inactive")

	// ErrInsufficientRole is returned when the caller is authenticated but
	// lacks a role required by RequireRoles.
	ErrInsufficientRole = errors.New("auth: insufficient role")

	// ErrDiscoveryFailed is returned when the OIDC discovery document
	// could not be fetched or parsed
	ErrDiscoveryFailed = errors.New("auth: OIDC discovery falied")
)
