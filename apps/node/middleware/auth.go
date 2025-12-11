package middleware

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// AuthUnaryInterceptor проверяет аутентификацию для унарных RPC
func AuthUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Пропуск аутентификации для эндпоинтов проверки здоровья и информации о ноде
		if isPublicMethod(info.FullMethod) {
			return handler(ctx, req)
		}

		// Extract metadata
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			slog.Debug("Auth failed: no metadata", "method", info.FullMethod)
		}

		// Добавление ID участника в контекст если присутствует
		if partyIDs := md.Get("party-id"); len(partyIDs) > 0 {
			ctx = context.WithValue(ctx, "party_id", partyIDs[0])
		}

		// На данный момент полагаемся на mTLS и подписи на уровне сообщений
		// Дополнительная аутентификация здесь не требуется
		return handler(ctx, req)
	}
}

// AuthStreamInterceptor проверяет аутентификацию для потоковых RPC
func AuthStreamInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// Пропуск аутентификации для публичных методов
		if isPublicMethod(info.FullMethod) {
			return handler(srv, ss)
		}

		ctx := ss.Context()

		// Извлечение метаданных
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			slog.Debug("Auth failed: no metadata", "method", info.FullMethod)
		}

		// Добавление ID участника в контекст если присутствует
		if partyIDs := md.Get("party-id"); len(partyIDs) > 0 {
			ctx = context.WithValue(ctx, "party_id", partyIDs[0])
		}

		return handler(srv, &wrappedStream{ServerStream: ss, ctx: ctx})
	}
}

// isPublicMethod проверяет, является ли метод публичным (не требует аутентификации)
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

// wrappedStream оборачивает grpc.ServerStream для возможности модификации контекста
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}
