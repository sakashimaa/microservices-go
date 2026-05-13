package middleware

import (
	"context"
	"crypto/rsa"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/metadata"
)

type contextKey string

const UserIDKey contextKey = "user_id"
const UserRolesKey contextKey = "user_roles"

func AuthMiddleware(publicKey *rsa.PublicKey) func(handler http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			enrichedReq, err := ValidateToken(r, publicKey)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, enrichedReq)
		})
	}
}

func ValidateToken(r *http.Request, publicKey *rsa.PublicKey) (*http.Request, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("no auth header")
	}

	if !strings.Contains(authHeader, "Bearer") {
		return nil, fmt.Errorf("invalid token")
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil, fmt.Errorf("invalid token")
	}
	tokenStr := parts[1]

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return publicKey, nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	sub, err := claims.GetSubject()
	if err != nil || sub == "" {
		subInter, ok := claims["sub"].(string)
		if !ok || subInter == "" {
			return nil, fmt.Errorf("user_id (sub) is missing from token")
		}
		sub = subInter
	}

	var roles []string
	if rolesClaim, ok := claims["roles"].([]interface{}); ok {
		for _, r := range rolesClaim {
			if roleStr, ok := r.(string); ok {
				roles = append(roles, roleStr)
			}
		}
	}

	ctx := metadata.AppendToOutgoingContext(r.Context(), "x-user-id", sub)

	ctx = context.WithValue(ctx, UserIDKey, sub)
	ctx = context.WithValue(ctx, UserRolesKey, roles)

	return r.WithContext(ctx), nil
}
