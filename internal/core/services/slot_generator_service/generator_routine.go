package slot_generator_service

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
)

func (s *SlotGeneratorService) generateRoutineSlots(schedule *domain.ScheduleRule, scheduleRuleGlobal *domain.ScheduleRuleGlobal, appointments []domain.Appointment, startTime, endTime time.Time, channels string, slotDuration time.Duration, slots *[]domain.Slot, mu *sync.Mutex, wg *sync.WaitGroup) {
	for currentDayDate := startTime; currentDayDate.Truncate(24 * time.Hour).Before(endTime.Truncate(24 * time.Hour).Add(24 * time.Hour)); currentDayDate = currentDayDate.AddDate(0, 0, 1) {
		wg.Add(1)
		go s.generateRoutineSlotsForAvailableTimes(schedule, scheduleRuleGlobal, appointments, currentDayDate, channels, slotDuration, slots, mu, wg)
	}
}

func (s *SlotGeneratorService) generateRoutineSlotsForAvailableTimes(schedule *domain.ScheduleRule, scheduleRuleGlobal *domain.ScheduleRuleGlobal, appointments []domain.Appointment, currentDayDate time.Time, channels string, slotDuration time.Duration, slots *[]domain.Slot, mu *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()

	for _, availableTime := range schedule.AvailableTimes {
		// Проверяем, есть ли канал доступности в настройках промежутка
		if !isChannelAvailable(availableTime, channels) {
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
		go s.generateRoutineSlotsForAvailableTime(schedule, scheduleRuleGlobal, appointments, currentDayDate, availableTime, slotDuration, slots, mu, wg)
	}
}

// Функция для генерации слотов в пределах доступного времени
func (s *SlotGeneratorService) generateRoutineSlotsForAvailableTime(schedule *domain.ScheduleRule, scheduleRuleGlobal *domain.ScheduleRuleGlobal, appointments []domain.Appointment, dayDate time.Time, availableTime domain.ScheduleRuleAvailableTime, slotDuration time.Duration, slots *[]domain.Slot, mu *sync.Mutex, wg *sync.WaitGroup) {
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
		go s.generateRoutineSlot(schedule, scheduleRuleGlobal, appointments, slotStartTime, availableTime, slotDuration, slots, mu, &slotWg)
		slotStartTime = slotStartTime.Add(slotDuration)
	}

	slotWg.Wait()
}

// Обработка каждого отдельного слота
func (s *SlotGeneratorService) generateRoutineSlot(schedule *domain.ScheduleRule, scheduleRuleGlobal *domain.ScheduleRuleGlobal, appointments []domain.Appointment, slotStartTime time.Time, availableTime domain.ScheduleRuleAvailableTime, slotDuration time.Duration, slots *[]domain.Slot, mu *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()

	// Если слот уже прошел, то не добавляем его в результат
	if slotStartTime.Before(time.Now()) {
		return
	}

	startTime := slotStartTime
	endTime := slotStartTime.Add(slotDuration)

	appointmentIDS := s.slotRoutineAppointmentIDS(appointments, startTime, endTime)

	// Создаем слот
	slot := domain.Slot{
		Channel:        availableTime.Channel,
		StartTime:      startTime,
		EndTime:        endTime,
		Week:           getSlotWeekday(scheduleRuleGlobal, startTime),
		AppointmentIDS: appointmentIDS,
		SlotType:       domain.AppointmentTypeRoutine,
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

func (s *SlotGeneratorService) slotRoutineAppointmentIDS(appointments []domain.Appointment, startTime, endTime time.Time) []uuid.UUID {
	var wg sync.WaitGroup
	var mu sync.Mutex
	ids := make([]uuid.UUID, 0)

	for _, appointment := range appointments {
		if appointment.Type == domain.AppointmentTypeRoutine {
			wg.Add(1)
			go s.applyAppointmentToRoutineSlot(appointment, startTime, endTime, &ids, &wg, &mu)
		}
	}
	wg.Wait()

	return ids
}

func (s *SlotGeneratorService) applyAppointmentToRoutineSlot(appointment domain.Appointment, startTime, endTime time.Time, ids *[]uuid.UUID, wg *sync.WaitGroup, mu *sync.Mutex) {
	defer wg.Done()

	slotStart := startTime
	slotEnd := endTime
	appointmentStart := appointment.StartDate.Date
	appointmentEnd := appointment.EndDate.Date

	startOverlapping := appointmentEnd.After(slotStart)
	endOverlapping := appointmentStart.Before(slotEnd)

	if startOverlapping && endOverlapping {
		mu.Lock()
		*ids = append(*ids, appointment.ID)
		mu.Unlock()
	}
}
