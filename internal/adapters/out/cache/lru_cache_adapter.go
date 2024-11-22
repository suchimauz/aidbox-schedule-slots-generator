package cache

import (
	"context"
	"github.com/google/uuid"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/config"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/out"
	"sync"
	"time"
)

type CacheEntry struct {
	Slots     []domain.Slot
	StartDate time.Time
	EndDate   time.Time
}

type LRUCacheAdapter struct {
	cache  *lru.Cache[uuid.UUID, *CacheEntry]
	mu     sync.RWMutex
	logger out.LoggerPort
}

func NewLRUCacheAdapter(cfg *config.Config, logger out.LoggerPort) (*LRUCacheAdapter, error) {
	cache, err := lru.New[uuid.UUID, *CacheEntry](cfg.Cache.Size)
	if err != nil {
		logger.Error("cache.init.failed", out.LogFields{
			"error": err.Error(),
			"size":  cfg.Cache.Size,
		})
		return nil, err
	}

	return &LRUCacheAdapter{
		cache:  cache,
		logger: logger.WithModule("CacheAdapter"),
	}, nil
}

func (c *LRUCacheAdapter) GetSlots(ctx context.Context, scheduleID uuid.UUID, startDate, endDate time.Time) ([]domain.Slot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.cache.Get(scheduleID)
	if !exists {
		c.logger.Debug("cache.get.miss", out.LogFields{
			"scheduleId": scheduleID,
		})
		return nil, false
	}

	if startDate.Before(entry.StartDate) || endDate.After(entry.EndDate) {
		c.logger.Debug("cache.get.date_range_mismatch", out.LogFields{
			"scheduleId":        scheduleID,
			"requestedStart":    startDate,
			"requestedEnd":      endDate,
			"cachedStart":       entry.StartDate,
			"cachedEnd":         entry.EndDate,
		})
		return nil, false
	}

	c.logger.Debug("cache.get.hit", out.LogFields{
		"scheduleId": scheduleID,
		"slotsCount": len(entry.Slots),
	})
	return entry.Slots, true
}

func (c *LRUCacheAdapter) StoreSlots(ctx context.Context, scheduleID uuid.UUID, slots []domain.Slot) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger.Debug("cache.store", out.LogFields{
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
	newEntry := &CacheEntry{
		Slots:     slots,
		StartDate: startDate,
		EndDate:   endDate,
	}

	c.cache.Add(scheduleID, newEntry)
}

func (c *LRUCacheAdapter) UpdateSlot(ctx context.Context, scheduleID uuid.UUID, slot domain.Slot) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.cache.Get(scheduleID)
	if !exists {
		return
	}

	// Находим индекс слота в записи кэша
	index := -1
	for i, s := range entry.Slots {
		if s.ID == slot.ID {
			index = i
			break
		}
	}

	if index != -1 {
		entry.Slots[index] = slot
	}

	// Находим минимальную и максимальную даты
	startDate := entry.Slots[0].StartTime
	endDate := entry.Slots[0].EndTime
	for _, s := range entry.Slots {
		if s.StartTime.Before(startDate) {
			startDate = s.StartTime
		}
		if s.EndTime.After(endDate) {
			endDate = s.EndTime
		}
	}

	// Обновляем запись в кэше
	newEntry := &CacheEntry{
		Slots:     entry.Slots,
		StartDate: startDate,
		EndDate:   endDate,
	}

	c.cache.Add(scheduleID, newEntry)
}

func (c *LRUCacheAdapter) InvalidateCache(ctx context.Context, scheduleID uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache.Remove(scheduleID)
}