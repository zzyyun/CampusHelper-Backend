package middleware

import (
	"net/http"
	"strings"

	"go_projects/praProject1/config"
	pkgjwt "go_projects/praProject1/pkg/jwt"

	"github.com/gin-gonic/gin"
)

const (
	CtxUserID = "user_id"
	CtxRole   = "user_role"
)

// JWTAuth validates the Bearer token and sets user_id / user_role in the Gin context.
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := pkgjwt.ParseToken(tokenStr, config.Conf.Jwt.AuthKey)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.Set(CtxUserID, claims.UserID) // int64
		c.Set(CtxRole, claims.Role)     // int8
		c.Next()
	}
}

// RequireRole aborts with 403 if the authenticated user's role is below minRole.
func RequireRole(minRole int8) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get(CtxRole)
		r, _ := role.(int8)
		if r < minRole {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			return
		}
		c.Next()
	}
}
