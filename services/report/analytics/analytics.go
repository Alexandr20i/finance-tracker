package analytics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"

	pb "github.com/Alexandr20i/finance-tracker/gen/transaction"
	"github.com/Alexandr20i/finance-tracker/services/report/cache"
	"github.com/Alexandr20i/finance-tracker/services/report/client"
)

// Analytics — считает отчёты на основе данных из Transaction Service
type Analytics struct {
	// txClient *client.TransactionClient
	txClient client.TransactionClientInterface
	cache    *cache.Cache
}

func NewAnalytics(txClient client.TransactionClientInterface, c *cache.Cache) *Analytics {
	return &Analytics{txClient: txClient, cache: c}
}

// Summary — сводка за период
func (a *Analytics) Summary(ctx context.Context, userID int64, from, to string) (*SummaryResult, error) {
	cacheKey := fmt.Sprintf("summary:%d:%s:%s", userID, from, to)

	// Пробуем достать из кэша
	var cached SummaryResult
	if err := a.cache.Get(ctx, cacheKey, &cached); err == nil {
		slog.Info("cache hit", "key", cacheKey)
		return &cached, nil
	} else if !errors.Is(err, redis.Nil) {
		slog.Warn("cache error", "error", err)
	}

	// Кэш miss — считаем
	slog.Info("cache miss", "key", cacheKey)

	balance, err := a.txClient.GetBalance(ctx, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	transactions, err := a.txClient.ListAllTransactions(ctx, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to list transactions: %w", err)
	}

	days := 30.0
	if from != "" && to != "" {
		fromDate, _ := time.Parse("2006-01-02", from)
		toDate, _ := time.Parse("2006-01-02", to)
		d := toDate.Sub(fromDate).Hours() / 24
		if d >= 1 {
			days = d
		}
	}

	result := &SummaryResult{
		TotalIncome:      balance.TotalIncome,
		TotalExpense:     balance.TotalExpense,
		Balance:          balance.Balance,
		TransactionCount: int32(len(transactions)),
		AvgDailyExpense:  balance.TotalExpense / days,
	}

	// Кэшируем на 5 минут
	if err := a.cache.Set(ctx, cacheKey, result, 5*time.Minute); err != nil {
		slog.Warn("failed to cache summary", "error", err)
	}

	return result, nil
}

// CategoryBreakdown — расходы по категориям
func (a *Analytics) CategoryBreakdown(ctx context.Context, userID int64, from, to string) ([]CategoryItem, error) {
	cacheKey := fmt.Sprintf("breakdown:%d:%s:%s", userID, from, to)

	var cached []CategoryItem
	if err := a.cache.Get(ctx, cacheKey, &cached); err == nil {
		slog.Info("cache hit", "key", cacheKey)
		return cached, nil
	}

	totals, err := a.txClient.GetCategoryTotals(ctx, userID, from, to)
	if err != nil {
		return nil, err
	}

	var items []CategoryItem
	for _, t := range totals.Categories {
		items = append(items, CategoryItem{
			Category:   t.Category,
			Amount:     t.Amount,
			Percentage: t.Percentage,
			Count:      t.Count,
		})
	}

	if err := a.cache.Set(ctx, cacheKey, items, 5*time.Minute); err != nil {
		slog.Warn("failed to cache breakdown", "error", err)
	}

	return items, nil
}

// MonthlyTrend — тренд по месяцам за последние N месяцев
func (a *Analytics) MonthlyTrend(ctx context.Context, userID int64, months int32) ([]MonthlyPoint, error) {
	now := time.Now()
	var points []MonthlyPoint

	// Идём по каждому месяцу назад
	for i := int(months) - 1; i >= 0; i-- {
		// Первый день месяца
		monthStart := time.Date(now.Year(), now.Month()-time.Month(i), 1, 0, 0, 0, 0, time.UTC)
		// Последний день месяца
		monthEnd := monthStart.AddDate(0, 1, -1)

		from := monthStart.Format("2006-01-02")
		to := monthEnd.Format("2006-01-02")

		balance, err := a.txClient.GetBalance(ctx, userID, from, to)
		if err != nil {
			continue
		}

		points = append(points, MonthlyPoint{
			Month:   monthStart.Format("2006-01"),
			Income:  balance.TotalIncome,
			Expense: balance.TotalExpense,
			Balance: balance.Balance,
		})
	}

	return points, nil
}

// Forecast — прогноз на следующий месяц
// Берём среднее расходов за последние 3 месяца по каждой категории
func (a *Analytics) Forecast(ctx context.Context, userID int64) (*ForecastResult, error) {
	now := time.Now()

	// Собираем данные за последние 3 месяца
	categoryAmounts := make(map[string][]float64)

	for i := 1; i <= 3; i++ {
		monthStart := time.Date(now.Year(), now.Month()-time.Month(i), 1, 0, 0, 0, 0, time.UTC)
		monthEnd := monthStart.AddDate(0, 1, -1)

		totals, err := a.txClient.GetCategoryTotals(
			ctx, userID,
			monthStart.Format("2006-01-02"),
			monthEnd.Format("2006-01-02"),
		)
		if err != nil {
			continue
		}

		for _, t := range totals.Categories {
			categoryAmounts[t.Category] = append(categoryAmounts[t.Category], t.Amount)
		}
	}

	// Считаем среднее по каждой категории
	var items []ForecastItem
	totalPredicted := 0.0

	for category, amounts := range categoryAmounts {
		sum := 0.0
		for _, a := range amounts {
			sum += a
		}
		avg := sum / float64(len(amounts))
		totalPredicted += avg

		items = append(items, ForecastItem{
			Category:        category,
			PredictedAmount: avg,
			AvgLast3Months:  avg,
		})
	}

	// Сортируем по убыванию предсказанной суммы
	sort.Slice(items, func(i, j int) bool {
		return items[i].PredictedAmount > items[j].PredictedAmount
	})

	return &ForecastResult{
		TotalPredicted: totalPredicted,
		ByCategory:     items,
	}, nil
}

// DailyExpenses — расходы по дням для графика
func (a *Analytics) DailyExpenses(ctx context.Context, userID int64, from, to string) ([]DailyPoint, error) {
	transactions, err := a.txClient.ListAllTransactions(ctx, userID, from, to)
	if err != nil {
		return nil, err
	}

	// Группируем расходы по дням
	dailyMap := make(map[string]float64)
	for _, tx := range transactions {
		if tx.Type == pb.TransactionType_EXPENSE {
			dailyMap[tx.Date] += tx.Amount
		}
	}

	// Конвертируем в слайс и сортируем по дате
	var points []DailyPoint
	for date, amount := range dailyMap {
		points = append(points, DailyPoint{Date: date, Amount: amount})
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i].Date < points[j].Date
	})

	return points, nil
}

// Модели результатов

type SummaryResult struct {
	TotalIncome      float64
	TotalExpense     float64
	Balance          float64
	TransactionCount int32
	AvgDailyExpense  float64
}

type CategoryItem struct {
	Category   string
	Amount     float64
	Percentage float64
	Count      int32
}

type MonthlyPoint struct {
	Month   string
	Income  float64
	Expense float64
	Balance float64
}

type ForecastItem struct {
	Category        string
	PredictedAmount float64
	AvgLast3Months  float64
}

type ForecastResult struct {
	TotalPredicted float64
	ByCategory     []ForecastItem
}

type DailyPoint struct {
	Date   string
	Amount float64
}
