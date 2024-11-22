package out

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
)

type AidboxPort interface {
	// Методы для работы с расписанием
	GetSchedule(ctx context.Context, scheduleID uuid.UUID) (*domain.Schedule, error)
	GetSchedules(ctx context.Context, scheduleIDs []uuid.UUID) (map[uuid.UUID]*domain.Schedule, error)

	// Методы для работы с записями на прием
	GetAppointments(ctx context.Context, doctorID uuid.UUID, startDate, endDate time.Time) ([]domain.Appointment, error)
	GetAppointmentByID(ctx context.Context, appointmentID uuid.UUID) (*domain.Appointment, error)
}
