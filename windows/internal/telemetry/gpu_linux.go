//go:build linux

package telemetry

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func parseSysfsFloat(path string) (float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	val, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	if err != nil {
		return 0, err
	}
	return val, nil
}

func readDrmGpu(cardDir string, index int) (GPUInfo, bool) {
	deviceDir := filepath.Join(cardDir, "device")
	if _, err := os.Stat(deviceDir); err != nil {
		return GPUInfo{}, false
	}

	gpu := GPUInfo{
		Index:  index,
		Vendor: "unknown",
	}

	vendorData, err := os.ReadFile(filepath.Join(deviceDir, "vendor"))
	if err == nil {
		vendorID := strings.TrimSpace(string(vendorData))
		switch vendorID {
		case "0x1002":
			gpu.Vendor = "AMD"
		case "0x8086":
			gpu.Vendor = "Intel"
		case "0x10de":
			gpu.Vendor = "NVIDIA"
		}
	}

	gpu.Name = fmt.Sprintf("%s GPU %d", gpu.Vendor, index)

	if usage, err := parseSysfsFloat(filepath.Join(deviceDir, "gpu_busy_percent")); err == nil {
		gpu.CoreUsagePercent = &usage
	}

	hwmonMatches, err := filepath.Glob(filepath.Join(deviceDir, "hwmon", "hwmon*", "temp1_input"))
	if err == nil && len(hwmonMatches) > 0 {
		if millidegrees, err := parseSysfsFloat(hwmonMatches[0]); err == nil {
			tempC := millidegrees / 1000.0
			gpu.TemperatureC = &tempC
		}
	}

	vramTotalPath := filepath.Join(deviceDir, "mem_info_vram_total")
	vramUsedPath := filepath.Join(deviceDir, "mem_info_vram_used")
	totalBytes, errTot := parseSysfsFloat(vramTotalPath)
	usedBytes, errUse := parseSysfsFloat(vramUsedPath)
	if errTot == nil && errUse == nil && totalBytes > 0 {
		totalMB := totalBytes / (1024 * 1024)
		usedMB := usedBytes / (1024 * 1024)
		pct := (usedMB / totalMB) * 100.0
		gpu.MemoryTotalMB = &totalMB
		gpu.MemoryUsedMB = &usedMB
		gpu.MemoryPercent = &pct
	}

	return gpu, true
}

// nvidiaSmiOutput is a seam so tests can feed canned nvidia-smi output.
var nvidiaSmiOutput = func() (string, error) {
	cmd := exec.Command("nvidia-smi", "--query-gpu=index,name,utilization.gpu,memory.used,memory.total,temperature.gpu,power.draw", "--format=csv,noheader,nounits")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	return out.String(), err
}

func queryNvidiaSmi() []GPUInfo {
	out, err := nvidiaSmiOutput()
	if err != nil {
		return nil
	}
	return parseNvidiaSmiCSV(out)
}

// optionalFloat parses an nvidia-smi field, returning nil for values the
// driver can't report ("[N/A]", "[Not Supported]", empty).
func optionalFloat(field string) *float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(field), 64)
	if err != nil {
		return nil
	}
	return &v
}

// parseNvidiaSmiCSV parses `nvidia-smi --format=csv,noheader,nounits` output
// for the fields index,name,utilization,mem used,mem total,temp,power.
func parseNvidiaSmiCSV(out string) []GPUInfo {
	var gpus []GPUInfo
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Split(line, ",")
		if len(fields) < 7 {
			continue
		}
		idx, err := strconv.Atoi(strings.TrimSpace(fields[0]))
		if err != nil {
			continue
		}
		gpu := GPUInfo{
			Index:            idx,
			Name:             strings.TrimSpace(fields[1]),
			Vendor:           "NVIDIA",
			CoreUsagePercent: optionalFloat(fields[2]),
			MemoryUsedMB:     optionalFloat(fields[3]),
			MemoryTotalMB:    optionalFloat(fields[4]),
			TemperatureC:     optionalFloat(fields[5]),
			PowerWatts:       optionalFloat(fields[6]),
		}
		if gpu.MemoryUsedMB != nil && gpu.MemoryTotalMB != nil && *gpu.MemoryTotalMB > 0 {
			pct := (*gpu.MemoryUsedMB / *gpu.MemoryTotalMB) * 100.0
			gpu.MemoryPercent = &pct
		}
		gpus = append(gpus, gpu)
	}
	return gpus
}

func GetGPUInfo(enableNVML ...bool) []GPUInfo {
	withNVML := false
	if len(enableNVML) > 0 {
		withNVML = enableNVML[0]
	}

	if withNVML {
		if nvidiaGpus := queryNvidiaSmi(); len(nvidiaGpus) > 0 {
			return nvidiaGpus
		}
	}

	matches, err := filepath.Glob(hostPath("/sys/class/drm/card[0-9]*"))
	if err != nil || len(matches) == 0 {
		return nil
	}

	var gpus []GPUInfo
	for _, cardPath := range matches {
		// card[0-9]* also matches connectors such as card0-HDMI-A-1.
		index, err := strconv.Atoi(strings.TrimPrefix(filepath.Base(cardPath), "card"))
		if err != nil {
			continue
		}
		if gpu, ok := readDrmGpu(cardPath, index); ok {
			gpus = append(gpus, gpu)
		}
	}
	return gpus
}
