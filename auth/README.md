# auth

OIDC authentication for Go services: JWT + JWKS + OpenID Connect discovery,
IdP-agnostic (Keycloak, Dex, Zitadel, Auth0, ...), with a Gin middleware
layer on top.

## Install

This is written as a standalone module for now. To fold it into an
existing SDK monorepo, drop the `auth/` directory in, delete `go.mod`
here, and run `go mod tidy` at the repo root — everything below still
works unchanged, only the import path changes.

```bash
cd auth
go mod tidy
```

## Quick start (JWKS mode — recommended default)

```go
ctx := context.Background()

a, err := auth.New(ctx, auth.Config{
	Issuer: "https://idp.example.com/realms/platform", // Keycloak realm issuer
	Mode:   auth.ModeJWKS,
	// Audience: "demo-service", // uncomment once clients set aud
})
if err != nil {
	log.Fatal(err)
}

r := gin.Default()

api := r.Group("/api")
api.Use(a.RequireAuth())
api.GET("/me", func(c *gin.Context) {
	claims, _ := auth.GetClaims(c)
	c.JSON(200, gin.H{
		"sub":   claims.Subject,
		"email": claims.Email,
	})
})

// role-gated route
api.DELETE("/widgets/:id", a.RequireRoles("admin"), deleteWidget)

r.Run(":8080")
```

`ctx` passed to `auth.New` controls the JWKS background-refresh goroutine —
tie it to the service's shutdown context (e.g. cancelled on SIGTERM), not
a per-request context.

## Introspection mode

Same `Config`, different `Mode`, plus the resource server's own client
credentials (used to authenticate to the introspection endpoint per RFC
7662 — these are *not* the end user's credentials):

```go
a, err := auth.New(ctx, auth.Config{
	Issuer:       "https://idp.example.com/realms/platform",
	Mode:         auth.ModeIntrospection,
	ClientID:     os.Getenv("OIDC_CLIENT_ID"),
	ClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
})
```

Everything else (`RequireAuth`, `RequireRoles`, `GetClaims`) is identical —
the validation strategy is an implementation detail behind `Config.Mode`.

## JWKS vs introspection

| | JWKS (`ModeJWKS`) | Introspection (`ModeIntrospection`) |
|---|---|---|
| Per-request cost | none (local signature check) | one HTTP call to the IdP |
| Sees revocation/logout immediately | no — only at natural `exp` | yes |
| Needs client credentials | no | yes |
| Good default for | most services | high-sensitivity endpoints (payments, admin actions, session-kill-sensitive flows) |

Config exposes both behind one flag so a service can start on JWKS and
switch a specific route group to introspection later without touching the
rest of the auth wiring.

## Roles

`RequireRoles` reads roles via a pluggable `RolesExtractor`
(`Config` has no such field — pass it as an `Option`):

```go
a, err := auth.New(ctx, cfg,
	auth.WithRolesExtractor(auth.GenericClaimRolesExtractor("permissions")),
)
```

The default extractor (`auth.DefaultRolesExtractor`) checks, in order,
Keycloak's `realm_access.roles`, a flat `roles` claim, then a flat
`groups` claim — good enough for early demo stages against most IdPs
without configuration.

## Everything is discovered, nothing is hardcoded

`auth.New` fetches `{issuer}/.well-known/openid-configuration` and pulls
`jwks_uri` / `introspection_endpoint` / etc. from it, and verifies the
document's `issuer` field matches what you configured. This is why the
package has no Keycloak/Dex/Zitadel-specific code — any spec-compliant
OIDC provider works by just changing `Issuer`.

## Non-Gin usage

`Auth.Validate(ctx, rawToken) (*Claims, error)` has no Gin dependency —
use it directly in a gRPC interceptor, an SQS consumer checking a
service-account token, a CLI, etc. `middleware_gin.go` is the only file
that imports `gin`.
