package auth

// RolesExtractor pulls the caller's roles out of validated Claims. Where
// roles live in the tokens varies by IdP, so this is pluggable - pass one
// via WithRolesExtractor when the default doens't fit
type RolesExtractor func(c *Claims) []string

// DefaultRolesExtractor tries, in order:
//  1. Keycloak-style realm_access.roles
//  2. a flat top-level "roles" claims
//  3. a flat top-level "groups" claim (common with Dex / generic OIDC)
//
// The first non-empty result wins. Combine providers explicitly with
// WithRolesExtractor if a service needs more than this
func DefaultRolesExtractor(c *Claims) []string {
	if roles := c.NestedStringSlice("realm_access", "roles"); len(roles) > 0 {
		return roles
	}

	if roles := c.StringSlice("roles"); len(roles) > 0 {
		return roles
	}

	if groups := c.StringSlice("groups"); len(groups) > 0 {
		return groups
	}

	return nil
}

// GenericClaimRolesExtractor builds an extractor that reads a flat
// top-level string/[]string claim, for IdP that don't use "roles" or
// "groups" as the claim name
func GenericClaimRolesExtractor(claim string) RolesExtractor {
	return func(c *Claims) []string {
		return c.StringSlice(claim)
	}
}

func hasAnyRole(have, want []string) bool {
	if len(want) == 0 {
		return true
	}

	set := make(map[string]struct{}, len(have))
	for _, r := range have {
		set[r] = struct{}{}
	}
	for _, w := range want {
		if _, ok := set[w]; ok {
			return true
		}
	}
	return false
}
