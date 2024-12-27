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
	Slots   map[domain.AppointmentType]map[time.Time]domain.Slot
	EndDate time.Time
}

type slotsCache struct {
	mu    sync.RWMutex
	cache *lru.Cache[string, *slotsCacheEntry]
}

// Кэширование слотов

func (c *CacheAdapter) GetSlots(ctx context.Context, scheduleID string, startDate time.Time, endDate time.Time, slotType domain.AppointmentType) ([]domain.Slot, time.Time, bool) {
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
					// Слоты в прошлом не нужны
					if slot.StartTime.Before(time.Now()) {
						continue
					}

					startOverlapping := endDate.After(slot.StartTime)
					endOverlapping := startDate.Before(slot.EndTime)
					slot.InCache = true
					if startOverlapping && endOverlapping {
						slots = append(slots, slot)
					}
				}
			}

			if slotType == domain.AppointmentTypeWalkin {
				for _, slot := range slotsMap {
					// Слоты в прошлом не нужны

					slotStartWithoutTime := time.Date(slot.StartTime.Year(), slot.StartTime.Month(), slot.StartTime.Day(), 0, 0, 0, 0, slot.StartTime.Location())
					startDateWithoutTime := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, startDate.Location())

					if slotStartWithoutTime.Before(startDateWithoutTime) {
						c.logger.Info("slots.get.walkin.slot", out.LogFields{
							"slot":      slot,
							"startDate": startDate.Truncate(24 * time.Hour),
							"endDate":   endDate,
						})
						continue
					}

					if slot.StartTime.Before(endDate) {
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

func (c *CacheAdapter) GetSlotByAppointment(ctx context.Context, scheduleID string, appointment domain.Appointment) (domain.Slot, bool) {
	c.slotsCache.mu.RLock()
	defer c.slotsCache.mu.RUnlock()

	entry, cacheExists := c.slotsCache.cache.Get(scheduleID)
	if cacheExists {
		slotsMap, slotsMapExists := entry.Slots[appointment.Type]
		if slotsMapExists {
			for _, slot := range slotsMap {
				startOverlapping := appointment.EndDate.Date.After(slot.StartTime)
				endOverlapping := appointment.StartDate.Date.Before(slot.EndTime)
				slot.InCache = true
				if startOverlapping && endOverlapping {
					return slot, true
				}
			}
		}
	}

	return domain.Slot{}, false
}

func (c *CacheAdapter) StoreSlots(ctx context.Context, scheduleID string, planningEndTime time.Time, slots []domain.Slot) {
	c.slotsCache.mu.Lock()
	defer c.slotsCache.mu.Unlock()

	c.logger.Debug("cache.slots.store", out.LogFields{
		"scheduleId": scheduleID,
		"slotsCount": len(slots),
	})

	slotsMap := make(map[domain.AppointmentType]map[time.Time]domain.Slot)
	for _, slot := range slots {
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
func (c *CacheAdapter) UpdateSlot(ctx context.Context, scheduleID string, slot domain.Slot) {
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
	c.slotsCache.mu.Lock()
	defer c.slotsCache.mu.Unlock()

	c.slotsCache.cache.Remove(scheduleID)
}

func (c *CacheAdapter) InvalidateAllSlotsCache(ctx context.Context) {
	c.slotsCache.mu.Lock()
	defer c.slotsCache.mu.Unlock()

	c.slotsCache.cache.Purge()
}
