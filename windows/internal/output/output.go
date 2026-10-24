// Package output sends telemetry snapshots to their destinations.
//
// Each destination (MQTT for Home Assistant, a generic HTTP webhook, ...) is a
// Sink. The main loop collects one snapshot per tick and hands it to every
// sink, so adding a new destination means adding one Sink implementation.
package output

import (
	"satellite/internal/telemetry"
)

// Sink receives telemetry snapshots. Publish must not block the caller for
// long, because every sink shares the same collection tick.
type Sink interface {
	Name() string
	Publish(snap telemetry.Snapshot) error
}

// SinkFunc adapts a plain publish function into a Sink.
type SinkFunc struct {
	SinkName string
	Fn       func(telemetry.Snapshot) error
}

func (f SinkFunc) Name() string                          { return f.SinkName }
func (f SinkFunc) Publish(snap telemetry.Snapshot) error { return f.Fn(snap) }

// PublishAll sends the snapshot to every sink. A failing sink does not stop
// the others.
func PublishAll(sinks []Sink, snap telemetry.Snapshot) {
	for _, s := range sinks {
		_ = s.Publish(snap)
	}
}
