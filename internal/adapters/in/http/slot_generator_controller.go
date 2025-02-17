package http

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/config"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/in"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/utils"
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
	with50PercentRuleParam := ctx.Query("with_50_percent_rule") == "true"
	fullDayParam := ctx.Query("full_day") == "true"
	startDateParam := ctx.Query("start_date")
	onlyFreeParam := ctx.Query("only_free") == "true"

	var generateSlotsCount int
	var err error
	generateSlotsCountParam := ctx.Query("slots_count")
	if generateSlotsCountParam == "" {
		// Если параметр пустой, устанавливаем количество как неограниченное
		generateSlotsCount = -1 // Используем -1 для обозначения неограниченного количества
	} else {
		generateSlotsCount, err = strconv.Atoi(generateSlotsCountParam)
		if err != nil {
			// Обработка ошибки, если преобразование не удалось
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid slots_count query parameter, must be integer"})
			return
		}
	}

	var startDate time.Time
	if startDateParam != "" {
		startDate, err = utils.ParseDate(startDateParam)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date query parameter, must be RFC3339 format"})
			return
		}
	}

	generateSlotsRequest := in.GenerateSlotsRequest{
		ScheduleID:        scheduleID,
		Channels:          channelsParam,
		SlotsCount:        generateSlotsCount,
		With50PercentRule: with50PercentRuleParam,
		FullDay:           fullDayParam,
		StartDate:         startDate,
		OnlyFree:          onlyFreeParam,
	}

	slots, debug, err := c.useCase.GenerateSlots(ctx.Request.Context(), generateSlotsRequest)
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
