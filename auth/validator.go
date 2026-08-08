package auth

import "context"

// Validator turns a raw bearer token string into validated Claims, or an
// error (ErrInvalidToken, ErrTokenExpired, ErrTokenInactive, ...).
// JWKSValidator and IntrospectionValidator both implemented it, selected via
// Config.Mode
type Validator interface {
	Validate(ctx context.Context, rawToken string) (*Claims, error)
}
