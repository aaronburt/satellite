package webui

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"satellite/internal/config"
)

func TestSensorThresholdBoundariesAndRecovery(t *testing.T) {
	thresholds := config.SensorThresholds{
		CPU:     config.SensorThreshold{Enabled: true, Warning: 80, Critical: 95},
		Battery: config.SensorThreshold{Enabled: true, Warning: 20, Critical: 10},
		GPU:     config.SensorThreshold{Enabled: true, Warning: 80, Critical: 95},
		Storage: config.SensorThreshold{Enabled: true, Warning: 80, Critical: 95},
	}
	for _, tt := range []struct {
		id    string
		value float64
		want  string
	}{
		{"cpu", 79.9, ""}, {"cpu", 80, "warning"}, {"cpu", 95, "critical"}, {"cpu", 100, "critical"}, {"cpu", 20, ""},
		{"battery", 21, ""}, {"battery", 20, "warning"}, {"battery", 10, "critical"}, {"battery", 0, "critical"},
		{"gpu0", 95, "critical"}, {"gpu1", 80, "warning"}, {"disk:C:", 95, "critical"}, {"disk:D:", 80, "warning"},
		{"memory", 99, ""},
	} {
		result := applySensorThresholds([]Sensor{{ID: tt.id, State: StateOK, Percent: ptr(tt.value)}}, thresholds)[0]
		if result.Threshold != tt.want {
			t.Errorf("%s at %v = %+v; want %s", tt.id, tt.value, result, tt.want)
		}
		if tt.want != "" && (result.State != StateWarn || result.ThresholdMessage == "") {
			t.Errorf("missing warning text: %+v", result)
		}
	}
	waiting := applySensorThresholds([]Sensor{{ID: "cpu", State: StateWaiting, Percent: ptr(99.0)}, {ID: "gpu0", State: StateOK}}, thresholds)
	for _, sensor := range waiting {
		if sensor.Threshold != "" {
			t.Errorf("missing reading flagged: %+v", sensor)
		}
	}
	original := Sensor{ID: "cpu", State: StateOK, Percent: ptr(99.0), Level: LevelHigh}
	disabled := applySensorThresholds([]Sensor{original}, config.SensorThresholds{})[0]
	if !reflect.DeepEqual(disabled, original) {
		t.Fatal("disabled threshold changed a sensor")
	}
}

func TestThresholdSettingsSaveDisableAndReject(t *testing.T) {
	_, base, token := startTestServer(t, configWith(allExpose()))
	settings := map[string]interface{}{"sensor_thresholds": map[string]interface{}{"cpu": map[string]interface{}{"enabled": true, "warning": 80, "critical": 95}}}
	resp, body := apiRequest(t, http.MethodPost, base+"/api/config", token, settings)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("save: %d %s", resp.StatusCode, body)
	}
	if got := config.Get().SensorThresholds.CPU; !got.Enabled || got.Warning != 80 || got.Critical != 95 {
		t.Fatalf("saved: %+v", got)
	}
	resp, body = apiRequest(t, http.MethodPost, base+"/api/config", token, map[string]interface{}{"sensor_thresholds": map[string]interface{}{"cpu": map[string]interface{}{"enabled": true, "warning": 95, "critical": 80}}})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid save: %d %s", resp.StatusCode, body)
	}
	if got := config.Get().SensorThresholds.CPU; got.Warning != 80 {
		t.Fatal("invalid save changed live settings")
	}
	resp, body = apiRequest(t, http.MethodPost, base+"/api/config", token, map[string]interface{}{"sensor_thresholds": map[string]interface{}{"cpu": map[string]interface{}{"enabled": false, "warning": 80, "critical": 95}}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("disable: %d %s", resp.StatusCode, body)
	}
	resp, body = apiRequest(t, http.MethodGet, base+"/api/config", token, nil)
	var cfg config.Config
	if err := json.Unmarshal(body, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.SensorThresholds.CPU.Enabled {
		t.Fatal("threshold remained enabled")
	}
}
