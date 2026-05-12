package client

import (
	"context"
	"fmt"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/Alexandr20i/finance-tracker/gen/transaction"
)

// TransactionClient — обёртка над сгенерированным gRPC клиентом
// Инкапсулирует логику работы со стримингом
type TransactionClient struct {
	client pb.TransactionServiceClient
}

func NewTransactionClient(addr string) (*TransactionClient, error) {
	// insecure.NewCredentials() — без TLS, для локальной разработки
	// В продакшене используют TLS сертификаты
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to transaction service: %w", err)
	}

	return &TransactionClient{
		client: pb.NewTransactionServiceClient(conn),
	}, nil
}

// GetBalance — простой Unary вызов
func (c *TransactionClient) GetBalance(ctx context.Context, userID int64, from, to string) (*pb.GetBalanceResponse, error) {
	return c.client.GetBalance(ctx, &pb.GetBalanceRequest{
		UserId: userID,
		From:   from,
		To:     to,
	})
}

// GetCategoryTotals — Unary вызов
func (c *TransactionClient) GetCategoryTotals(ctx context.Context, userID int64, from, to string) (*pb.GetCategoryTotalsResponse, error) {
	return c.client.GetCategoryTotals(ctx, &pb.GetCategoryTotalsRequest{
		UserId: userID,
		From:   from,
		To:     to,
	})
}

// ListTransactions — Server Streaming
// Читаем все транзакции из потока и собираем в слайс
func (c *TransactionClient) ListAllTransactions(ctx context.Context, userID int64, from, to string) ([]*pb.Transaction, error) {
	// Открываем стрим
	stream, err := c.client.ListTransactions(ctx, &pb.ListTransactionsRequest{
		UserId: userID,
		From:   from,
		To:     to,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open stream: %w", err)
	}

	// Читаем из потока пока не получим io.EOF
	var transactions []*pb.Transaction
	for {
		tx, err := stream.Recv()
		if err == io.EOF {
			break // стрим закончился
		}
		if err != nil {
			return nil, fmt.Errorf("stream error: %w", err)
		}
		transactions = append(transactions, tx)
	}

	return transactions, nil
}
