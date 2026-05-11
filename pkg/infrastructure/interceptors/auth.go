package interceptors

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const UserIDKey contextKey = "user_id"

var publicHandlers = map[string]struct{}{
	"/auth.AuthService/Login":    {},
	"/auth.AuthService/Register": {},
	"/auth.AuthService/Refresh":  {},
}

func AuthInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		if _, isPublic := publicHandlers[info.FullMethod]; isPublic {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "metadata missing")
		}

		userIds := md.Get("x-user-id")
		if len(userIds) == 0 || userIds[0] == "" {
			return nil, status.Errorf(codes.Unauthenticated, "no user id in metadata")
		}

		uid, err := uuid.Parse(userIds[0])
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "invalid user id format")
		}

		newCtx := context.WithValue(ctx, UserIDKey, uid)
		return handler(newCtx, req)
	}
}

func UserIdFromContext(ctx context.Context) (uuid.UUID, error) {
	uid, ok := ctx.Value(UserIDKey).(uuid.UUID)
	if !ok {
		return uuid.Nil, fmt.Errorf("invalid user id format")
	}

	return uid, nil
}
