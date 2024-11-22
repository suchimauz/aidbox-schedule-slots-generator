package out

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
)

type CachePort interface {
	GetSlots(ctx context.Context, scheduleID uuid.UUID, startDate, endDate time.Time) ([]domain.Slot, bool)
	StoreSlots(ctx context.Context, scheduleID uuid.UUID, slots []domain.Slot)
	UpdateSlot(ctx context.Context, scheduleID uuid.UUID, slot domain.Slot)
	InvalidateCache(ctx context.Context, scheduleID uuid.UUID)
}
