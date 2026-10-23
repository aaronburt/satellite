//go:build linux

package telemetry

import (
	"bufio"
	"errors"
	"math"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

var errNotSupported = errors.New("not supported on this platform")

// hostRoot is prefixed to every /proc, /sys and /dev path so tests can point
// the readers at a fake filesystem tree.
var hostRoot = "/"

func hostPath(p string) string {
	return filepath.Join(hostRoot, p)
}

func GetCPUTimes() (uint64, uint64, uint64, error) {
	file, err := os.Open(hostPath("/proc/stat"))
	if err != nil {
		return 0, 0, 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) < 8 {
				break
			}
			user, _ := strconv.ParseUint(fields[1], 10, 64)
			nice, _ := strconv.ParseUint(fields[2], 10, 64)
			system, _ := strconv.ParseUint(fields[3], 10, 64)
			idle, _ := strconv.ParseUint(fields[4], 10, 64)
			iowait, _ := strconv.ParseUint(fields[5], 10, 64)
			irq, _ := strconv.ParseUint(fields[6], 10, 64)
			softirq, _ := strconv.ParseUint(fields[7], 10, 64)
			var steal uint64
			if len(fields) > 8 {
				steal, _ = strconv.ParseUint(fields[8], 10, 64)
			}

			totalIdle := idle + iowait
			totalKernel := system + irq + softirq + totalIdle
			totalUser := user + nice + steal

			return totalIdle, totalKernel, totalUser, nil
		}
	}
	return 0, 0, 0, errors.New("cpu line not found in /proc/stat")
}

func GetRAMUsage() (float64, float64, float64, error) {
	file, err := os.Open(hostPath("/proc/meminfo"))
	if err != nil {
		return 0, 0, 0, err
	}
	defer file.Close()

	var memTotalKB, memAvailKB float64
	foundTotal, foundAvail := false, false

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[0] == "MemTotal:" {
			memTotalKB, _ = strconv.ParseFloat(fields[1], 64)
			foundTotal = true
		} else if fields[0] == "MemAvailable:" {
			memAvailKB, _ = strconv.ParseFloat(fields[1], 64)
			foundAvail = true
		}
		if foundTotal && foundAvail {
			break
		}
	}

	if !foundTotal || memTotalKB == 0 {
		return 0, 0, 0, errors.New("unable to determine total memory")
	}

	totalGB := math.Round((memTotalKB/(1024*1024))*100) / 100
	availGB := math.Round((memAvailKB/(1024*1024))*100) / 100
	usedGB := math.Round((totalGB-availGB)*100) / 100
	pct := math.Round((usedGB/totalGB)*1000) / 10

	return usedGB, totalGB, pct, nil
}

func GetAllDrives() []DiskInfo {
	file, err := os.Open(hostPath("/proc/mounts"))
	if err != nil {
		return nil
	}
	defer file.Close()

	seenMounts := make(map[string]bool)
	var drives []DiskInfo

	supportedFs := map[string]bool{
		"ext4":    true,
		"ext3":    true,
		"ext2":    true,
		"btrfs":   true,
		"xfs":     true,
		"zfs":     true,
		"vfat":    true,
		"ntfs":    true,
		"ntfs3":   true,
		"fuseblk": true,
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		mountPoint := unescapeMountPath(fields[1])
		fsType := fields[2]

		if !supportedFs[fsType] || seenMounts[mountPoint] {
			continue
		}
		if strings.HasPrefix(mountPoint, "/snap") || strings.HasPrefix(mountPoint, "/var/lib/docker") {
			continue
		}
		seenMounts[mountPoint] = true

		var stat syscall.Statfs_t
		if err := syscall.Statfs(hostPath(mountPoint), &stat); err != nil {
			continue
		}

		totalBytes := stat.Blocks * uint64(stat.Bsize)
		freeBytes := stat.Bavail * uint64(stat.Bsize)
		if totalBytes == 0 {
			continue
		}

		totalGB := math.Round((float64(totalBytes)/(1024*1024*1024))*10) / 10
		usedBytes := totalBytes - freeBytes
		usedGB := math.Round((float64(usedBytes)/(1024*1024*1024))*10) / 10
		pct := math.Round((usedGB/totalGB)*1000) / 10

		drives = append(drives, DiskInfo{
			Mount:       mountPoint,
			UsedGB:      usedGB,
			TotalGB:     totalGB,
			UsedPercent: pct,
		})
	}
	return drives
}

// unescapeMountPath decodes the octal escapes /proc/mounts uses for spaces,
// tabs, newlines and backslashes in mount points (e.g. "/mnt/My\040Drive").
func unescapeMountPath(p string) string {
	if !strings.Contains(p, `\`) {
		return p
	}
	var b strings.Builder
	for i := 0; i < len(p); i++ {
		if p[i] == '\\' && i+4 <= len(p) {
			if v, err := strconv.ParseUint(p[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(v))
				i += 3
				continue
			}
		}
		b.WriteByte(p[i])
	}
	return b.String()
}

func GetNetworkOctets() (uint64, uint64, error) {
	file, err := os.Open(hostPath("/proc/net/dev"))
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	var totalRx, totalTx uint64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}
		iface := strings.TrimSpace(parts[0])
		if iface == "lo" || strings.HasPrefix(iface, "docker") || strings.HasPrefix(iface, "veth") || strings.HasPrefix(iface, "br-") {
			continue
		}

		fields := strings.Fields(parts[1])
		if len(fields) < 9 {
			continue
		}
		rx, _ := strconv.ParseUint(fields[0], 10, 64)
		tx, _ := strconv.ParseUint(fields[8], 10, 64)
		totalRx += rx
		totalTx += tx
	}
	return totalRx, totalTx, nil
}

func GetSystemUptimeSeconds() uint64 {
	data, err := os.ReadFile(hostPath("/proc/uptime"))
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	val, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return uint64(val)
}

func GetUserIdleSeconds() (uint64, bool) {
	conn, err := dbus.SessionBus()
	if err == nil {
		defer conn.Close()
		obj := conn.Object("org.freedesktop.ScreenSaver", "/org/freedesktop/ScreenSaver")
		var idleMs uint32
		if callErr := obj.Call("org.freedesktop.ScreenSaver.GetSessionIdleTime", 0).Store(&idleMs); callErr == nil {
			return uint64(idleMs / 1000), true
		}
	}

	sysConn, err := dbus.SystemBus()
	if err != nil {
		return 0, false
	}
	defer sysConn.Close()

	obj := sysConn.Object("org.freedesktop.login1", "/org/freedesktop/login1/session/auto")
	hintVal, err := obj.GetProperty("org.freedesktop.login1.Session.IdleHint")
	if err != nil {
		return 0, false
	}
	sinceVal, err := obj.GetProperty("org.freedesktop.login1.Session.IdleSinceHint")
	if err != nil {
		return 0, false
	}
	idle, _ := hintVal.Value().(bool)
	since, _ := sinceVal.Value().(uint64)
	return idleSecondsFromLogind(idle, since, time.Now())
}

// idleSecondsFromLogind turns logind's IdleHint/IdleSinceHint into seconds
// idle. IdleSinceHint is a CLOCK_REALTIME timestamp in microseconds of the
// last IdleHint change, so it only measures idleness while IdleHint is true.
func idleSecondsFromLogind(idleHint bool, sinceRealtimeUsec uint64, now time.Time) (uint64, bool) {
	if !idleHint {
		return 0, true
	}
	if sinceRealtimeUsec == 0 {
		return 0, false
	}
	nowUsec := now.UnixMicro()
	if nowUsec <= 0 || uint64(nowUsec) <= sinceRealtimeUsec {
		return 0, true
	}
	return (uint64(nowUsec) - sinceRealtimeUsec) / 1_000_000, true
}

func IsSessionLocked() bool {
	conn, err := dbus.SystemBus()
	if err != nil {
		return false
	}
	defer conn.Close()

	obj := conn.Object("org.freedesktop.login1", "/org/freedesktop/login1/session/auto")
	val, err := obj.GetProperty("org.freedesktop.login1.Session.LockedHint")
	if err == nil {
		if locked, ok := val.Value().(bool); ok {
			return locked
		}
	}
	return false
}

func IsMicrophoneInUse() bool {
	matches, err := filepath.Glob(hostPath("/proc/[0-9]*/fd/*"))
	if err != nil {
		return false
	}
	for _, match := range matches {
		target, err := os.Readlink(match)
		if err == nil && strings.HasPrefix(target, "/dev/snd/pcm") && strings.HasSuffix(target, "c") {
			return true
		}
	}
	return false
}

func GetWebcamStatus() (bool, string) {
	videoMatches, err := filepath.Glob(hostPath("/dev/video*"))
	if err != nil || len(videoMatches) == 0 {
		return false, ""
	}

	procDir := hostPath("/proc")
	matches, err := filepath.Glob(filepath.Join(procDir, "[0-9]*", "fd", "*"))
	if err != nil {
		return false, ""
	}

	for _, match := range matches {
		target, err := os.Readlink(match)
		if err != nil || !strings.HasPrefix(target, "/dev/video") {
			continue
		}
		// match is <procDir>/<pid>/fd/<n>
		rel, err := filepath.Rel(procDir, match)
		if err == nil {
			pid := strings.SplitN(filepath.ToSlash(rel), "/", 2)[0]
			if commData, err := os.ReadFile(filepath.Join(procDir, pid, "comm")); err == nil {
				return true, strings.TrimSpace(string(commData))
			}
		}
		return true, "camera"
	}
	return false, ""
}

func IsFullscreenActive() bool {
	return false
}

func GetSystemTheme() string {
	conn, err := dbus.SessionBus()
	if err != nil {
		return "dark"
	}
	defer conn.Close()

	obj := conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")
	var schemeVariant dbus.Variant
	call := obj.Call("org.freedesktop.portal.Settings.Read", 0, "org.freedesktop.appearance", "color-scheme")
	if call.Err != nil || call.Store(&schemeVariant) != nil {
		return "dark"
	}
	return themeFromColorScheme(schemeVariant)
}

// themeFromColorScheme maps the XDG portal "color-scheme" setting
// (0 = no preference, 1 = prefer dark, 2 = prefer light) to a theme name.
// Settings.Read wraps the value in an extra variant; both shapes are accepted.
func themeFromColorScheme(v dbus.Variant) string {
	val := v.Value()
	if inner, ok := val.(dbus.Variant); ok {
		val = inner.Value()
	}
	num, ok := val.(uint32)
	if !ok {
		return "dark"
	}
	if num == 2 {
		return "light"
	}
	if num == 1 {
		return "dark"
	}
	return "light"
}

func GetLocalIPv4() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		if localAddr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			ip := localAddr.IP.To4()
			if ip != nil && !ip.IsLoopback() {
				return ip.String()
			}
		}
	}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ip := ipnet.IP.To4(); ip != nil {
				return ip.String()
			}
		}
	}
	return ""
}

func GetBatteryAndPower() (bool, int, bool) {
	matches, err := filepath.Glob(hostPath("/sys/class/power_supply/BAT*"))
	if err != nil || len(matches) == 0 {
		return false, 0, false
	}

	batDir := matches[0]
	capData, err := os.ReadFile(filepath.Join(batDir, "capacity"))
	if err != nil {
		return false, 0, false
	}
	pct, err := strconv.Atoi(strings.TrimSpace(string(capData)))
	if err != nil {
		return false, 0, false
	}
	if pct < 0 {
		pct = 0
	} else if pct > 100 {
		pct = 100
	}

	plugged := false
	if statusData, err := os.ReadFile(filepath.Join(batDir, "status")); err == nil {
		plugged = batteryStatusMeansPlugged(strings.TrimSpace(string(statusData)))
	}
	return true, pct, plugged
}

// batteryStatusMeansPlugged reports whether a sysfs battery status implies
// the charger is connected. "Not charging" is what laptops report when held
// at a charge limit while on AC.
func batteryStatusMeansPlugged(status string) bool {
	switch status {
	case "Charging", "Full", "Not charging":
		return true
	}
	return false
}

func GetActiveWindowAndProcess() (string, string) {
	return "", ""
}

func IsRebootPending() bool {
	_, err := os.Stat(hostPath("/var/run/reboot-required"))
	return err == nil
}

func GetActiveMediaInfo() MediaInfo {
	conn, err := dbus.SessionBus()
	if err != nil {
		return MediaInfo{Status: "idle"}
	}
	defer conn.Close()

	var names []string
	if err := conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names); err != nil {
		return MediaInfo{Status: "idle"}
	}

	var players []MediaInfo
	for _, name := range names {
		if !strings.HasPrefix(name, "org.mpris.MediaPlayer2.") {
			continue
		}
		player := conn.Object(name, "/org/mpris/MediaPlayer2")
		statusProp, err := player.GetProperty("org.mpris.MediaPlayer2.Player.PlaybackStatus")
		if err != nil {
			continue
		}
		statusStr, _ := statusProp.Value().(string)
		var meta map[string]dbus.Variant
		if metaProp, err := player.GetProperty("org.mpris.MediaPlayer2.Player.Metadata"); err == nil {
			meta, _ = metaProp.Value().(map[string]dbus.Variant)
		}
		players = append(players, mediaFromMPRIS(name, statusStr, meta))
	}
	return selectActiveMedia(players)
}

// mediaFromMPRIS builds a MediaInfo from an MPRIS player's bus name,
// PlaybackStatus and Metadata properties.
func mediaFromMPRIS(busName, status string, meta map[string]dbus.Variant) MediaInfo {
	media := MediaInfo{
		Status: strings.ToLower(status),
		AppID:  strings.TrimPrefix(busName, "org.mpris.MediaPlayer2."),
	}
	if title, ok := meta["xesam:title"].Value().(string); ok {
		media.Title = title
	}
	if artists, ok := meta["xesam:artist"].Value().([]string); ok && len(artists) > 0 {
		media.Artist = artists[0]
	}
	if album, ok := meta["xesam:album"].Value().(string); ok {
		media.Album = album
	}
	return media
}

// selectActiveMedia picks the player to report: the first one playing,
// otherwise the first one paused, otherwise idle.
func selectActiveMedia(players []MediaInfo) MediaInfo {
	for _, p := range players {
		if p.Status == "playing" {
			return p
		}
	}
	for _, p := range players {
		if p.Status == "paused" {
			return p
		}
	}
	return MediaInfo{Status: "idle"}
}

func GetActiveAudioOutputDevice() string {
	conn, err := dbus.SessionBus()
	if err == nil {
		defer conn.Close()
	}
	return "Default Output"
}

func GetWifiStatus() (string, *int) {
	data, err := os.ReadFile(hostPath("/proc/net/wireless"))
	if err != nil {
		return "", nil
	}
	return parseProcNetWireless(string(data))
}

// parseProcNetWireless reads the first interface from /proc/net/wireless and
// converts its link quality (0-70 on most drivers) to a percentage.
func parseProcNetWireless(data string) (string, *int) {
	lines := strings.Split(data, "\n")
	if len(lines) < 3 {
		return "", nil
	}
	fields := strings.Fields(lines[2])
	if len(fields) < 3 {
		return "", nil
	}
	iface := strings.TrimSuffix(fields[0], ":")
	linkQuality, err := strconv.ParseFloat(strings.TrimSuffix(fields[2], "."), 64)
	if err != nil {
		return iface, nil
	}
	pct := int(math.Min(100, math.Max(0, (linkQuality/70.0)*100.0)))
	return iface, &pct
}

func IsDisplayPowered() bool {
	return true
}

// runDisplayCommand is a seam so tests don't turn off the test machine's screen.
var runDisplayCommand = func(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

func SleepDisplays() error {
	if os.Getenv("DISPLAY") != "" {
		return runDisplayCommand("xset", "dpms", "force", "off")
	}
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if err := runDisplayCommand("wlopm", "--off", "*"); err == nil {
			return nil
		}
	}
	return errNotSupported
}

func TrimWorkingSet() {
	debug.FreeOSMemory()
}
