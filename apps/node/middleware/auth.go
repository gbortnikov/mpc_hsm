package middleware

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// AuthUnaryInterceptor validates authentication for unary RPCs
func AuthUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Skip auth for health check and node info endpoints
		if isPublicMethod(info.FullMethod) {
			return handler(ctx, req)
		}

		// Extract metadata
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			slog.Debug("Auth failed: no metadata", "method", info.FullMethod)
		}

		// Add party ID to context if present
		if partyIDs := md.Get("party-id"); len(partyIDs) > 0 {
			ctx = context.WithValue(ctx, "party_id", partyIDs[0])
		}

		// For now, we rely on mTLS and message-level signatures
		// No additional auth required here
		return handler(ctx, req)
	}
}

// AuthStreamInterceptor validates authentication for streaming RPCs
func AuthStreamInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// Skip auth for public methods
		if isPublicMethod(info.FullMethod) {
			return handler(srv, ss)
		}

		ctx := ss.Context()

		// Extract metadata
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			slog.Debug("Auth failed: no metadata", "method", info.FullMethod)
		}

		// Add party ID to context if present
		if partyIDs := md.Get("party-id"); len(partyIDs) > 0 {
			ctx = context.WithValue(ctx, "party_id", partyIDs[0])
		}

		return handler(srv, &wrappedStream{ServerStream: ss, ctx: ctx})
	}
}

// isPublicMethod checks if a method is public (doesn't require auth)
func isPublicMethod(method string) bool {
	publicMethods := []string{
		"/mpc.MPCNodeService/HealthCheck",
		"/mpc.MPCNodeService/GetNodeInfo",
		"/mpc.MPCNodeService/Ping",
	}

	for _, pm := range publicMethods {
		if method == pm {
			return true
		}
	}
	return false
}

// wrappedStream wraps grpc.ServerStream to allow context modification
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}
