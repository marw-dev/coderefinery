package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"coderefinery/internal/infrastructure/auth"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Define custom context keys to avoid collisions (SA1029)
type contextKey string

const (
	userIDKey contextKey = "userID"
	roleKey   contextKey = "role"
)

// AuthMiddleware prüft JWT Tokens und setzt den User in den Context
func AuthMiddleware(jwtService *auth.JWTService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}

		tokenString := parts[1]

		claims, err := jwtService.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		userID := claims.UserID

		// 1. Gin Context (String keys sind hier okay)
		c.Set("userID", userID)
		c.Set("role", claims.Role)

		// 2. Standard Go Context (Typed keys verwenden)
		ctx := context.WithValue(c.Request.Context(), userIDKey, userID)
		ctx = context.WithValue(ctx, roleKey, claims.Role)

		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// GetUserIDFromContext extrahiert die UserID aus dem Kontext
func GetUserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	val := ctx.Value(userIDKey)
	if val == nil {
		return uuid.Nil, errors.New("no user in context")
	}

	id, ok := val.(uuid.UUID)
	if !ok {
		return uuid.Nil, errors.New("invalid user id type in context")
	}

	return id, nil
}

// GetUserRoleFromContext extrahiert die Rolle
func GetUserRoleFromContext(ctx context.Context) (string, error) {
	val := ctx.Value(roleKey)
	if val == nil {
		return "", errors.New("no role in context")
	}
	role, ok := val.(string)
	if !ok {
		return "", errors.New("invalid role type in context")
	}
	return role, nil
}
