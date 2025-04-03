package cache

import (
	"context"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
)

type slotsCacheEntry struct {
	Slots     map[domain.AppointmentType]map[time.Time]domain.Slot
	StartDate time.Time
	EndDate   time.Time
}

type slotsCache struct {
	mu    sync.RWMutex
	cache *lru.Cache[string, *slotsCacheEntry]
}

// Кэширование слотов

func (c *CacheAdapter) GetSlotsCachedMeta(ctx context.Context, scheduleID string) (time.Time, time.Time, bool) {
	if !c.cfg.Cache.Enabled {
		c.logger.Debug("cache.slots.get_slots_cached_meta.disabled", out.LogFields{
			"scheduleId": scheduleID,
		})
		return time.Time{}, time.Time{}, false
	}

	c.slotsCache.mu.RLock()
	defer c.slotsCache.mu.RUnlock()

	entry, cacheExists := c.slotsCache.cache.Get(scheduleID)
	if cacheExists {
		return entry.StartDate, entry.EndDate, true
	}

	return time.Time{}, time.Time{}, false
}

func (c *CacheAdapter) GetSlots(ctx context.Context, scheduleID string, startDate time.Time, endDate time.Time, slotType domain.AppointmentType) ([]domain.Slot, bool) {
	if !c.cfg.Cache.Enabled {
		c.logger.Debug("cache.slots.get_slots.disabled", out.LogFields{
			"scheduleId": scheduleID,
		})
		return []domain.Slot{}, false
	}

	c.slotsCache.mu.RLock()
	defer c.slotsCache.mu.RUnlock()

	entry, cacheExists := c.slotsCache.cache.Get(scheduleID)
	if cacheExists {
		slotsMap, slotsMapExists := entry.Slots[slotType]
		if slotsMapExists {
			var slots []domain.Slot

			// Для слотов должно быть пересечение с диапазоном
			for _, slot := range slotsMap {
				startOverlapping := endDate.After(slot.StartTime)
				endOverlapping := startDate.Before(slot.EndTime)
				slot.InCache = true
				if startOverlapping && endOverlapping {
					slots = append(slots, slot)
				}
			}
			return slots, true
		}
	}

	return []domain.Slot{}, false
}

func (c *CacheAdapter) GetSlotByAppointment(ctx context.Context, scheduleID string, appointment domain.Appointment) (domain.Slot, bool) {
	if !c.cfg.Cache.Enabled {
		c.logger.Debug("cache.slots.get_slot_by_appointment.disabled", out.LogFields{
			"scheduleId": scheduleID,
		})
		return domain.Slot{}, false
	}

	c.slotsCache.mu.RLock()
	defer c.slotsCache.mu.RUnlock()

	entry, cacheExists := c.slotsCache.cache.Get(scheduleID)
	if cacheExists {
		slotsMap, slotsMapExists := entry.Slots[appointment.Type]
		if slotsMapExists {
			for _, slot := range slotsMap {
				if slot.SlotType == domain.AppointmentTypeRoutine {
					startOverlapping := appointment.EndDate.Date.After(slot.StartTime)
					endOverlapping := appointment.StartDate.Date.Before(slot.EndTime)
					slot.InCache = true
					if startOverlapping && endOverlapping {
						return slot, true
					}
				} else if slot.SlotType == domain.AppointmentTypeWalkin {
					if slot.StartTime.Equal(appointment.StartDate.Date) {
						return slot, true
					}
				}
			}
		}
	}

	return domain.Slot{}, false
}

func (c *CacheAdapter) StoreSlots(ctx context.Context, scheduleID string, startDate time.Time, endDate time.Time, slots []domain.Slot) {
	if !c.cfg.Cache.Enabled {
		c.logger.Debug("cache.slots.store_slots.disabled", out.LogFields{
			"scheduleId": scheduleID,
		})
		return
	}

	c.slotsCache.mu.Lock()
	defer c.slotsCache.mu.Unlock()

	if len(slots) == 0 {
		return
	}

	c.logger.Debug("cache.slots.store", out.LogFields{
		"scheduleId": scheduleID,
		"slotsCount": len(slots),
	})

	entry, exists := c.slotsCache.cache.Get(scheduleID)
	if !exists {
		entry = &slotsCacheEntry{
			Slots: make(map[domain.AppointmentType]map[time.Time]domain.Slot),
		}
	}

	for _, slot := range slots {
		if entry.Slots[slot.SlotType] == nil {
			entry.Slots[slot.SlotType] = make(map[time.Time]domain.Slot)
		}
		if _, exists := entry.Slots[slot.SlotType][slot.StartTime]; !exists {
			entry.Slots[slot.SlotType][slot.StartTime] = slot
		}
	}

	// Обновляем startDate и endDate
	if entry.StartDate.IsZero() || startDate.Before(entry.StartDate) {
		entry.StartDate = startDate
	}
	if entry.EndDate.IsZero() || endDate.After(entry.EndDate) {
		entry.EndDate = endDate
	}

	c.slotsCache.cache.Add(scheduleID, entry)
}

// Обновление слотов если они уже есть в кэше
func (c *CacheAdapter) UpdateSlot(ctx context.Context, scheduleID string, slot domain.Slot) {
	if !c.cfg.Cache.Enabled {
		c.logger.Debug("cache.slots.update_slot.disabled", out.LogFields{
			"scheduleId": scheduleID,
		})
		return
	}

	c.slotsCache.mu.Lock()
	defer c.slotsCache.mu.Unlock()

	entry, exists := c.slotsCache.cache.Get(scheduleID)
	if !exists {
		return
	}

	entry.Slots[slot.SlotType][slot.StartTime] = slot

	// Обновляем запись в кэше
	c.slotsCache.cache.Add(scheduleID, entry)
}

func (c *CacheAdapter) InvalidateSlotsCache(ctx context.Context, scheduleID string) {
	if !c.cfg.Cache.Enabled {
		c.logger.Debug("cache.slots.invalidate_slots.disabled", out.LogFields{
			"scheduleId": scheduleID,
		})
		return
	}

	select {
	case <-ctx.Done():
		c.logger.Debug("cache.invalidate_slots.context_canceled", out.LogFields{
			"scheduleId": scheduleID,
			"error":      ctx.Err().Error(),
		})
		return
	default:
		c.slotsCache.mu.Lock()
		defer c.slotsCache.mu.Unlock()

		c.slotsCache.cache.Remove(scheduleID)
	}
}

func (c *CacheAdapter) InvalidateAllSlotsCache(ctx context.Context) {
	if !c.cfg.Cache.Enabled {
		c.logger.Debug("cache.slots.invalidate_all_slots.disabled", out.LogFields{})
		return
	}

	c.slotsCache.mu.Lock()
	defer c.slotsCache.mu.Unlock()

	c.slotsCache.cache.Purge()
}
