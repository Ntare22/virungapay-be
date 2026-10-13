package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	ctxUserID = "auth_user_id"
	ctxRole   = "auth_role"
)

// TokenParser validates a bearer token.
type TokenParser interface {
	ParseToken(raw string) (TokenClaims, error)
}

// TokenClaims is the subset of a session the middleware stores.
type TokenClaims struct {
	Subject string
	Role    string
}

// RequireAuth rejects requests without a valid bearer token.
func RequireAuth(parser TokenParser) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := strings.TrimSpace(c.GetHeader("Authorization"))
		raw = strings.TrimSpace(strings.TrimPrefix(raw, "Bearer "))
		if raw == "" {
			WriteError(c, Unauthorized("invalid_credentials", "sign in again"))
			return
		}
		claims, err := parser.ParseToken(raw)
		if err != nil {
			WriteError(c, err)
			return
		}
		c.Set(ctxUserID, claims.Subject)
		c.Set(ctxRole, claims.Role)
		c.Next()
	}
}

// RequireRoles rejects a signed-in caller whose role is not listed.
func RequireRoles(roles ...string) gin.HandlerFunc {
	allowed := map[string]struct{}{}
	for _, role := range roles {
		allowed[role] = struct{}{}
	}
	return func(c *gin.Context) {
		if _, ok := allowed[Role(c)]; !ok {
			WriteError(c, Forbidden("forbidden", "you cannot use this route"))
			return
		}
		c.Next()
	}
}

// UserID is the subject stored by RequireAuth.
func UserID(c *gin.Context) string {
	id, _ := c.Get(ctxUserID)
	value, _ := id.(string)
	return value
}

// Role is the role stored by RequireAuth.
func Role(c *gin.Context) string {
	role, _ := c.Get(ctxRole)
	value, _ := role.(string)
	return value
}
