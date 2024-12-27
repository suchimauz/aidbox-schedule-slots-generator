package domain

import (
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/json_types"
)

type ScheduleRuleGlobalReplacement struct {
	Date      json_types.Date        `json:"date"`
	DayOfWeek ScheduleRuleDaysOfWeek `json:"dayOfWeek"`
}

type ScheduleRuleGlobal struct {
	ID                string                          `json:"id"`
	NotAvailableTimes []ScheduleRuleNotAvailableTime  `json:"notAvailable"`
	Replacements      []ScheduleRuleGlobalReplacement `json:"replacement"`
}
