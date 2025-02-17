package out

import (
	"context"
	"encoding/json"
	"time"

	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
)

type AidboxPort interface {
	// Методы для работы с расписанием
	GetScheduleRule(ctx context.Context, scheduleRuleID string) (*domain.ScheduleRule, error)
	GetScheduleRules(ctx context.Context, scheduleRuleIDs []string) (map[string]*domain.ScheduleRule, error)

	// Методы для работы с записями на прием
	GetScheduleRuleAppointments(ctx context.Context, scheduleRuleID string, startDate, endDate time.Time) ([]domain.Appointment, error)
	GetAppointmentByID(ctx context.Context, appointmentID string) (*domain.Appointment, error)

	// Методы для работы с производственным календарем
	GetScheduleRuleGlobal(ctx context.Context) (*domain.ScheduleRuleGlobal, error)

	// Методы для работы с услугами
	GetHealthcareServiceByID(ctx context.Context, healthcareServiceID string) (*domain.HealthcareService, error)
}

type AidboxBundleEntryItem struct {
	Resource json.RawMessage `json:"resource"`
}

type AidboxBundleResponse struct {
	Entry []AidboxBundleEntryItem `json:"entry"`
}
