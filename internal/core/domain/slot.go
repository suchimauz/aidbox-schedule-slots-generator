package domain

import (
	"time"

	"github.com/google/uuid"
)

type Slot struct {
	StartTime      time.Time              `json:"begin"`
	EndTime        time.Time              `json:"end"`
	Week           ScheduleRuleDaysOfWeek `json:"week"`
	Channel        []ScheduleRuleChannel  `json:"channel"`
	AppointmentIDS []uuid.UUID            `json:"app"`
	SlotType       AppointmentType        `json:"type"`
	InCache        bool                   `json:"-"`
}
