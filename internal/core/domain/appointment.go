package domain

import (
	"time"

	"github.com/google/uuid"
)

type AppointmentStatus string

const (
	AppointmentStatusActive AppointmentStatus = "active"
)

type Appointment struct {
	ID             uuid.UUID
	ScheduleRuleID uuid.UUID
	Status         AppointmentStatus
	StartTime      time.Time
	EndTime        time.Time
}
