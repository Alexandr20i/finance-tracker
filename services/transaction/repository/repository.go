package repository

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// User модель
type User struct {
	ID           int64     `db:"id"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"password_hash"`
	Name         string    `db:"name"`
	CreatedAt    time.Time `db:"created_at"`
}

// Transaction модель
type Transaction struct {
	ID          int64     `db:"id"`
	UserID      int64     `db:"user_id"`
	Amount      float64   `db:"amount"`
	Type        string    `db:"type"`
	Category    string    `db:"category"`
	Description string    `db:"description"`
	Date        time.Time `db:"date"`
	CreatedAt   time.Time `db:"created_at"`
}

// CategoryTotal для агрегации
type CategoryTotal struct {
	Category   string  `db:"category"`
	Amount     float64 `db:"amount"`
	Percentage float64 `db:"percentage"`
	Count      int32   `db:"count"`
}

// BalanceSummary для баланса
type BalanceSummary struct {
	TotalIncome  float64 `db:"total_income"`
	TotalExpense float64 `db:"total_expense"`
	Balance      float64 `db:"balance"`
}

// Фильтры для ListTransactions
type ListFilter struct {
	UserID   int64
	From     string
	To       string
	Category string
	Type     string
}

// ===================== User Repository =====================

type UserRepository struct{ db *sqlx.DB }

func NewUserRepository(db *sqlx.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(email, passwordHash, name string) (*User, error) {
	u := &User{}
	return u, r.db.QueryRowx(`
		INSERT INTO users (email, password_hash, name)
		VALUES ($1, $2, $3)
		RETURNING *`,
		email, passwordHash, name,
	).StructScan(u)
}

func (r *UserRepository) FindByEmail(email string) (*User, error) {
	u := &User{}
	return u, r.db.QueryRowx(
		`SELECT * FROM users WHERE email = $1`, email,
	).StructScan(u)
}

func (r *UserRepository) FindByID(id int64) (*User, error) {
	u := &User{}
	return u, r.db.QueryRowx(
		`SELECT * FROM users WHERE id = $1`, id,
	).StructScan(u)
}

func (r *UserRepository) ListAll() ([]User, error) {
	var users []User
	return users, r.db.Select(&users, `SELECT * FROM users ORDER BY id`)
}

// ===================== Transaction Repository =====================

type TransactionRepository struct{ db *sqlx.DB }

func NewTransactionRepository(db *sqlx.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

func (r *TransactionRepository) Create(t *Transaction) (*Transaction, error) {
	result := &Transaction{}
	return result, r.db.QueryRowx(`
		INSERT INTO transactions (user_id, amount, type, category, description, date)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING *`,
		t.UserID, t.Amount, t.Type, t.Category, t.Description, t.Date,
	).StructScan(result)
}

func (r *TransactionRepository) FindByID(id, userID int64) (*Transaction, error) {
	t := &Transaction{}
	return t, r.db.QueryRowx(
		`SELECT * FROM transactions WHERE id = $1 AND user_id = $2`, id, userID,
	).StructScan(t)
}

// List — динамически строим WHERE в зависимости от фильтров
// Это важный паттерн — никогда не конкатенируй SQL строки напрямую
func (r *TransactionRepository) List(f ListFilter) ([]Transaction, error) {
	// Начинаем с базового запроса
	query := `SELECT * FROM transactions WHERE user_id = $1`
	args := []interface{}{f.UserID}
	i := 2 // счётчик параметров

	// Добавляем фильтры только если они заданы
	if f.From != "" {
		query += fmt.Sprintf(" AND date >= $%d", i)
		args = append(args, f.From)
		i++
	}
	if f.To != "" {
		query += fmt.Sprintf(" AND date <= $%d", i)
		args = append(args, f.To)
		i++
	}
	if f.Category != "" {
		query += fmt.Sprintf(" AND category = $%d", i)
		args = append(args, f.Category)
		i++
	}
	if f.Type != "" {
		query += fmt.Sprintf(" AND type = $%d", i)
		args = append(args, f.Type)
		i++
	}

	query += " ORDER BY date DESC"

	var list []Transaction
	return list, r.db.Select(&list, query, args...)
}

func (r *TransactionRepository) Delete(id, userID int64) error {
	res, err := r.db.Exec(
		`DELETE FROM transactions WHERE id = $1 AND user_id = $2`, id, userID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("transaction not found")
	}
	return nil
}

func (r *TransactionRepository) GetBalance(userID int64, from, to string) (*BalanceSummary, error) {
	s := &BalanceSummary{}

	// Одним запросом считаем и доходы и расходы
	// FILTER (WHERE ...) — это PostgreSQL синтаксис для условной агрегации
	query := `
		SELECT
			COALESCE(SUM(amount) FILTER (WHERE type = 'income'), 0)  AS total_income,
			COALESCE(SUM(amount) FILTER (WHERE type = 'expense'), 0) AS total_expense,
			COALESCE(SUM(amount) FILTER (WHERE type = 'income'), 0) -
			COALESCE(SUM(amount) FILTER (WHERE type = 'expense'), 0) AS balance
		FROM transactions
		WHERE user_id = $1`

	args := []interface{}{userID}
	i := 2

	if from != "" {
		query += fmt.Sprintf(" AND date >= $%d", i)
		args = append(args, from)
		i++
	}
	if to != "" {
		query += fmt.Sprintf(" AND date <= $%d", i)
		args = append(args, to)
	}

	return s, r.db.QueryRowx(query, args...).StructScan(s)
}

func (r *TransactionRepository) GetCategoryTotals(userID int64, from, to string) ([]CategoryTotal, error) {
	// Считаем сумму по каждой категории + процент от общего
	// SUM(...) OVER () — это оконная функция PostgreSQL
	// Она считает общую сумму по всем строкам чтобы посчитать процент
	query := `
		SELECT
			category,
			SUM(amount) AS amount,
			COUNT(*) AS count,
			ROUND(
				SUM(amount) * 100.0 / NULLIF(SUM(SUM(amount)) OVER (), 0),
				2
			) AS percentage
		FROM transactions
		WHERE user_id = $1 AND type = 'expense'`

	args := []interface{}{userID}
	i := 2

	if from != "" {
		query += fmt.Sprintf(" AND date >= $%d", i)
		args = append(args, from)
		i++
	}
	if to != "" {
		query += fmt.Sprintf(" AND date <= $%d", i)
		args = append(args, to)
	}

	query += " GROUP BY category ORDER BY amount DESC"

	var list []CategoryTotal
	return list, r.db.Select(&list, query, args...)
}
