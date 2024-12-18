package in

import (
	"context"

	"github.com/google/uuid"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
)

type SlotGeneratorUseCase interface {
	// Генерация слотов для одного расписания
	GenerateSlots(ctx context.Context, scheduleID uuid.UUID, channelParam string) ([]domain.Slot, []domain.DebugInfo, error)

	// Генерация слотов для нескольких расписаний
	GenerateBatchSlots(ctx context.Context, scheduleIDs []uuid.UUID, channelParam string) (map[uuid.UUID][]domain.Slot, error)
}
