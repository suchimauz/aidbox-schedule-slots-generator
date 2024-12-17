package domain

import (
	"time"

	"github.com/google/uuid"
)

type SlotStatus string

const (
	SlotStatusFree     SlotStatus = "free"
	SlotStatusOccupied SlotStatus = "occupied"
)

type Slot struct {
	StartTime      time.Time              `json:"begin"`
	EndTime        time.Time              `json:"end"`
	Week           ScheduleRuleDaysOfWeek `json:"week"`
	Channel        []ScheduleRuleChannel  `json:"channel"`
	AppointmentIDS []uuid.UUID            `json:"app"`
}
