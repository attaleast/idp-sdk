package auth

import (
	"context"
	"fmt"
	"time"
)

func secondsToLeeway(s int) time.Duration {
	return time.Duration(s) * time.Second
}

// Auth is the entry point of the package: it discovers the IdP's OIDC
// configuration, builds the configured Validator (JWKS or introspection),
// and exposes both a low-level Validate method and Gin middleware  built
// on top of it
type Auth struct {
	cfg            Config
	discovery      *Discovery
	validator      Validator
	rolesExtractor RolesExtractor
}

func New(ctx context.Context, cfg Config, opts ...Option) (*Auth, error) {
	cfg.setDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	doc, err := discover(ctx, cfg.Issuer, cfg.HTTPClient)
	if err != nil {
		return nil, err
	}

	a := &Auth{
		cfg:            cfg,
		discovery:      doc,
		rolesExtractor: DefaultRolesExtractor,
	}

	for _, opt := range opts {
		opt(a)
	}

	switch cfg.Mode {
	case ModeJWKS:
		v, err := newJWKSValidator(ctx, doc.JWKSURI, cfg.Issuer, cfg.Audience, secondsToLeeway(cfg.LeewaySeconds))
		if err != nil {
			return nil, err
		}
		a.validator = v
	case ModeInstrospection:
		if doc.IntrospectionEndpoint == "" {
			return nil, fmt.Errorf("auth: IdP discovery document has no introspection_endpoint, ModeInstrospection unavailable")
		}
		a.validator = newIntrospectionValidator(
			doc.IntrospectionEndpoint,
			cfg.ClientID,
			cfg.ClientSecret,
			cfg.Issuer,
			cfg.Audience,
			cfg.HTTPClient,
		)
	}

	return a, nil
}

// Validate validates a raw bearer token and returns its Claims. This is
// what the Gin middleware calls under the hood; use it directly for
// non-HTTP entry points (gRPC interceptors, background jobs consuming a
// service-account token, etc)
func (a *Auth) Validate(ctx context.Context, rawToken string) (*Claims, error) {
	return a.validator.Validate(ctx, rawToken)
}

// Discovery exposes the resolved OIDC discovery document, e.g. to build an
// authorization-code login flow (AuthorizationEndpoint, TokenEndpoint)
// alongside this package's resource-server-side validation.
func (a *Auth) Discovery() Discovery {
	return *a.discovery
}
