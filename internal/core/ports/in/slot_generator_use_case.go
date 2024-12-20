package in

import (
	"context"

	"github.com/google/uuid"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
)

type SlotGeneratorUseCase interface {
	// Генерация слотов для одного расписания
	GenerateSlots(ctx context.Context, scheduleID uuid.UUID, channelParam string) (map[domain.AppointmentType][]domain.Slot, []domain.DebugInfo, error)

	// Кэширование слотов
	ProccessAppointmentCacheSlot(ctx context.Context, scheduleID uuid.UUID, appointment domain.Appointment) error
	InvalidateSlotsCache(ctx context.Context, scheduleID uuid.UUID) error

	// Кэширование производственного календаря
	InvalidateScheduleRuleGlobalCache(ctx context.Context) error

	// Кэширование расписаний
	InvalidateScheduleRuleCache(ctx context.Context, scheduleID uuid.UUID) error
	InvalidateAllScheduleRuleCache(ctx context.Context) error
}
