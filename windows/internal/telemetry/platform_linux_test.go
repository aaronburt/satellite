//go:build linux

package telemetry

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"

	"satellite/internal/config"
)

// fakeHost points the Linux readers at a temporary directory tree.
type fakeHost struct {
	t    *testing.T
	root string
}

func newFakeHost(t *testing.T) *fakeHost {
	t.Helper()
	root := t.TempDir()
	orig := hostRoot
	hostRoot = root
	t.Cleanup(func() { hostRoot = orig })
	return &fakeHost{t: t, root: root}
}

func (h *fakeHost) write(path, content string) {
	h.t.Helper()
	full := filepath.Join(h.root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		h.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		h.t.Fatal(err)
	}
}

func (h *fakeHost) symlink(path, target string) {
	h.t.Helper()
	full := filepath.Join(h.root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		h.t.Fatal(err)
	}
	if err := os.Symlink(target, full); err != nil {
		h.t.Fatal(err)
	}
}

func (h *fakeHost) mkdir(path string) {
	h.t.Helper()
	if err := os.MkdirAll(filepath.Join(h.root, path), 0o755); err != nil {
		h.t.Fatal(err)
	}
}

func TestGetCPUTimes(t *testing.T) {
	h := newFakeHost(t)
	h.write("/proc/stat", "cpu  100 20 30 400 50 6 7 8 0 0\ncpu0 1 2 3 4 5 6 7 8\nintr 1\n")
	idle, kernel, user, err := GetCPUTimes()
	if err != nil {
		t.Fatal(err)
	}
	// idle = idle+iowait; kernel = system+irq+softirq+idle; user = user+nice+steal
	if idle != 450 || kernel != 30+6+7+450 || user != 100+20+8 {
		t.Fatalf("got idle=%d kernel=%d user=%d", idle, kernel, user)
	}
}

func TestGetCPUTimesWithoutStealColumn(t *testing.T) {
	h := newFakeHost(t)
	h.write("/proc/stat", "cpu 10 0 10 80 0 0 0\n")
	idle, kernel, user, err := GetCPUTimes()
	if err != nil || idle != 80 || kernel != 90 || user != 10 {
		t.Fatalf("got %d %d %d %v", idle, kernel, user, err)
	}
}

func TestGetCPUTimesErrors(t *testing.T) {
	h := newFakeHost(t)
	if _, _, _, err := GetCPUTimes(); err == nil {
		t.Fatal("expected error when /proc/stat is missing")
	}
	h.write("/proc/stat", "cpu0 1 2 3 4 5 6 7\n")
	if _, _, _, err := GetCPUTimes(); err == nil {
		t.Fatal("expected error without aggregate cpu line")
	}
	h.write("/proc/stat", "cpu 1 2 3\n")
	if _, _, _, err := GetCPUTimes(); err == nil {
		t.Fatal("expected error for truncated cpu line")
	}
}

func TestGetRAMUsage(t *testing.T) {
	h := newFakeHost(t)
	// 16 GiB total, 4 GiB available -> 12 GiB used, 75%.
	h.write("/proc/meminfo", "MemTotal:       16777216 kB\nMemFree:  1 kB\nMemAvailable:    4194304 kB\nBuffers: 1 kB\n")
	used, total, pct, err := GetRAMUsage()
	if err != nil {
		t.Fatal(err)
	}
	if used != 12 || total != 16 || pct != 75 {
		t.Fatalf("got used=%v total=%v pct=%v", used, total, pct)
	}
}

func TestGetRAMUsageErrors(t *testing.T) {
	h := newFakeHost(t)
	if _, _, _, err := GetRAMUsage(); err == nil {
		t.Fatal("expected error when /proc/meminfo is missing")
	}
	h.write("/proc/meminfo", "MemFree: 100 kB\n")
	if _, _, _, err := GetRAMUsage(); err == nil {
		t.Fatal("expected error without MemTotal")
	}
}

func TestGetNetworkOctetsSkipsVirtualInterfaces(t *testing.T) {
	h := newFakeHost(t)
	h.write("/proc/net/dev", `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 999999      10    0    0    0     0          0         0   999999      10    0    0    0     0       0          0
  eth0:    1000      10    0    0    0     0          0         0     2000      10    0    0    0     0       0          0
 wlan0:     300       1    0    0    0     0          0         0      400       1    0    0    0     0       0          0
docker0: 77777      1    0    0    0     0          0         0    77777       1    0    0    0     0       0          0
vethab12: 5555      1    0    0    0     0          0         0     5555       1    0    0    0     0       0          0
br-1234: 4444       1    0    0    0     0          0         0     4444       1    0    0    0     0       0          0
`)
	rx, tx, err := GetNetworkOctets()
	if err != nil {
		t.Fatal(err)
	}
	if rx != 1300 || tx != 2400 {
		t.Fatalf("rx=%d tx=%d, want 1300/2400", rx, tx)
	}
}

func TestGetSystemUptimeSeconds(t *testing.T) {
	h := newFakeHost(t)
	if got := GetSystemUptimeSeconds(); got != 0 {
		t.Fatalf("missing file: got %d", got)
	}
	h.write("/proc/uptime", "12345.67 54321.00\n")
	if got := GetSystemUptimeSeconds(); got != 12345 {
		t.Fatalf("got %d, want 12345", got)
	}
	h.write("/proc/uptime", "garbage\n")
	if got := GetSystemUptimeSeconds(); got != 0 {
		t.Fatalf("garbage: got %d", got)
	}
}

func TestUnescapeMountPath(t *testing.T) {
	cases := map[string]string{
		"/":                   "/",
		"/mnt/My\\040Drive":   "/mnt/My Drive",
		"/mnt/a\\011b":        "/mnt/a\tb",
		"/mnt/back\\134slash": "/mnt/back\\slash",
		"/mnt/trailing\\04":   "/mnt/trailing\\04",
		"/mnt/notoctal\\999":  "/mnt/notoctal\\999",
	}
	for in, want := range cases {
		if got := unescapeMountPath(in); got != want {
			t.Errorf("unescapeMountPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGetAllDrivesFiltersAndDeduplicates(t *testing.T) {
	h := newFakeHost(t)
	h.mkdir("/")
	h.mkdir("/home")
	h.mkdir("/mnt/My Drive")
	h.mkdir("/snap/core/1")
	h.mkdir("/var/lib/docker/overlay")
	h.write("/proc/mounts", `/dev/sda1 / ext4 rw 0 0
proc /proc proc rw 0 0
tmpfs /run tmpfs rw 0 0
/dev/sda2 /home btrfs rw 0 0
/dev/sda2 /home btrfs rw 0 0
/dev/sdb1 /mnt/My\040Drive ntfs3 rw 0 0
/dev/loop0 /snap/core/1 ext4 ro 0 0
/dev/sda1 /var/lib/docker/overlay ext4 rw 0 0
/dev/sdc1 /mnt/missing ext4 rw 0 0
`)
	drives := GetAllDrives()
	var mounts []string
	for _, d := range drives {
		mounts = append(mounts, d.Mount)
		if d.TotalGB <= 0 || d.UsedGB < 0 || d.UsedPercent < 0 || d.UsedPercent > 100 {
			t.Errorf("implausible drive %+v", d)
		}
	}
	want := []string{"/", "/home", "/mnt/My Drive"}
	if !reflect.DeepEqual(mounts, want) {
		t.Fatalf("mounts = %v, want %v", mounts, want)
	}
}

func TestIsMicrophoneInUse(t *testing.T) {
	h := newFakeHost(t)
	h.symlink("/proc/100/fd/0", "/dev/null")
	h.symlink("/proc/100/fd/1", "/dev/snd/pcmC0D0p") // playback only
	if IsMicrophoneInUse() {
		t.Fatal("playback device must not count as microphone")
	}
	h.symlink("/proc/200/fd/7", "/dev/snd/pcmC1D0c")
	if !IsMicrophoneInUse() {
		t.Fatal("capture device open should count as microphone in use")
	}
}

func TestGetWebcamStatus(t *testing.T) {
	h := newFakeHost(t)
	if inUse, _ := GetWebcamStatus(); inUse {
		t.Fatal("no /dev/video* should mean not in use")
	}
	h.write("/dev/video0", "")
	h.symlink("/proc/10/fd/0", "/dev/null")
	if inUse, app := GetWebcamStatus(); inUse || app != "" {
		t.Fatalf("got %v %q, want not in use", inUse, app)
	}
	h.symlink("/proc/4242/fd/9", "/dev/video0")
	h.write("/proc/4242/comm", "zoom\n")
	if inUse, app := GetWebcamStatus(); !inUse || app != "zoom" {
		t.Fatalf("got %v %q, want true zoom", inUse, app)
	}
}

func TestGetWebcamStatusWithoutComm(t *testing.T) {
	h := newFakeHost(t)
	h.write("/dev/video0", "")
	h.symlink("/proc/77/fd/3", "/dev/video0")
	if inUse, app := GetWebcamStatus(); !inUse || app != "camera" {
		t.Fatalf("got %v %q, want true camera", inUse, app)
	}
}

func TestGetBatteryAndPower(t *testing.T) {
	h := newFakeHost(t)
	if has, _, _ := GetBatteryAndPower(); has {
		t.Fatal("no battery directory should mean no battery")
	}

	h.write("/sys/class/power_supply/BAT0/capacity", "87\n")
	h.write("/sys/class/power_supply/BAT0/status", "Discharging\n")
	has, pct, plugged := GetBatteryAndPower()
	if !has || pct != 87 || plugged {
		t.Fatalf("got %v %d %v, want true 87 false", has, pct, plugged)
	}

	h.write("/sys/class/power_supply/BAT0/capacity", "104\n")
	if _, pct, _ := GetBatteryAndPower(); pct != 100 {
		t.Fatalf("capacity should clamp to 100, got %d", pct)
	}

	h.write("/sys/class/power_supply/BAT0/capacity", "n/a\n")
	if has, _, _ := GetBatteryAndPower(); has {
		t.Fatal("unparseable capacity should mean no battery reading")
	}
}

func TestBatteryStatusMeansPlugged(t *testing.T) {
	for status, want := range map[string]bool{
		"Charging": true, "Full": true, "Not charging": true,
		"Discharging": false, "Unknown": false, "": false,
	} {
		if got := batteryStatusMeansPlugged(status); got != want {
			t.Errorf("batteryStatusMeansPlugged(%q) = %v, want %v", status, got, want)
		}
	}
}

func TestIsRebootPending(t *testing.T) {
	h := newFakeHost(t)
	if IsRebootPending() {
		t.Fatal("no marker file means no reboot pending")
	}
	h.write("/var/run/reboot-required", "*** System restart required ***\n")
	if !IsRebootPending() {
		t.Fatal("marker file means reboot pending")
	}
}

func TestParseProcNetWireless(t *testing.T) {
	header := "Inter-| sta-|   Quality        |   Discarded packets               | Missed | WE\n face | tus | link level noise |  nwid  crypt   frag  retry   misc | beacon | 22\n"
	cases := []struct {
		name      string
		data      string
		wantIface string
		wantPct   *int
	}{
		{"typical", header + "wlp2s0: 0000   56.  -54.  -256        0      0      0      0      0        0\n", "wlp2s0", intPtr(80)},
		{"clamped above 70", header + "wlan0: 0000   90.  -20.  -256  0 0 0 0 0 0\n", "wlan0", intPtr(100)},
		{"unparseable quality", header + "wlan0: 0000   ???  -20.  -256  0 0 0 0 0 0\n", "wlan0", nil},
		{"header only", header, "", nil},
		{"empty", "", "", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			iface, pct := parseProcNetWireless(tc.data)
			if iface != tc.wantIface || !reflect.DeepEqual(pct, tc.wantPct) {
				t.Fatalf("got %q %v, want %q %v", iface, deref(pct), tc.wantIface, deref(tc.wantPct))
			}
		})
	}
}

func TestGetWifiStatusReadsProcFile(t *testing.T) {
	h := newFakeHost(t)
	if iface, pct := GetWifiStatus(); iface != "" || pct != nil {
		t.Fatal("missing file should give no wifi")
	}
	h.write("/proc/net/wireless", "h1\nh2\nwlan0: 0000   35.  -54.  -256 0 0 0 0 0 0\n")
	iface, pct := GetWifiStatus()
	if iface != "wlan0" || pct == nil || *pct != 50 {
		t.Fatalf("got %q %v", iface, deref(pct))
	}
}

func TestIdleSecondsFromLogind(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	since := uint64(now.Add(-90 * time.Second).UnixMicro())

	if s, ok := idleSecondsFromLogind(true, since, now); !ok || s != 90 {
		t.Fatalf("idle for 90s: got %d %v", s, ok)
	}
	if s, ok := idleSecondsFromLogind(false, since, now); !ok || s != 0 {
		t.Fatalf("not idle should be 0 seconds: got %d %v", s, ok)
	}
	if _, ok := idleSecondsFromLogind(true, 0, now); ok {
		t.Fatal("missing timestamp should be unknown")
	}
	future := uint64(now.Add(time.Minute).UnixMicro())
	if s, ok := idleSecondsFromLogind(true, future, now); !ok || s != 0 {
		t.Fatalf("clock skew should clamp to 0: got %d %v", s, ok)
	}
}

func TestThemeFromColorScheme(t *testing.T) {
	cases := []struct {
		name string
		v    dbus.Variant
		want string
	}{
		{"prefer dark (nested)", dbus.MakeVariant(dbus.MakeVariant(uint32(1))), "dark"},
		{"prefer light (nested)", dbus.MakeVariant(dbus.MakeVariant(uint32(2))), "light"},
		{"no preference", dbus.MakeVariant(dbus.MakeVariant(uint32(0))), "light"},
		{"unwrapped value", dbus.MakeVariant(uint32(1)), "dark"},
		{"wrong type", dbus.MakeVariant("dark"), "dark"},
	}
	for _, tc := range cases {
		if got := themeFromColorScheme(tc.v); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestMediaFromMPRIS(t *testing.T) {
	meta := map[string]dbus.Variant{
		"xesam:title":  dbus.MakeVariant("Song"),
		"xesam:artist": dbus.MakeVariant([]string{"Artist A", "Artist B"}),
		"xesam:album":  dbus.MakeVariant("Album"),
	}
	got := mediaFromMPRIS("org.mpris.MediaPlayer2.spotify", "Playing", meta)
	want := MediaInfo{Status: "playing", Title: "Song", Artist: "Artist A", Album: "Album", AppID: "spotify"}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	bare := mediaFromMPRIS("org.mpris.MediaPlayer2.vlc", "Paused", nil)
	if bare != (MediaInfo{Status: "paused", AppID: "vlc"}) {
		t.Fatalf("no metadata: got %+v", bare)
	}

	wrongTypes := mediaFromMPRIS("org.mpris.MediaPlayer2.x", "Stopped", map[string]dbus.Variant{
		"xesam:title":  dbus.MakeVariant(42),
		"xesam:artist": dbus.MakeVariant([]string{}),
	})
	if wrongTypes.Title != "" || wrongTypes.Artist != "" {
		t.Fatalf("bad metadata types should be ignored: %+v", wrongTypes)
	}
}

func TestSelectActiveMediaPrefersPlayingOverPaused(t *testing.T) {
	paused := MediaInfo{Status: "paused", AppID: "spotify"}
	playing := MediaInfo{Status: "playing", AppID: "firefox"}
	stopped := MediaInfo{Status: "stopped", AppID: "vlc"}

	if got := selectActiveMedia([]MediaInfo{stopped, paused, playing}); got != playing {
		t.Fatalf("got %+v, want playing player", got)
	}
	if got := selectActiveMedia([]MediaInfo{stopped, paused}); got != paused {
		t.Fatalf("got %+v, want paused player", got)
	}
	if got := selectActiveMedia([]MediaInfo{stopped}); got != (MediaInfo{Status: "idle"}) {
		t.Fatalf("got %+v, want idle", got)
	}
	if got := selectActiveMedia(nil); got.Status != "idle" {
		t.Fatalf("got %+v, want idle", got)
	}
}

func TestSleepDisplays(t *testing.T) {
	orig := runDisplayCommand
	t.Cleanup(func() { runDisplayCommand = orig })
	var ran [][]string
	fail := map[string]bool{}
	runDisplayCommand = func(name string, args ...string) error {
		ran = append(ran, append([]string{name}, args...))
		if fail[name] {
			return errors.New("fail")
		}
		return nil
	}

	t.Setenv("DISPLAY", ":0")
	t.Setenv("WAYLAND_DISPLAY", "")
	if err := SleepDisplays(); err != nil || !reflect.DeepEqual(ran, [][]string{{"xset", "dpms", "force", "off"}}) {
		t.Fatalf("X11: err=%v ran=%v", err, ran)
	}

	ran = nil
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	if err := SleepDisplays(); err != nil || !reflect.DeepEqual(ran, [][]string{{"wlopm", "--off", "*"}}) {
		t.Fatalf("Wayland: err=%v ran=%v", err, ran)
	}

	fail["wlopm"] = true
	if err := SleepDisplays(); !errors.Is(err, errNotSupported) {
		t.Fatalf("Wayland without wlopm: err=%v", err)
	}

	t.Setenv("WAYLAND_DISPLAY", "")
	if err := SleepDisplays(); !errors.Is(err, errNotSupported) {
		t.Fatalf("headless: err=%v", err)
	}
}

func TestParseNvidiaSmiCSV(t *testing.T) {
	out := "0, NVIDIA GeForce RTX 4090, 37, 6144, 24564, 61, 182.45\n" +
		"1, Tesla T4, [N/A], 0, 15360, 40, [Not Supported]\n" +
		"bogus line\n" +
		"x, Bad Index, 1, 1, 1, 1, 1\n"
	gpus := parseNvidiaSmiCSV(out)
	if len(gpus) != 2 {
		t.Fatalf("got %d GPUs, want 2: %+v", len(gpus), gpus)
	}
	g := gpus[0]
	if g.Index != 0 || g.Name != "NVIDIA GeForce RTX 4090" || g.Vendor != "NVIDIA" ||
		*g.CoreUsagePercent != 37 || *g.MemoryUsedMB != 6144 || *g.MemoryTotalMB != 24564 ||
		*g.TemperatureC != 61 || *g.PowerWatts != 182.45 {
		t.Fatalf("unexpected first GPU: %+v", g)
	}
	if pct := *g.MemoryPercent; pct < 25.0 || pct > 25.02 {
		t.Fatalf("memory percent = %v, want ~25.01", pct)
	}
	t4 := gpus[1]
	if t4.CoreUsagePercent != nil || t4.PowerWatts != nil {
		t.Fatalf("[N/A] fields must be nil, not zero: %+v", t4)
	}
	if t4.MemoryPercent == nil || *t4.MemoryPercent != 0 {
		t.Fatalf("memory percent should be 0, got %v", t4.MemoryPercent)
	}
}

func TestGetGPUInfoPrefersNvidiaSmiWhenEnabled(t *testing.T) {
	h := newFakeHost(t)
	h.write("/sys/class/drm/card0/device/vendor", "0x8086\n")
	orig := nvidiaSmiOutput
	t.Cleanup(func() { nvidiaSmiOutput = orig })
	nvidiaSmiOutput = func() (string, error) { return "0, RTX, 1, 2, 4, 50, 100\n", nil }

	if gpus := GetGPUInfo(true); len(gpus) != 1 || gpus[0].Vendor != "NVIDIA" {
		t.Fatalf("with NVML: %+v", gpus)
	}
	if gpus := GetGPUInfo(); len(gpus) != 1 || gpus[0].Vendor != "Intel" {
		t.Fatalf("without NVML should read sysfs: %+v", gpus)
	}

	nvidiaSmiOutput = func() (string, error) { return "", errors.New("not installed") }
	if gpus := GetGPUInfo(true); len(gpus) != 1 || gpus[0].Vendor != "Intel" {
		t.Fatalf("nvidia-smi failure should fall back to sysfs: %+v", gpus)
	}
}

func TestGetGPUInfoReadsDRMSysfs(t *testing.T) {
	h := newFakeHost(t)
	// AMD card with full metrics.
	h.write("/sys/class/drm/card0/device/vendor", "0x1002\n")
	h.write("/sys/class/drm/card0/device/gpu_busy_percent", "42\n")
	h.write("/sys/class/drm/card0/device/hwmon/hwmon3/temp1_input", "55000\n")
	h.write("/sys/class/drm/card0/device/mem_info_vram_total", "8589934592\n")
	h.write("/sys/class/drm/card0/device/mem_info_vram_used", "2147483648\n")
	// Connector entries must be ignored.
	h.mkdir("/sys/class/drm/card0-HDMI-A-1")
	// Card without a device directory is skipped.
	h.mkdir("/sys/class/drm/card1")
	// Unknown vendor with no metrics.
	h.write("/sys/class/drm/card2/device/vendor", "0x1af4\n")

	gpus := GetGPUInfo()
	if len(gpus) != 2 {
		t.Fatalf("got %d GPUs, want 2: %+v", len(gpus), gpus)
	}
	amd := gpus[0]
	if amd.Index != 0 || amd.Vendor != "AMD" || amd.Name != "AMD GPU 0" {
		t.Fatalf("unexpected AMD identity: %+v", amd)
	}
	if *amd.CoreUsagePercent != 42 || *amd.TemperatureC != 55 ||
		*amd.MemoryTotalMB != 8192 || *amd.MemoryUsedMB != 2048 || *amd.MemoryPercent != 25 {
		t.Fatalf("unexpected AMD metrics: %+v", amd)
	}
	other := gpus[1]
	if other.Index != 2 || other.Vendor != "unknown" || other.CoreUsagePercent != nil || other.MemoryPercent != nil {
		t.Fatalf("unexpected unknown GPU: %+v", other)
	}
}

func TestGetGPUInfoNoCards(t *testing.T) {
	newFakeHost(t)
	if gpus := GetGPUInfo(); gpus != nil {
		t.Fatalf("got %+v, want nil", gpus)
	}
}

// End-to-end: two collections against a changing fake /proc produce the
// expected CPU percentage and network rates.
func TestCollectorComputesRatesBetweenSamples(t *testing.T) {
	h := newFakeHost(t)
	h.write("/proc/stat", "cpu 100 0 100 800 0 0 0 0\n")
	h.write("/proc/net/dev", "h1\nh2\n  eth0: 10240 0 0 0 0 0 0 0 20480 0 0 0 0 0 0 0\n")

	clock := time.Unix(1_700_000_000, 0)
	c := NewCollector()
	c.now = func() time.Time { return clock }

	expose := config.ExposeConfig{CPU: true, Network: true}
	first := c.Collect(expose)
	if first.CPUPercent != nil || first.NetworkRxKBs != nil {
		t.Fatalf("first sample has no baseline, got cpu=%v rx=%v", first.CPUPercent, first.NetworkRxKBs)
	}

	// +100 ticks total, of which 60 idle -> 40% busy.
	h.write("/proc/stat", "cpu 120 0 120 860 0 0 0 0\n")
	// +20 kB rx, +10 kB tx over 2 seconds.
	h.write("/proc/net/dev", "h1\nh2\n  eth0: 30720 0 0 0 0 0 0 0 30720 0 0 0 0 0 0 0\n")
	clock = clock.Add(2 * time.Second)

	second := c.Collect(expose)
	if second.CPUPercent == nil || *second.CPUPercent != 40 {
		t.Fatalf("cpu = %v, want 40", deref64(second.CPUPercent))
	}
	if second.NetworkRxKBs == nil || *second.NetworkRxKBs != 10 || *second.NetworkTxKBs != 5 {
		t.Fatalf("rx=%v tx=%v, want 10/5", deref64(second.NetworkRxKBs), deref64(second.NetworkTxKBs))
	}
	if second.Timestamp != clock.Unix() {
		t.Fatalf("timestamp = %d, want %d", second.Timestamp, clock.Unix())
	}
	if c.Last().Timestamp != second.Timestamp {
		t.Fatal("Last() should return the most recent snapshot")
	}
}

func TestCollectorBatteryOnlyReportedWhenExposed(t *testing.T) {
	h := newFakeHost(t)
	h.write("/sys/class/power_supply/BAT1/capacity", "50\n")
	h.write("/sys/class/power_supply/BAT1/status", "Charging\n")
	c := NewCollector()

	hidden := c.Collect(config.ExposeConfig{})
	if !hidden.HasBattery || hidden.BatteryPercent != nil || hidden.PowerPlugged != nil {
		t.Fatalf("HasBattery must always be set but values hidden: %+v", hidden)
	}
	shown := c.Collect(config.ExposeConfig{Battery: true})
	if shown.BatteryPercent == nil || *shown.BatteryPercent != 50 || !*shown.PowerPlugged {
		t.Fatalf("battery exposed: %+v", shown)
	}
}

func intPtr(v int) *int { return &v }

func deref(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func deref64(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

// Each expose setting controls whether its reading appears in the snapshot,
// so turning a sensor off in the UI really stops it being published.
func TestCollectorOnlyReportsExposedReadings(t *testing.T) {
	h := newFakeHost(t)
	h.write("/proc/meminfo", "MemTotal: 1048576 kB\nMemAvailable: 524288 kB\n")
	h.write("/proc/uptime", "100.0 1.0\n")
	h.write("/proc/net/wireless", "h\nh\nwlan0: 0000 70. 0 0 0 0 0 0 0 0\n")

	none := NewCollector().Collect(config.ExposeConfig{})
	if none.MemoryPercent != nil || none.UptimeSeconds != nil || none.SessionLocked != nil ||
		none.MicrophoneInUse != nil || none.FullscreenActive != nil || none.UpdatePending != nil ||
		none.Media != nil || none.WebcamInUse != nil || none.DisplayPowered != nil ||
		none.WifiSSID != "" || none.SystemTheme != "" || none.Drives != nil || none.GPUs != nil {
		t.Fatalf("nothing exposed but snapshot has readings: %+v", none)
	}

	all := NewCollector().Collect(config.ExposeConfig{
		Memory: true, Uptime: true, SessionLock: true, Microphone: true, Fullscreen: true,
		UpdatePending: true, MediaControl: true, Webcam: true, DisplayState: true, Wifi: true,
	})
	if all.MemoryPercent == nil || *all.MemoryPercent != 50 {
		t.Errorf("memory = %v", deref64(all.MemoryPercent))
	}
	if all.UptimeSeconds == nil || *all.UptimeSeconds != 100 {
		t.Errorf("uptime = %v", all.UptimeSeconds)
	}
	if all.SessionLocked == nil || all.MicrophoneInUse == nil || all.FullscreenActive == nil ||
		all.UpdatePending == nil || all.Media == nil || all.WebcamInUse == nil || all.DisplayPowered == nil {
		t.Errorf("exposed readings missing: %+v", all)
	}
	if all.WifiSSID != "wlan0" || all.WifiSignalPercent == nil || *all.WifiSignalPercent != 100 {
		t.Errorf("wifi = %q %v", all.WifiSSID, deref(all.WifiSignalPercent))
	}
}
