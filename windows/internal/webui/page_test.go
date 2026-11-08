package webui

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"satellite/internal/config"
	"satellite/internal/telemetry"
)

// The page is one file with no build step, so these tests read it as text to
// keep it and the Go sensor list agreeing with each other.

func jsObjectKeys(t *testing.T, name string) []string {
	t.Helper()
	page := string(indexHTML)
	start := strings.Index(page, "const "+name+" = {")
	if start < 0 {
		t.Fatalf("page has no %s", name)
	}
	body := page[start:]
	end := strings.Index(body, "\n};")
	if end < 0 {
		t.Fatalf("%s is not closed", name)
	}
	var keys []string
	for _, m := range regexp.MustCompile(`(?m)^  ([A-Za-z_]+):`).FindAllStringSubmatch(body[:end], -1) {
		keys = append(keys, m[1])
	}
	return keys
}

func TestPageKnowsEverySensorGroup(t *testing.T) {
	m := regexp.MustCompile(`const SENSOR_GROUPS = \[([^\]]*)\]`).FindStringSubmatch(string(indexHTML))
	if m == nil {
		t.Fatal("page has no SENSOR_GROUPS")
	}
	for _, g := range []string{GroupSystem, GroupNetwork, GroupSession, GroupDevices, GroupScreen} {
		if !strings.Contains(m[1], "'"+g+"'") {
			t.Errorf("page does not list the %q group, so its sensors would sort last", g)
		}
	}
}

// A sensor without an icon would show the CPU's, which reads as a bug.
func TestPageHasAnIconForEverySensor(t *testing.T) {
	icons := map[string]bool{}
	for _, k := range jsObjectKeys(t, "SENSOR_ICONS") {
		icons[k] = true
	}
	snap := fullSnapshot()
	snap.GPUs = append(snap.GPUs, telemetry.GPUInfo{Index: 1, Name: "Second"})
	list := BuildSensors(allExpose(), allCaps(), snap, Frame{Have: true, At: testNow}, testNow)
	// Without a battery or GPU, the plain "gpu"/"storage" ids appear too.
	empty := BuildSensors(config.ExposeConfig{GPU: true, Storage: true}, allCaps(), telemetry.Snapshot{Timestamp: 1}, Frame{}, testNow)
	for _, s := range append(list, empty...) {
		key := s.ID
		switch {
		case strings.HasPrefix(key, "gpu"):
			key = "gpu"
		case strings.HasPrefix(key, "disk"), key == "storage":
			key = "disk"
		}
		if !icons[key] {
			t.Errorf("sensor %q (icon %q) has no entry in SENSOR_ICONS", s.ID, key)
		}
	}
}

func TestPageStylesEverySensorState(t *testing.T) {
	page := string(indexHTML)
	// "ok" is the plain look. The rest each change the card.
	for _, state := range []string{StateActive, StateWarn, StateIdle, StateWaiting} {
		if !strings.Contains(page, `[data-state="`+state+`"]`) {
			t.Errorf("no CSS for state %q", state)
		}
	}
	for _, level := range []string{LevelMid, LevelHigh} {
		if !regexp.MustCompile(`\.heat-fill\.` + level + `\{`).MatchString(page) {
			t.Errorf("no CSS for gauge level %q", level)
		}
	}
}

// Every data-click the page can produce needs a handler, or the button does
// nothing and says nothing.
func TestEveryClickActionHasAHandler(t *testing.T) {
	page := string(indexHTML)
	handlers := map[string]bool{}
	for _, k := range jsObjectKeys(t, "clickHandlers") {
		handlers[k] = true
	}
	used := map[string]bool{}
	for _, m := range regexp.MustCompile(`data-click="([A-Za-z]+)"`).FindAllStringSubmatch(page, -1) {
		used[m[1]] = true
	}
	for _, m := range regexp.MustCompile(`dataset\.click = '([A-Za-z]+)'`).FindAllStringSubmatch(page, -1) {
		used[m[1]] = true
	}
	var missing []string
	for name := range used {
		if !handlers[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("data-click actions with no handler: %v", missing)
	}
	if len(used) < 8 {
		t.Fatalf("found only %d click actions; the check is not reading the page", len(used))
	}
}

func TestEveryNavButtonHasAPanel(t *testing.T) {
	page := string(indexHTML)
	tabs := regexp.MustCompile(`(?m)^  ([a-z]+): \{ panel: '([a-z-]+)', nav: '([a-z-]+)'`).FindAllStringSubmatch(page, -1)
	if len(tabs) != 5 {
		t.Fatalf("found %d tabs, want 5", len(tabs))
	}
	for _, m := range tabs {
		tab, panel, nav := m[1], m[2], m[3]
		if !strings.Contains(page, `id="`+panel+`"`) {
			t.Errorf("tab %s: no element #%s", tab, panel)
		}
		if !regexp.MustCompile(`id="` + nav + `"[^>]*data-arg="` + tab + `"`).MatchString(page) {
			t.Errorf("tab %s: button #%s does not switch to it", tab, nav)
		}
	}
}

func TestSensorPanelHasItsPlaceholders(t *testing.T) {
	page := string(indexHTML)
	for _, id := range []string{"sensors-groups", "sensors-empty", "sensors-nomatch", "sum-count", "sum-warn", "sum-updated", "filter-input"} {
		if !strings.Contains(page, `id="`+id+`"`) {
			t.Errorf("page has no #%s", id)
		}
	}
	if strings.Contains(page, "Task Manager") {
		t.Error("the page is no longer styled as a Task Manager")
	}
}

// Sensor text comes from the machine (window titles, track names, network
// names) and must never be parsed as HTML.
func TestSensorCardsNeverInjectReadingsAsHTML(t *testing.T) {
	page := string(indexHTML)
	start := strings.Index(page, "function buildSensorCard")
	end := strings.Index(page, "function renderSensorSummary")
	if start < 0 || end < start {
		t.Fatal("cannot find the sensor rendering code")
	}
	code := page[start:end]
	for _, banned := range []string{".innerHTML", "insertAdjacentHTML", "document.write", "outerHTML"} {
		// svgIcon sets innerHTML from constants and lives outside this range.
		if strings.Contains(code, banned) {
			t.Errorf("sensor rendering uses %s", banned)
		}
	}
}

func TestPreviewRequestIsOnlyMadeWhileTheSensorsTabIsVisible(t *testing.T) {
	page := string(indexHTML)
	if !strings.Contains(page, "activeTab === 'sensors' && !document.hidden") {
		t.Error("the page must only ask for the screen preview while it is showing it")
	}
	if !strings.Contains(page, "'/api/status' + (wantThumb ? '?thumb=1' : '')") {
		t.Error("the thumb=1 flag the agent waits for is not sent")
	}
	if !strings.Contains(page, "document.addEventListener('visibilitychange'") {
		t.Error("polling should pause while the tab is hidden")
	}
}

func TestPrivacyExplainsEveryPermission(t *testing.T) {
	explained := map[string]bool{}
	for _, key := range jsObjectKeys(t, "PRIVACY_FEATURES") {
		explained[key] = true
	}
	report := privacyOverview(configWith(allExpose()), allCaps(), false, false)
	for _, permission := range report["permissions"].([]privacyPermission) {
		if !explained[permission.Key] {
			t.Errorf("privacy tab has no explanation for %s", permission.Key)
		}
	}
}
