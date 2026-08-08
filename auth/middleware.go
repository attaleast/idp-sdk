package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// claimsContextKey is the gin.Context key Claims are stored under
const claimsContextKey = "auth.claims"

// RequiredAuth returns Gin middleware that rejects requests without a
// valid bearer token (401) and stores the validated Claims in the request
// context on success. Use GetClaims to read them downstream
func (a *Auth) RequiredAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := a.authenticate(c)
		if err != nil {
			abortUnauthorized(c, err)
			return
		}
		c.Set(claimsContextKey, claims)
		c.Next()
	}
}

// OptionalAuth validates the token when present but never aborts the
// request when it's missing or invalid - handlers can check GetClaims and
// branch on whether the caller is authenticated. Useful for endpoints
// that personalize output for logged-in users but also server anonymous
// traffic.
func (a *Auth) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := a.authenticate(c)
		if err == nil {
			c.Set(claimsContextKey, claims)
		}
		c.Next()
	}
}

// RequireRoles returns Gin middleware that checks the authenticated
// claller has at least one of the given roles, responding 403 othewise.
// It must run after RequireAuth (it reads Claims from context, it does
// not validate the token itself)
func (a *Auth) RequireRoles(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := GetClaims(c)
		if !ok {
			abortUnauthorized(c, ErrMissingToken)
			return
		}
		have := a.rolesExtractor(claims)
		if !hasAnyRole(have, roles) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": ErrInsufficientRole.Error(),
			})
			return
		}
		c.Next()
	}
}

// GetClaims reads the Claims stored by RequiredAuth/OptionalAuth out of the
// Gin context. ok is false if not (valid) token was presented.
func GetClaims(c *gin.Context) (*Claims, bool) {
	v, ok := c.Get(claimsContextKey)
	if !ok {
		return nil, false
	}
	claims, ok := v.(*Claims)
	return claims, ok
}

func (a *Auth) authenticate(c *gin.Context) (*Claims, error) {
	token, err := extractBearerToken(c.GetHeader("Authorization"))
	if err != nil {
		return nil, err
	}
	return a.Validate(c.Request.Context(), token)
}

func extractBearerToken(header string) (string, error) {
	const prefix = "Bearer "
	if header == "" || !strings.HasPrefix(header, prefix) {
		return "", ErrMissingToken
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if token == "" {
		return "", ErrMissingToken
	}

	return token, nil
}

func abortUnauthorized(c *gin.Context, err error) {
	status := http.StatusUnauthorized
	body := gin.H{"error": err.Error()}

	switch {
	case errors.Is(err, ErrMissingToken):
		body["error"] = ErrMissingToken.Error()
	case errors.Is(err, ErrTokenExpired):
		body["error"] = ErrTokenExpired.Error()
	case errors.Is(err, ErrTokenInactive):
		body["error"] = ErrTokenInactive.Error()
	case errors.Is(err, ErrInvalidToken):
		body["error"] = ErrInvalidToken.Error()
	}

	c.Header("WWW-Authenticate", `Bearer error="invalid_token"`)
	c.AbortWithStatusJSON(status, body)
}
