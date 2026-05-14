package client

import (
	"context"

	pb "github.com/Alexandr20i/finance-tracker/gen/transaction"
)

// TransactionClientInterface — интерфейс для мока в тестах
// Реальный клиент и мок реализуют один интерфейс
type TransactionClientInterface interface {
	GetBalance(ctx context.Context, userID int64, from, to string) (*pb.GetBalanceResponse, error)
	GetCategoryTotals(ctx context.Context, userID int64, from, to string) (*pb.GetCategoryTotalsResponse, error)
	ListAllTransactions(ctx context.Context, userID int64, from, to string) ([]*pb.Transaction, error)
}
