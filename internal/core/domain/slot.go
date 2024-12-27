package domain

import (
	"time"
)

type Slot struct {
	StartTime      time.Time              `json:"begin"`
	EndTime        time.Time              `json:"end"`
	Week           ScheduleRuleDaysOfWeek `json:"week"`
	Channel        []ScheduleRuleChannel  `json:"channel"`
	AppointmentIDS []string               `json:"app"`
	SlotType       AppointmentType        `json:"type"`
	InCache        bool                   `json:"-"`
}
