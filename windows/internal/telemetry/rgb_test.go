package telemetry

import (
	"testing"
	"time"

	"satellite/internal/config"
	"satellite/internal/icue"
	"satellite/internal/privacy"
)

func TestRGBCollectionAndPrivacy(t *testing.T) {
	if config.DefaultConfig().Expose.ICUERGB {
		t.Fatal("RGB must be off by default")
	}
	t.Cleanup(privacy.Default.Resume)
	c := NewCollector()
	var enabled bool
	c.readRGB = func(on bool) *icue.State {
		enabled = on
		if !on {
			return nil
		}
		return &icue.State{Status: "connected", Devices: []icue.Device{{ID: "keyboard", LEDs: []icue.LED{{ID: 1, R: 255}}}}}
	}
	if snap := c.Collect(config.ExposeConfig{}); snap.RGB != nil || enabled {
		t.Fatal("RGB collected while disabled")
	}
	e := config.ExposeConfig{ICUERGB: true}
	snap := c.Collect(e)
	if snap.RGB == nil || !enabled {
		t.Fatal("RGB missing while enabled")
	}
	privacy.Default.Pause(time.Minute)
	if c.Last().RGB != nil || snap.ApplyPrivacy().RGB != nil {
		t.Fatal("cached RGB exposed during pause")
	}
	if paused := c.Collect(e); paused.RGB != nil || enabled {
		t.Fatal("RGB read during pause")
	}
	privacy.Default.Resume()
	if c.Last().RGB != nil {
		t.Fatal("old RGB returned after resume")
	}
	if c.Collect(e).RGB == nil {
		t.Fatal("RGB did not resume")
	}
}

func TestRGBChangesTriggerPublication(t *testing.T) {
	a := Snapshot{RGB: &icue.State{Status: "connected", Devices: []icue.Device{{LEDs: []icue.LED{{ID: 1, R: 1}}}}}}
	b := Snapshot{RGB: &icue.State{Status: "connected", Devices: []icue.Device{{LEDs: []icue.LED{{ID: 1, R: 2}}}}}}
	if !HasSignificantDelta(a, b) || !HasSignificantDelta(a, Snapshot{}) || !HasSignificantDelta(Snapshot{}, a) {
		t.Fatal("RGB changes were ignored")
	}
	if HasSignificantDelta(a, a) {
		t.Fatal("unchanged RGB triggered publication")
	}
}
