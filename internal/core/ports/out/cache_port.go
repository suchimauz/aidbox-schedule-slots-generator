package out

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
)

type CachePort interface {
	// Кэширование слотов
	GetSlots(ctx context.Context, scheduleID uuid.UUID, startDate time.Time, endDate time.Time, slotType domain.AppointmentType) ([]domain.Slot, time.Time, bool)
	StoreSlots(ctx context.Context, scheduleID uuid.UUID, planningEndTime time.Time, slots []domain.Slot)
	UpdateSlot(ctx context.Context, scheduleID uuid.UUID, slot domain.Slot)
	InvalidateSlotsCache(ctx context.Context, scheduleID uuid.UUID)

	// Кэширование производственного календаря
	GetScheduleRuleGlobal(ctx context.Context) (*domain.ScheduleRuleGlobal, bool)
	StoreScheduleRuleGlobal(ctx context.Context, scheduleRuleGlobal domain.ScheduleRuleGlobal)
	InvalidateScheduleRuleGlobalCache(ctx context.Context)

	// Кэширование расписаний
	GetScheduleRule(ctx context.Context, scheduleID uuid.UUID) (*domain.ScheduleRule, bool)
	StoreScheduleRule(ctx context.Context, scheduleRule domain.ScheduleRule)
	InvalidateScheduleRuleCache(ctx context.Context, scheduleID uuid.UUID)
	InvalidateAllScheduleRuleCache(ctx context.Context)
}
