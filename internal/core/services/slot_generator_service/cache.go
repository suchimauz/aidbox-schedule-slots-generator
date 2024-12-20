package slot_generator_service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
)

func (s *SlotGeneratorService) GetSlotsCache(ctx context.Context, scheduleID uuid.UUID, startDate time.Time, endDate time.Time, slotType domain.AppointmentType) ([]domain.Slot, time.Time, bool) {
	if s.cachePort == nil {
		return []domain.Slot{}, time.Time{}, false
	}

	return s.cachePort.GetSlots(ctx, scheduleID, startDate, endDate, slotType)
}

func (s *SlotGeneratorService) ProccessAppointmentCacheSlot(ctx context.Context, scheduleID uuid.UUID, appointment domain.Appointment) error {
	return nil
}

func (s *SlotGeneratorService) InvalidateSlotsCache(ctx context.Context, scheduleID uuid.UUID) error {
	if s.cachePort != nil {
		s.cachePort.InvalidateSlotsCache(ctx, scheduleID)
	}

	return nil
}

func (s *SlotGeneratorService) GetScheduleRuleCache(ctx context.Context, scheduleID uuid.UUID) (*domain.ScheduleRule, bool) {
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

func (s *SlotGeneratorService) InvalidateScheduleRuleCache(ctx context.Context, scheduleID uuid.UUID) error {
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
