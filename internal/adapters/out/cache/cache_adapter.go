package cache

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/config"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
)

type SlotsCacheEntry struct {
	Slots     []domain.Slot
	StartDate time.Time
	EndDate   time.Time
}

type slotsCache struct {
	cache *lru.Cache[uuid.UUID, *SlotsCacheEntry]
}

type scheduleRuleGlobalCache struct {
	cache     *domain.ScheduleRuleGlobal
	timestamp time.Time
	ttl       time.Duration
	ticker    *time.Ticker
}

type CacheAdapter struct {
	slotsCache              *slotsCache
	scheduleRuleGlobalCache *scheduleRuleGlobalCache
	mu                      sync.RWMutex
	logger                  out.LoggerPort
}

func NewCacheAdapter(cfg *config.Config, logger out.LoggerPort) (*CacheAdapter, error) {
	if !cfg.Cache.Enabled {
		logger.Info("cache.disabled", out.LogFields{
			"message": "Cache is disabled",
		})
		return nil, nil
	}

	lruSlotsCache, err := lru.New[uuid.UUID, *SlotsCacheEntry](cfg.Cache.SlotsSize)
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

	return &CacheAdapter{
		slotsCache:              slotsCache,
		scheduleRuleGlobalCache: scheduleRuleGlobalCache,
		logger:                  logger.WithModule("CacheAdapter"),
	}, nil
}

func (c *CacheAdapter) GetSlots(ctx context.Context, scheduleID uuid.UUID, startDate, endDate time.Time) ([]domain.Slot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.slotsCache.cache.Get(scheduleID)
	if !exists {
		c.logger.Debug("cache.get.miss", out.LogFields{
			"scheduleId": scheduleID,
		})
		return nil, false
	}

	if startDate.Before(entry.StartDate) || endDate.After(entry.EndDate) {
		c.logger.Debug("cache.slots.get.date_range_mismatch", out.LogFields{
			"scheduleId":     scheduleID,
			"requestedStart": startDate,
			"requestedEnd":   endDate,
			"cachedStart":    entry.StartDate,
			"cachedEnd":      entry.EndDate,
		})
		return nil, false
	}

	c.logger.Debug("cache.slots.get.hit", out.LogFields{
		"scheduleId": scheduleID,
		"slotsCount": len(entry.Slots),
	})
	return entry.Slots, true
}

func (c *CacheAdapter) StoreSlots(ctx context.Context, scheduleID uuid.UUID, slots []domain.Slot) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger.Debug("cache.slots.store", out.LogFields{
		"scheduleId": scheduleID,
		"slotsCount": len(slots),
	})

	if len(slots) == 0 {
		return
	}

	// Находим минимальную и максимальную даты
	startDate := slots[0].StartTime
	endDate := slots[0].EndTime
	for _, slot := range slots {
		if slot.StartTime.Before(startDate) {
			startDate = slot.StartTime
		}
		if slot.EndTime.After(endDate) {
			endDate = slot.EndTime
		}
	}

	// Создаем новую запись в кэше
	newEntry := &SlotsCacheEntry{
		Slots:     slots,
		StartDate: startDate,
		EndDate:   endDate,
	}

	c.slotsCache.cache.Add(scheduleID, newEntry)
}

func (c *CacheAdapter) UpdateSlot(ctx context.Context, scheduleID uuid.UUID, slot domain.Slot) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.slotsCache.cache.Get(scheduleID)
	if !exists {
		return
	}

	// Находим индекс слота в записи кэша по времени
	index := -1
	for i, s := range entry.Slots {
		if s.StartTime.Equal(slot.StartTime) && s.EndTime.Equal(slot.EndTime) {
			index = i
			break
		}
	}

	if index != -1 {
		// Обновляем слот
		entry.Slots[index] = slot
	}

	// Обновляем запись в кэше
	c.slotsCache.cache.Add(scheduleID, entry)
}

func (c *CacheAdapter) InvalidateSlotsCache(ctx context.Context, scheduleID uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.slotsCache.cache.Remove(scheduleID)
}

// Кэширование производственного календаря

func (c *CacheAdapter) GetScheduleRuleGlobal(ctx context.Context) (*domain.ScheduleRuleGlobal, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.scheduleRuleGlobalCache.cache == nil || time.Since(c.scheduleRuleGlobalCache.timestamp) > c.scheduleRuleGlobalCache.ttl {
		return nil, false
	}

	return c.scheduleRuleGlobalCache.cache, true
}

func (c *CacheAdapter) StoreScheduleRuleGlobal(ctx context.Context, scheduleRuleGlobal domain.ScheduleRuleGlobal) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.scheduleRuleGlobalCache.cache = &scheduleRuleGlobal
	c.scheduleRuleGlobalCache.timestamp = time.Now()
}

func (c *CacheAdapter) InvalidateScheduleRuleGlobalCache(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.scheduleRuleGlobalCache.cache = nil
	c.scheduleRuleGlobalCache.timestamp = time.Time{}
}
