// Package interceptors holds ConnectRPC interceptors shared across services.
package interceptors

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"

	"github.com/sushiAlii/torogan-be/pkg/services"
)

type contextKey string

const (
	userIDContextKey contextKey = "userID"
	roleContextKey   contextKey = "role"
)

// NewAuthInterceptor returns an "optional auth" unary interceptor: it reads
// the Authorization header, and if it carries a valid access token, injects
// the caller's user ID and role into the request context. A missing or
// invalid token is NOT rejected here — it just leaves the context empty, so
// handlers that need to require auth must check for it explicitly (via
// MustUserID) and return connect.CodeUnauthenticated themselves. This lets
// the same interceptor serve both public endpoints (e.g. GetPropertyByID,
// which behaves differently for authenticated callers) and protected ones.
func NewAuthInterceptor(as *services.AuthService) connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if token, ok := bearerToken(req.Header().Get("Authorization")); ok {
				if userID, role, err := as.ValidateAccessToken(token); err == nil {
					ctx = context.WithValue(ctx, userIDContextKey, userID)
					ctx = context.WithValue(ctx, roleContextKey, role)
				}
			}
			return next(ctx, req)
		}
	}
	return connect.UnaryInterceptorFunc(interceptor)
}

func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if token == "" {
		return "", false
	}
	return token, true
}

// UserIDFromContext returns the authenticated caller's user ID, if any.
func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDContextKey).(string)
	return userID, ok
}

// RoleFromContext returns the authenticated caller's role, if any.
func RoleFromContext(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(roleContextKey).(string)
	return role, ok
}

// MustUserID returns the authenticated caller's user ID, or a
// connect.CodeUnauthenticated error if the request carried no valid access
// token. Handlers on protected endpoints should call this first.
func MustUserID(ctx context.Context) (string, error) {
	userID, ok := UserIDFromContext(ctx)
	if !ok || userID == "" {
		return "", connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}
	return userID, nil
}
