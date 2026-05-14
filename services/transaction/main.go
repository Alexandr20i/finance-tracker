package main

import (
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	pb "github.com/Alexandr20i/finance-tracker/gen/transaction"
	grpcServer "github.com/Alexandr20i/finance-tracker/services/transaction/grpc"
	"github.com/Alexandr20i/finance-tracker/services/transaction/repository"
	"github.com/Alexandr20i/finance-tracker/shared/config"
	"github.com/Alexandr20i/finance-tracker/shared/interceptor"
)

func main() {
	// Настраиваем логи
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// Загружаем конфиг
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	// Подключаемся к БД
	db, err := sqlx.Connect("postgres", cfg.DB.DSN())
	if err != nil {
		log.Fatalf("db connect error: %v", err)
	}
	defer db.Close()
	slog.Info("connected to PostgreSQL")

	// Создаём репозитории
	txRepo := repository.NewTransactionRepository(db)
	userRepo := repository.NewUserRepository(db)

	// Создаём TCP listener
	// gRPC работает поверх HTTP/2 и TCP
	lis, err := net.Listen("tcp", ":"+cfg.GRPC.Port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Создаём gRPC сервер
	// grpc.NewServer() — как chi.NewRouter() но для gRPC
	// s := grpc.NewServer()

	s := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			interceptor.Recovery, // сначала recover — чтобы поймать панику из logger
			interceptor.Logger,
		),
		grpc.StreamInterceptor(interceptor.StreamLogger),
	)

	// Регистрируем нашу реализацию
	pb.RegisterTransactionServiceServer(s, grpcServer.NewServer(txRepo, userRepo))

	// reflection позволяет инструментам (типа evans) узнать
	// какие методы есть у сервера — удобно для отладки
	reflection.Register(s)

	// slog.Info("transaction service started", "port", cfg.GRPC.Port)

	// // Запускаем сервер — блокирующий вызов
	// if err := s.Serve(lis); err != nil {
	// 	log.Fatalf("failed to serve: %v", err)
	// }

	// Запускаем gRPC сервер в горутине
	go func() {
		slog.Info("transaction service started", "port", cfg.GRPC.Port)
		if err := s.Serve(lis); err != nil {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Ждём сигнал
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down...")
	s.GracefulStop() // gRPC аналог http.Server.Shutdown()
	slog.Info("stopped")

}
