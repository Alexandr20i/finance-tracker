package repository

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type Budget struct {
	ID          int64     `db:"id"`
	UserID      int64     `db:"user_id"`
	Category    string    `db:"category"`
	LimitAmount float64   `db:"limit_amount"`
	SpentAmount float64   `db:"spent_amount"`
	Period      string    `db:"period"`
	CreatedAt   time.Time `db:"created_at"`
}

type BudgetRepository struct{ db *sqlx.DB }

func NewBudgetRepository(db *sqlx.DB) *BudgetRepository {
	return &BudgetRepository{db: db}
}

func (r *BudgetRepository) Create(b *Budget) (*Budget, error) {
	result := &Budget{}
	return result, r.db.QueryRowx(`
		INSERT INTO budgets (user_id, category, limit_amount, period)
		VALUES ($1, $2, $3, $4)
		RETURNING *`,
		b.UserID, b.Category, b.LimitAmount, b.Period,
	).StructScan(result)
}

func (r *BudgetRepository) FindByID(id, userID int64) (*Budget, error) {
	b := &Budget{}
	return b, r.db.QueryRowx(
		`SELECT * FROM budgets WHERE id = $1 AND user_id = $2`, id, userID,
	).StructScan(b)
}

func (r *BudgetRepository) FindByCategory(userID int64, category, period string) (*Budget, error) {
	b := &Budget{}
	return b, r.db.QueryRowx(
		`SELECT * FROM budgets WHERE user_id = $1 AND category = $2 AND period = $3`,
		userID, category, period,
	).StructScan(b)
}

func (r *BudgetRepository) ListByUser(userID int64) ([]Budget, error) {
	var list []Budget
	return list, r.db.Select(&list,
		`SELECT * FROM budgets WHERE user_id = $1 ORDER BY category`, userID,
	)
}

func (r *BudgetRepository) UpdateLimit(id, userID int64, limit float64) (*Budget, error) {
	b := &Budget{}
	return b, r.db.QueryRowx(`
		UPDATE budgets SET limit_amount = $1
		WHERE id = $2 AND user_id = $3
		RETURNING *`,
		limit, id, userID,
	).StructScan(b)
}

// AddSpent — атомарно увеличивает spent_amount
// RETURNING * — возвращает обновлённую запись одним запросом
func (r *BudgetRepository) AddSpent(userID int64, category string, amount float64) (*Budget, error) {
	b := &Budget{}
	return b, r.db.QueryRowx(`
		UPDATE budgets
		SET spent_amount = spent_amount + $1
		WHERE user_id = $2 AND category = $3 AND period = 'monthly'
		RETURNING *`,
		amount, userID, category,
	).StructScan(b)
}

func (r *BudgetRepository) Delete(id, userID int64) error {
	res, err := r.db.Exec(
		`DELETE FROM budgets WHERE id = $1 AND user_id = $2`, id, userID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}
