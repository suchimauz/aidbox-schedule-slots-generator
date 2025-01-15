package slot_generator_service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/config"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/utils"
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

func (s *SlotGeneratorService) prepareResponseSlots(debugInfo *SlotGeneratorServiceDebug, slots []domain.Slot, channelsParam string, generateSlotsCount int) map[domain.AppointmentType][]domain.Slot {
	routineSlots := make([]domain.Slot, 0)
	walkinSlots := make([]domain.Slot, 0)

	// В ответе разделяем слоты по их типу
	// Например:
	// ROUTINE: [slot1, slot2, slot3]
	// WALKIN: [slot4, slot5, slot6]
	for _, slot := range slots {
		if slot.SlotType == domain.AppointmentTypeRoutine {
			// Проверяем, доступен ли слот по каналам
			if isChannelAvailable(slot.Channel, channelsParam) {
				routineSlots = append(routineSlots, slot)
			}
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

	// Если количество слотов ограничено, то отрезаем их
	// И возвращаем только нужные слоты
	// Walkin слоты не возвращаем, т.к. они не нужны при данном варианте
	if generateSlotsCount != -1 {
		routineSlots = routineSlots[:generateSlotsCount]

		return map[domain.AppointmentType][]domain.Slot{
			domain.AppointmentTypeRoutine: routineSlots[:generateSlotsCount],
		}
	}

	return map[domain.AppointmentType][]domain.Slot{
		domain.AppointmentTypeRoutine: routineSlots,
		domain.AppointmentTypeWalkin:  walkinSlots,
	}
}

func (s *SlotGeneratorService) GenerateSlots(ctx context.Context, scheduleID string, channelsParam string, generateSlotsCount int, with50PercentRule bool) (map[domain.AppointmentType][]domain.Slot, []domain.DebugInfo, error) {
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

	schedule, err := s.getScheduleRule(ctx, scheduleID)
	if err != nil {
		return nil, nil, err
	}
	get_schedule_rule_debug.Elapse()
	debugInfo.AddDebugInfo(get_schedule_rule_debug)

	// Проверяем кэш только если он включен
	// if s.cachePort != nil && s.cfg.Cache.Enabled {
	// 	if slots, exists := s.cachePort.GetSlots(ctx, scheduleID, schedule.PlanningHorizon.Start.Date, schedule.PlanningHorizon.End.Date); exists {
	// 		s.logger.Debug("slots.generate.cache.hit", out.LogFields{
	// 			"scheduleId": scheduleID,
	// 			"slotsCount": len(slots),
	// 		})
	// 		return s.prepareResponseSlots(&debugInfo, slots), debugInfo.data, nil
	// 	}
	// }

	s.logger.Debug("slots.generate.cache.miss", out.LogFields{
		"scheduleId": scheduleID,
	})

	// Генерируем слоты с интервалами
	slots, err := s.generateSlotsForSchedule(ctx, &debugInfo, schedule, with50PercentRule)
	if err != nil {
		return nil, nil, err
	}

	return s.prepareResponseSlots(&debugInfo, slots, channelsParam, generateSlotsCount), debugInfo.data, nil
}

func (s *SlotGeneratorService) generateSlotsForSchedule(ctx context.Context, debugInfo *SlotGeneratorServiceDebug, schedule *domain.ScheduleRule, with50PercentRule bool) ([]domain.Slot, error) {
	var startTime, endTime, planningEndTime time.Time
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
		planningEndTime = nowPlusPlanningActiveDuration
		// Иначе используем планируемый горизонт
	} else {
		planningEndTime = schedule.PlanningHorizon.End.Date
	}
	endTime = utils.StartNextDay(planningEndTime)

	s.logger.Info("slots.generate.planning_end_time", out.LogFields{
		"planningEndTime": planningEndTime,
		"endTime":         endTime,
	})

	cachedRoutineSlots, cachedPlanningEndTime, exists := s.GetSlotsCache(ctx, schedule.ID, startTime, endTime, domain.AppointmentTypeRoutine)
	cachedWalkinSlots, cachedPlanningEndTime, exists := s.GetSlotsCache(ctx, schedule.ID, startTime, endTime, domain.AppointmentTypeWalkin)

	// Используем мьютекс для безопасного доступа к слайсу slots
	// И группу ожидания для ожидания завершения всех горутин
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Если кэш есть, то начинаем с последнего слота в кэше
	if exists && (cachedPlanningEndTime.After(endTime) || cachedPlanningEndTime.Equal(endTime)) {
		s.logger.Debug("slots.generate.cache.hit", out.LogFields{
			"scheduleId": schedule.ID,
			"slotsCount": len(cachedRoutineSlots),
		})
		get_from_cache_debug := domain.DebugInfo{
			Event: "slots.generate.cache.get",
		}
		get_from_cache_debug.Start()

		slots = cachedRoutineSlots

		s.logger.Info("slots.generate.cache.get", out.LogFields{
			"routineLength": len(cachedRoutineSlots),
			"walkinLength":  len(cachedWalkinSlots),
		})

		slots = append(slots, cachedWalkinSlots...)

		get_from_cache_debug.Elapse()
		debugInfo.AddDebugInfo(get_from_cache_debug)

		apply_50_percent_rule_debug := domain.DebugInfo{
			Event: "slots.generate.50_percent_rule.apply",
		}
		apply_50_percent_rule_debug.Start()
		// Применяем правило 50%
		s.apply50PercentRule(startTime, endTime, with50PercentRule, &slots, &mu, &wg)
		// Ждем завершения всех горутин
		wg.Wait()
		apply_50_percent_rule_debug.Elapse()
		debugInfo.AddDebugInfo(apply_50_percent_rule_debug)

		return slots, nil
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

	generate_routine_slots_debug := domain.DebugInfo{
		Event: "slots.generate.routine_slots.generate",
	}
	generate_walkin_slots_debug := domain.DebugInfo{
		Event: "slots.generate.walkin_slots.generate",
	}

	generate_routine_slots_debug.Start()
	// Генерируем слоты с учетом availableTime
	s.generateRoutineSlots(schedule, scheduleRuleGlobal, appointments, startTime, endTime, slotDuration, &slots, &mu, &wg)
	// Ждем завершения всех горутин
	wg.Wait()

	routineSlotsLen := len(slots)
	s.logger.Info("slots.generate.routine_slots.generated", out.LogFields{
		"length": routineSlotsLen,
	})
	generate_routine_slots_debug.Elapse()

	generate_walkin_slots_debug.Start()
	// Генерируем слоты без времени
	s.generateWalkinSlots(schedule, scheduleRuleGlobal, appointments, &slots, &mu, &wg)

	// Ждем завершения всех горутин
	wg.Wait()
	generate_walkin_slots_debug.Elapse()

	s.logger.Info("slots.generate.walkin_slots.generated", out.LogFields{
		"length": len(slots) - routineSlotsLen,
	})

	// Сохраняем в кэш слоты
	s.cachePort.StoreSlots(ctx, schedule.ID, endTime, slots)

	debugInfo.AddDebugInfo(generate_routine_slots_debug)
	debugInfo.AddDebugInfo(generate_walkin_slots_debug)

	apply_50_percent_rule_debug := domain.DebugInfo{
		Event: "slots.generate.50_percent_rule.apply",
	}
	apply_50_percent_rule_debug.Start()
	// Применяем правило 50%
	s.apply50PercentRule(startTime, endTime, with50PercentRule, &slots, &mu, &wg)
	// Ждем завершения всех горутин
	wg.Wait()
	apply_50_percent_rule_debug.Elapse()
	debugInfo.AddDebugInfo(apply_50_percent_rule_debug)

	return slots, nil
}

func (s *SlotGeneratorService) getScheduleRuleGlobal(ctx context.Context) (*domain.ScheduleRuleGlobal, error) {
	var scheduleRuleGlobal *domain.ScheduleRuleGlobal

	scheduleRuleGlobal, exists := s.GetScheduleRuleGlobalCache(ctx)
	if exists {
		s.logger.Debug("scheduleruleglobal.cache.hit", out.LogFields{})
		return scheduleRuleGlobal, nil
	}

	s.logger.Debug("scheduleruleglobal.cache.miss", out.LogFields{})

	// Запрашиваем из AidboxAdapter
	scheduleRuleGlobal, err := s.aidboxPort.GetScheduleRuleGlobal(ctx)
	if err != nil {
		return nil, err
	}

	scheduleRuleGlobal, err = s.StoreScheduleRuleGlobalCache(ctx, *scheduleRuleGlobal)
	if err != nil {
		s.logger.Error("scheduleruleglobal.cache.store_failed", out.LogFields{
			"error": err.Error(),
		})
	}

	return scheduleRuleGlobal, nil
}

func (s *SlotGeneratorService) getScheduleRule(ctx context.Context, scheduleId string) (*domain.ScheduleRule, error) {
	scheduleRule, exists := s.GetScheduleRuleCache(ctx, scheduleId)
	if exists {
		s.logger.Debug("schedulerule.cache.hit", out.LogFields{
			"scheduleId": scheduleId,
		})
		return scheduleRule, nil
	}

	s.logger.Debug("schedulerule.cache.miss", out.LogFields{
		"scheduleId": scheduleId,
	})

	scheduleRule, err := s.aidboxPort.GetScheduleRule(ctx, scheduleId)
	if err != nil {
		return nil, err
	}

	scheduleRule, err = s.StoreScheduleRuleCache(ctx, *scheduleRule)
	if err != nil {
		s.logger.Error("schedulerule.cache.store_failed", out.LogFields{
			"error": err.Error(),
		})
	}

	return scheduleRule, nil
}
