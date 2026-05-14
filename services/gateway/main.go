package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Alexandr20i/finance-tracker/services/gateway/client"
	"github.com/Alexandr20i/finance-tracker/services/gateway/handler"
	"github.com/Alexandr20i/finance-tracker/services/gateway/middleware"
	"github.com/Alexandr20i/finance-tracker/shared/config"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	clients, err := client.NewClients(
		cfg.GRPC.TransactionAddr,
		cfg.GRPC.BudgetAddr,
		cfg.GRPC.ReportAddr,
	)
	if err != nil {
		log.Fatalf("failed to connect to services: %v", err)
	}
	slog.Info("connected to all gRPC services")

	authHandler := handler.NewAuthHandler(clients.Transaction, cfg.JWT.Secret, cfg.JWT.ExpirationHours)
	transactionHandler := handler.NewTransactionHandler(clients.Transaction, clients.Budget)
	budgetHandler := handler.NewBudgetHandler(clients.Budget)
	reportHandler := handler.NewReportHandler(clients.Report)

	r := gin.Default()

	auth := r.Group("/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
	}

	api := r.Group("/")
	api.Use(middleware.Auth(cfg.JWT.Secret))
	{
		api.POST("/transactions", transactionHandler.Create)
		api.GET("/transactions", transactionHandler.List)
		api.GET("/transactions/:id", transactionHandler.Get)
		api.DELETE("/transactions/:id", transactionHandler.Delete)

		api.POST("/budget", budgetHandler.Create)
		api.GET("/budget", budgetHandler.List)
		api.PUT("/budget/:id", budgetHandler.Update)
		api.DELETE("/budget/:id", budgetHandler.Delete)

		api.GET("/reports/summary", reportHandler.Summary)
		api.GET("/reports/by-category", reportHandler.CategoryBreakdown)
		api.GET("/reports/trend", reportHandler.MonthlyTrend)
		api.GET("/reports/forecast", reportHandler.Forecast)
		api.GET("/reports/daily", reportHandler.DailyExpenses)
	}

	// Запускаем сервер в горутине
	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: r,
	}

	go func() {
		slog.Info("gateway started", "addr", "http://localhost:"+cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("gateway error", "error", err)
			os.Exit(1)
		}
	}()

	// Ждём сигнал остановки
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down gateway...")

	// Даём 10 секунд завершить текущие запросы
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", "error", err)
	}

	slog.Info("gateway stopped")
}
