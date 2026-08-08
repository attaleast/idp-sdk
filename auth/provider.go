package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Discovery holds the fields we need from the IdP's
// /.well-known/openid-configuration documet (RFC 8414 / OIDC Discovery)
// Only the fields this package uses are parsed; everything else in the
// document is ignored
type Discovery struct {
	Issuer                string `json:"issuer"`
	JWKSURI               string `json:"jwks_uri"`
	IntrospectionEndpoint string `json:"introspection_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
	EndSessionEndpoint    string `json:"end_session_endpoint"`
}

func discover(ctx context.Context, issuer string, client *http.Client) (*Discovery, error) {
	wellKnown := strings.TrimSuffix(issuer, "/") + "/.well-known/openid-configuration"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wellKnown, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: building request: %v", ErrDiscoveryFailed, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: fetching %s: %v", ErrDiscoveryFailed, wellKnown, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: returned status %d", ErrDiscoveryFailed, resp.StatusCode)
	}

	var doc Discovery
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("%w: decoding response from %s: %v", ErrDiscoveryFailed, wellKnown, err)
	}

	if doc.Issuer != issuer {
		return nil, fmt.Errorf("%w: issuer mismatch: configured %q, document says %q", ErrDiscoveryFailed, issuer, doc.Issuer)
	}
	if doc.JWKSURI == "" {
		return nil, fmt.Errorf("%w: document has no jwks_uri", ErrDiscoveryFailed)
	}

	return &doc, nil
}
