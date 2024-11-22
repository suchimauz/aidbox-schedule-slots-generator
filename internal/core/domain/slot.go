package domain

import (
	"time"
)

type SlotStatus string

const (
	SlotStatusFree     SlotStatus = "free"
	SlotStatusOccupied SlotStatus = "occupied"
)

type Slot struct {
	ScheduleRule ScheduleRule
	StartTime    time.Time
	EndTime      time.Time
	Appointment  Appointment
	Status       SlotStatus
}
