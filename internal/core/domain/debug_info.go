package domain

import "time"

type DebugInfo struct {
	Event     string            `json:"event"`
	Timing    int64             `json:"timing"`
	StartTime time.Time         `json:"-"`
	Options   map[string]string `json:"options,omitempty"`
}

func (d *DebugInfo) Start() {
	d.StartTime = time.Now()
}

func (d *DebugInfo) Elapse() {
	d.Timing = time.Since(d.StartTime).Milliseconds()
}

func (d *DebugInfo) AddOption(key string, value string) {
	if d.Options == nil {
		d.Options = make(map[string]string)
	}
	d.Options[key] = value
}
