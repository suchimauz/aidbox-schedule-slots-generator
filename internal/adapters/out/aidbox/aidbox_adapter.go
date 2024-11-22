package aidbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

func (a *AidboxAdapter) GetSchedule(ctx context.Context, scheduleID uuid.UUID) (*domain.Schedule, error) {
	a.logger.Info("aidbox.schedule.fetch", out.LogFields{
		"scheduleId": scheduleID,
	})

	url := fmt.Sprintf("%s/Schedule/%s", a.baseURL, scheduleID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		a.logger.Error("aidbox.schedule.fetch_failed", out.LogFields{
			"scheduleId": scheduleID,
			"error":      err.Error(),
		})
		return nil, err
	}

	req.SetBasicAuth(a.username, a.password)

	resp, err := a.client.Do(req)
	if err != nil {
		a.logger.Error("aidbox.schedule.fetch_failed", out.LogFields{
			"scheduleId": scheduleID,
			"error":      err.Error(),
		})
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		a.logger.Error("aidbox.schedule.fetch_failed", out.LogFields{
			"scheduleId": scheduleID,
			"status":     resp.StatusCode,
		})
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var schedule domain.Schedule
	if err := json.NewDecoder(resp.Body).Decode(&schedule); err != nil {
		a.logger.Error("aidbox.schedule.decode_failed", out.LogFields{
			"scheduleId": scheduleID,
			"error":      err.Error(),
		})
		return nil, err
	}

	a.logger.Debug("aidbox.schedule.fetch_success", out.LogFields{
		"scheduleId": scheduleID,
		"startDate":  schedule.StartDate,
		"endDate":    schedule.EndDate,
	})

	return &schedule, nil
}