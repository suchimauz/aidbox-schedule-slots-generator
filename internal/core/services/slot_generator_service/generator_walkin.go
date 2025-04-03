package slot_generator_service

import (
	"sync"
	"time"

	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/utils"
)

func (s *SlotGeneratorService) generateWalkinSlots(schedule *domain.ScheduleRule, scheduleRuleGlobal *domain.ScheduleRuleGlobal, appointments []domain.Appointment, walkinSlots *[]domain.Slot, routineSlots *[]domain.Slot, mu *sync.Mutex, wg *sync.WaitGroup) {
	if schedule.OverbookingCount == 0 {
		return
	}

	uniqueDaysMap := make(map[time.Time]struct{})

	for _, slot := range *routineSlots {
		if slot.SlotType == domain.AppointmentTypeRoutine {
			uniqueDaysMap[slot.StartTime.Truncate(24*time.Hour)] = struct{}{}
		}
	}

	for day := range uniqueDaysMap {
		wg.Add(1)
		go s.generateWalkinSlot(schedule, scheduleRuleGlobal, appointments, day, walkinSlots, mu, wg)
	}
}

func (s *SlotGeneratorService) generateWalkinSlot(schedule *domain.ScheduleRule, scheduleRuleGlobal *domain.ScheduleRuleGlobal, appointments []domain.Appointment, currentDayDate time.Time, slots *[]domain.Slot, mu *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()

	location := time.FixedZone("Europe/Moscow", 3*60*60)

	startTime := time.Date(currentDayDate.Year(), currentDayDate.Month(), currentDayDate.Day(), 0, 0, 0, 0, location)
	endTime := utils.StartNextDay(startTime)

	is_weekday_available := false
	for _, availableTime := range schedule.AvailableTimes {
		if isWeekdayAvailable(scheduleRuleGlobal, currentDayDate, availableTime) {
			is_weekday_available = true
			break
		}
	}

	if !is_weekday_available {
		return
	}

	appointmentIDS := s.slotWalkinAppointmentIDS(appointments, startTime, endTime)

	// Создаем слот
	slot := domain.Slot{
		Channel:        []domain.ScheduleRuleChannel{},
		StartTime:      startTime,
		EndTime:        endTime,
		Week:           getSlotWeekday(scheduleRuleGlobal, startTime),
		AppointmentIDS: appointmentIDS,
		SlotType:       domain.AppointmentTypeWalkin,
	}

	mu.Lock()
	*slots = append(*slots, slot)
	mu.Unlock()
}

func (s *SlotGeneratorService) slotWalkinAppointmentIDS(appointments []domain.Appointment, startTime, endTime time.Time) []string {
	var wg sync.WaitGroup
	var mu sync.Mutex
	ids := make([]string, 0)

	for _, appointment := range appointments {
		if appointment.Type == domain.AppointmentTypeWalkin {
			wg.Add(1)
			go s.applyAppointmentToWalkinSlot(appointment, startTime, &ids, &wg, &mu)
		}
	}
	wg.Wait()

	return ids
}

func (s *SlotGeneratorService) applyAppointmentToWalkinSlot(appointment domain.Appointment, startTime time.Time, ids *[]string, wg *sync.WaitGroup, mu *sync.Mutex) {
	defer wg.Done()

	slotStart := startTime.Truncate(24 * time.Hour)
	appointmentStart := appointment.StartDate.Date.Truncate(24 * time.Hour)

	if slotStart.Equal(appointmentStart) {
		mu.Lock()
		*ids = append(*ids, appointment.ID)
		mu.Unlock()
	}
}
