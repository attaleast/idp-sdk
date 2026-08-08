package auth

import (
	"fmt"
	"net/http"
	"time"
)

type Mode string

const (
	// ModeJWKS validates the token signature locally, against the IdP's
	// public keys (fast, offline, no per-request network call)
	ModeJWKS Mode = "jwks"

	// ModeInstrospection validates token via IdP's RFC 7662
	// introspection endpoint on every call (authoritative, sess revocation)
	ModeInstrospection Mode = "introspection"
)

// Config configures an Auth instance. Issuer is the only field that is
// always required - everything else (jwks_uri, introspection_endpoint, etc)
// is discovered automatically from the issuer's
// /.well-known/openid-configuration document
type Config struct {
	// Issuer is the OIDC issuer URL, e.g.
	// "https://idp.example.com/realms/platform" for Keycloak.
	// Required.
	Issuer string

	// Mode selects the validation strategy. Defaults to ModeJWKS
	Mode Mode

	// Audience, if set, is required to be present in the token's "aud"
	// claim. Leave empty to skip audience validation (not recommended for
	// production, fine for early demo stages).
	Audience string

	// ClientID / ClientSecret are required only for ModeInstrospection,
	// where they authenticate the introspection request itself
	// (RFC 7662 typically expects the resource server's own credentials,
	// not the end user's)
	ClientID     string
	ClientSecret string

	// JWKSRefreshInterval controls how often the JWKS key set is
	// refreshed in the background. Defaults to 1 hour. Only used in
	// ModeJWKS
	JWKSRefreshInterval time.Duration

	// RequestTimeout bounds every outbound HTTP call this package makes
	// (discovery, JWKS refresh, introspection). Defaults to 5 seconds.
	RequestTimeout time.Duration

	// LeewaySeconds allows small clock skew between this service and the
	// IdP when checking exp/iat/nbf. Defaults to 5
	LeewaySeconds int

	// HTTPClient overrides the HTTP client used for discovery, JWKS
	// fetching and introspection. Defaults to a client built from
	// RequestTimeout.
	HTTPClient *http.Client
}

func (c *Config) setDefaults() {
	if c.Mode == "" {
		c.Mode = ModeJWKS
	}

	if c.JWKSRefreshInterval <= 0 {
		c.JWKSRefreshInterval = time.Hour
	}

	if c.RequestTimeout <= 0 {
		c.RequestTimeout = 5 * time.Second
	}

	if c.LeewaySeconds <= 0 {
		c.LeewaySeconds = 5
	}

	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: c.RequestTimeout}
	}
}

func (c *Config) validate() error {
	if c.Issuer == "" {
		return fmt.Errorf("auth: config.Issuer is required")
	}

	switch c.Mode {
	case ModeJWKS:
	// nothing extra required
	case ModeInstrospection:
		if c.ClientID == "" || c.ClientSecret == "" {
			return fmt.Errorf("auth: ModeInstrospection requires ClientID and ClientSecret")
		}
	default:
		return fmt.Errorf("auth: unknown Mode %q (want %q or %q)", c.Mode, ModeJWKS, ModeInstrospection)
	}

	return nil
}
