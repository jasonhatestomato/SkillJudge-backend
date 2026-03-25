package middleware

import (
	"net/http"
	"strings"

	"skilljudge/backend/internal/http/response"
	"skilljudge/backend/internal/modules/auth"
	"skilljudge/backend/internal/modules/user"
	platformjwt "skilljudge/backend/internal/platform/jwt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	contextClaimsKey = "authClaims"
	contextUserKey   = "authUser"
)

func RequireAuth(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			response.Error(c, http.StatusUnauthorized, "missing bearer token", nil)
			c.Abort()
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")
		claims, err := authService.ValidateAccessToken(c.Request.Context(), token)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "unauthorized", nil)
			c.Abort()
			return
		}

		userID, err := uuid.Parse(claims.UserID)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "unauthorized", nil)
			c.Abort()
			return
		}

		var schoolID *uuid.UUID
		if claims.SchoolID != "" {
			parsed, err := uuid.Parse(claims.SchoolID)
			if err != nil {
				response.Error(c, http.StatusUnauthorized, "unauthorized", nil)
				c.Abort()
				return
			}
			schoolID = &parsed
		}

		c.Set(contextClaimsKey, claims)
		c.Set(contextUserKey, user.UserContext{
			UserID:   userID,
			Role:     claims.Role,
			SchoolID: schoolID,
		})
		c.Next()
	}
}

func RequirePermission(userService *user.Service, permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := CurrentUser(c)
		// Reload the actor snapshot on each request so permission checks always
		// follow the latest RBAC bindings in the database.
		me, err := userService.GetMe(c.Request.Context(), actor.UserID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, "failed to load permissions", nil)
			c.Abort()
			return
		}

		for _, item := range me.Permissions {
			if item == permission {
				c.Next()
				return
			}
		}

		response.Error(c, http.StatusForbidden, "forbidden", nil)
		c.Abort()
	}
}

func CurrentUser(c *gin.Context) user.UserContext {
	value, _ := c.Get(contextUserKey)
	return value.(user.UserContext)
}

func CurrentClaims(c *gin.Context) *platformjwt.Claims {
	value, _ := c.Get(contextClaimsKey)
	return value.(*platformjwt.Claims)
}
