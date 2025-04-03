package cache

import (
	"context"
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
)

type healthcareServiceCache struct {
	mu    sync.RWMutex
	cache *lru.Cache[string, *domain.HealthcareService]
}

// Кэширование услуг

func (c *CacheAdapter) GetHealthcareService(ctx context.Context, healthcareServiceID string) (*domain.HealthcareService, bool) {
	if !c.cfg.Cache.Enabled {
		c.logger.Debug("cache.healthcare_service.get_healthcare_service.disabled", out.LogFields{
			"healthcareServiceId": healthcareServiceID,
		})
		return nil, false
	}

	c.healthcareServiceCache.mu.RLock()
	defer c.healthcareServiceCache.mu.RUnlock()

	entry, exists := c.healthcareServiceCache.cache.Get(healthcareServiceID)
	if !exists {
		c.logger.Debug("cache.get.miss", out.LogFields{
			"healthcareServiceId": healthcareServiceID,
		})
		return nil, false
	}

	return entry, true
}

func (c *CacheAdapter) StoreHealthcareService(ctx context.Context, healthcareService domain.HealthcareService) {
	if !c.cfg.Cache.Enabled {
		c.logger.Debug("cache.healthcare_service.store_healthcare_service.disabled", out.LogFields{
			"healthcareServiceId": healthcareService.ID,
		})
		return
	}

	c.healthcareServiceCache.mu.Lock()
	defer c.healthcareServiceCache.mu.Unlock()

	c.healthcareServiceCache.cache.Add(healthcareService.ID, &healthcareService)
}

func (c *CacheAdapter) InvalidateHealthcareServiceCache(ctx context.Context, healthcareServiceID string) {
	if !c.cfg.Cache.Enabled {
		c.logger.Debug("cache.healthcare_service.invalidate_healthcare_service.disabled", out.LogFields{
			"healthcareServiceId": healthcareServiceID,
		})
		return
	}

	c.healthcareServiceCache.mu.Lock()
	defer c.healthcareServiceCache.mu.Unlock()

	c.healthcareServiceCache.cache.Remove(healthcareServiceID)
}

func (c *CacheAdapter) InvalidateAllHealthcareServiceCache(ctx context.Context) {
	if !c.cfg.Cache.Enabled {
		c.logger.Debug("cache.healthcare_service.invalidate_all_healthcare_service.disabled", out.LogFields{})
		return
	}

	c.healthcareServiceCache.mu.Lock()
	defer c.healthcareServiceCache.mu.Unlock()

	c.healthcareServiceCache.cache.Purge()
}
