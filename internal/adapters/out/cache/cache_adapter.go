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

	return &CacheAdapter{
		slotsCache:              slotsCache,
		scheduleRuleGlobalCache: scheduleRuleGlobalCache,
		scheduleRuleCache:       scheduleRuleCache,
		logger:                  logger.WithModule("CacheAdapter"),
	}, nil
}
