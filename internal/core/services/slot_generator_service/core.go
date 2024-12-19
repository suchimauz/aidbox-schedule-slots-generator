package slot_generator_service

import (
	"context"
	"fmt"
	"sync"
	"time"

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

func (s *SlotGeneratorService) prepareResponseSlots(debugInfo *SlotGeneratorServiceDebug, slots []domain.Slot) map[domain.AppointmentType][]domain.Slot {
	routineSlots := make([]domain.Slot, 0)
	walkinSlots := make([]domain.Slot, 0)

	// В ответе разделяем слоты по их типу
	// Например:
	// ROUTINE: [slot1, slot2, slot3]
	// WALKIN: [slot4, slot5, slot6]
	for _, slot := range slots {
		if slot.SlotType == domain.AppointmentTypeRoutine {
			routineSlots = append(routineSlots, slot)
		}
		if slot.SlotType == domain.AppointmentTypeWalkin {
			walkinSlots = append(walkinSlots, slot)
		}
	}

	// Сортируем слоты по времени
	sort_routine_slots_debug := domain.DebugInfo{
		Event: "slots.generate.routine_slots.sort",
	}
	sort_routine_slots_debug.Start()
	routineSlots = SlotSlice(routineSlots).quickSort()
	walkinSlots = SlotSlice(walkinSlots).quickSort()
	sort_routine_slots_debug.Elapse()
	debugInfo.AddDebugInfo(sort_routine_slots_debug)

	return map[domain.AppointmentType][]domain.Slot{
		domain.AppointmentTypeRoutine: routineSlots,
		domain.AppointmentTypeWalkin:  walkinSlots,
	}
}

func (s *SlotGeneratorService) GenerateSlots(ctx context.Context, scheduleID uuid.UUID, channelsParam string) (map[domain.AppointmentType][]domain.Slot, []domain.DebugInfo, error) {
	debugInfo := SlotGeneratorServiceDebug{
		data: make([]domain.DebugInfo, 0),
	}
	s.logger.Info("slots.generate.started", out.LogFields{
		"scheduleId": scheduleID,
	})

	get_schedule_rule_debug := domain.DebugInfo{
		Event: "slots.generate.schedule.fetch",
	}
	get_schedule_rule_debug.Start()

	schedule, err := s.aidboxPort.GetScheduleRule(ctx, scheduleID)
	if err != nil {
		s.logger.Error("slots.generate.schedule.fetch_failed", out.LogFields{
			"scheduleId": scheduleID,
			"error":      err.Error(),
		})
		return nil, nil, fmt.Errorf("slots.generate.schedule.fetch_failed: %w", err)
	}
	get_schedule_rule_debug.Elapse()
	debugInfo.AddDebugInfo(get_schedule_rule_debug)

	// Проверяем кэш только если он включен
	if s.cachePort != nil && s.cfg.Cache.Enabled {
		if slots, exists := s.cachePort.GetSlots(ctx, scheduleID, schedule.PlanningHorizon.Start.Date, schedule.PlanningHorizon.End.Date); exists {
			s.logger.Debug("slots.generate.cache.hit", out.LogFields{
				"scheduleId": scheduleID,
				"slotsCount": len(slots),
			})
			return s.prepareResponseSlots(&debugInfo, slots), debugInfo.data, nil
		}
	}

	s.logger.Debug("slots.generate.cache.miss", out.LogFields{
		"scheduleId": scheduleID,
	})

	// Генерируем слоты с интервалами
	slots, err := s.generateSlotsForSchedule(ctx, &debugInfo, schedule, channelsParam)
	if err != nil {
		return nil, nil, err
	}

	// Сохраняем в кэш только если он включен
	if s.cachePort != nil && s.cfg.Cache.Enabled {
		s.cachePort.StoreSlots(ctx, scheduleID, slots)
	}

	return s.prepareResponseSlots(&debugInfo, slots), debugInfo.data, nil
}

func (s *SlotGeneratorService) generateSlotsForSchedule(ctx context.Context, debugInfo *SlotGeneratorServiceDebug, schedule *domain.ScheduleRule, channels string) ([]domain.Slot, error) {
	var endTime time.Time
	var startTime time.Time
	var scheduleRuleGlobal *domain.ScheduleRuleGlobal

	slots := make([]domain.Slot, 0)

	get_schedule_rule_global_debug := domain.DebugInfo{
		Event: "slots.generate.schedule_rule_global.fetch",
	}
	get_schedule_rule_global_debug.Start()

	scheduleRuleGlobal, err := s.getScheduleRuleGlobal(ctx)
	if err != nil {
		s.logger.Error("slots.generate.schedule_rule_global.fetch_failed", out.LogFields{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("slots.generate.schedule_rule_global.fetch_failed: %w", err)
	}
	get_schedule_rule_global_debug.Elapse()
	debugInfo.AddDebugInfo(get_schedule_rule_global_debug)

	// Начальная дата, равная началу планируемого горизонта или текущей датой, если горизонт не задан
	// Или горизонт задан в прошлом
	if schedule.PlanningHorizon.Start.Date.IsZero() || schedule.PlanningHorizon.Start.Date.Before(time.Now()) {
		startTime = time.Now()
	} else {
		startTime = schedule.PlanningHorizon.Start.Date
	}

	// Вычисляем длительность слота
	slotDuration := time.Duration(schedule.MinutesDuration) * time.Minute

	// Вычисляем длительность активного периода, пока подразумевается что там всегда недели
	planningActiveDuration := time.Duration(schedule.PlanningActive.Quantity) * 7 * 24 * time.Hour
	// Вычисляем конец активного периода
	nowPlusPlanningActiveDuration := time.Now().Add(planningActiveDuration)

	// Проверяем есть ли вообще планируемый горизонт
	// Или планируемый активный период заканчивается раньше, чем планируемый горизонт, то используем планируемый активный период
	if schedule.PlanningHorizon.End.Date.IsZero() || nowPlusPlanningActiveDuration.Before(schedule.PlanningHorizon.End.Date) {
		endTime = nowPlusPlanningActiveDuration
		// Иначе используем планируемый горизонт
	} else {
		endTime = schedule.PlanningHorizon.End.Date
	}

	get_appointments_debug := domain.DebugInfo{
		Event: "slots.generate.appointments.fetch",
	}
	get_appointments_debug.Start()

	appointments, err := s.aidboxPort.GetScheduleRuleAppointments(ctx, schedule.ID, startTime, endTime)
	if err != nil {
		s.logger.Error("slots.generate.appointments.fetch_failed", out.LogFields{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("slots.generate.appointments.fetch_failed: %w", err)
	}

	get_appointments_debug.Elapse()
	debugInfo.AddDebugInfo(get_appointments_debug)

	// Используем мьютекс для безопасного доступа к слайсу slots
	// И группу ожидания для ожидания завершения всех горутин
	var mu sync.Mutex
	var wg sync.WaitGroup

	generate_routine_slots_debug := domain.DebugInfo{
		Event: "slots.generate.routine_slots.generate",
	}
	generate_walkin_slots_debug := domain.DebugInfo{
		Event: "slots.generate.walkin_slots.generate",
	}

	generate_routine_slots_debug.Start()
	// Генерируем слоты с учетом availableTime
	s.generateRoutineSlots(schedule, scheduleRuleGlobal, appointments, startTime, endTime, channels, slotDuration, &slots, &mu, &wg)
	// Ждем завершения всех горутин
	wg.Wait()
	generate_routine_slots_debug.Elapse()

	generate_walkin_slots_debug.Start()
	// Генерируем слоты без времени
	s.generateWalkinSlots(schedule, scheduleRuleGlobal, appointments, &slots, &mu, &wg)

	// Ждем завершения всех горутин
	wg.Wait()
	generate_walkin_slots_debug.Elapse()

	debugInfo.AddDebugInfo(generate_routine_slots_debug)
	debugInfo.AddDebugInfo(generate_walkin_slots_debug)

	return slots, nil
}

func (s *SlotGeneratorService) getScheduleRuleGlobal(ctx context.Context) (*domain.ScheduleRuleGlobal, error) {
	var scheduleRuleGlobal *domain.ScheduleRuleGlobal

	// Проверяем, инициализирован ли cachePort
	if s.cachePort != nil {
		scheduleRuleGlobal, exists := s.cachePort.GetScheduleRuleGlobal(ctx)
		if exists {
			return scheduleRuleGlobal, nil
		}
	}

	s.logger.Debug("scheduleruleglobal.cache.miss", out.LogFields{})

	// Запрашиваем из AidboxAdapter
	scheduleRuleGlobal, err := s.aidboxPort.GetScheduleRuleGlobal(ctx)
	if err != nil {
		return nil, err
	}

	if s.cachePort != nil {
		// Сохраняем в кэш
		s.cachePort.StoreScheduleRuleGlobal(ctx, *scheduleRuleGlobal)
	}

	return scheduleRuleGlobal, nil
}
