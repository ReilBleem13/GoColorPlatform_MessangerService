package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ReilBleem13/MessangerV2/internal/domain"
	"github.com/ReilBleem13/MessangerV2/internal/utils"
)

type contextKey string

const UserIDKey contextKey = "user_id"

func AuthMiddleware(secret string) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeError(w, domain.ErrUnauthorizedError)
				return
			}

			tokenString, err := utils.ExtractToken(authHeader)
			if err != nil {
				hadleError(w, err)
				return
			}

			claims, err := utils.ValidateAccessToken(tokenString, secret)
			if err != nil {
				hadleError(w, err)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			h.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUserIDFromContext(ctx context.Context) (int, error) {
	userID, ok := ctx.Value(UserIDKey).(int)
	if !ok {
		return 0, fmt.Errorf("user ID not found in context")
	}
	return userID, nil
}
