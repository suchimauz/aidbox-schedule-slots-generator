package cache

import (
	"context"
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
)

type scheduleRuleCache struct {
	mu    sync.RWMutex
	cache *lru.Cache[string, *domain.ScheduleRule]
}

// Кэширование расписаний

func (c *CacheAdapter) GetScheduleRule(ctx context.Context, scheduleID string) (*domain.ScheduleRule, bool) {
	c.scheduleRuleCache.mu.RLock()
	defer c.scheduleRuleCache.mu.RUnlock()

	entry, exists := c.scheduleRuleCache.cache.Get(scheduleID)
	if !exists {
		c.logger.Debug("cache.get.miss", out.LogFields{
			"scheduleId": scheduleID,
		})
		return nil, false
	}

	return entry, true
}

func (c *CacheAdapter) StoreScheduleRule(ctx context.Context, scheduleRule domain.ScheduleRule) {
	c.scheduleRuleCache.mu.Lock()
	defer c.scheduleRuleCache.mu.Unlock()

	c.scheduleRuleCache.cache.Add(scheduleRule.ID, &scheduleRule)
}

func (c *CacheAdapter) InvalidateScheduleRuleCache(ctx context.Context, scheduleID string) {
	c.scheduleRuleCache.mu.Lock()
	defer c.scheduleRuleCache.mu.Unlock()

	c.scheduleRuleCache.cache.Remove(scheduleID)
}

func (c *CacheAdapter) InvalidateAllScheduleRuleCache(ctx context.Context) {
	c.scheduleRuleCache.mu.Lock()
	defer c.scheduleRuleCache.mu.Unlock()

	c.scheduleRuleCache.cache.Purge()
}
