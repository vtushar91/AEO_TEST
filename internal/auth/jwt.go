package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v4"
)

type JWTClaims struct {
	Email  string `json:"email"`
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

func GenerateAccessToken(secret string, email string, userID string, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	claims := JWTClaims{
		Email:  email,
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ParseToken(secret string, tokenStr string) (*JWTClaims, error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := tok.Claims.(*JWTClaims); ok && tok.Valid {
		return claims, nil
	}
	return nil, err
}
