package utils

import (
	"errors"
	"fmt"

	"github.com/ReilBleem13/MessangerV2/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

const bearerPrefix = "Bearer "

type AccessClaims struct {
	UserID int `json:"user_id"`
	jwt.RegisteredClaims
}

type RefreshClaims struct {
	UserID  int    `json:"user_id"`
	TokenID string `json:"token_id"`
	jwt.RegisteredClaims
}

func ValidateAccessToken(tokenString, secret string) (*AccessClaims, error) {
	claims := &AccessClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w, unexpected signing method", domain.ErrInvalidToken)
		}
		return []byte(secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, domain.ErrInvalidToken
		}
		return nil, domain.ErrInvalidToken
	}

	if !token.Valid {
		return nil, domain.ErrInvalidToken
	}
	return claims, nil
}

func ValidateRefreshToken(tokenString, secret string) (*RefreshClaims, error) {
	claims := &RefreshClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w, unexpected signing method", domain.ErrInvalidToken)
		}
		return []byte(secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, domain.ErrInvalidToken
		}
		return nil, domain.ErrInvalidToken
	}

	if !token.Valid {
		return nil, domain.ErrInvalidToken
	}
	return claims, nil
}

func ExtractToken(authHeader string) (string, error) {
	if authHeader == "" {
		return "", fmt.Errorf("%w, header is empty", domain.ErrInvalidToken)
	}

	if len(authHeader) < len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
		return "", fmt.Errorf("%w, invalid format, forgot 'Bearer '?)", domain.ErrInvalidToken)
	}
	return authHeader[len(bearerPrefix):], nil
}
