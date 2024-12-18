package domain

import "time"

type DebugInfo struct {
	Event     string `json:"event"`
	Timing    int64  `json:"timing"`
	StartTime time.Time
}

func (d *DebugInfo) Start() {
	d.StartTime = time.Now()
}

func (d *DebugInfo) Elapse() {
	d.Timing = time.Since(d.StartTime).Milliseconds()
}
