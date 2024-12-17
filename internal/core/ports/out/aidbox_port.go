package out

import (
	"context"
	"encoding/json"
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

	// Методы для работы с производственным календарем
	GetScheduleRuleGlobal(ctx context.Context) (*domain.ScheduleRuleGlobal, error)
}

type AidboxBundleEntryItem struct {
	Resource json.RawMessage `json:"resource"`
}

type AidboxBundleResponse struct {
	Entry []AidboxBundleEntryItem `json:"entry"`
}
