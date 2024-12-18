package services

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/config"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
)

type SlotGeneratorServiceDebug struct {
	mu   sync.Mutex
	data []domain.DebugInfo
}

func (d *SlotGeneratorServiceDebug) AddDebugInfo(info domain.DebugInfo) {
	d.mu.Lock()
	d.data = append(d.data, info)
	d.mu.Unlock()
}

type SlotGeneratorService struct {
	aidboxPort out.AidboxPort
	cachePort  out.CachePort
	logger     out.LoggerPort
	cfg        *config.Config
	debug      SlotGeneratorServiceDebug
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
		debug: SlotGeneratorServiceDebug{
			data: make([]domain.DebugInfo, 0),
		},
	}
}

type SlotSlice []domain.Slot

// quickSort — функция для сортировки SlotSlice
func (s SlotSlice) quickSort() SlotSlice {
	if len(s) < 2 {
		return s
	}

	// Выбираем опорный элемент
	pivot := s[len(s)/2]

	// Разделяем слайс на три части
	less := SlotSlice{}
	equal := SlotSlice{}
	greater := SlotSlice{}

	for _, slot := range s {
		if slot.StartTime.Before(pivot.StartTime) {
			less = append(less, slot)
		} else if slot.StartTime.Equal(pivot.StartTime) {
			equal = append(equal, slot)
		} else {
			greater = append(greater, slot)
		}
	}

	// Рекурсивно сортируем подмассивы и объединяем их
	return append(append(less.quickSort(), equal...), greater.quickSort()...)
}

func (s *SlotGeneratorService) GenerateSlots(ctx context.Context, scheduleID uuid.UUID, channelParam string) ([]domain.Slot, []domain.DebugInfo, error) {
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
		return nil, nil, err
	}
	get_schedule_rule_debug.Elapse()
	s.debug.AddDebugInfo(get_schedule_rule_debug)

	// Проверяем кэш только если он включен
	if s.cachePort != nil && s.cfg.Cache.Enabled {
		if slots, exists := s.cachePort.GetSlots(ctx, scheduleID, schedule.PlanningHorizon.Start.Date, schedule.PlanningHorizon.End.Date); exists {
			s.logger.Debug("slots.generate.cache.hit", out.LogFields{
				"scheduleId": scheduleID,
				"slotsCount": len(slots),
			})
			return slots, s.debug.data, nil
		}
	}

	s.logger.Debug("slots.generate.cache.miss", out.LogFields{
		"scheduleId": scheduleID,
	})

	// Генерируем слоты
	slots := s.generateSlotsForSchedule(ctx, schedule, channelParam)

	sort_slots_debug := domain.DebugInfo{
		Event: "slots.generate.slots.sort",
	}
	sort_slots_debug.Start()
	slots = SlotSlice(slots).quickSort()
	sort_slots_debug.Elapse()
	s.debug.AddDebugInfo(sort_slots_debug)

	// Сохраняем в кэш только если он включен
	if s.cachePort != nil && s.cfg.Cache.Enabled {
		s.cachePort.StoreSlots(ctx, scheduleID, slots)
	}

	return slots, s.debug.data, nil
}

func (s *SlotGeneratorService) GenerateBatchSlots(ctx context.Context, scheduleIDs []uuid.UUID, channelParam string) (map[uuid.UUID][]domain.Slot, error) {
	result := make(map[uuid.UUID][]domain.Slot)
	var mu sync.Mutex
	var wg sync.WaitGroup
	errCh := make(chan error, len(scheduleIDs))
	for _, id := range scheduleIDs {
		wg.Add(1)
		go func(scheduleID uuid.UUID, channelParam string) {
			defer wg.Done()

			slots, _, err := s.GenerateSlots(ctx, scheduleID, channelParam)
			if err != nil {
				errCh <- err
				return
			}

			mu.Lock()
			result[scheduleID] = slots
			mu.Unlock()
		}(id, channelParam)
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

func (s *SlotGeneratorService) generateSlotsForSchedule(ctx context.Context, schedule *domain.ScheduleRule, channel string) []domain.Slot {
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
	}
	get_schedule_rule_global_debug.Elapse()
	s.debug.AddDebugInfo(get_schedule_rule_global_debug)

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

	// Используем мьютекс для безопасного доступа к слайсу slots
	// И группу ожидания для ожидания завершения всех горутин
	var mu sync.Mutex
	var wg sync.WaitGroup

	get_appointments_debug := domain.DebugInfo{
		Event: "slots.generate.appointments.fetch",
	}
	get_appointments_debug.Start()

	appointments, err := s.aidboxPort.GetScheduleRuleAppointments(ctx, schedule.ID, startTime, endTime)
	if err != nil {
		s.logger.Error("slots.generate.appointments.fetch_failed", out.LogFields{
			"error": err.Error(),
		})
		return nil
	}

	get_appointments_debug.Elapse()
	s.debug.AddDebugInfo(get_appointments_debug)

	generate_slots_debug := domain.DebugInfo{
		Event: "slots.generate.slots.generate",
	}
	generate_slots_debug.Start()

	// Генерируем слоты с учетом availableTime
	for currentDayDate := startTime; currentDayDate.Truncate(24 * time.Hour).Before(endTime.Truncate(24 * time.Hour).Add(24 * time.Hour)); currentDayDate = currentDayDate.AddDate(0, 0, 1) {
		for _, availableTime := range schedule.AvailableTimes {
			// Проверяем, есть ли канал доступности в настройках промежутка
			if !availableTime.ContainsChannel(channel) {
				continue
			}

			// Проверяем, попадает ли текущий день в доступные дни
			if !isDayAvailable(currentDayDate, availableTime) {
				continue
			}

			// Проверяем, попадает ли текущий день недели в доступные дни недели
			if !isWeekdayAvailable(scheduleRuleGlobal, currentDayDate, availableTime) {
				continue
			}

			// Генерируем слоты в параллельных горутинах
			wg.Add(1)
			go s.generateSlotsForAvailableTime(schedule, scheduleRuleGlobal, appointments, currentDayDate, availableTime, slotDuration, &slots, &mu, &wg)
		}
	}

	wg.Wait() // Ждем завершения всех горутин

	generate_slots_debug.Elapse()
	s.debug.AddDebugInfo(generate_slots_debug)

	return slots
}

// Функция для генерации слотов в пределах доступного времени
func (s *SlotGeneratorService) generateSlotsForAvailableTime(schedule *domain.ScheduleRule, scheduleRuleGlobal *domain.ScheduleRuleGlobal, appointments []domain.Appointment, dayDate time.Time, availableTime domain.ScheduleRuleAvailableTime, slotDuration time.Duration, slots *[]domain.Slot, mu *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()

	// Начинаем с доступного времени
	slotStartTime := time.Date(dayDate.Year(), dayDate.Month(), dayDate.Day(),
		availableTime.StartTime.Time.Hour(), availableTime.StartTime.Time.Minute(), 0, 0, dayDate.Location())

	// Определяем конец доступного времени
	endTime := time.Date(dayDate.Year(), dayDate.Month(), dayDate.Day(),
		availableTime.EndTime.Time.Hour(), availableTime.EndTime.Time.Minute(), 0, 0, dayDate.Location())

	var slotWg sync.WaitGroup

	// Генерируем слоты в пределах доcтупного времени
	for slotStartTime.Add(slotDuration).Before(endTime) || slotStartTime.Add(slotDuration).Equal(endTime) {
		slotWg.Add(1)
		go s.generateSlot(schedule, scheduleRuleGlobal, appointments, slotStartTime, availableTime, slotDuration, slots, mu, &slotWg)
		slotStartTime = slotStartTime.Add(slotDuration)
	}

	slotWg.Wait()
}

// Обработка каждого отдельного слота
func (s *SlotGeneratorService) generateSlot(schedule *domain.ScheduleRule, scheduleRuleGlobal *domain.ScheduleRuleGlobal, appointments []domain.Appointment, slotStartTime time.Time, availableTime domain.ScheduleRuleAvailableTime, slotDuration time.Duration, slots *[]domain.Slot, mu *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()

	// Если слот уже прошел, то не добавляем его в результат
	if slotStartTime.Before(time.Now()) {
		return
	}

	startTime := slotStartTime
	endTime := slotStartTime.Add(slotDuration)

	appointmentIDS := s.slotAppointmentIDS(appointments, startTime, endTime)

	// Создаем слот
	slot := domain.Slot{
		Channel:        availableTime.Channel,
		StartTime:      startTime,
		EndTime:        endTime,
		Week:           getSlotWeekday(scheduleRuleGlobal, startTime),
		AppointmentIDS: appointmentIDS,
	}

	// Проверяем, нет ли недоступного времени в глобальном расписании
	// В случае если глобальное расписание не игнорируется
	globalNotAvailable := false
	if !schedule.IsIgnoreGlobalRule {
		globalNotAvailable = isNotAvailableTime(startTime, scheduleRuleGlobal.NotAvailableTimes)
	}
	// Проверяем, нет ли недоступного времени в локальном расписании
	localNotAvailable := isNotAvailableTime(startTime, schedule.NotAvailableTimes)

	// Если слот не доступен в локальном расписании или в глобальном расписании, то не добавляем слот в результат
	if !localNotAvailable && !globalNotAvailable {
		// Добавляем слот в результат
		mu.Lock()
		*slots = append(*slots, slot)
		mu.Unlock()
	}
}

func (s *SlotGeneratorService) slotAppointmentIDS(appointments []domain.Appointment, startTime, endTime time.Time) []uuid.UUID {
	var wg sync.WaitGroup
	var mu sync.Mutex
	ids := make([]uuid.UUID, 0)

	for _, appointment := range appointments {
		wg.Add(1)
		go s.applyAppointmentToSlot(appointment, startTime, endTime, &ids, &wg, &mu)
	}
	wg.Wait()

	return ids
}

func (s *SlotGeneratorService) applyAppointmentToSlot(appointment domain.Appointment, startTime, endTime time.Time, ids *[]uuid.UUID, wg *sync.WaitGroup, mu *sync.Mutex) {
	defer wg.Done()

	slotStart := startTime
	slotEnd := endTime
	appointmentStart := appointment.StartDate.Date
	appointmentEnd := appointment.EndDate.Date

	startOverlapping := appointmentEnd.After(slotStart) || appointmentEnd.Equal(slotStart)
	endOverlapping := appointmentStart.Before(slotEnd) || appointmentStart.Equal(slotEnd)

	if startOverlapping && endOverlapping {
		mu.Lock()
		*ids = append(*ids, appointment.ID)
		mu.Unlock()
	}
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

func getSlotWeekday(scheduleRuleGlobal *domain.ScheduleRuleGlobal, date time.Time) domain.ScheduleRuleDaysOfWeek {
	date_weekday := date.Weekday()
	weekday := domain.ScheduleRuleDaysOfWeekMap[date_weekday]

	// Проверяем попадает ли дата в Замену дня недели
	// Если попадает, то заменяем текущий день недели на заменяемый день недели из производственного календаря
	for _, replacement := range scheduleRuleGlobal.Replacements {
		replacementDay := replacement.Date.Date.Format("2006-01-02")
		slotDay := date.Truncate(24 * time.Hour).Format("2006-01-02")
		if replacementDay == slotDay {
			weekday = replacement.DayOfWeek
		}
	}

	return weekday
}

// Функция для проверки доступности дня недели
func isWeekdayAvailable(scheduleRuleGlobal *domain.ScheduleRuleGlobal, date time.Time, availableTime domain.ScheduleRuleAvailableTime) bool {
	weekday := getSlotWeekday(scheduleRuleGlobal, date)

	return slices.Contains(availableTime.DaysOfWeek, weekday)
}

// Функция для проверки вхождения даты в период недоступности
func isNotAvailableTime(date time.Time, notAvailableTimes []domain.ScheduleRuleNotAvailableTime) bool {
	for _, notAvailableTime := range notAvailableTimes {
		start := notAvailableTime.During.Start.Date
		end := notAvailableTime.During.End.Date
		if (date.After(start) || date.Equal(start)) && date.Before(end) {
			return true
		}
	}
	return false
}

// Функция для проверки доступности дня
func isDayAvailable(date time.Time, availableTime domain.ScheduleRuleAvailableTime) bool {
	dayOfMonth := date.Day()

	// Логика проверки доступности дня
	// Проверка на четность/нечетность или конкретные дни недели
	if availableTime.Parity == domain.ScheduleRuleParityEven {
		return dayOfMonth%2 == 0
	} else if availableTime.Parity == domain.ScheduleRuleParityOdd {
		return dayOfMonth%2 != 0
	}

	// Если нет четности, то доступен любой день
	return true
}

// Функция для проверки доступности времени
func isTimeAvailable(currentTime time.Time, availableTime domain.ScheduleRuleAvailableTime) bool {
	start := availableTime.StartTime.Time
	end := availableTime.EndTime.Time
	return (currentTime.After(start) || currentTime.Equal(start)) && (currentTime.Before(end) || currentTime.Equal(end))
}
