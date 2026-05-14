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

	pb "github.com/Alexandr20i/finance-tracker/gen/report"
	"github.com/Alexandr20i/finance-tracker/services/report/analytics"
	"github.com/Alexandr20i/finance-tracker/services/report/client"
	grpcServer "github.com/Alexandr20i/finance-tracker/services/report/grpc"
	"github.com/Alexandr20i/finance-tracker/shared/config"
	"github.com/Alexandr20i/finance-tracker/shared/interceptor"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	// Report Service подключается к Transaction Service как клиент
	txClient, err := client.NewTransactionClient(cfg.GRPC.TransactionAddr)
	if err != nil {
		log.Fatalf("failed to connect to transaction service: %v", err)
	}
	slog.Info("connected to transaction service", "addr", cfg.GRPC.TransactionAddr)

	// Создаём аналитику
	a := analytics.NewAnalytics(txClient)

	port := os.Getenv("GRPC_PORT")
	if port == "" {
		port = "50053"
	}

	lis, err := net.Listen("tcp", ":"+port)

	// lis, err := net.Listen("tcp", ":50053")
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

	pb.RegisterReportServiceServer(s, grpcServer.NewServer(a))
	reflection.Register(s)

	// slog.Info("report service started", "port", "50053")

	// if err := s.Serve(lis); err != nil {
	// 	log.Fatalf("failed to serve: %v", err)
	// }

	// Запускаем gRPC сервер в горутине
	go func() {
		slog.Info("report service started", "port", cfg.GRPC.Port)
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
