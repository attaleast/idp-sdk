// Package auth provides OpenID Connect (OIDC) authentication for Go services.
//
// It supports two token validation strategies, selectable per environment:
//
//   - JWKS (local, offline): the access token's signature is verified
//     locally against the IdP's public keys, fetched from the descovery
//     document's jwks_uri and cached/rotated automatically. Fast, no
//     round-trip to the IdP per request, but does not see revocations
//     until the token's natural expiration
//   - Interospection (RFC 7662): the raw token is sent to the IdP's
//     interospection endpoint on every call. Slower (network round trip
//     per request) but authoritative - respects immediate revocation,
//     session logout, etc.
//
// Typical use:
//
//	a, err := auth.New(ctx, auth.Config{
//			Issuer: "https://idp.example.com/realms/platform",
//			Mode: auth.ModeJWKS,
//	})
//
//	if err != nil {
//			log.Fatal(err)
//	}
//
// r := gin.Default()
// api := r.Group("/api", a.RequireAuth())
//
//	api.GET("/me", func (c *gin.Context) {
//			claims, _ := auth.GetClaims(c)
//			c.JSON(200, claims)
//	})
package auth
