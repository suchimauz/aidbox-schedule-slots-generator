package slot_generator_service

import (
	"context"
	"errors"
	"time"

	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
)

// Кэширование слотов

func (s *SlotGeneratorService) GetSlotsCache(ctx context.Context, scheduleID string, startTime time.Time, endTime time.Time, slotType domain.AppointmentType) ([]domain.Slot, bool) {
	return s.cachePort.GetSlots(ctx, scheduleID, startTime, endTime, slotType)
}

func (s *SlotGeneratorService) StoreAppointmentCacheSlot(ctx context.Context, scheduleID string, appointment domain.Appointment) error {
	slot, exists := s.cachePort.GetSlotByAppointment(ctx, scheduleID, appointment)
	s.logger.Info("slot.store", out.LogFields{
		"slot":   slot,
		"exists": exists,
	})
	if !exists {
		return errors.New("slot not found")
	}

	uniqueAppointmentIds := make(map[string]struct{})
	for _, appointmentId := range slot.AppointmentIDS {
		uniqueAppointmentIds[appointmentId] = struct{}{}
	}
	uniqueAppointmentIds[appointment.ID] = struct{}{}

	appointmentIds := make([]string, 0, len(uniqueAppointmentIds))
	for id := range uniqueAppointmentIds {
		appointmentIds = append(appointmentIds, id)
	}

	slot.AppointmentIDS = appointmentIds

	s.cachePort.UpdateSlot(ctx, scheduleID, slot)

	return nil
}

func (s *SlotGeneratorService) InvalidateAppointmentCacheSlot(ctx context.Context, scheduleID string, appointment domain.Appointment) error {
	slot, exists := s.cachePort.GetSlotByAppointment(ctx, scheduleID, appointment)
	if !exists {
		return errors.New("slot not found")
	}

	// Удаляем из списка слотов appointment.ID
	uniqueAppointmentIds := make(map[string]struct{})
	for _, appointmentId := range slot.AppointmentIDS {
		if appointmentId != appointment.ID {
			uniqueAppointmentIds[appointmentId] = struct{}{}
		}
	}

	appointmentIds := make([]string, 0, len(uniqueAppointmentIds))
	for id := range uniqueAppointmentIds {
		appointmentIds = append(appointmentIds, id)
	}

	slot.AppointmentIDS = appointmentIds

	s.cachePort.UpdateSlot(ctx, scheduleID, slot)

	return nil
}

func (s *SlotGeneratorService) InvalidateSlotsCache(ctx context.Context, scheduleID string) error {
	s.cachePort.InvalidateSlotsCache(ctx, scheduleID)

	return nil
}

func (s *SlotGeneratorService) InvalidateAllSlotsCache(ctx context.Context) error {
	s.cachePort.InvalidateAllSlotsCache(ctx)

	return nil
}

func (s *SlotGeneratorService) GetScheduleRuleCache(ctx context.Context, scheduleID string) (*domain.ScheduleRule, bool) {
	scheduleRule, exists := s.cachePort.GetScheduleRule(ctx, scheduleID)
	return scheduleRule, exists
}

func (s *SlotGeneratorService) StoreScheduleRuleCache(ctx context.Context, scheduleRule domain.ScheduleRule) (*domain.ScheduleRule, error) {
	s.cachePort.StoreScheduleRule(ctx, scheduleRule)

	return &scheduleRule, nil
}

func (s *SlotGeneratorService) InvalidateScheduleRuleCache(ctx context.Context, scheduleID string) error {
	s.cachePort.InvalidateScheduleRuleCache(ctx, scheduleID)

	return nil
}

func (s *SlotGeneratorService) InvalidateAllScheduleRuleCache(ctx context.Context) error {
	s.cachePort.InvalidateAllScheduleRuleCache(ctx)

	return nil
}

func (s *SlotGeneratorService) GetScheduleRuleGlobalCache(ctx context.Context) (*domain.ScheduleRuleGlobal, bool) {
	scheduleRuleGlobal, exists := s.cachePort.GetScheduleRuleGlobal(ctx)
	if exists {
		return scheduleRuleGlobal, true
	}

	return nil, false
}

func (s *SlotGeneratorService) StoreScheduleRuleGlobalCache(ctx context.Context, scheduleRuleGlobal domain.ScheduleRuleGlobal) (*domain.ScheduleRuleGlobal, error) {
	s.cachePort.StoreScheduleRuleGlobal(ctx, scheduleRuleGlobal)

	return &scheduleRuleGlobal, nil
}

func (s *SlotGeneratorService) InvalidateScheduleRuleGlobalCache(ctx context.Context) error {
	return nil
}

func (s *SlotGeneratorService) GetHealthcareServiceCache(ctx context.Context, healthcareServiceID string) (*domain.HealthcareService, bool) {
	return nil, false

	healthcareService, exists := s.cachePort.GetHealthcareService(ctx, healthcareServiceID)
	return healthcareService, exists
}

func (s *SlotGeneratorService) StoreHealthcareServiceCache(ctx context.Context, healthcareService domain.HealthcareService) (*domain.HealthcareService, error) {
	s.cachePort.StoreHealthcareService(ctx, healthcareService)

	return &healthcareService, nil
}

func (s *SlotGeneratorService) InvalidateHealthcareServiceCache(ctx context.Context, healthcareServiceID string) error {
	s.cachePort.InvalidateHealthcareServiceCache(ctx, healthcareServiceID)

	return nil
}

func (s *SlotGeneratorService) InvalidateAllHealthcareServiceCache(ctx context.Context) error {
	s.cachePort.InvalidateAllHealthcareServiceCache(ctx)

	return nil
}
