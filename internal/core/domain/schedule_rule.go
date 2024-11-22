package domain

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
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

type ScheduleRuleParity string

const (
	ScheduleRuleParityOdd  ScheduleRuleParity = "odd"
	ScheduleRuleParityEven ScheduleRuleParity = "even"
)

type AvailableTime struct {
	time.Time
}

func (t *AvailableTime) UnmarshalJSON(data []byte) error {
	// Убираем кавычки вокруг строки
	str := string(data[1 : len(data)-1])
	parsedTime, err := time.Parse("15:04:05", str)
	if err != nil {
		return fmt.Errorf("failed to parse time: %v", err)
	}
	t.Time = parsedTime
	return nil
}

func (t AvailableTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Format("15:04:05"))
}

type ScheduleRuleAvailableTime struct {
	StartTime  AvailableTime            `json:"availableStartTime"`
	EndTime    AvailableTime            `json:"availableEndTime"`
	Channel    []ScheduleRuleChannel    `json:"channel"`
	DaysOfWeek []ScheduleRuleDaysOfWeek `json:"daysOfWeek"`
	Parity     ScheduleRuleParity       `json:"parity"`
}

type ScheduleRule struct {
	ID              uuid.UUID                   `json:"id"`
	StartDate       time.Time                   `json:"startDate"`
	EndDate         time.Time                   `json:"endDate"`
	MinutesDuration int                         `json:"minutesDuration"`
	AvailableTimes  []ScheduleRuleAvailableTime `json:"availableTime"`
}
