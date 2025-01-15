package slot_generator_service

import (
	"slices"
	"time"

	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
)

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
