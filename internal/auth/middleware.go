package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	CtxUserIDKey = "user_id"
	CtxEmailKey  = "email"
	CookieName   = "recipes_token"
)

func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prefer cookie
		tok, _ := c.Cookie(CookieName)

		// Fallback to bearer token (useful for /api calls)
		if tok == "" {
			h := c.GetHeader("Authorization")
			if strings.HasPrefix(h, "Bearer ") {
				tok = strings.TrimPrefix(h, "Bearer ")
			}
		}

		if tok == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}

		claims, err := Verify(tok)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.Set(CtxUserIDKey, claims.UserID)
		c.Set(CtxEmailKey, claims.Email)
		c.Next()
	}
}

func CurrentUser(c *gin.Context) (int64, string, bool) {
	uid := c.GetInt64(CtxUserIDKey)
	email := c.GetString(CtxEmailKey)
	if uid == 0 || email == "" {
		return 0, "", false
	}
	return uid, email, true
}
