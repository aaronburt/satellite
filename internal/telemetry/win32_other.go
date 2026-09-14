//go:build !windows

package telemetry

import "errors"

var errNotSupported = errors.New("not supported on this platform")

func GetCPUTimes() (uint64, uint64, uint64, error) {
	return 0, 0, 0, errNotSupported
}

func GetRAMUsage() (float64, float64, float64, error) {
	return 0, 0, 0, errNotSupported
}

func GetAllDrives() []DiskInfo {
	return nil
}

func GetNetworkOctets() (uint64, uint64, error) {
	return 0, 0, errNotSupported
}

func GetSystemUptimeSeconds() uint64 {
	return 0
}

func GetUserIdleSeconds() (uint64, bool) {
	return 0, false
}

func IsSessionLocked() bool {
	return false
}

func IsMicrophoneInUse() bool {
	return false
}

func GetWebcamStatus() (bool, string) {
	return false, ""
}

func IsFullscreenActive() bool {
	return false
}

func GetWindowsTheme() string {
	return "dark"
}

func GetLocalIPv4() string {
	return ""
}

func GetBatteryAndPower() (bool, int, bool) {
	return false, 0, false
}

func GetActiveWindowAndProcess() (string, string) {
	return "", ""
}

func IsRebootPending() bool {
	return false
}

func GetActiveMediaInfo() MediaInfo {
	return MediaInfo{Status: "idle"}
}

func GetActiveAudioOutputDevice() string {
	return ""
}

func GetWifiStatus() (string, *int) {
	return "", nil
}

func IsDisplayPowered() bool {
	return true
}

func SleepDisplays() error {
	return errNotSupported
}

func TrimWorkingSet() {}
