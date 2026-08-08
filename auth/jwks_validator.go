package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// JWKSValidator verifies token signatures locally against the IdP's JWK
// Set. The key set is fetched once and kept fresh in the background
// (roatation-safe): no network call happens on the hot path of Validate
type JWKSValidator struct {
	keys     keyfunc.Keyfunc
	issuer   string
	audience string
	leeway   time.Duration
}

// newJWKSValidator starts a background-refreshing JWKS client for jwksURI.
// ctx controls the lifetime of the refresh goroutine - cancel it. (e.g. on
// service shutdown) to stop background refreshes.
func newJWKSValidator(ctx context.Context, jwksURI, issuer, audience string, leeway time.Duration) (*JWKSValidator, error) {
	k, err := keyfunc.NewDefaultCtx(ctx, []string{jwksURI})
	if err != nil {
		return nil, fmt.Errorf("auth: creating JWKS client for %s: %w", jwksURI, err)
	}

	return &JWKSValidator{
		keys:     k,
		issuer:   issuer,
		audience: audience,
		leeway:   leeway,
	}, nil
}

func (v *JWKSValidator) Validate(ctx context.Context, rawToken string) (*Claims, error) {
	claims := &Claims{}

	opts := []jwt.ParserOption{
		jwt.WithIssuer(v.issuer),
		jwt.WithLeeway(v.leeway),
		jwt.WithValidMethods([]string{"RS256", "RS384", "RS512", "ES256", "ES384", "ES512"}),
	}
	if v.audience != "" {
		opts = append(opts, jwt.WithAudience(v.audience))
	}

	token, err := jwt.ParseWithClaims(rawToken, claims, v.keys.Keyfunc, opts...)
	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrTokenExpired):
			return nil, ErrTokenExpired
		default:
			return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
		}
	}
	if !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
