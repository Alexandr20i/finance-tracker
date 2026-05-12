package grpc

import (
	"context"
	"io"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/Alexandr20i/finance-tracker/gen/budget"
	"github.com/Alexandr20i/finance-tracker/services/budget/repository"
)

type Server struct {
	pb.UnimplementedBudgetServiceServer
	repo *repository.BudgetRepository
}

func NewServer(repo *repository.BudgetRepository) *Server {
	return &Server{repo: repo}
}

// CreateBudget — Unary
func (s *Server) CreateBudget(
	ctx context.Context,
	req *pb.CreateBudgetRequest,
) (*pb.CreateBudgetResponse, error) {

	if req.LimitAmount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "limit must be greater than 0")
	}
	if req.Category == "" {
		return nil, status.Error(codes.InvalidArgument, "category is required")
	}

	period := periodToString(req.Period)

	b, err := s.repo.Create(&repository.Budget{
		UserID:      req.UserId,
		Category:    req.Category,
		LimitAmount: req.LimitAmount,
		Period:      period,
	})
	if err != nil {
		slog.Error("failed to create budget", "error", err)
		return nil, status.Error(codes.Internal, "failed to create budget")
	}

	return &pb.CreateBudgetResponse{Budget: toProto(b)}, nil
}

// GetBudget — Unary
func (s *Server) GetBudget(
	ctx context.Context,
	req *pb.GetBudgetRequest,
) (*pb.Budget, error) {

	b, err := s.repo.FindByID(req.Id, req.UserId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "budget not found")
	}
	return toProto(b), nil
}

// ListBudgets — Unary
func (s *Server) ListBudgets(
	ctx context.Context,
	req *pb.ListBudgetsRequest,
) (*pb.ListBudgetsResponse, error) {

	list, err := s.repo.ListByUser(req.UserId)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to fetch budgets")
	}

	var budgets []*pb.Budget
	for _, b := range list {
		budgets = append(budgets, toProto(&b))
	}
	return &pb.ListBudgetsResponse{Budgets: budgets}, nil
}

// UpdateBudget — Unary
func (s *Server) UpdateBudget(
	ctx context.Context,
	req *pb.UpdateBudgetRequest,
) (*pb.UpdateBudgetResponse, error) {

	b, err := s.repo.UpdateLimit(req.Id, req.UserId, req.LimitAmount)
	if err != nil {
		return nil, status.Error(codes.NotFound, "budget not found")
	}
	return &pb.UpdateBudgetResponse{Budget: toProto(b)}, nil
}

// DeleteBudget — Unary
func (s *Server) DeleteBudget(
	ctx context.Context,
	req *pb.DeleteBudgetRequest,
) (*pb.DeleteBudgetResponse, error) {

	if err := s.repo.Delete(req.Id, req.UserId); err != nil {
		return nil, status.Error(codes.NotFound, "budget not found")
	}
	return &pb.DeleteBudgetResponse{Success: true}, nil
}

// CheckAndUpdateBudget — Unary
// Вызывается Transaction Service при каждом расходе
func (s *Server) CheckAndUpdateBudget(
	ctx context.Context,
	req *pb.CheckBudgetRequest,
) (*pb.CheckBudgetResponse, error) {

	// Обновляем потраченную сумму
	b, err := s.repo.AddSpent(req.UserId, req.Category, req.Amount)
	if err != nil {
		// Если бюджет не найден — это не ошибка,
		// просто для этой категории нет лимита
		return &pb.CheckBudgetResponse{}, nil
	}

	// Считаем процент использования
	percentage := 0.0
	if b.LimitAmount > 0 {
		percentage = (b.SpentAmount / b.LimitAmount) * 100
	}

	isExceeded := b.SpentAmount >= b.LimitAmount
	isWarning := percentage >= 80 && !isExceeded

	slog.Info("budget checked",
		"user_id", req.UserId,
		"category", req.Category,
		"spent", b.SpentAmount,
		"limit", b.LimitAmount,
		"percentage", percentage,
		"exceeded", isExceeded,
	)

	return &pb.CheckBudgetResponse{
		IsExceeded: isExceeded,
		IsWarning:  isWarning,
		Spent:      b.SpentAmount,
		Limit:      b.LimitAmount,
		Percentage: percentage,
	}, nil
}

// WatchBudgetAlerts — Bidirectional Streaming
// Клиент (Transaction Service) шлёт события о расходах,
// сервер отвечает алертами если лимит превышен
func (s *Server) WatchBudgetAlerts(
	stream pb.BudgetService_WatchBudgetAlertsServer,
) error {
	slog.Info("budget alert stream opened")

	for {
		// Читаем следующее сообщение от клиента
		req, err := stream.Recv()
		if err == io.EOF {
			// Клиент закрыл свою сторону стрима
			slog.Info("budget alert stream closed by client")
			return nil
		}
		if err != nil {
			return status.Error(codes.Internal, "stream error")
		}

		// Обновляем бюджет
		b, err := s.repo.AddSpent(req.UserId, req.Category, req.Amount)
		if err != nil {
			continue // бюджет не найден — пропускаем
		}

		percentage := 0.0
		if b.LimitAmount > 0 {
			percentage = (b.SpentAmount / b.LimitAmount) * 100
		}

		// Отправляем алерт только если есть превышение или предупреждение
		if percentage >= 80 {
			exceeded := b.SpentAmount >= b.LimitAmount
			msg := "80% лимита достигнуто"
			if exceeded {
				msg = "лимит превышен!"
			}

			if err := stream.Send(&pb.BudgetAlert{
				UserId:     req.UserId,
				Category:   req.Category,
				Percentage: percentage,
				Exceeded:   exceeded,
				Message:    msg,
			}); err != nil {
				return err
			}
		}
	}
}

// periodToString конвертирует enum в строку
func periodToString(p pb.BudgetPeriod) string {
	switch p {
	case pb.BudgetPeriod_WEEKLY:
		return "weekly"
	case pb.BudgetPeriod_YEARLY:
		return "yearly"
	default:
		return "monthly"
	}
}

// toProto конвертирует модель БД в protobuf
func toProto(b *repository.Budget) *pb.Budget {
	percentage := 0.0
	if b.LimitAmount > 0 {
		percentage = (b.SpentAmount / b.LimitAmount) * 100
	}

	return &pb.Budget{
		Id:          b.ID,
		UserId:      b.UserID,
		Category:    b.Category,
		LimitAmount: b.LimitAmount,
		SpentAmount: b.SpentAmount,
		Period:      stringToPeriod(b.Period),
		IsExceeded:  b.SpentAmount >= b.LimitAmount,
		Percentage:  percentage,
	}
}

func stringToPeriod(p string) pb.BudgetPeriod {
	switch p {
	case "weekly":
		return pb.BudgetPeriod_WEEKLY
	case "yearly":
		return pb.BudgetPeriod_YEARLY
	default:
		return pb.BudgetPeriod_MONTHLY
	}
}
