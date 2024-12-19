package slot_generator_service

import (
	"sync"

	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
)

type SlotGeneratorServiceDebug struct {
	mu   sync.Mutex
	data []domain.DebugInfo
}

func (d *SlotGeneratorServiceDebug) AddDebugInfo(info domain.DebugInfo) {
	d.mu.Lock()
	d.data = append(d.data, info)
	d.mu.Unlock()
}
