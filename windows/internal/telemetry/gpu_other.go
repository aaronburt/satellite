//go:build !windows && !linux

package telemetry

func GetGPUInfo(enableNVML ...bool) []GPUInfo {
	return nil
}
