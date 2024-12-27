package http

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/config"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/in"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
)

type SlotGeneratorController struct {
	useCase in.SlotGeneratorUseCase
	cfg     *config.Config
	logger  out.LoggerPort
}

type SlotGeneratorResponse struct {
	ScheduleID string                                   `json:"scheduleId"`
	Slots      map[domain.AppointmentType][]domain.Slot `json:"slots"`
	Debug      []domain.DebugInfo                       `json:"debug"`
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
		api.GET("/slots/generate/:scheduleId", c.generateSlots)
	}
}

type GenerateBatchSlotsRequest struct {
	ScheduleIDs []string `json:"scheduleIds" binding:"required,min=1"`
}

func (c *SlotGeneratorController) generateSlots(ctx *gin.Context) {
	scheduleID := ctx.Param("scheduleId")
	channelsParam := ctx.Query("channel")

	slots, debug, err := c.useCase.GenerateSlots(ctx.Request.Context(), scheduleID, channelsParam)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	slotsCount := 0
	for _, slots := range slots {
		slotsCount += len(slots)
	}

	c.logger.Debug("slots.generated", out.LogFields{
		"scheduleId": scheduleID,
		"slotsCount": slotsCount,
		"debug":      debug,
	})

	ctx.JSON(http.StatusOK, SlotGeneratorResponse{
		ScheduleID: scheduleID,
		Slots:      slots,
		Debug:      debug,
	})
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
