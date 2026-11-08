package config

import (
	"fmt"
	"math"
)

type SensorThreshold struct {
	Enabled  bool    `json:"enabled"`
	Warning  float64 `json:"warning"`
	Critical float64 `json:"critical"`
}

type SensorThresholds struct {
	CPU     SensorThreshold `json:"cpu"`
	Memory  SensorThreshold `json:"memory"`
	GPU     SensorThreshold `json:"gpu"`
	Storage SensorThreshold `json:"storage"`
	Battery SensorThreshold `json:"battery"`
}

func (t SensorThresholds) Validate() error {
	for name, threshold := range map[string]SensorThreshold{"cpu": t.CPU, "memory": t.Memory, "gpu": t.GPU, "storage": t.Storage, "battery": t.Battery} {
		if !threshold.Enabled {
			continue
		}
		w, c := threshold.Warning, threshold.Critical
		if math.IsNaN(w) || math.IsNaN(c) || math.IsInf(w, 0) || math.IsInf(c, 0) || w < 0 || w > 100 || c < 0 || c > 100 {
			return fmt.Errorf("%s thresholds must be percentages between 0 and 100", name)
		}
		if (name == "battery" && c >= w) || (name != "battery" && c <= w) {
			return fmt.Errorf("%s critical threshold must be more severe than its warning threshold", name)
		}
	}
	return nil
}
