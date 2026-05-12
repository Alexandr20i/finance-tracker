package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	pbb "github.com/Alexandr20i/finance-tracker/gen/budget"
	"github.com/Alexandr20i/finance-tracker/services/gateway/middleware"
)

type BudgetHandler struct {
	client pbb.BudgetServiceClient
}

func NewBudgetHandler(client pbb.BudgetServiceClient) *BudgetHandler {
	return &BudgetHandler{client: client}
}

type createBudgetRequest struct {
	Category    string  `json:"category"     binding:"required"`
	LimitAmount float64 `json:"limit_amount" binding:"required,gt=0"`
	Period      string  `json:"period"       binding:"oneof=monthly weekly yearly"`
}

func (h *BudgetHandler) Create(c *gin.Context) {
	var req createBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	period := pbb.BudgetPeriod_MONTHLY
	switch req.Period {
	case "weekly":
		period = pbb.BudgetPeriod_WEEKLY
	case "yearly":
		period = pbb.BudgetPeriod_YEARLY
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := h.client.CreateBudget(ctx, &pbb.CreateBudgetRequest{
		UserId:      middleware.GetUserID(c),
		Category:    req.Category,
		LimitAmount: req.LimitAmount,
		Period:      period,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create budget"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": resp.Budget})
}

func (h *BudgetHandler) List(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := h.client.ListBudgets(ctx, &pbb.ListBudgetsRequest{
		UserId: middleware.GetUserID(c),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch budgets"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp.Budgets})
}

func (h *BudgetHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req struct {
		LimitAmount float64 `json:"limit_amount" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := h.client.UpdateBudget(ctx, &pbb.UpdateBudgetRequest{
		Id:          id,
		UserId:      middleware.GetUserID(c),
		LimitAmount: req.LimitAmount,
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "budget not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp.Budget})
}

func (h *BudgetHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	_, err = h.client.DeleteBudget(ctx, &pbb.DeleteBudgetRequest{
		Id:     id,
		UserId: middleware.GetUserID(c),
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "budget not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
