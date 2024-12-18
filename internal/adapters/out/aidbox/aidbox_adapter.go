package aidbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	nurl "net/url"
	"time"

	"github.com/google/uuid"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/config"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
)

type AidboxAdapter struct {
	client   *http.Client
	config   *config.Config
	baseURL  string
	username string
	password string
	logger   out.LoggerPort
}

func NewAidboxAdapter(cfg *config.Config, logger out.LoggerPort) *AidboxAdapter {
	return &AidboxAdapter{
		client:   &http.Client{Timeout: 10 * time.Second},
		baseURL:  cfg.Aidbox.URL,
		username: cfg.Aidbox.Username,
		password: cfg.Aidbox.Password,
		logger:   logger,
	}
}

func (a *AidboxAdapter) GetScheduleRuleGlobal(ctx context.Context) (*domain.ScheduleRuleGlobal, error) {
	a.logger.Info("aidbox.schedule_rule_global.fetch", out.LogFields{})

	url := fmt.Sprintf("%s/ScheduleRuleGlobal", a.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		a.logger.Error("aidbox.schedule_rule_global.fetch_failed", out.LogFields{
			"error": err.Error(),
		})
		return nil, err
	}

	req.URL.Query().Add("count", "1")
	req.SetBasicAuth(a.username, a.password)

	resp, err := a.client.Do(req)
	if err != nil {
		a.logger.Error("aidbox.schedule_rule_global.fetch_failed", out.LogFields{
			"error": err.Error(),
		})
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		a.logger.Error("aidbox.schedule_rule_global.fetch_failed", out.LogFields{
			"status": resp.StatusCode,
		})
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var bundleResponse out.AidboxBundleResponse
	if err := json.NewDecoder(resp.Body).Decode(&bundleResponse); err != nil {
		a.logger.Error("aidbox.schedule_rule_global.decode_response_failed", out.LogFields{
			"error": err.Error(),
		})
		return nil, err
	}

	if len(bundleResponse.Entry) == 0 {
		a.logger.Error("aidbox.schedule_rule_global.no_entry", out.LogFields{})
		return nil, nil
	}

	var scheduleGlobal domain.ScheduleRuleGlobal
	if err := json.Unmarshal(bundleResponse.Entry[0].Resource, &scheduleGlobal); err != nil {
		a.logger.Error("aidbox.schedule_rule_global.decode_resource_failed", out.LogFields{
			"error": err.Error(),
		})
		return nil, err
	}

	a.logger.Debug("aidbox.schedule_rule_global.fetch_success", out.LogFields{
		"id": scheduleGlobal.ID,
	})

	return &scheduleGlobal, nil
}

func (a *AidboxAdapter) GetScheduleRule(ctx context.Context, scheduleID uuid.UUID) (*domain.ScheduleRule, error) {
	a.logger.Info("aidbox.schedule_rule.fetch", out.LogFields{
		"scheduleId": scheduleID,
	})

	url := fmt.Sprintf("%s/ScheduleRule/%s", a.baseURL, scheduleID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		a.logger.Error("aidbox.schedule_rule.fetch_failed", out.LogFields{
			"scheduleId": scheduleID,
			"error":      err.Error(),
		})
		return nil, err
	}

	req.SetBasicAuth(a.username, a.password)

	resp, err := a.client.Do(req)
	if err != nil {
		a.logger.Error("aidbox.schedule_rule.fetch_failed", out.LogFields{
			"scheduleId": scheduleID,
			"error":      err.Error(),
		})
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		a.logger.Error("aidbox.schedule_rule.fetch_failed", out.LogFields{
			"scheduleId": scheduleID,
			"status":     resp.StatusCode,
		})
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var schedule domain.ScheduleRule
	if err := json.NewDecoder(resp.Body).Decode(&schedule); err != nil {
		a.logger.Error("aidbox.schedule_rule.decode_failed", out.LogFields{
			"scheduleId": scheduleID,
			"error":      err.Error(),
		})
		return nil, err
	}

	a.logger.Debug("aidbox.schedule_rule.fetch_success", out.LogFields{
		"scheduleId": scheduleID,
		"startDate":  schedule.PlanningHorizon.Start,
		"endDate":    schedule.PlanningHorizon.End,
	})

	return &schedule, nil
}

// Получение нескольких расписаний по массиву ID
func (a *AidboxAdapter) GetScheduleRules(ctx context.Context, scheduleRuleIDs []uuid.UUID) (map[uuid.UUID]*domain.ScheduleRule, error) {
	schedules := make(map[uuid.UUID]*domain.ScheduleRule)

	for _, id := range scheduleRuleIDs {
		schedule, err := a.GetScheduleRule(ctx, id)
		if err != nil {
			return nil, err
		}
		schedules[id] = schedule
	}

	return schedules, nil
}

// Получение записи на прием по ID
func (a *AidboxAdapter) GetAppointmentByID(ctx context.Context, appointmentID uuid.UUID) (*domain.Appointment, error) {
	url := fmt.Sprintf("%s/Appointment/%s", a.baseURL, appointmentID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(a.username, a.password)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var appointment domain.Appointment
	if err := json.NewDecoder(resp.Body).Decode(&appointment); err != nil {
		return nil, err
	}

	return &appointment, nil
}

// Получение нескольких расписаний по массиву ID
func (a *AidboxAdapter) GetScheduleRuleAppointments(ctx context.Context, scheduleRuleID uuid.UUID, startDate, endDate time.Time) ([]domain.Appointment, error) {
	a.logger.Info("aidbox.schedule_rule_appointments.fetch", out.LogFields{})

	url := fmt.Sprintf("%s/ScheduleRule/%s/$appointments", a.baseURL, scheduleRuleID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		a.logger.Error("aidbox.schedule_rule_appointments.fetch_failed", out.LogFields{
			"error": err.Error(),
		})
		return nil, err
	}

	query := nurl.Values{}
	query.Add("begin", startDate.Format("2006-01-02T15:04:05"))
	query.Add("end", endDate.Format("2006-01-02T15:04:05"))
	query.Add("minify", "true")
	req.URL.RawQuery = query.Encode()

	req.SetBasicAuth(a.username, a.password)

	resp, err := a.client.Do(req)
	if err != nil {
		a.logger.Error("aidbox.schedule_rule_appointments.fetch_failed", out.LogFields{
			"error": err.Error(),
		})
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		a.logger.Error("aidbox.schedule_rule_appointments.fetch_failed", out.LogFields{
			"status": resp.StatusCode,
		})
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var bundleResponse out.AidboxBundleResponse
	if err := json.NewDecoder(resp.Body).Decode(&bundleResponse); err != nil {
		a.logger.Error("aidbox.schedule_rule_appointments.decode_response_failed", out.LogFields{
			"error": err.Error(),
		})
		return nil, err
	}

	if len(bundleResponse.Entry) == 0 {
		a.logger.Info("aidbox.schedule_rule_appointments.no_entry", out.LogFields{})
		return nil, nil
	}

	var appointments []domain.Appointment

	for _, entry := range bundleResponse.Entry {
		var appointment domain.Appointment
		if err := json.Unmarshal(entry.Resource, &appointment); err != nil {
			a.logger.Error("aidbox.schedule_rule_appointments.decode_resource_failed", out.LogFields{
				"error": err.Error(),
			})
			return nil, err
		}
		appointments = append(appointments, appointment)
	}

	a.logger.Debug("aidbox.schedule_rule_appointments.fetch_success", out.LogFields{
		"count": len(appointments),
	})

	return appointments, nil
}
