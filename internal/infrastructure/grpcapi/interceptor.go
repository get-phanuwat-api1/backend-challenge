package grpcapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/get-phanuwat-api1/backend-challenge/internal/application"
	"github.com/get-phanuwat-api1/backend-challenge/internal/domain"
	"github.com/get-phanuwat-api1/backend-challenge/pkg/bearer"
	"github.com/get-phanuwat-api1/backend-challenge/pkg/requestctx"
)

// RequestIDInterceptor stores a fresh request ID in the context for log correlation.
func RequestIDInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		return handler(requestctx.WithRequestID(ctx, hex.EncodeToString(b)), req)
	}
}

// LoggingInterceptor logs the method, status code and execution time of each RPC.
func LoggingInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		level := slog.LevelInfo
		if err != nil {
			level = slog.LevelWarn
		}
		logger.LogAttrs(ctx, level, "grpc request",
			slog.String("method", info.FullMethod),
			slog.String("code", status.Code(err).String()),
			slog.Float64("latency_ms", float64(time.Since(start).Microseconds())/1000),
		)
		return resp, err
	}
}

// AuthInterceptor requires an "authorization: Bearer <jwt>" metadata entry,
// verifies it through the AuthService use case and stores the authenticated
// user ID in the context (see requestctx.UserID).
func AuthInterceptor(authService application.AuthService) grpc.UnaryServerInterceptor {
	unauthenticated := status.Error(codes.Unauthenticated, string(domain.CodeUnauthorized))
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		values := md.Get("authorization")
		if len(values) == 0 {
			return nil, unauthenticated
		}
		raw, ok := bearer.Parse(values[0])
		if !ok {
			return nil, unauthenticated
		}
		userID, err := authService.VerifyAccessToken(raw)
		if err != nil {
			return nil, unauthenticated
		}
		return handler(requestctx.WithUserID(ctx, userID), req)
	}
}
