package auth

import (
	"encoding/json"

	"github.com/golang-jwt/jwt/v5"
)

// Claims holds the standart OIDC claims plus a Raw map with everything the
// IdP put in the token, so provider-specific claims (Keycloack's
// realm_access.roles, custom tenant IDs, etc.) are always reachable without
// needing a provider-specific struct.
type Claims struct {
	jwt.RegisteredClaims

	Email             string `json:"email,omitempty"`
	EmailVerified     bool   `json:"email_verified,omitempty"`
	PreferredUsername string `json:"preferred_username,omitempty"`
	Name              string `json:"name,omitempty"`

	// Raw contains every claim from the token, keyed by claim name,
	// including the typed ones above and anything provider-specific
	Raw map[string]any `json:"-"`
}

// UnmarshalJSON decodes the known claims into their typed fields and keeps
// a full copy of every claim in Raw
func (c *Claims) UnmarshalJSON(data []byte) error {
	type alias Claims
	aux := &struct{ *alias }{alias: (*alias)(c)}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	return json.Unmarshal(data, &c.Raw)
}

// String returns the string claim at key, or "" if absent / wrong type
func (c *Claims) String(key string) string {
	v, _ := c.Raw[key].(string)
	return v
}

// StringSlice returns the claim at key as []string. Handles both JSON
// arrays of strings and a single string (some IdP collapse
// single-value claims)
func (c *Claims) StringSlice(key string) []string {
	switch v := c.Raw[key].(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		return []string{v}
	default:
		return nil
	}
}

// NestedStringSlice walks a chain of nested objects (e.g. "realm_access",
// "roles" for Keycloack) and returns the string slice at the end of the
// path, or nil if the path doesn't exist.
func (c *Claims) NestedStringSlice(path ...string) []string {
	if len(path) == 0 {
		return nil
	}

	var cur any = map[string]any(c.Raw)
	for _, key := range path[:len(path)-1] {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur, ok = m[key]
		if !ok {
			return nil
		}
	}

	m, ok := cur.(map[string]any)
	if !ok {
		return nil
	}

	last := path[len(path)-1]
	switch v := m[last].(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		return []string{v}
	default:
		return nil
	}
}
