package in

import (
	"context"

	"github.com/google/uuid"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
)

type SlotGeneratorUseCase interface {
	// Генерация слотов для одного расписания
	GenerateSlots(ctx context.Context, scheduleID uuid.UUID) ([]domain.Slot, error)

	// Генерация слотов для нескольких расписаний
	GenerateBatchSlots(ctx context.Context, scheduleIDs []uuid.UUID) (map[uuid.UUID][]domain.Slot, error)

	// Обновление статуса слота при изменении записи на прием
	UpdateSlotStatus(ctx context.Context, appointmentID uuid.UUID) error
}
