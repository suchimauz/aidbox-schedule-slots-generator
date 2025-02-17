package cache

import (
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/config"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
)

type CacheAdapter struct {
	slotsCache              *slotsCache
	scheduleRuleGlobalCache *scheduleRuleGlobalCache
	scheduleRuleCache       *scheduleRuleCache
	healthcareServiceCache  *healthcareServiceCache
	logger                  out.LoggerPort
}

func NewCacheAdapter(cfg *config.Config, logger out.LoggerPort) (*CacheAdapter, error) {
	if !cfg.Cache.Enabled {
		logger.Info("cache.disabled", out.LogFields{
			"message": "Cache is disabled",
		})
		return nil, nil
	}

	lruSlotsCache, err := lru.New[string, *slotsCacheEntry](cfg.Cache.SlotsSize)
	if err != nil {
		logger.Error("cache.slots.init.failed", out.LogFields{
			"error": err.Error(),
			"size":  cfg.Cache.SlotsSize,
		})
		return nil, err
	}

	slotsCache := &slotsCache{
		cache: lruSlotsCache,
	}

	scheduleRuleGlobalCache := &scheduleRuleGlobalCache{
		ttl:    30 * time.Minute,
		ticker: time.NewTicker(time.Minute), // Проверка каждую минуту
	}

	lruScheduleRuleCache, err := lru.New[string, *domain.ScheduleRule](cfg.Cache.ScheduleRuleSize)
	if err != nil {
		logger.Error("cache.schedule_rule.init.failed", out.LogFields{
			"error": err.Error(),
			"size":  cfg.Cache.ScheduleRuleSize,
		})
		return nil, err
	}

	scheduleRuleCache := &scheduleRuleCache{
		cache: lruScheduleRuleCache,
	}

	lruHealthcareServiceCache, err := lru.New[string, *domain.HealthcareService](cfg.Cache.HealthcareServiceSize)
	if err != nil {
		logger.Error("cache.healthcare_service.init.failed", out.LogFields{
			"error": err.Error(),
			"size":  cfg.Cache.HealthcareServiceSize,
		})
		return nil, err
	}

	healthcareServiceCache := &healthcareServiceCache{
		cache: lruHealthcareServiceCache,
	}

	return &CacheAdapter{
		slotsCache:              slotsCache,
		scheduleRuleGlobalCache: scheduleRuleGlobalCache,
		scheduleRuleCache:       scheduleRuleCache,
		healthcareServiceCache:  healthcareServiceCache,
		logger:                  logger.WithModule("CacheAdapter"),
	}, nil
}
