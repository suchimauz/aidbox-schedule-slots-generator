package domain

import (
	"time"

	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/json_types"
)

type ScheduleRuleChannel string

const (
	ScheduleRuleChannelDoctor      ScheduleRuleChannel = "doctor"
	ScheduleRuleChannelReg         ScheduleRuleChannel = "reg"
	ScheduleRuleChannelKc          ScheduleRuleChannel = "kc"
	ScheduleRuleChannelKcMo        ScheduleRuleChannel = "kc-mo"
	ScheduleRuleChannelWeb         ScheduleRuleChannel = "web"
	ScheduleRuleChannelFreg        ScheduleRuleChannel = "freg"
	ScheduleRuleChannelWebReferral ScheduleRuleChannel = "web-referral"
	ScheduleRuleChannelTmkOnline   ScheduleRuleChannel = "tmk-online"
)

type ScheduleRuleDaysOfWeek string

const (
	ScheduleRuleDaysOfWeekMon ScheduleRuleDaysOfWeek = "mon"
	ScheduleRuleDaysOfWeekTue ScheduleRuleDaysOfWeek = "tue"
	ScheduleRuleDaysOfWeekWed ScheduleRuleDaysOfWeek = "wed"
	ScheduleRuleDaysOfWeekThu ScheduleRuleDaysOfWeek = "thu"
	ScheduleRuleDaysOfWeekFri ScheduleRuleDaysOfWeek = "fri"
	ScheduleRuleDaysOfWeekSat ScheduleRuleDaysOfWeek = "sat"
	ScheduleRuleDaysOfWeekSun ScheduleRuleDaysOfWeek = "sun"
)

var ScheduleRuleDaysOfWeekMap = map[time.Weekday]ScheduleRuleDaysOfWeek{
	time.Monday:    ScheduleRuleDaysOfWeekMon,
	time.Tuesday:   ScheduleRuleDaysOfWeekTue,
	time.Wednesday: ScheduleRuleDaysOfWeekWed,
	time.Thursday:  ScheduleRuleDaysOfWeekThu,
	time.Friday:    ScheduleRuleDaysOfWeekFri,
	time.Saturday:  ScheduleRuleDaysOfWeekSat,
	time.Sunday:    ScheduleRuleDaysOfWeekSun,
}

type ScheduleRuleParity string

const (
	ScheduleRuleParityOdd  ScheduleRuleParity = "odd"
	ScheduleRuleParityEven ScheduleRuleParity = "even"
)

type ScheduleRuleAvailableTime struct {
	StartTime  json_types.Time          `json:"availableStartTime"`
	EndTime    json_types.Time          `json:"availableEndTime"`
	Channel    []ScheduleRuleChannel    `json:"channel"`
	DaysOfWeek []ScheduleRuleDaysOfWeek `json:"daysOfWeek"`
	Parity     ScheduleRuleParity       `json:"parity"`
}

func (s *ScheduleRuleAvailableTime) ContainsChannel(channel string) bool {
	for _, c := range s.Channel {
		if string(c) == channel {
			return true
		}
	}
	return false
}

type ScheduleRuleNotAvailableTimeDuring struct {
	Start json_types.Date `json:"start"`
	End   json_types.Date `json:"end"`
}

type ScheduleRuleNotAvailableTime struct {
	During ScheduleRuleNotAvailableTimeDuring `json:"during"`
}

type ScheduleRulePlanningHorizon struct {
	Start json_types.DateTimeOrEmpty `json:"start"`
	End   json_types.DateTimeOrEmpty `json:"end"`
}

type ScheduleRulePlanningActive struct {
	Type     string `json:"type"`
	Quantity int    `json:"quantity"`
}

type ScheduleRule struct {
	ID                 string                         `json:"id"`
	PlanningHorizon    ScheduleRulePlanningHorizon    `json:"planningHorizon"`
	MinutesDuration    int                            `json:"minutesDuration"`
	AvailableTimes     []ScheduleRuleAvailableTime    `json:"availableTime"`
	NotAvailableTimes  []ScheduleRuleNotAvailableTime `json:"notAvailable"`
	PlanningActive     ScheduleRulePlanningActive     `json:"planningActive"`
	IsIgnoreGlobalRule bool                           `json:"ignore-global-rule"`
}
