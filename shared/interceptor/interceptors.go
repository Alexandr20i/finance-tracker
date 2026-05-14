package interceptor

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Logger — логирует каждый gRPC вызов
// grpc.UnaryServerInterceptor — тип для Unary interceptor
func Logger(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	start := time.Now()

	// Вызываем реальный хендлер
	resp, err := handler(ctx, req)

	// Логируем после выполнения
	level := slog.LevelInfo
	if err != nil {
		level = slog.LevelError
	}

	slog.Log(ctx, level, "gRPC call",
		"method", info.FullMethod, // например /transaction.TransactionService/CreateTransaction
		"duration", time.Since(start).String(),
		"error", err,
	)

	return resp, err
}

// Recovery — ловит панику и возвращает gRPC ошибку вместо падения сервиса
func Recovery(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp interface{}, err error) {
	// defer + recover — стандартный Go паттерн для поимки паники
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic recovered",
				"method", info.FullMethod,
				"panic", r,
				"stack", string(debug.Stack()),
			)
			// Возвращаем Internal ошибку клиенту вместо падения
			err = status.Error(codes.Internal, "internal server error")
		}
	}()

	return handler(ctx, req)
}

// StreamLogger — логирует Streaming вызовы
// Отдельный тип для streaming interceptor
func StreamLogger(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	start := time.Now()

	err := handler(srv, ss)

	slog.Info("gRPC stream",
		"method", info.FullMethod,
		"duration", time.Since(start).String(),
		"error", err,
	)

	return err
}
