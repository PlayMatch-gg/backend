package auth

import (
	"fmt"
	"playmatch/backend/internal/config"
	"strings"

	"github.com/gin-gonic/gin"
	gojwt "github.com/golang-jwt/jwt/v5"
)

// OptionalAuthMiddleware inspects for a token and sets the userID if present and valid,
// but does not fail if the token is missing or invalid.
func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				tokenString := parts[1]
				token, err := gojwt.Parse(tokenString, func(token *gojwt.Token) (interface{}, error) {
					if _, ok := token.Method.(*gojwt.SigningMethodHMAC); !ok {
						return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
					}
					return []byte(config.AppConfig.JWTSecret), nil
				})

				if err == nil {
					if claims, ok := token.Claims.(gojwt.MapClaims); ok && token.Valid {
						if userIDFloat, ok := claims["sub"].(float64); ok {
							c.Set("userID", uint(userIDFloat))
						}
					}
				}
			}
		}
		c.Next()
	}
}
