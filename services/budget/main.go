package main

import (
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "github.com/Alexandr20i/finance-tracker/gen/budget"
	grpcServer "github.com/Alexandr20i/finance-tracker/services/budget/grpc"
	"github.com/Alexandr20i/finance-tracker/services/budget/repository"
	"github.com/Alexandr20i/finance-tracker/shared/config"
	"github.com/Alexandr20i/finance-tracker/shared/interceptor"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	// У Budget Service своя БД
	budgetDSN := cfg.DB.DSN()
	db, err := sqlx.Connect("postgres", budgetDSN)
	if err != nil {
		log.Fatalf("db connect error: %v", err)
	}
	defer db.Close()
	slog.Info("connected to PostgreSQL")

	repo := repository.NewBudgetRepository(db)

	port := os.Getenv("GRPC_PORT")
	if port == "" {
		port = "50052"
	}

	lis, err := net.Listen("tcp", ":"+port)

	// lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// s := grpc.NewServer()

	s := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			interceptor.Recovery, // сначала recover — чтобы поймать панику из logger
			interceptor.Logger,
		),
		grpc.StreamInterceptor(interceptor.StreamLogger),
	)

	pb.RegisterBudgetServiceServer(s, grpcServer.NewServer(repo))
	reflection.Register(s)

	// slog.Info("budget service started", "port", "50052")

	// if err := s.Serve(lis); err != nil {
	// 	log.Fatalf("failed to serve: %v", err)
	// }

	// Запускаем gRPC сервер в горутине
	go func() {
		slog.Info("budget service started", "port", cfg.GRPC.Port)
		if err := s.Serve(lis); err != nil {
			slog.Error("serve error", "error", err)
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
