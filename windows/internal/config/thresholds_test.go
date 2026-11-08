package config

import (
	"math"
	"testing"
)

func TestSensorThresholdValidation(t *testing.T) {
	tests := []struct {
		name       string
		thresholds SensorThresholds
		valid      bool
	}{
		{"disabled", SensorThresholds{}, true},
		{"usage", SensorThresholds{CPU: SensorThreshold{true, 80, 95}}, true},
		{"battery", SensorThresholds{Battery: SensorThreshold{true, 20, 10}}, true},
		{"equal", SensorThresholds{Memory: SensorThreshold{true, 80, 80}}, false},
		{"reversed usage", SensorThresholds{GPU: SensorThreshold{true, 95, 80}}, false},
		{"reversed battery", SensorThresholds{Battery: SensorThreshold{true, 10, 20}}, false},
		{"negative", SensorThresholds{Storage: SensorThreshold{true, -1, 95}}, false},
		{"over 100", SensorThresholds{CPU: SensorThreshold{true, 80, 101}}, false},
		{"NaN", SensorThresholds{CPU: SensorThreshold{true, math.NaN(), 95}}, false},
		{"infinity", SensorThresholds{CPU: SensorThreshold{true, 80, math.Inf(1)}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.thresholds.Validate(); (err == nil) != tt.valid {
				t.Fatalf("Validate = %v, valid = %v", err, tt.valid)
			}
		})
	}
}
