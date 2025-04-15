package slot_generator_service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/config"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/in"
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

func (s *SlotGeneratorService) prepareResponseSlots(debugInfo *SlotGeneratorServiceDebug, slots []domain.Slot, request in.GenerateSlotsRequest, startTime time.Time, endTime time.Time) map[domain.AppointmentType][]domain.Slot {
	routineSlots := make([]domain.Slot, 0)
	walkinSlots := make([]domain.Slot, 0)

	splitted_channels := strings.Split(request.Channels, ",")
	has_freg_channel := false
	for _, channel := range splitted_channels {
		if channel == "freg" {
			has_freg_channel = true
			break
		}
	}

	// В ответе разделяем слоты по их типу
	// Например:
	// ROUTINE: [slot1, slot2, slot3]
	// WALKIN: [slot4, slot5, slot6]
	for _, slot := range slots {
		// Если есть freg канал, то мы пропускаем этот слот если его дата начала меньше чем startTime + 1 день
		// freg канал - это канал для других МО
		if has_freg_channel {
			startOverlapping := slot.StartTime.Before(endTime)
			endOverlapping := slot.EndTime.After(startTime.AddDate(0, 0, 1))

			if !(startOverlapping && endOverlapping) {
				continue
			}
		}

		if slot.SlotType == domain.AppointmentTypeRoutine {
			// Проверяем, доступен ли слот по каналам
			if isChannelAvailable(slot.Channel, request.Channels) {
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

	// Если нужно вернуть только свободные слоты, то отфильтровываем их
	if request.OnlyFree {
		onlyFreeRoutineSlots := make([]domain.Slot, 0)

		for _, slot := range routineSlots {
			if len(slot.AppointmentIDS) == 0 {
				onlyFreeRoutineSlots = append(onlyFreeRoutineSlots, slot)
			}
		}

		routineSlots = onlyFreeRoutineSlots
	}

	// Если количество слотов ограничено, то отрезаем их
	// И возвращаем только нужные слоты
	// Walkin слоты не возвращаем, т.к. они не нужны при данном варианте
	if request.SlotsCount != -1 {
		if len(routineSlots) > request.SlotsCount {
			routineSlots = routineSlots[:request.SlotsCount]
		}

		return map[domain.AppointmentType][]domain.Slot{
			domain.AppointmentTypeRoutine: routineSlots,
		}
	}

	return map[domain.AppointmentType][]domain.Slot{
		domain.AppointmentTypeRoutine: routineSlots,
		domain.AppointmentTypeWalkin:  walkinSlots,
	}
}

func (s *SlotGeneratorService) GenerateSlots(ctx context.Context, request in.GenerateSlotsRequest) (map[domain.AppointmentType][]domain.Slot, []domain.DebugInfo, error) {
	debugInfo := SlotGeneratorServiceDebug{
		data: make([]domain.DebugInfo, 0),
	}
	s.logger.Info("slots.generate.started", out.LogFields{
		"scheduleId": request.ScheduleID,
	})

	get_schedule_rule_debug := domain.DebugInfo{
		Event: "slots.generate.schedule.fetch",
	}
	get_schedule_rule_debug.Start()

	schedule, err := s.getScheduleRule(ctx, request.ScheduleID)
	if err != nil {
		return nil, nil, err
	}
	get_schedule_rule_debug.Elapse()
	debugInfo.AddDebugInfo(get_schedule_rule_debug)

	s.logger.Debug("slots.generate.cache.miss", out.LogFields{
		"scheduleId": request.ScheduleID,
	})

	var startTime, endTime, planningEndTime time.Time
	defaultStartTime := time.Now().In(config.TimeZone)
	// Начальная дата, равная началу планируемого горизонта или текущей датой, если горизонт не задан
	// Или горизонт задан в прошлом
	if schedule.PlanningHorizon.Start.Date.IsZero() || schedule.PlanningHorizon.Start.Date.Before(time.Now()) {
		if request.StartDate.IsZero() {
			startTime = defaultStartTime
		} else {
			startTime = request.StartDate
		}
	} else {
		startTime = schedule.PlanningHorizon.Start.Date
	}

	generateStartTime := utils.StartCurrentDay(startTime)

	// Вычисляем длительность слота
	slotDuration := time.Duration(schedule.MinutesDuration) * time.Minute
	if slotDuration == 0 {
		get_healthcare_service_debug := domain.DebugInfo{
			Event: "slots.generate.healthcare_service.fetch",
		}
		get_healthcare_service_debug.Start()

		healthcareService, err := s.getHealthcareService(ctx, schedule.HealthcareServiceRef[0].ID)
		if err != nil {
			s.logger.Error("slots.generate.healthcare_service.fetch_failed", out.LogFields{
				"error": err.Error(),
			})
			return nil, nil, fmt.Errorf("slots.generate.healthcare_service.fetch_failed: %w", err)
		}
		slotDuration = time.Duration(healthcareService.MinutesDuration) * time.Minute

		get_healthcare_service_debug.Elapse()
		debugInfo.AddDebugInfo(get_healthcare_service_debug)
	}

	if slotDuration == 0 {
		s.logger.Error("slots.generate.slots_duration_is_0", out.LogFields{})
		return nil, nil, fmt.Errorf("slots duration is 0")
	}

	// Вычисляем длительность активного периода, пока подразумевается что там всегда недели
	planningActiveDuration := time.Duration(schedule.PlanningActive.Quantity) * 7 * 24 * time.Hour
	// Вычисляем конец активного периода
	nowPlusPlanningActiveDuration := generateStartTime.Add(planningActiveDuration)

	// Проверяем есть ли вообще планируемый горизонт
	// Или планируемый активный период заканчивается раньше, чем планируемый горизонт, то используем планируемый активный период
	if schedule.PlanningHorizon.End.Date.IsZero() || nowPlusPlanningActiveDuration.Before(schedule.PlanningHorizon.End.Date) {
		planningEndTime = nowPlusPlanningActiveDuration
		// Иначе используем планируемый горизонт
	} else {
		planningEndTime = schedule.PlanningHorizon.End.Date
	}
	endTime = utils.StartNextDay(planningEndTime)

	s.logger.Info("slots.generate.planning_time_range", out.LogFields{
		"planningEndTime": planningEndTime,
		"endTime":         endTime,
		"startTime":       startTime,
	})

	// Используем мьютекс для безопасного доступа к слайсу slots
	// И группу ожидания для ожидания завершения всех горутин
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Генерируем слоты с интервалами
	slots, err := s.generateSlotsForSchedule(ctx, &debugInfo, schedule, generateStartTime, endTime, slotDuration, request, &mu, &wg)
	if err != nil {
		return nil, nil, err
	}

	returnSlots := make([]domain.Slot, 0)

	// Добавляем только те слоты, которые начинаются после startTime
	for _, slot := range slots {
		if slot.SlotType == domain.AppointmentTypeRoutine {
			startOverlapping := slot.StartTime.After(startTime)
			endOverlapping := slot.EndTime.Before(endTime)
			if startOverlapping && endOverlapping {
				returnSlots = append(returnSlots, slot)
			}
		} else if slot.SlotType == domain.AppointmentTypeWalkin {
			returnSlots = append(returnSlots, slot)
		}
	}

	apply_50_percent_rule_debug := domain.DebugInfo{
		Event: "slots.generate.50_percent_rule.apply",
	}
	apply_50_percent_rule_debug.Start()
	// Применяем правило 50%
	s.apply50PercentRule(startTime, endTime, request.With50PercentRule, &returnSlots, &mu, &wg)
	// Ждем завершения всех горутин
	wg.Wait()
	apply_50_percent_rule_debug.Elapse()
	debugInfo.AddDebugInfo(apply_50_percent_rule_debug)

	return s.prepareResponseSlots(&debugInfo, returnSlots, request, startTime, endTime), debugInfo.data, nil
}

func (s *SlotGeneratorService) generateRangeSlotsForSchedule(ctx context.Context, debugInfo *SlotGeneratorServiceDebug, schedule *domain.ScheduleRule, scheduleRuleGlobal *domain.ScheduleRuleGlobal, startTime time.Time, endTime time.Time, slotDuration time.Duration, mu *sync.Mutex, wg *sync.WaitGroup) ([]domain.Slot, []domain.Slot, error) {
	get_appointments_debug := domain.DebugInfo{
		Event: "slots.generate.appointments.fetch",
	}
	get_appointments_debug.Start()

	appointments, err := s.aidboxPort.GetScheduleRuleAppointments(ctx, schedule.ID, startTime, endTime)
	if err != nil {
		s.logger.Error("slots.generate.appointments.fetch_failed", out.LogFields{
			"error": err.Error(),
		})
		return nil, nil, fmt.Errorf("slots.generate.appointments.fetch_failed: %w", err)
	}

	get_appointments_debug.Elapse()
	debugInfo.AddDebugInfo(get_appointments_debug)

	generate_routine_slots_debug := domain.DebugInfo{
		Event: "slots.generate.routine_slots.generate",
		Options: map[string]string{
			"generateStartTime": startTime.Format(time.RFC3339),
			"generateEndTime":   endTime.Format(time.RFC3339),
		},
	}
	generate_walkin_slots_debug := domain.DebugInfo{
		Event: "slots.generate.walkin_slots.generate",
		Options: map[string]string{
			"generateStartTime": startTime.Format(time.RFC3339),
			"generateEndTime":   endTime.Format(time.RFC3339),
		},
	}

	generate_routine_slots_debug.Start()
	var routineSlots []domain.Slot
	s.generateRoutineSlots(schedule, scheduleRuleGlobal, appointments, startTime, endTime, slotDuration, &routineSlots, mu, wg)
	wg.Wait()
	generate_routine_slots_debug.Elapse()
	generate_routine_slots_debug.AddOption("routineSlotsCount", strconv.Itoa(len(routineSlots)))
	debugInfo.AddDebugInfo(generate_routine_slots_debug)

	generate_walkin_slots_debug.Start()
	var walkinSlots []domain.Slot
	s.generateWalkinSlots(schedule, scheduleRuleGlobal, appointments, &walkinSlots, &routineSlots, mu, wg)
	wg.Wait()
	generate_walkin_slots_debug.Elapse()
	generate_walkin_slots_debug.AddOption("walkinSlotsCount", strconv.Itoa(len(walkinSlots)))
	debugInfo.AddDebugInfo(generate_walkin_slots_debug)

	// Сохраняем в кэш слоты
	s.cachePort.StoreSlots(ctx, schedule.ID, startTime, endTime, routineSlots)
	s.cachePort.StoreSlots(ctx, schedule.ID, startTime, endTime, walkinSlots)

	return routineSlots, walkinSlots, nil
}

func (s *SlotGeneratorService) generateSlotsForSchedule(ctx context.Context, debugInfo *SlotGeneratorServiceDebug, schedule *domain.ScheduleRule, startTime, endTime time.Time, slotDuration time.Duration, request in.GenerateSlotsRequest, mu *sync.Mutex, wg *sync.WaitGroup) ([]domain.Slot, error) {
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

	cachedRoutineSlots, _ := s.GetSlotsCache(ctx, schedule.ID, startTime, endTime, domain.AppointmentTypeRoutine)
	cachedWalkinSlots, _ := s.GetSlotsCache(ctx, schedule.ID, startTime, endTime, domain.AppointmentTypeWalkin)
	cachedGenerateStartTime, cachedGenerateEndTime, cacheExists := s.cachePort.GetSlotsCachedMeta(ctx, schedule.ID)

	s.logger.Debug("slots.generate.cache", out.LogFields{
		"scheduleId":              schedule.ID,
		"cachedGenerateStartTime": cachedGenerateStartTime.Format(time.RFC3339),
		"cachedGenerateEndTime":   cachedGenerateEndTime.Format(time.RFC3339),
		"generateStartTime":       startTime.Format(time.RFC3339),
		"generateEndTime":         endTime.Format(time.RFC3339),
	})

	// Если кэш есть, то начинаем с последнего слота в кэше
	if cacheExists {
		// Если начало генерации раньше, чем начало кэша, то генерируем слоты до начала кэша
		if startTime.Before(cachedGenerateStartTime) {
			routineSlots, walkinSlots, err := s.generateRangeSlotsForSchedule(ctx, debugInfo, schedule, scheduleRuleGlobal, startTime, cachedGenerateStartTime, slotDuration, mu, wg)
			if err != nil {
				return nil, err
			}
			cachedRoutineSlots = append(cachedRoutineSlots, routineSlots...)
			cachedWalkinSlots = append(cachedWalkinSlots, walkinSlots...)
		}

		// Если конец даты позже, чем конец кэша, то генерируем слоты от конца кэша до конца даты
		if endTime.After(cachedGenerateEndTime) {
			routineSlots, walkinSlots, err := s.generateRangeSlotsForSchedule(ctx, debugInfo, schedule, scheduleRuleGlobal, cachedGenerateEndTime, endTime, slotDuration, mu, wg)
			if err != nil {
				return nil, err
			}
			cachedRoutineSlots = append(cachedRoutineSlots, routineSlots...)
			cachedWalkinSlots = append(cachedWalkinSlots, walkinSlots...)
		}

		s.logger.Debug("slots.generate.cache.hit", out.LogFields{
			"scheduleId": schedule.ID,
			"slotsCount": len(cachedRoutineSlots),
		})
		get_from_cache_debug := domain.DebugInfo{
			Event: "slots.generate.cache.get",
			Options: map[string]string{
				"cachedPlanningHorizonStart": cachedGenerateStartTime.Format(time.RFC3339),
				"cachedPlanningHorizonEnd":   cachedGenerateEndTime.Format(time.RFC3339),
			},
		}
		get_from_cache_debug.Start()

		slots = cachedRoutineSlots

		s.logger.Info("slots.generate.cache.get", out.LogFields{
			"routineLength": len(cachedRoutineSlots),
			"walkinLength":  len(cachedWalkinSlots),
		})

		slots = append(slots, cachedWalkinSlots...)

		get_from_cache_debug.Elapse()
		get_from_cache_debug.AddOption("routineSlotsCount", strconv.Itoa(len(cachedRoutineSlots)))
		get_from_cache_debug.AddOption("walkinSlotsCount", strconv.Itoa(len(cachedWalkinSlots)))
		debugInfo.AddDebugInfo(get_from_cache_debug)

		return slots, nil
	}

	routineSlots, walkinSlots, err := s.generateRangeSlotsForSchedule(ctx, debugInfo, schedule, scheduleRuleGlobal, startTime, endTime, slotDuration, mu, wg)
	if err != nil {
		return nil, err
	}

	slots = append(slots, routineSlots...)
	slots = append(slots, walkinSlots...)

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

func (s *SlotGeneratorService) getHealthcareService(ctx context.Context, healthcareServiceID string) (*domain.HealthcareService, error) {
	healthcareService, exists := s.GetHealthcareServiceCache(ctx, healthcareServiceID)
	if exists {
		s.logger.Debug("healthcare_service.cache.hit", out.LogFields{
			"healthcareServiceId": healthcareServiceID,
		})
		return healthcareService, nil
	}

	s.logger.Debug("healthcare_service.cache.miss", out.LogFields{
		"healthcareServiceId": healthcareServiceID,
	})

	healthcareService, err := s.aidboxPort.GetHealthcareServiceByID(ctx, healthcareServiceID)
	if err != nil {
		s.logger.Error("healthcare_service.fetch_failed", out.LogFields{
			"error": err.Error(),
		})
		return nil, err
	}

	healthcareService, err = s.StoreHealthcareServiceCache(ctx, *healthcareService)
	if err != nil {
		s.logger.Error("healthcare_service.cache.store_failed", out.LogFields{
			"error": err.Error(),
		})
	}

	return healthcareService, nil
}
