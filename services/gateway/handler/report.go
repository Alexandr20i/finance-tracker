package handler

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	pbr "github.com/Alexandr20i/finance-tracker/gen/report"
	"github.com/Alexandr20i/finance-tracker/services/gateway/middleware"
)

type ReportHandler struct {
	client pbr.ReportServiceClient
}

func NewReportHandler(client pbr.ReportServiceClient) *ReportHandler {
	return &ReportHandler{client: client}
}

func (h *ReportHandler) Summary(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.client.GetSummary(ctx, &pbr.SummaryRequest{
		UserId: middleware.GetUserID(c),
		From:   c.Query("from"),
		To:     c.Query("to"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get summary"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

func (h *ReportHandler) CategoryBreakdown(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.client.GetCategoryBreakdown(ctx, &pbr.CategoryBreakdownRequest{
		UserId: middleware.GetUserID(c),
		From:   c.Query("from"),
		To:     c.Query("to"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get breakdown"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp.Categories})
}

func (h *ReportHandler) MonthlyTrend(c *gin.Context) {
	months := int32(6)
	if m := c.Query("months"); m != "" {
		if v, err := strconv.ParseInt(m, 10, 32); err == nil {
			months = int32(v)
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.client.GetMonthlyTrend(ctx, &pbr.MonthlyTrendRequest{
		UserId: middleware.GetUserID(c),
		Months: months,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get trend"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp.Points})
}

func (h *ReportHandler) Forecast(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.client.GetForecast(ctx, &pbr.ForecastRequest{
		UserId: middleware.GetUserID(c),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get forecast"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// DailyExpenses — Server Streaming через REST
// Читаем стрим и собираем в JSON массив
func (h *ReportHandler) DailyExpenses(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	stream, err := h.client.StreamDailyExpenses(ctx, &pbr.DailyExpenseRequest{
		UserId: middleware.GetUserID(c),
		From:   c.Query("from"),
		To:     c.Query("to"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stream expenses"})
		return
	}

	var points []interface{}
	for {
		point, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			slog.Error("daily expenses stream error", "error", err)
			break
		}
		points = append(points, point)
	}

	if points == nil {
		points = []interface{}{}
	}

	c.JSON(http.StatusOK, gin.H{"data": points})
}
