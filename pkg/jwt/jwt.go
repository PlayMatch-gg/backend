package jwt

import (
	"playmatch/backend/internal/config"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
)

// GenerateToken creates a new JWT for a given user ID.
func GenerateToken(userID uint) (string, error) {
	claims := gojwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(time.Hour * 24 * 7).Unix(), // Token expires in 7 days
		"iat": time.Now().Unix(),
	}

	token := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(config.AppConfig.JWTSecret))
}
