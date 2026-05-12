package handler

import (
	"context"
	//"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	pbt "github.com/Alexandr20i/finance-tracker/gen/transaction"
	"github.com/Alexandr20i/finance-tracker/services/gateway/middleware"
)

type AuthHandler struct {
	txClient  pbt.TransactionServiceClient
	jwtSecret string
	jwtExpH   int
}

func NewAuthHandler(txClient pbt.TransactionServiceClient, secret string, expH int) *AuthHandler {
	return &AuthHandler{txClient: txClient, jwtSecret: secret, jwtExpH: expH}
}

type registerRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Name     string `json:"name"     binding:"required"`
}

type loginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Register godoc
// @Summary      Регистрация
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body registerRequest true "Данные"
// @Success      201 {object} map[string]interface{}
// @Router       /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest

	// binding:"required" — Gin сам валидирует
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Хэшируем пароль
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	// Создаём пользователя через gRPC в Transaction Service
	// Transaction Service хранит пользователей потому что
	// транзакции привязаны к user_id
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// Используем внутренний RPC для создания пользователя
	// Это не публичный метод — только Gateway его вызывает
	resp, err := h.txClient.CreateUser(ctx, &pbt.CreateUserRequest{
		Email:        req.Email,
		PasswordHash: string(hash),
		Name:         req.Name,
	})
	if err != nil {
		slog.Error("failed to create user", "error", err)
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	}

	token, err := middleware.GenerateToken(resp.User.Id, h.jwtSecret, h.jwtExpH)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token": token,
		"user": gin.H{
			"id":    resp.User.Id,
			"email": resp.User.Email,
			"name":  resp.User.Name,
		},
	})
}

// Login godoc
// @Summary      Вход
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body loginRequest true "Данные"
// @Success      200 {object} map[string]interface{}
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// Получаем пользователя из Transaction Service
	resp, err := h.txClient.GetUserByEmail(ctx, &pbt.GetUserByEmailRequest{
		Email: req.Email,
	})
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// Проверяем пароль
	if err := bcrypt.CompareHashAndPassword(
		[]byte(resp.User.PasswordHash),
		[]byte(req.Password),
	); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, err := middleware.GenerateToken(resp.User.Id, h.jwtSecret, h.jwtExpH)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":    resp.User.Id,
			"email": resp.User.Email,
			"name":  resp.User.Name,
		},
	})
}
