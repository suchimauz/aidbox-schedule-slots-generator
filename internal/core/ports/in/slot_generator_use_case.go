package in

import (
	"context"
	"time"

	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
)

type GenerateSlotsRequest struct {
	ScheduleID        string
	Channels          string
	SlotsCount        int
	With50PercentRule bool
	FullDay           bool
	StartDate         time.Time
}

type SlotGeneratorUseCase interface {
	// Генерация слотов для одного расписания
	GenerateSlots(ctx context.Context, request GenerateSlotsRequest) (map[domain.AppointmentType][]domain.Slot, []domain.DebugInfo, error)

	// Кэширование слотов
	StoreAppointmentCacheSlot(ctx context.Context, scheduleID string, appointment domain.Appointment) error
	InvalidateAppointmentCacheSlot(ctx context.Context, scheduleID string, appointment domain.Appointment) error
	InvalidateSlotsCache(ctx context.Context, scheduleID string) error
	InvalidateAllSlotsCache(ctx context.Context) error

	// Кэширование производственного календаря
	InvalidateScheduleRuleGlobalCache(ctx context.Context) error

	// Кэширование расписаний
	InvalidateScheduleRuleCache(ctx context.Context, scheduleID string) error
	InvalidateAllScheduleRuleCache(ctx context.Context) error

	// Кэширование услуг
	InvalidateHealthcareServiceCache(ctx context.Context, healthcareServiceID string) error
	InvalidateAllHealthcareServiceCache(ctx context.Context) error
}
