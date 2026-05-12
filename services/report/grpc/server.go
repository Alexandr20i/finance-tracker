package grpc

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/Alexandr20i/finance-tracker/gen/report"
	"github.com/Alexandr20i/finance-tracker/services/report/analytics"
)

type Server struct {
	pb.UnimplementedReportServiceServer
	analytics *analytics.Analytics
}

func NewServer(a *analytics.Analytics) *Server {
	return &Server{analytics: a}
}

// GetSummary — Unary
func (s *Server) GetSummary(
	ctx context.Context,
	req *pb.SummaryRequest,
) (*pb.SummaryResponse, error) {

	result, err := s.analytics.Summary(ctx, req.UserId, req.From, req.To)
	if err != nil {
		slog.Error("summary failed", "error", err)
		return nil, status.Error(codes.Internal, "failed to generate summary")
	}

	return &pb.SummaryResponse{
		TotalIncome:      result.TotalIncome,
		TotalExpense:     result.TotalExpense,
		Balance:          result.Balance,
		TransactionCount: result.TransactionCount,
		AvgDailyExpense:  result.AvgDailyExpense,
	}, nil
}

// GetCategoryBreakdown — Unary
func (s *Server) GetCategoryBreakdown(
	ctx context.Context,
	req *pb.CategoryBreakdownRequest,
) (*pb.CategoryBreakdownResponse, error) {

	items, err := s.analytics.CategoryBreakdown(ctx, req.UserId, req.From, req.To)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get breakdown")
	}

	var categories []*pb.CategoryItem
	for _, item := range items {
		categories = append(categories, &pb.CategoryItem{
			Category:   item.Category,
			Amount:     item.Amount,
			Percentage: item.Percentage,
			Count:      item.Count,
		})
	}

	return &pb.CategoryBreakdownResponse{Categories: categories}, nil
}

// GetMonthlyTrend — Unary
func (s *Server) GetMonthlyTrend(
	ctx context.Context,
	req *pb.MonthlyTrendRequest,
) (*pb.MonthlyTrendResponse, error) {

	months := req.Months
	if months <= 0 {
		months = 6 // по умолчанию 6 месяцев
	}

	points, err := s.analytics.MonthlyTrend(ctx, req.UserId, months)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get trend")
	}

	var protoPoints []*pb.MonthlyPoint
	for _, p := range points {
		protoPoints = append(protoPoints, &pb.MonthlyPoint{
			Month:   p.Month,
			Income:  p.Income,
			Expense: p.Expense,
			Balance: p.Balance,
		})
	}

	return &pb.MonthlyTrendResponse{Points: protoPoints}, nil
}

// GetForecast — Unary
func (s *Server) GetForecast(
	ctx context.Context,
	req *pb.ForecastRequest,
) (*pb.ForecastResponse, error) {

	forecast, err := s.analytics.Forecast(ctx, req.UserId)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate forecast")
	}

	var items []*pb.ForecastItem
	for _, item := range forecast.ByCategory {
		items = append(items, &pb.ForecastItem{
			Category:        item.Category,
			PredictedAmount: item.PredictedAmount,
			AvgLast_3Months: item.AvgLast3Months,
		})
	}

	return &pb.ForecastResponse{
		TotalPredicted: forecast.TotalPredicted,
		ByCategory:     items,
	}, nil
}

// StreamDailyExpenses — Server Streaming
// Отправляем данные для графика расходов по дням
func (s *Server) StreamDailyExpenses(
	req *pb.DailyExpenseRequest,
	stream pb.ReportService_StreamDailyExpensesServer,
) error {

	points, err := s.analytics.DailyExpenses(stream.Context(), req.UserId, req.From, req.To)
	if err != nil {
		return status.Error(codes.Internal, "failed to get daily expenses")
	}

	// Стримим каждую точку отдельно
	// Клиент может начать рисовать график не дожидаясь всех данных
	for _, point := range points {
		if err := stream.Context().Err(); err != nil {
			return status.Error(codes.Canceled, "client disconnected")
		}

		if err := stream.Send(&pb.DailyPoint{
			Date:   point.Date,
			Amount: point.Amount,
		}); err != nil {
			return err
		}
	}

	return nil
}
