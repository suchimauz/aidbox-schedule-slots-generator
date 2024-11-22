package http

import (
	"crypto/subtle"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/config"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/in"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
)

type SlotGeneratorController struct {
	useCase in.SlotGeneratorUseCase
	cfg     *config.Config
	logger  out.LoggerPort
}

func NewSlotGeneratorController(useCase in.SlotGeneratorUseCase, cfg *config.Config, logger out.LoggerPort) *SlotGeneratorController {
	return &SlotGeneratorController{
		useCase: useCase,
		cfg:     cfg,
		logger:  logger,
	}
}

func (c *SlotGeneratorController) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/v1")
	api.Use(c.basicAuth())
	{
		api.POST("/slots/generate/:scheduleId", c.generateSlots)
		api.POST("/slots/generate-batch", c.generateBatchSlots)
	}
}

type GenerateSlotsRequest struct {
	StartDate string `json:"startDate" binding:"required"`
	EndDate   string `json:"endDate" binding:"required"`
}

type GenerateBatchSlotsRequest struct {
	ScheduleIDs []uuid.UUID `json:"scheduleIds" binding:"required,min=1"`
	StartDate   string      `json:"startDate" binding:"required"`
	EndDate     string      `json:"endDate" binding:"required"`
}

func (c *SlotGeneratorController) generateSlots(ctx *gin.Context) {
	scheduleID, err := uuid.Parse(ctx.Param("scheduleId"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid schedule ID format"})
		return
	}

	var req GenerateSlotsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err = time.Parse(time.RFC3339, req.StartDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start date format"})
		return
	}

	_, err = time.Parse(time.RFC3339, req.EndDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end date format"})
		return
	}

	slots, err := c.useCase.GenerateSlots(ctx.Request.Context(), scheduleID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"scheduleId": scheduleID,
		"slots":      slots,
	})
}

func (c *SlotGeneratorController) generateBatchSlots(ctx *gin.Context) {
	var req GenerateBatchSlotsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err := time.Parse(time.RFC3339, req.StartDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start date format"})
		return
	}

	_, err = time.Parse(time.RFC3339, req.EndDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end date format"})
		return
	}

	result, err := c.useCase.GenerateBatchSlots(ctx.Request.Context(), req.ScheduleIDs)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"results": result})
}

func (c *SlotGeneratorController) basicAuth() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		username, password, hasAuth := ctx.Request.BasicAuth()
		if !hasAuth {
			ctx.Header("WWW-Authenticate", "Basic realm=Authorization Required")
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// Проверка авторизации для нескольких клиентов
		authenticated := false
		for _, client := range c.cfg.Auth.BasicClients {
			if subtle.ConstantTimeCompare([]byte(username), []byte(client.Username)) == 1 &&
				subtle.ConstantTimeCompare([]byte(password), []byte(client.Password)) == 1 {
				authenticated = true
				break
			}
		}

		if !authenticated {
			ctx.Header("WWW-Authenticate", "Basic realm=Authorization Required")
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		ctx.Next()
	}
}
