package cache

import (
	"context"
	"sync"
	"time"

	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
)

type scheduleRuleGlobalCache struct {
	mu        sync.RWMutex
	cache     *domain.ScheduleRuleGlobal
	timestamp time.Time
	ttl       time.Duration
	ticker    *time.Ticker
}

// Кэширование производственного календаря

func (c *CacheAdapter) GetScheduleRuleGlobal(ctx context.Context) (*domain.ScheduleRuleGlobal, bool) {
	c.scheduleRuleGlobalCache.mu.RLock()
	defer c.scheduleRuleGlobalCache.mu.RUnlock()

	if c.scheduleRuleGlobalCache.cache == nil || time.Since(c.scheduleRuleGlobalCache.timestamp) > c.scheduleRuleGlobalCache.ttl {
		return nil, false
	}

	return c.scheduleRuleGlobalCache.cache, true
}

func (c *CacheAdapter) StoreScheduleRuleGlobal(ctx context.Context, scheduleRuleGlobal domain.ScheduleRuleGlobal) {
	c.scheduleRuleGlobalCache.mu.Lock()
	defer c.scheduleRuleGlobalCache.mu.Unlock()

	c.scheduleRuleGlobalCache.cache = &scheduleRuleGlobal
	c.scheduleRuleGlobalCache.timestamp = time.Now()
}

func (c *CacheAdapter) InvalidateScheduleRuleGlobalCache(ctx context.Context) {
	c.scheduleRuleGlobalCache.mu.Lock()
	defer c.scheduleRuleGlobalCache.mu.Unlock()

	c.scheduleRuleGlobalCache.cache = nil
	c.scheduleRuleGlobalCache.timestamp = time.Time{}
}
