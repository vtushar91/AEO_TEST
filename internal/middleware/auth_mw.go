package middleware

import (
	"auth-microservice/internal/auth"
	"auth-microservice/internal/pkg"
	"net/http"
	"strings"
)

// JWTAuth is middleware that validates a JWT token and injects the email into the request context
func JWTAuth(secret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "missing authorization header", http.StatusUnauthorized)
			return
		}

		parts := strings.Fields(authHeader)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, "invalid authorization header", http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]

		// verify JWT...
		claims, err := auth.ParseToken(secret, tokenString)
		if err != nil {
			http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
			return
		}
		// Get email & UserID from claims
		email := claims.Email
		userID := claims.UserID
		// Store in context
		ctx := pkg.WithEmail(r.Context(), email)
		ctx = pkg.WithUserID(ctx, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
