package out

import (
	"context"
	"time"

	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
)

type CachePort interface {
	// Кэширование слотов
	GetSlots(ctx context.Context, scheduleID string, startDate time.Time, endDate time.Time, slotType domain.AppointmentType) ([]domain.Slot, time.Time, bool)
	GetSlotByAppointment(ctx context.Context, scheduleID string, appointment domain.Appointment) (domain.Slot, bool)
	StoreSlots(ctx context.Context, scheduleID string, planningEndTime time.Time, slots []domain.Slot)
	UpdateSlot(ctx context.Context, scheduleID string, slot domain.Slot)
	InvalidateSlotsCache(ctx context.Context, scheduleID string)
	InvalidateAllSlotsCache(ctx context.Context)

	// Кэширование производственного календаря
	GetScheduleRuleGlobal(ctx context.Context) (*domain.ScheduleRuleGlobal, bool)
	StoreScheduleRuleGlobal(ctx context.Context, scheduleRuleGlobal domain.ScheduleRuleGlobal)
	InvalidateScheduleRuleGlobalCache(ctx context.Context)

	// Кэширование расписаний
	GetScheduleRule(ctx context.Context, scheduleID string) (*domain.ScheduleRule, bool)
	StoreScheduleRule(ctx context.Context, scheduleRule domain.ScheduleRule)
	InvalidateScheduleRuleCache(ctx context.Context, scheduleID string)
	InvalidateAllScheduleRuleCache(ctx context.Context)
}
