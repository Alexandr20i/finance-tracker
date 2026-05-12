package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const UserIDKey = "userID"

type Claims struct {
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

func GenerateToken(userID int64, secret string, expirationHours int) (string, error) {
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(
				time.Duration(expirationHours) * time.Hour,
			)),
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// Auth — Gin middleware для проверки JWT
// В Gin middleware возвращает func(*gin.Context)
// В chi было func(http.Handler) http.Handler — вот главное отличие
func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort() // останавливаем цепочку middleware
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			c.Abort()
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(parts[1], claims, func(t *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		// Кладём userID в контекст Gin
		// В chi было context.WithValue — в Gin это c.Set/c.Get
		c.Set(UserIDKey, claims.UserID)
		c.Next() // передаём управление следующему хендлеру
	}
}

// GetUserID достаёт userID из контекста Gin
func GetUserID(c *gin.Context) int64 {
	id, _ := c.Get(UserIDKey)
	userID, _ := id.(int64)
	return userID
}
