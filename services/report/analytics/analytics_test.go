package analytics_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	pb "github.com/Alexandr20i/finance-tracker/gen/transaction"
	"github.com/Alexandr20i/finance-tracker/services/report/analytics"
	"github.com/Alexandr20i/finance-tracker/services/report/analytics/mocks"
)

// TestSummary — проверяем что Summary правильно считает среднедневной расход
func TestSummary(t *testing.T) {
	mockClient := new(mocks.TransactionClient)

	// Говорим моку что вернуть при вызове GetBalance
	mockClient.On("GetBalance", mock.Anything, int64(1), "2026-05-01", "2026-05-31").
		Return(&pb.GetBalanceResponse{
			TotalIncome:  100000,
			TotalExpense: 30000,
			Balance:      70000,
		}, nil)

	// ListAllTransactions вернёт 3 транзакции
	mockClient.On("ListAllTransactions", mock.Anything, int64(1), "2026-05-01", "2026-05-31").
		Return([]*pb.Transaction{
			{Id: 1, Amount: 10000, Type: pb.TransactionType_EXPENSE},
			{Id: 2, Amount: 15000, Type: pb.TransactionType_EXPENSE},
			{Id: 3, Amount: 100000, Type: pb.TransactionType_INCOME},
		}, nil)

	a := analytics.NewAnalytics(mockClient)
	result, err := a.Summary(context.Background(), 1, "2026-05-01", "2026-05-31")

	assert.NoError(t, err)
	assert.Equal(t, float64(100000), result.TotalIncome)
	assert.Equal(t, float64(30000), result.TotalExpense)
	assert.Equal(t, float64(70000), result.Balance)
	assert.Equal(t, int32(3), result.TransactionCount)

	// Среднедневной расход: 30000 / 30 дней = 1000
	assert.InDelta(t, 1000.0, result.AvgDailyExpense, 1.0)

	mockClient.AssertExpectations(t)
}

// TestCategoryBreakdown — проверяем разбивку по категориям
func TestCategoryBreakdown(t *testing.T) {
	mockClient := new(mocks.TransactionClient)

	mockClient.On("GetCategoryTotals", mock.Anything, int64(1), "2026-05-01", "2026-05-31").
		Return(&pb.GetCategoryTotalsResponse{
			Categories: []*pb.CategoryTotal{
				{Category: "еда", Amount: 15000, Percentage: 50, Count: 10},
				{Category: "транспорт", Amount: 9000, Percentage: 30, Count: 5},
				{Category: "развлечения", Amount: 6000, Percentage: 20, Count: 3},
			},
		}, nil)

	a := analytics.NewAnalytics(mockClient)
	items, err := a.CategoryBreakdown(context.Background(), 1, "2026-05-01", "2026-05-31")

	assert.NoError(t, err)
	assert.Len(t, items, 3)
	assert.Equal(t, "еда", items[0].Category)
	assert.Equal(t, float64(15000), items[0].Amount)
	assert.Equal(t, float64(50), items[0].Percentage)

	mockClient.AssertExpectations(t)
}

// TestDailyExpenses — проверяем группировку расходов по дням
func TestDailyExpenses(t *testing.T) {
	mockClient := new(mocks.TransactionClient)

	// Два расхода в один день — должны суммироваться
	mockClient.On("ListAllTransactions", mock.Anything, int64(1), "2026-05-01", "2026-05-03").
		Return([]*pb.Transaction{
			{Type: pb.TransactionType_EXPENSE, Amount: 500, Date: "2026-05-01"},
			{Type: pb.TransactionType_EXPENSE, Amount: 1500, Date: "2026-05-01"}, // тот же день
			{Type: pb.TransactionType_EXPENSE, Amount: 800, Date: "2026-05-02"},
			{Type: pb.TransactionType_INCOME, Amount: 50000, Date: "2026-05-01"}, // доход — не считаем
		}, nil)

	a := analytics.NewAnalytics(mockClient)
	points, err := a.DailyExpenses(context.Background(), 1, "2026-05-01", "2026-05-03")

	assert.NoError(t, err)
	assert.Len(t, points, 2) // два уникальных дня

	// Первый день: 500 + 1500 = 2000
	assert.Equal(t, "2026-05-01", points[0].Date)
	assert.Equal(t, float64(2000), points[0].Amount)

	// Второй день: 800
	assert.Equal(t, "2026-05-02", points[1].Date)
	assert.Equal(t, float64(800), points[1].Amount)

	mockClient.AssertExpectations(t)
}

// TestForecast_EmptyHistory — прогноз без истории
func TestForecast_EmptyHistory(t *testing.T) {
	mockClient := new(mocks.TransactionClient)

	// Нет данных за последние 3 месяца
	mockClient.On("GetCategoryTotals", mock.Anything, int64(1), mock.AnythingOfType("string"), mock.AnythingOfType("string")).
		Return(&pb.GetCategoryTotalsResponse{
			Categories: []*pb.CategoryTotal{},
		}, nil)

	a := analytics.NewAnalytics(mockClient)
	result, err := a.Forecast(context.Background(), 1)

	assert.NoError(t, err)
	assert.Equal(t, float64(0), result.TotalPredicted)
	assert.Empty(t, result.ByCategory)
}

// TestSummary_ZeroExpense — нет расходов
func TestSummary_ZeroExpense(t *testing.T) {
	mockClient := new(mocks.TransactionClient)

	mockClient.On("GetBalance", mock.Anything, int64(1), "", "").
		Return(&pb.GetBalanceResponse{
			TotalIncome:  50000,
			TotalExpense: 0,
			Balance:      50000,
		}, nil)

	mockClient.On("ListAllTransactions", mock.Anything, int64(1), "", "").
		Return([]*pb.Transaction{
			{Id: 1, Amount: 50000, Type: pb.TransactionType_INCOME},
		}, nil)

	a := analytics.NewAnalytics(mockClient)
	result, err := a.Summary(context.Background(), 1, "", "")

	assert.NoError(t, err)
	assert.Equal(t, float64(0), result.TotalExpense)
	assert.Equal(t, float64(50000), result.Balance)
	assert.Equal(t, float64(0), result.AvgDailyExpense)
}
