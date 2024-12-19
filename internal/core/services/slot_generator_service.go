package services

import (
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/ports/out"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/services/slot_generator_service"
)

func NewSlotGeneratorService(
	aidboxPort out.AidboxPort,
	cachePort out.CachePort,
	logger out.LoggerPort,
) *slot_generator_service.SlotGeneratorService {
	return slot_generator_service.NewSlotGeneratorService(aidboxPort, cachePort, logger)
}
