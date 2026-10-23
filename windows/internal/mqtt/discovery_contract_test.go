package mqtt

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"satellite/internal/actions"
	"satellite/internal/capabilities"
	"satellite/internal/config"
	"satellite/internal/telemetry"
)

// Home Assistant only accepts discovery topics of this shape; anything else
// is silently ignored, so the entity never appears.
var haDiscoveryTopic = regexp.MustCompile(`^homeassistant/(sensor|binary_sensor|button|media_player|switch|number)/[A-Za-z0-9_-]+/[A-Za-z0-9_-]+/config$`)

var valueJSONField = regexp.MustCompile(`value_json\.([a-z_]+)`)

func allCaps() capabilities.PlatformCapabilities {
	var caps capabilities.PlatformCapabilities
	v := reflect.ValueOf(&caps).Elem()
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).Kind() == reflect.Bool {
			v.Field(i).SetBool(true)
		}
	}
	return caps
}

func everythingExposed() config.ExposeConfig {
	var e config.ExposeConfig
	v := reflect.ValueOf(&e).Elem()
	for i := 0; i < v.NumField(); i++ {
		v.Field(i).SetBool(true)
	}
	return e
}

func snapshotJSONFields() map[string]bool {
	fields := map[string]bool{}
	t := reflect.TypeOf(telemetry.Snapshot{})
	for i := 0; i < t.NumField(); i++ {
		name := strings.Split(t.Field(i).Tag.Get("json"), ",")[0]
		fields[name] = true
	}
	return fields
}

func stubHardware(t *testing.T, drives []telemetry.DiskInfo, gpus []telemetry.GPUInfo) {
	t.Helper()
	origDrives, origGPUs := listDrives, listGPUs
	t.Cleanup(func() { listDrives, listGPUs = origDrives, origGPUs })
	listDrives = func() []telemetry.DiskInfo { return drives }
	listGPUs = func(...bool) []telemetry.GPUInfo { return gpus }
}

func f64(v float64) *float64 { return &v }

type discovered struct {
	item    DiscoveryItem
	payload map[string]any
}

func discoverAll(t *testing.T, cfg config.Config) []discovered {
	t.Helper()
	var out []discovered
	for _, item := range GetAllDiscoveryItems(cfg, true, allCaps()) {
		d := discovered{item: item}
		if item.ShouldRun {
			if err := json.Unmarshal(item.Payload, &d.payload); err != nil {
				t.Fatalf("%s: invalid JSON payload: %v", item.Key, err)
			}
		}
		out = append(out, d)
	}
	return out
}

func TestDiscoveryPayloadsAreValidForHomeAssistant(t *testing.T) {
	stubHardware(t,
		[]telemetry.DiskInfo{{Mount: "C:"}, {Mount: "/"}, {Mount: "/mnt/My Drive"}},
		[]telemetry.GPUInfo{{Index: 0, Name: "RTX", TemperatureC: f64(50), PowerWatts: f64(100)}, {Index: 1}},
	)
	cfg := config.DefaultConfig()
	cfg.NodeID = "desk.local" // dots are not allowed in discovery topics
	cfg.MQTT.TopicPrefix = "sat"
	cfg.Expose = everythingExposed()

	fields := snapshotJSONFields()
	topics := map[string]string{}
	uniqueIDs := map[string]string{}

	for _, d := range discoverAll(t, cfg) {
		item := d.item
		if !haDiscoveryTopic.MatchString(item.Topic) {
			t.Errorf("%s: topic %q is not a valid Home Assistant discovery topic", item.Key, item.Topic)
		}
		if prev, dup := topics[item.Topic]; dup {
			t.Errorf("%s and %s share topic %q", prev, item.Key, item.Topic)
		}
		topics[item.Topic] = item.Key

		if !item.ShouldRun {
			t.Errorf("%s: everything is exposed, item should be enabled", item.Key)
			continue
		}
		p := d.payload

		uid, _ := p["unique_id"].(string)
		if uid == "" {
			t.Errorf("%s: missing unique_id", item.Key)
		}
		if prev, dup := uniqueIDs[uid]; dup {
			t.Errorf("%s and %s share unique_id %q", prev, item.Key, uid)
		}
		uniqueIDs[uid] = item.Key

		if p["availability_topic"] != "sat/desk.local/status" {
			t.Errorf("%s: availability_topic = %v", item.Key, p["availability_topic"])
		}
		if name, _ := p["name"].(string); name == "" {
			t.Errorf("%s: missing name", item.Key)
		}
		dev, _ := p["device"].(map[string]any)
		if ids, _ := dev["identifiers"].([]any); len(ids) != 1 || ids[0] != "satellite_desk.local" {
			t.Errorf("%s: device identifiers = %v", item.Key, dev["identifiers"])
		}

		if st, ok := p["state_topic"]; ok && st != "sat/desk.local/state" {
			t.Errorf("%s: state_topic = %v", item.Key, st)
		}
		if ct, ok := p["command_topic"]; ok && ct != "sat/desk.local/command" {
			t.Errorf("%s: command_topic = %v", item.Key, ct)
		}

		// Every template must read a field the agent actually publishes.
		for _, key := range []string{"value_template", "json_attributes_template"} {
			tmpl, _ := p[key].(string)
			for _, m := range valueJSONField.FindAllStringSubmatch(tmpl, -1) {
				if !fields[m[1]] {
					t.Errorf("%s: %s reads value_json.%s, which telemetry.Snapshot never publishes", item.Key, key, m[1])
				}
			}
		}
	}
}

// Every button / media player command Home Assistant can send must be
// executed by the agent when the entity that offers it is enabled.
func TestEveryDiscoveredCommandIsAccepted(t *testing.T) {
	stubHardware(t, nil, nil)
	cfg := config.DefaultConfig()
	cfg.NodeID = "node"
	cfg.Expose = everythingExposed()

	known := map[string]bool{
		"lock": true, "media_play_pause": true, "media_play": true, "media_pause": true,
		"media_next": true, "media_prev": true, "media_stop": true, "volume_mute": true,
		"volume_up": true, "volume_down": true, "display_sleep": true,
	}
	commandCount := 0
	for _, d := range discoverAll(t, cfg) {
		for key, val := range d.payload {
			if !strings.HasPrefix(key, "payload_") || key == "payload_on" || key == "payload_off" {
				continue
			}
			commandCount++
			action := actions.Canonical(val.(string))
			if !known[action] {
				t.Errorf("%s: %s=%q is not an action the agent implements", d.item.Key, key, val)
			}
			if !commandAllowed(cfg.Expose, action) {
				t.Errorf("%s: %s=%q is rejected even with every setting enabled", d.item.Key, key, val)
			}
		}
	}
	if commandCount < 8 {
		t.Fatalf("only found %d command payloads; discovery shape changed?", commandCount)
	}
}

func TestDisabledEntitiesAreRemovedWithEmptyPayload(t *testing.T) {
	stubHardware(t, nil, nil)
	cfg := config.DefaultConfig()
	cfg.NodeID = "node"
	cfg.Expose = config.ExposeConfig{} // nothing exposed

	items := GetAllDiscoveryItems(cfg, true, allCaps())
	if len(items) == 0 {
		t.Fatal("disabled entities must still be listed so they can be removed")
	}
	for _, item := range items {
		if item.ShouldRun || len(item.Payload) != 0 {
			t.Errorf("%s: disabled entity should have empty payload, got %q", item.Key, item.Payload)
		}
	}
}

func TestUnsupportedCapabilitiesDisableEntities(t *testing.T) {
	stubHardware(t, []telemetry.DiskInfo{{Mount: "/"}}, []telemetry.GPUInfo{{Index: 0}})
	cfg := config.DefaultConfig()
	cfg.NodeID = "node"
	cfg.Expose = everythingExposed()

	for _, item := range GetAllDiscoveryItems(cfg, true, capabilities.PlatformCapabilities{}) {
		if item.ShouldRun {
			t.Errorf("%s: enabled although the platform does not support it", item.Key)
		}
	}
}

func TestBatterySensorsDependOnHardware(t *testing.T) {
	stubHardware(t, nil, nil)
	cfg := config.DefaultConfig()
	cfg.NodeID = "node"
	cfg.Expose = everythingExposed()

	enabled := func(hasBattery bool) map[string]bool {
		out := map[string]bool{}
		for _, item := range GetAllDiscoveryItems(cfg, hasBattery, allCaps()) {
			if item.ShouldRun {
				out[item.Key] = true
			}
		}
		return out
	}
	with, without := enabled(true), enabled(false)
	var batteryKeys []string
	for k := range with {
		if !without[k] {
			batteryKeys = append(batteryKeys, k)
		}
	}
	if len(batteryKeys) == 0 {
		t.Fatal("no entity depends on having a battery")
	}
	for _, k := range batteryKeys {
		if !strings.Contains(k, "battery") && !strings.Contains(k, "power") {
			t.Errorf("%s unexpectedly depends on battery presence", k)
		}
	}
}

func TestDriveAndGPUEntities(t *testing.T) {
	stubHardware(t,
		[]telemetry.DiskInfo{{Mount: "C:"}, {Mount: "/"}, {Mount: "/mnt/data"}, {Mount: "/mnt/it's"}},
		[]telemetry.GPUInfo{{Index: 0, Name: "Radeon"}},
	)
	cfg := config.DefaultConfig()
	cfg.NodeID = "box"
	cfg.Expose.Storage = true
	cfg.Expose.GPU = true

	byKey := map[string]discovered{}
	for _, d := range discoverAll(t, cfg) {
		byKey[d.item.Key] = d
	}
	for key, mount := range map[string]string{"drive_c": "C:", "drive_root": "/", "drive_mnt_data": "/mnt/data"} {
		d, ok := byKey[key]
		if !ok {
			t.Errorf("missing %s", key)
			continue
		}
		if !strings.Contains(d.payload["value_template"].(string), "'"+mount+"'") {
			t.Errorf("%s: template does not select mount %q: %s", key, mount, d.payload["value_template"])
		}
	}
	if tmpl := byKey["drive_mnt_it_s"].payload["value_template"].(string); !strings.Contains(tmpl, `'/mnt/it\'s'`) {
		t.Errorf("quote in mount must be escaped in the template: %s", tmpl)
	}

	// A single GPU is named after the card; temperature/power only appear
	// when the GPU reports them.
	if name := byKey["gpu_0_usage"].payload["name"]; name != "Radeon Usage" {
		t.Errorf("gpu name = %v", name)
	}
	if _, ok := byKey["gpu_0_temp"]; ok {
		t.Error("temperature entity created for a GPU without a temperature reading")
	}
	if _, ok := byKey["gpu_0_power"]; ok {
		t.Error("power entity created for a GPU without a power reading")
	}
}

func TestDriveKey(t *testing.T) {
	for in, want := range map[string]string{
		"C:": "c", "D:\\": "d", "/": "root", "/home": "home", "/mnt/My Drive": "mnt_my_drive", "": "root",
	} {
		if got := driveKey(in); got != want {
			t.Errorf("driveKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCommandAllowedGatesEachActionBySetting(t *testing.T) {
	media := []string{"media_play_pause", "media_play", "media_pause", "media_next", "media_prev", "media_stop", "volume_mute", "volume_up", "volume_down"}

	onlyMedia := config.ExposeConfig{MediaControl: true}
	for _, a := range media {
		if !commandAllowed(onlyMedia, a) {
			t.Errorf("%s should be allowed with media control on", a)
		}
	}
	if commandAllowed(onlyMedia, "lock") || commandAllowed(onlyMedia, "display_sleep") {
		t.Error("media control must not allow lock or display sleep")
	}

	onlyLock := config.ExposeConfig{RemoteLock: true}
	if !commandAllowed(onlyLock, "lock") {
		t.Error("lock should be allowed with remote lock on")
	}
	for _, a := range append(media, "display_sleep") {
		if commandAllowed(onlyLock, a) {
			t.Errorf("remote lock must not allow %s", a)
		}
	}

	if !commandAllowed(config.ExposeConfig{DisplayState: true}, "display_sleep") {
		t.Error("display sleep should be allowed with display state on")
	}
	if commandAllowed(everythingExposed(), "shutdown") {
		t.Error("unknown actions must never be allowed")
	}
	if acceptsCommands(config.ExposeConfig{CPU: true, Notifications: true}) {
		t.Error("no command settings enabled, should not subscribe")
	}
}
