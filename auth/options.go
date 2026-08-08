package auth

// Option customizes an Auth instance beyond what Config covers
type Option func(*Auth)

// WithRolesExtractor overrides how RequireRoles reads roles out of Claims
// Use this when the IdP puts roles somewhere DefaultRolesExtractor
// doesn't look e.g. a custom claim.
func WithRolesExtractor(fn RolesExtractor) Option {
	return func(a *Auth) {
		a.rolesExtractor = fn
	}
}
