package webui

import (
	"fmt"
	"math"
	"strings"

	"satellite/internal/config"
)

func applySensorThresholds(sensors []Sensor, thresholds config.SensorThresholds) []Sensor {
	for i := range sensors {
		s := &sensors[i]
		var threshold config.SensorThreshold
		switch {
		case s.ID == "cpu":
			threshold = thresholds.CPU
		case s.ID == "memory":
			threshold = thresholds.Memory
		case strings.HasPrefix(s.ID, "gpu"):
			threshold = thresholds.GPU
		case strings.HasPrefix(s.ID, "disk:"):
			threshold = thresholds.Storage
		case s.ID == "battery":
			threshold = thresholds.Battery
		}
		if !threshold.Enabled || s.Percent == nil || s.State == StateWaiting || math.IsNaN(*s.Percent) || math.IsInf(*s.Percent, 0) {
			continue
		}
		s.Level = ""
		crossed := func(limit float64) bool {
			if s.ID == "battery" {
				return *s.Percent <= limit
			}
			return *s.Percent >= limit
		}
		severity, limit := "", 0.0
		if crossed(threshold.Critical) {
			severity, limit, s.Level = "Critical", threshold.Critical, LevelHigh
		} else if crossed(threshold.Warning) {
			severity, limit, s.Level = "Warning", threshold.Warning, LevelMid
		}
		if severity != "" {
			direction := "at or above"
			if s.ID == "battery" {
				direction = "at or below"
			}
			s.State = StateWarn
			s.Threshold = strings.ToLower(severity)
			s.ThresholdMessage = fmt.Sprintf("%s: %s %g%%", severity, direction, limit)
		}
	}
	return sensors
}
