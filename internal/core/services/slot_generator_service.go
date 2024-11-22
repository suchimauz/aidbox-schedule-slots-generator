package services

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/config"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
)

type SlotGeneratorService struct {
	aidboxPort out.AidboxPort
	cachePort  out.CachePort
	logger     out.LoggerPort
	cfg        *config.Config
}

func NewSlotGeneratorService(
	aidboxPort out.AidboxPort,
	cachePort out.CachePort,
	logger out.LoggerPort,
) *SlotGeneratorService {
	return &SlotGeneratorService{
		aidboxPort: aidboxPort,
		cachePort:  cachePort,
		logger:     logger.WithModule("SlotGeneratorService"),
	}
}

func (s *SlotGeneratorService) GenerateSlots(ctx context.Context, scheduleID uuid.UUID) ([]domain.Slot, error) {
	s.logger.Info("slots.generate.started", out.LogFields{
		"scheduleId": scheduleID,
	})

	schedule, err := s.aidboxPort.GetScheduleRule(ctx, scheduleID)
	if err != nil {
		s.logger.Error("slots.generate.schedule.fetch_failed", out.LogFields{
			"scheduleId": scheduleID,
			"error":      err.Error(),
		})
		return nil, err
	}

	// Проверяем кэш только если он включен
	if s.cachePort != nil && s.cfg.Cache.Enabled {
		if slots, exists := s.cachePort.GetSlots(ctx, scheduleID, schedule.StartDate, schedule.EndDate); exists {
			s.logger.Debug("slots.generate.cache.hit", out.LogFields{
				"scheduleId": scheduleID,
				"slotsCount": len(slots),
			})
			return slots, nil
		}
	}

	s.logger.Debug("slots.generate.cache.miss", out.LogFields{
		"scheduleId": scheduleID,
	})

	// Генерируем слоты
	slots := s.generateSlotsForSchedule(schedule)

	// Получаем и применяем статусы занятости
	appointments, err := s.aidboxPort.GetScheduleRuleAppointments(ctx, schedule.ID, schedule.StartDate, schedule.EndDate)
	if err != nil {
		return nil, err
	}

	s.applyAppointmentsToSlots(slots, appointments)

	// Сохраняем в кэш только если он включен
	if s.cachePort != nil && s.cfg.Cache.Enabled {
		s.cachePort.StoreSlots(ctx, scheduleID, slots)
	}

	return slots, nil
}

func (s *SlotGeneratorService) GenerateBatchSlots(ctx context.Context, scheduleIDs []uuid.UUID) (map[uuid.UUID][]domain.Slot, error) {
	result := make(map[uuid.UUID][]domain.Slot)
	var mu sync.Mutex
	var wg sync.WaitGroup
	errCh := make(chan error, len(scheduleIDs))

	for _, id := range scheduleIDs {
		wg.Add(1)
		go func(scheduleID uuid.UUID) {
			defer wg.Done()

			slots, err := s.GenerateSlots(ctx, scheduleID)
			if err != nil {
				errCh <- err
				return
			}

			mu.Lock()
			result[scheduleID] = slots
			mu.Unlock()
		}(id)
	}

	wg.Wait()
	close(errCh)

	// Проверяем ошибки
	for err := range errCh {
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (s *SlotGeneratorService) generateSlotsForSchedule(schedule *domain.ScheduleRule) []domain.Slot {
	var slots []domain.Slot
	currentTime := schedule.StartDate

	for currentTime.Before(schedule.EndDate) {
		slotEndTime := currentTime.Add(schedule.SlotDuration)

		slot := domain.Slot{
			ScheduleRule: *schedule,
			StartTime:    currentTime,
			EndTime:      slotEndTime,
			Status:       domain.SlotStatusFree,
		}

		slots = append(slots, slot)
		currentTime = slotEndTime
	}

	return slots
}

func (s *SlotGeneratorService) applyAppointmentsToSlots(slots []domain.Slot, appointments []domain.Appointment) {
	for _, appointment := range appointments {
		if appointment.Status != domain.AppointmentStatusActive {
			continue
		}

		for i, slot := range slots {
			if slot.StartTime.Equal(appointment.StartTime) {
				slots[i].Status = domain.SlotStatusOccupied
				break
			}
		}
	}
}

func (s *SlotGeneratorService) UpdateSlotStatus(ctx context.Context, appointmentID uuid.UUID) error {
	return nil
}
