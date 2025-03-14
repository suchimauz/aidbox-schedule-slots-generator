package domain

import (
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/json_types"
)

type AppointmentStatus string

const (
	AppointmentStatusPending   AppointmentStatus = "pending"
	AppointmentStatusBooked    AppointmentStatus = "booked"
	AppointmentStatusFulfilled AppointmentStatus = "fulfilled"
	AppointmentStatusArrived   AppointmentStatus = "arrived"
	AppointmentStatusNoshow    AppointmentStatus = "noshow"
)

type AppointmentType string

const (
	AppointmentTypeRoutine AppointmentType = "ROUTINE"
	AppointmentTypeWalkin  AppointmentType = "WALKIN"
)

type Appointment struct {
	ID        string                     `json:"id"`
	StartDate json_types.DateTimeOrEmpty `json:"start"`
	EndDate   json_types.DateTimeOrEmpty `json:"end"`
	Status    AppointmentStatus          `json:"status"`
	Type      AppointmentType            `json:"type"`
}
