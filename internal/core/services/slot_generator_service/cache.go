package slot_generator_service

import (
	"context"
	"errors"
	"time"

	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
)

func (s *SlotGeneratorService) GetSlotsCache(ctx context.Context, scheduleID string, startDate time.Time, endDate time.Time, slotType domain.AppointmentType) ([]domain.Slot, time.Time, bool) {
	if s.cachePort == nil {
		return []domain.Slot{}, time.Time{}, false
	}

	return s.cachePort.GetSlots(ctx, scheduleID, startDate, endDate, slotType)
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
	if s.cachePort != nil {
		s.cachePort.InvalidateSlotsCache(ctx, scheduleID)
	}

	return nil
}

func (s *SlotGeneratorService) InvalidateAllSlotsCache(ctx context.Context) error {
	if s.cachePort != nil {
		s.cachePort.InvalidateAllSlotsCache(ctx)
	}

	return nil
}

func (s *SlotGeneratorService) GetScheduleRuleCache(ctx context.Context, scheduleID string) (*domain.ScheduleRule, bool) {
	if s.cachePort == nil {
		return nil, false
	}

	scheduleRule, exists := s.cachePort.GetScheduleRule(ctx, scheduleID)
	return scheduleRule, exists
}

func (s *SlotGeneratorService) StoreScheduleRuleCache(ctx context.Context, scheduleRule domain.ScheduleRule) (*domain.ScheduleRule, error) {
	if s.cachePort != nil {
		s.cachePort.StoreScheduleRule(ctx, scheduleRule)
	}

	return &scheduleRule, nil
}

func (s *SlotGeneratorService) InvalidateScheduleRuleCache(ctx context.Context, scheduleID string) error {
	if s.cachePort != nil {
		s.cachePort.InvalidateScheduleRuleCache(ctx, scheduleID)
	}

	return nil
}

func (s *SlotGeneratorService) InvalidateAllScheduleRuleCache(ctx context.Context) error {
	if s.cachePort != nil {
		s.cachePort.InvalidateAllScheduleRuleCache(ctx)
	}

	return nil
}

func (s *SlotGeneratorService) GetScheduleRuleGlobalCache(ctx context.Context) (*domain.ScheduleRuleGlobal, bool) {
	if s.cachePort == nil {
		return nil, false
	}

	scheduleRuleGlobal, exists := s.cachePort.GetScheduleRuleGlobal(ctx)
	if exists {
		return scheduleRuleGlobal, true
	}

	return nil, false
}

func (s *SlotGeneratorService) StoreScheduleRuleGlobalCache(ctx context.Context, scheduleRuleGlobal domain.ScheduleRuleGlobal) (*domain.ScheduleRuleGlobal, error) {
	if s.cachePort != nil {
		s.cachePort.StoreScheduleRuleGlobal(ctx, scheduleRuleGlobal)
	}

	return &scheduleRuleGlobal, nil
}

func (s *SlotGeneratorService) InvalidateScheduleRuleGlobalCache(ctx context.Context) error {
	return nil
}
