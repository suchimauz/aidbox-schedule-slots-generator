package slot_generator_service

import (
	"time"

	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
)

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
