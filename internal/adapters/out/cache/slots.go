package cache

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
)

type slotsCacheEntry struct {
	Slots   map[domain.AppointmentType]map[time.Time]domain.Slot
	EndDate time.Time
}

type slotsCache struct {
	mu    sync.RWMutex
	cache *lru.Cache[uuid.UUID, *slotsCacheEntry]
}

// Кэширование слотов

func (c *CacheAdapter) GetSlots(ctx context.Context, scheduleID uuid.UUID, startDate time.Time, endDate time.Time, slotType domain.AppointmentType) ([]domain.Slot, time.Time, bool) {
	c.slotsCache.mu.RLock()
	defer c.slotsCache.mu.RUnlock()

	entry, cacheExists := c.slotsCache.cache.Get(scheduleID)
	if cacheExists {
		slotsMap, slotsMapExists := entry.Slots[slotType]
		if slotsMapExists {
			var slots []domain.Slot

			// Для слотов со временем должно быть пересечение с диапазоном
			if slotType == domain.AppointmentTypeRoutine {
				for _, slot := range slotsMap {
					startOverlapping := endDate.After(slot.StartTime)
					endOverlapping := startDate.Before(slot.EndTime)
					slot.InCache = true
					if startOverlapping && endOverlapping {
						slots = append(slots, slot)
					}
				}
			}
			// Для слотов без времени только дата начала должна быть равна дате начала
			if slotType == domain.AppointmentTypeWalkin {
				for _, slot := range slotsMap {
					if slot.StartTime.Equal(startDate) {
						slot.InCache = true
						slots = append(slots, slot)
					}
				}
			}
			return slots, entry.EndDate, true
		}
	}

	return []domain.Slot{}, time.Time{}, false
}

func (c *CacheAdapter) StoreSlots(ctx context.Context, scheduleID uuid.UUID, planningEndTime time.Time, slots []domain.Slot) {
	c.slotsCache.mu.Lock()
	defer c.slotsCache.mu.Unlock()

	c.logger.Debug("cache.slots.store", out.LogFields{
		"scheduleId": scheduleID,
		"slotsCount": len(slots),
	})

	slotsMap := make(map[domain.AppointmentType]map[time.Time]domain.Slot)
	for _, slot := range slots {
		if slot.StartTime.Before(time.Now()) {
			continue
		}
		if slotsMap[slot.SlotType] == nil {
			slotsMap[slot.SlotType] = make(map[time.Time]domain.Slot)
		}
		slotsMap[slot.SlotType][slot.StartTime] = slot
	}

	// Создаем новую запись в кэше
	newEntry := &slotsCacheEntry{
		Slots:   slotsMap,
		EndDate: planningEndTime,
	}

	c.slotsCache.cache.Add(scheduleID, newEntry)
}

// Обновление слотов если они уже есть в кэше
func (c *CacheAdapter) UpdateSlot(ctx context.Context, scheduleID uuid.UUID, slot domain.Slot) {
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

func (c *CacheAdapter) InvalidateSlotsCache(ctx context.Context, scheduleID uuid.UUID) {
	c.slotsCache.mu.Lock()
	defer c.slotsCache.mu.Unlock()

	c.slotsCache.cache.Remove(scheduleID)
}
