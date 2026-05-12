package handler

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	pbb "github.com/Alexandr20i/finance-tracker/gen/budget"
	pbt "github.com/Alexandr20i/finance-tracker/gen/transaction"
	"github.com/Alexandr20i/finance-tracker/services/gateway/middleware"
)

type TransactionHandler struct {
	txClient     pbt.TransactionServiceClient
	budgetClient pbb.BudgetServiceClient
}

func NewTransactionHandler(
	tx pbt.TransactionServiceClient,
	budget pbb.BudgetServiceClient,
) *TransactionHandler {
	return &TransactionHandler{txClient: tx, budgetClient: budget}
}

type createTransactionRequest struct {
	Amount      float64 `json:"amount"      binding:"required,gt=0"`
	Type        string  `json:"type"        binding:"required,oneof=income expense"`
	Category    string  `json:"category"    binding:"required"`
	Description string  `json:"description"`
	Date        string  `json:"date"`
}

// Create godoc
// @Summary      Создать транзакцию
// @Tags         transactions
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body body createTransactionRequest true "Транзакция"
// @Success      201 {object} map[string]interface{}
// @Router       /transactions [post]
func (h *TransactionHandler) Create(c *gin.Context) {
	var req createTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := middleware.GetUserID(c)

	txType := pbt.TransactionType_EXPENSE
	if req.Type == "income" {
		txType = pbt.TransactionType_INCOME
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// Создаём транзакцию в Transaction Service
	resp, err := h.txClient.CreateTransaction(ctx, &pbt.CreateTransactionRequest{
		UserId:      userID,
		Amount:      req.Amount,
		Type:        txType,
		Category:    req.Category,
		Description: req.Description,
		Date:        req.Date,
	})
	if err != nil {
		slog.Error("failed to create transaction", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create transaction"})
		return
	}

	// Если это расход — проверяем бюджет через Budget Service
	// Делаем это асинхронно чтобы не замедлять ответ пользователю
	if req.Type == "expense" {
		go func() {
			budgetCtx, budgetCancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer budgetCancel()

			budgetResp, err := h.budgetClient.CheckAndUpdateBudget(budgetCtx, &pbb.CheckBudgetRequest{
				UserId:   userID,
				Category: req.Category,
				Amount:   req.Amount,
			})
			if err != nil {
				slog.Warn("budget check failed", "error", err)
				return
			}

			if budgetResp.IsExceeded {
				slog.Warn("budget exceeded",
					"user_id", userID,
					"category", req.Category,
					"percentage", budgetResp.Percentage,
				)
			} else if budgetResp.IsWarning {
				slog.Warn("budget warning",
					"user_id", userID,
					"category", req.Category,
					"percentage", budgetResp.Percentage,
				)
			}
		}()
	}

	c.JSON(http.StatusCreated, gin.H{"data": resp.Transaction})
}

// List godoc
// @Summary      История транзакций
// @Tags         transactions
// @Security     BearerAuth
// @Produce      json
// @Param        from     query string false "От (YYYY-MM-DD)"
// @Param        to       query string false "До (YYYY-MM-DD)"
// @Param        category query string false "Категория"
// @Param        type     query string false "Тип (income/expense)"
// @Success      200 {object} map[string]interface{}
// @Router       /transactions [get]
func (h *TransactionHandler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Открываем Server Streaming
	stream, err := h.txClient.ListTransactions(ctx, &pbt.ListTransactionsRequest{
		UserId:   userID,
		From:     c.Query("from"),
		To:       c.Query("to"),
		Category: c.Query("category"),
		Type:     c.Query("type"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch transactions"})
		return
	}

	// Читаем все транзакции из стрима
	var transactions []interface{}
	for {
		tx, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			slog.Error("stream error", "error", err)
			break
		}
		transactions = append(transactions, tx)
	}

	if transactions == nil {
		transactions = []interface{}{}
	}

	c.JSON(http.StatusOK, gin.H{"data": transactions})
}

// Get godoc
// @Summary      Детали транзакции
// @Tags         transactions
// @Security     BearerAuth
// @Produce      json
// @Param        id path int true "ID транзакции"
// @Success      200 {object} map[string]interface{}
// @Router       /transactions/{id} [get]
func (h *TransactionHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	tx, err := h.txClient.GetTransaction(ctx, &pbt.GetTransactionRequest{
		Id:     id,
		UserId: middleware.GetUserID(c),
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": tx})
}

// Delete godoc
// @Summary      Удалить транзакцию
// @Tags         transactions
// @Security     BearerAuth
// @Produce      json
// @Param        id path int true "ID транзакции"
// @Success      200 {object} map[string]interface{}
// @Router       /transactions/{id} [delete]
func (h *TransactionHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	_, err = h.txClient.DeleteTransaction(ctx, &pbt.DeleteTransactionRequest{
		Id:     id,
		UserId: middleware.GetUserID(c),
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
