package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/Alexandr20i/finance-tracker/gen/transaction"
	"github.com/Alexandr20i/finance-tracker/services/transaction/repository"
)

// Server реализует интерфейс TransactionServiceServer
// который был сгенерирован из transaction.proto
type Server struct {
	pb.UnimplementedTransactionServiceServer // встраиваем чтобы не реализовывать все методы сразу
	txRepo                                   *repository.TransactionRepository
	userRepo                                 *repository.UserRepository
}

func NewServer(
	txRepo *repository.TransactionRepository,
	userRepo *repository.UserRepository,
) *Server {
	return &Server{txRepo: txRepo, userRepo: userRepo}
}

// CreateTransaction — Unary вызов
// Создаёт новую транзакцию и возвращает её
func (s *Server) CreateTransaction(
	ctx context.Context,
	req *pb.CreateTransactionRequest,
) (*pb.CreateTransactionResponse, error) {

	// Валидация — в gRPC возвращаем status ошибки с кодом
	// Это как HTTP статус коды но для gRPC
	if req.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be greater than 0")
	}
	if req.Category == "" {
		return nil, status.Error(codes.InvalidArgument, "category is required")
	}

	// Парсим дату
	date := time.Now()
	if req.Date != "" {
		var err error
		date, err = time.Parse("2006-01-02", req.Date)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid date format, use YYYY-MM-DD")
		}
	}

	// Конвертируем enum в строку
	txType := "expense"
	if req.Type == pb.TransactionType_INCOME {
		txType = "income"
	}

	// Создаём в БД
	t, err := s.txRepo.Create(&repository.Transaction{
		UserID:      req.UserId,
		Amount:      req.Amount,
		Type:        txType,
		Category:    req.Category,
		Description: req.Description,
		Date:        date,
	})
	if err != nil {
		slog.Error("failed to create transaction", "error", err)
		return nil, status.Error(codes.Internal, "failed to create transaction")
	}

	slog.Info("transaction created", "id", t.ID, "user_id", t.UserID, "amount", t.Amount)

	return &pb.CreateTransactionResponse{
		Transaction: toProto(t),
	}, nil
}

// GetTransaction — Unary вызов
func (s *Server) GetTransaction(
	ctx context.Context,
	req *pb.GetTransactionRequest,
) (*pb.Transaction, error) {

	t, err := s.txRepo.FindByID(req.Id, req.UserId)
	if err != nil {
		// codes.NotFound — аналог HTTP 404
		return nil, status.Error(codes.NotFound, "transaction not found")
	}

	return toProto(t), nil
}

// ListTransactions — Server Streaming
// Отправляем транзакции по одной в поток
// Это полезно когда транзакций может быть очень много —
// не грузим всё в память сразу
func (s *Server) ListTransactions(
	req *pb.ListTransactionsRequest,
	stream pb.TransactionService_ListTransactionsServer,
) error {

	transactions, err := s.txRepo.List(repository.ListFilter{
		UserID:   req.UserId,
		From:     req.From,
		To:       req.To,
		Category: req.Category,
		Type:     req.Type,
	})
	if err != nil {
		return status.Error(codes.Internal, "failed to fetch transactions")
	}

	// Отправляем каждую транзакцию отдельно через поток
	for _, t := range transactions {
		// Проверяем не отменён ли контекст (клиент отключился)
		if err := stream.Context().Err(); err != nil {
			return status.Error(codes.Canceled, "client disconnected")
		}

		if err := stream.Send(toProto(&t)); err != nil {
			return status.Error(codes.Internal, fmt.Sprintf("failed to send: %v", err))
		}
	}

	// Возвращаем nil — это сигнал что стрим закончен
	return nil
}

// DeleteTransaction — Unary вызов
func (s *Server) DeleteTransaction(
	ctx context.Context,
	req *pb.DeleteTransactionRequest,
) (*pb.DeleteTransactionResponse, error) {

	if err := s.txRepo.Delete(req.Id, req.UserId); err != nil {
		return nil, status.Error(codes.NotFound, "transaction not found")
	}

	return &pb.DeleteTransactionResponse{Success: true}, nil
}

// GetBalance — Unary вызов
func (s *Server) GetBalance(
	ctx context.Context,
	req *pb.GetBalanceRequest,
) (*pb.GetBalanceResponse, error) {

	balance, err := s.txRepo.GetBalance(req.UserId, req.From, req.To)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to calculate balance")
	}

	return &pb.GetBalanceResponse{
		TotalIncome:  balance.TotalIncome,
		TotalExpense: balance.TotalExpense,
		Balance:      balance.Balance,
	}, nil
}

// GetCategoryTotals — Unary вызов
func (s *Server) GetCategoryTotals(
	ctx context.Context,
	req *pb.GetCategoryTotalsRequest,
) (*pb.GetCategoryTotalsResponse, error) {

	totals, err := s.txRepo.GetCategoryTotals(req.UserId, req.From, req.To)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get category totals")
	}

	var categories []*pb.CategoryTotal
	for _, t := range totals {
		categories = append(categories, &pb.CategoryTotal{
			Category:   t.Category,
			Amount:     t.Amount,
			Percentage: t.Percentage,
			Count:      t.Count,
		})
	}

	return &pb.GetCategoryTotalsResponse{Categories: categories}, nil
}

// toProto конвертирует модель БД в protobuf структуру
// Это стандартный паттерн — держать конвертацию отдельно
func toProto(t *repository.Transaction) *pb.Transaction {
	txType := pb.TransactionType_EXPENSE
	if t.Type == "income" {
		txType = pb.TransactionType_INCOME
	}

	return &pb.Transaction{
		Id:          t.ID,
		UserId:      t.UserID,
		Amount:      t.Amount,
		Type:        txType,
		Category:    t.Category,
		Description: t.Description,
		Date:        t.Date.Format("2006-01-02"),
		CreatedAt:   t.CreatedAt.Format(time.RFC3339),
	}
}
