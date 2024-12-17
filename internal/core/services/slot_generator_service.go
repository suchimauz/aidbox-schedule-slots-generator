package services

import (
	"context"
	"slices"
	"sort"
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

// Определяем тип для среза слотов
type SlotSlice []domain.Slot

// Реализуем интерфейс sort.Interface
func (s SlotSlice) Len() int {
	return len(s)
}

func (s SlotSlice) Less(i, j int) bool {
	return s[i].StartTime.Before(s[j].StartTime) // Сравниваем по StartTime
}

func (s SlotSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
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
		if slots, exists := s.cachePort.GetSlots(ctx, scheduleID, schedule.PlanningHorizon.Start.Date, schedule.PlanningHorizon.End.Date); exists {
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
	slots := s.generateSlotsForSchedule(ctx, schedule)

	// Сортируем слоты по возрастанию StartTime
	sort.Sort(SlotSlice(slots))

	// Получаем и применяем статусы занятости
	appointments, err := s.aidboxPort.GetScheduleRuleAppointments(ctx, schedule.ID, schedule.PlanningHorizon.Start.Date, schedule.PlanningHorizon.End.Date)
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

func (s *SlotGeneratorService) generateSlotsForSchedule(ctx context.Context, schedule *domain.ScheduleRule) []domain.Slot {
	var slots []domain.Slot
	var endTime time.Time
	var startTime time.Time
	var scheduleRuleGlobal *domain.ScheduleRuleGlobal

	scheduleRuleGlobal, err := s.getScheduleRuleGlobal(ctx)
	if err != nil {
		s.logger.Error("slots.generate.schedule_rule_global.fetch_failed", out.LogFields{
			"error": err.Error(),
		})
	}

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

	// Генерируем слоты с учетом availableTime
	for currentDayDate := startTime; currentDayDate.Truncate(24 * time.Hour).Before(endTime.Truncate(24 * time.Hour).Add(24 * time.Hour)); currentDayDate = currentDayDate.AddDate(0, 0, 1) {
		for _, availableTime := range schedule.AvailableTimes {
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
			go s.generateSlotsForAvailableTime(schedule, scheduleRuleGlobal, currentDayDate, availableTime, slotDuration, &slots, &mu, &wg)
		}
	}

	wg.Wait() // Ждем завершения всех горутин
	return slots
}

// Функция для генерации слотов в пределах доступного времени
func (s *SlotGeneratorService) generateSlotsForAvailableTime(schedule *domain.ScheduleRule, scheduleRuleGlobal *domain.ScheduleRuleGlobal, dayDate time.Time, availableTime domain.ScheduleRuleAvailableTime, slotDuration time.Duration, slots *[]domain.Slot, mu *sync.Mutex, wg *sync.WaitGroup) {
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
		go s.generateSlot(schedule, scheduleRuleGlobal, slotStartTime, availableTime, slotDuration, slots, mu, &slotWg)
		slotStartTime = slotStartTime.Add(slotDuration)
	}

	slotWg.Wait()
}

// Обработка каждого отдельного слота
func (s *SlotGeneratorService) generateSlot(schedule *domain.ScheduleRule, scheduleRuleGlobal *domain.ScheduleRuleGlobal, slotStartTime time.Time, availableTime domain.ScheduleRuleAvailableTime, slotDuration time.Duration, slots *[]domain.Slot, mu *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()

	// Если слот уже прошел, то не добавляем его в результат
	if slotStartTime.Before(time.Now()) {
		return
	}

	// Создаем слот
	slot := domain.Slot{
		Channel:        availableTime.Channel,
		StartTime:      slotStartTime,
		EndTime:        slotStartTime.Add(slotDuration),
		Week:           getSlotWeekday(scheduleRuleGlobal, slotStartTime),
		AppointmentIDS: []uuid.UUID{},
	}

	// Проверяем, нет ли недоступного времени в глобальном расписании
	// В случае если глобальное расписание не игнорируется
	globalNotAvailable := false
	if !schedule.IsIgnoreGlobalRule {
		globalNotAvailable = isNotAvailableTime(slotStartTime, scheduleRuleGlobal.NotAvailableTimes)
	}
	// Проверяем, нет ли недоступного времени в локальном расписании
	localNotAvailable := isNotAvailableTime(slotStartTime, schedule.NotAvailableTimes)

	// Если слот не доступен в локальном расписании или в глобальном расписании, то не добавляем слот в результат
	if !localNotAvailable && !globalNotAvailable {
		// Добавляем слот в результат
		mu.Lock()
		*slots = append(*slots, slot)
		mu.Unlock()
	}
}

func (s *SlotGeneratorService) applyAppointmentsToSlots(slots []domain.Slot, appointments []domain.Appointment) {
	// for _, appointment := range appointments {
	// 	if appointment.Status != domain.AppointmentStatusActive {
	// 		continue
	// 	}

	// 	for i, slot := range slots {
	// 		if slot.StartTime.Equal(appointment.StartTime) {
	// 			slots[i].Status = domain.SlotStatusOccupied
	// 			break
	// 		}
	// 	}
	// }
}

func (s *SlotGeneratorService) UpdateSlotStatus(ctx context.Context, appointmentID uuid.UUID) error {
	return nil
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
