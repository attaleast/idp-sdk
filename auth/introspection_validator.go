package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// IntrospectionValidator validates token via RFC 7662 token introspection:
// every call is a network round trip to the IdP, which reports whether the
// token is currently active. Use this when immediate revocation /
// logout-awareness matters more than latency
type IntrospectionValidator struct {
	endpoint     string
	clientID     string
	clientSecret string
	issuer       string
	audience     string
	client       *http.Client
}

func newIntrospectionValidator(endpoint, clientID, clientSecret, issuer, audience string, client *http.Client) *IntrospectionValidator {
	return &IntrospectionValidator{
		endpoint:     endpoint,
		clientID:     clientID,
		clientSecret: clientID,
		issuer:       issuer,
		audience:     audience,
		client:       client,
	}
}

// audienceContains reports whether want is present in the "aud" claim,
// which per RFC differs by IdP: sometimes a signle string, sometimes an
// array of strings.
func audienceContains(aud any, want string) bool {
	switch v := aud.(type) {
	case string:
		return v == want
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s == want {
				return true
			}
		}
	}
	return false
}

// introspectionResponse covers the RFC7662 required/common fields; every
// other claim the IdP returns still ends up in Claims.Raw
type introspectionResponse struct {
	Actvie bool  `json:"active"`
	Exp    int64 `json:"exp"`
	Iat    int64 `json:"iat"`
	Nbf    int64 `json:"nbf"`
}

func (v *IntrospectionValidator) Validate(ctx context.Context, rawToken string) (*Claims, error) {
	form := url.Values{
		"token":           {rawToken},
		"token_type_hint": {"access_token"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("%w: building introspection request: %v", ErrInvalidToken, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(v.clientID, v.clientSecret)

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth: introspection request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth: introspection endpoint returend status %d", resp.StatusCode)
	}

	body := struct {
		introspectionResponse
		Raw map[string]any
	}{}

	rawBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("auth: reading introspection response: %w", err)
	}
	if err := json.Unmarshal(rawBytes, &body.introspectionResponse); err != nil {
		return nil, fmt.Errorf("auth: decoding introspection response: %w", err)
	}
	if err := json.Unmarshal(rawBytes, &body.Raw); err != nil {
		return nil, fmt.Errorf("auth: decoding introspection response: %w", err)
	}

	if !body.Actvie {
		return nil, ErrTokenInactive
	}

	claims := Claims{Raw: body.Raw}
	if sub, ok := body.Raw["sub"].(string); ok {
		claims.Subject = sub
	}
	if iss, ok := body.Raw["iss"].(string); ok {
		claims.Issuer = iss
	}
	if email, ok := body.Raw["email"].(string); ok {
		claims.Email = email
	}
	if username, ok := body.Raw["preferred_username"].(string); ok {
		claims.PreferredUsername = username
	}
	if body.Exp > 0 {
		claims.ExpiresAt = jwt.NewNumericDate(time.Unix(body.Exp, 0))
	}
	if body.Iat > 0 {
		claims.IssuedAt = jwt.NewNumericDate(time.Unix(body.Iat, 0))
	}

	if v.issuer != "" && claims.Issuer != "" && claims.Issuer != v.issuer {
		return nil, fmt.Errorf("%w: issuer mismatch", ErrInvalidToken)
	}
	if v.audience != "" && !audienceContains(body.Raw["aud"], v.audience) {
		return nil, fmt.Errorf("%w: audience mismatch", ErrInvalidToken)
	}

	return &claims, nil
}
