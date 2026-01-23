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

// AuthMiddleware prüft JWT Tokens und setzt den User in den Context
func AuthMiddleware(jwtService *auth.JWTService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		// Kein Header? -> Request durchlassen (Guest), aber kein User im Context
		if authHeader == "" {
			c.Next()
			return
		}

		// Header Format prüfen: "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}

		tokenString := parts[1]

		// Token validieren
		claims, err := jwtService.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		// Parse UUID aus Claims (falls es dort als String vorliegt, sonst direkter Cast)
		// Wir gehen davon aus, dass claims.UserID bereits uuid.UUID ist (gemäß jwt.go).
		// Falls nicht, müsste hier uuid.Parse(claims.UserID) stehen.
		userID := claims.UserID

		// 1. User in den Gin-Context setzen (für REST Handler)
		c.Set("userID", userID)
		c.Set("role", claims.Role)

		// 2. User in den Standard Go Context setzen (WICHTIG für GraphQL Resolver!)
		// Wir nutzen hier Strings als Key der Einfachheit halber.
		ctx := context.WithValue(c.Request.Context(), "userID", userID)
		ctx = context.WithValue(ctx, "role", claims.Role)

		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// GetUserIDFromContext extrahiert die UserID aus dem Kontext (für GraphQL Resolver)
func GetUserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	val := ctx.Value("userID")
	if val == nil {
		return uuid.Nil, errors.New("no user in context")
	}

	id, ok := val.(uuid.UUID)
	if !ok {
		return uuid.Nil, errors.New("invalid user id type in context")
	}

	return id, nil
}

// GetUserRoleFromContext extrahiert die Rolle (Optional, für spätere Checks)
func GetUserRoleFromContext(ctx context.Context) (string, error) {
	val := ctx.Value("role")
	if val == nil {
		return "", errors.New("no role in context")
	}
	role, ok := val.(string)
	if !ok {
		return "", errors.New("invalid role type in context")
	}
	return role, nil
}
