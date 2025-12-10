package middleware

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// LoggingUnaryInterceptor logs all unary RPC calls
func LoggingUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()

		slog.Debug("gRPC request started",
			"method", info.FullMethod,
		)

		// Call the handler
		resp, err := handler(ctx, req)

		// Log completion
		duration := time.Since(start)
		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}

		logLevel := slog.LevelInfo
		if code != codes.OK {
			logLevel = slog.LevelError
		}

		slog.Log(ctx, logLevel, "gRPC request completed",
			"method", info.FullMethod,
			"duration_ms", duration.Milliseconds(),
			"code", code.String(),
			"error", err,
		)

		return resp, err
	}
}

// LoggingStreamInterceptor logs all streaming RPC calls
func LoggingStreamInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()

		slog.Debug("gRPC stream started",
			"method", info.FullMethod,
			"is_client_stream", info.IsClientStream,
			"is_server_stream", info.IsServerStream,
		)

		// Call the handler
		err := handler(srv, ss)

		// Log completion
		duration := time.Since(start)
		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}

		logLevel := slog.LevelInfo
		if code != codes.OK {
			logLevel = slog.LevelError
		}

		slog.Log(ss.Context(), logLevel, "gRPC stream completed",
			"method", info.FullMethod,
			"duration_ms", duration.Milliseconds(),
			"code", code.String(),
			"error", err,
		)

		return err
	}
}
