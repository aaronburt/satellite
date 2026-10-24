package output

import (
	"errors"
	"testing"

	"satellite/internal/telemetry"
)

func TestPublishAllContinuesPastFailingSinks(t *testing.T) {
	var got []string
	snap := telemetry.Snapshot{Timestamp: 42}
	sink := func(name string, err error) Sink {
		return SinkFunc{SinkName: name, Fn: func(s telemetry.Snapshot) error {
			if s.Timestamp != 42 {
				t.Errorf("%s received the wrong snapshot", name)
			}
			got = append(got, name)
			return err
		}}
	}
	PublishAll([]Sink{sink("mqtt", errors.New("offline")), sink("webhook", nil), sink("third", nil)}, snap)
	if len(got) != 3 || got[0] != "mqtt" || got[1] != "webhook" || got[2] != "third" {
		t.Fatalf("sinks called = %v, want all three in order", got)
	}
	PublishAll(nil, snap) // no sinks is fine
}

func TestSinkFuncName(t *testing.T) {
	if (SinkFunc{SinkName: "mqtt"}).Name() != "mqtt" {
		t.Fatal("Name() should return SinkName")
	}
}
