package domain

import (
	"time"

	"github.com/google/uuid"
)

type ScheduleRule struct {
	ID           uuid.UUID
	DoctorID     uuid.UUID
	StartDate    time.Time
	EndDate      time.Time
	SlotDuration time.Duration
}
