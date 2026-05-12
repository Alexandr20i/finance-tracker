package main

import (
	"log"
	"log/slog"
	"os"

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

	// Подключаем все gRPC клиенты
	clients, err := client.NewClients(
		cfg.GRPC.TransactionAddr,
		cfg.GRPC.BudgetAddr,
		cfg.GRPC.ReportAddr,
	)
	if err != nil {
		log.Fatalf("failed to connect to services: %v", err)
	}
	slog.Info("connected to all gRPC services")

	// Хендлеры
	authHandler := handler.NewAuthHandler(clients.Transaction, cfg.JWT.Secret, cfg.JWT.ExpirationHours)
	transactionHandler := handler.NewTransactionHandler(clients.Transaction, clients.Budget)
	budgetHandler := handler.NewBudgetHandler(clients.Budget)
	reportHandler := handler.NewReportHandler(clients.Report)

	// Gin роутер
	// gin.Default() включает Logger и Recovery middleware
	r := gin.Default()

	// Публичные маршруты
	auth := r.Group("/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
	}

	// Защищённые маршруты
	// r.Group + Use(middleware) — аналог chi Group + r.Use
	api := r.Group("/")
	api.Use(middleware.Auth(cfg.JWT.Secret))
	{
		// Транзакции
		api.POST("/transactions", transactionHandler.Create)
		api.GET("/transactions", transactionHandler.List)
		api.GET("/transactions/:id", transactionHandler.Get)
		api.DELETE("/transactions/:id", transactionHandler.Delete)

		// Бюджеты
		api.POST("/budget", budgetHandler.Create)
		api.GET("/budget", budgetHandler.List)
		api.PUT("/budget/:id", budgetHandler.Update)
		api.DELETE("/budget/:id", budgetHandler.Delete)

		// Отчёты
		api.GET("/reports/summary", reportHandler.Summary)
		api.GET("/reports/by-category", reportHandler.CategoryBreakdown)
		api.GET("/reports/trend", reportHandler.MonthlyTrend)
		api.GET("/reports/forecast", reportHandler.Forecast)
		api.GET("/reports/daily", reportHandler.DailyExpenses)
	}

	addr := ":" + cfg.Server.Port
	slog.Info("gateway started", "addr", "http://localhost"+addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("gateway error: %v", err)
	}
}
