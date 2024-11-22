package out

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
)

type AidboxPort interface {
	// Методы для работы с расписанием
	GetScheduleRule(ctx context.Context, scheduleRuleID uuid.UUID) (*domain.ScheduleRule, error)
	GetScheduleRules(ctx context.Context, scheduleRuleIDs []uuid.UUID) (map[uuid.UUID]*domain.ScheduleRule, error)

	// Методы для работы с записями на прием
	GetScheduleRuleAppointments(ctx context.Context, scheduleRuleID uuid.UUID, startDate, endDate time.Time) ([]domain.Appointment, error)
	GetAppointmentByID(ctx context.Context, appointmentID uuid.UUID) (*domain.Appointment, error)
}
