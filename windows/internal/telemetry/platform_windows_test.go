//go:build windows

package telemetry

import (
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestFiletimeToUint64(t *testing.T) {
	ft := filetime{dwLowDateTime: 0x89ABCDEF, dwHighDateTime: 0x01234567}
	if got := ft.toUint64(); got != 0x0123456789ABCDEF {
		t.Fatalf("toUint64 = %#x", got)
	}
}

func TestVendorFromID(t *testing.T) {
	for id, want := range map[uint32]string{0x10DE: "NVIDIA", 0x1002: "AMD", 0x8086: "Intel", 0x1414: "Microsoft", 0x1234: "Unknown"} {
		if got := vendorFromID(id); got != want {
			t.Errorf("vendorFromID(%#x) = %q, want %q", id, got, want)
		}
	}
}

func TestHStringRoundTrip(t *testing.T) {
	for _, s := range []string{"Windows.Media.Control", "ünïcødé ♫", "x"} {
		hs, err := hstringFromString(s)
		if err != nil {
			t.Fatal(err)
		}
		if got := stringFromHstring(hs); got != s {
			t.Errorf("round trip %q -> %q", s, got)
		}
	}
	if stringFromHstring(0) != "" {
		t.Error("null HSTRING should be empty")
	}
}

func TestUTF16PtrToString(t *testing.T) {
	buf, _ := syscall.UTF16FromString("Speakers (Realtek)")
	padded := append(buf, make([]uint16, 8)...)
	if got := utf16PtrToString(uintptr(unsafe.Pointer(&padded[0])), len(padded)); got != "Speakers (Realtek)" {
		t.Fatalf("got %q", got)
	}
	// No terminator within maxLen: stop at maxLen instead of reading past it.
	raw := []uint16{'a', 'b', 'c'}
	if got := utf16PtrToString(uintptr(unsafe.Pointer(&raw[0])), 2); got != "ab" {
		t.Fatalf("got %q", got)
	}
	if utf16PtrToString(0, 10) != "" {
		t.Fatal("nil pointer should be empty")
	}
}

// The readers below talk to the real OS. They check invariants that hold
// on any Windows machine, including the CI runner.

func TestCPUTimesAreCumulative(t *testing.T) {
	idle1, kernel1, user1, err := GetCPUTimes()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	idle2, kernel2, user2, err := GetCPUTimes()
	if err != nil {
		t.Fatal(err)
	}
	if idle2 < idle1 || kernel2 < kernel1 || user2 < user1 {
		t.Fatal("CPU counters went backwards")
	}
	if kernel1 < idle1 {
		t.Fatal("kernel time should include idle time on Windows")
	}
}

func TestRAMUsageIsConsistent(t *testing.T) {
	used, total, pct, err := GetRAMUsage()
	if err != nil {
		t.Fatal(err)
	}
	if total <= 0 || used <= 0 || used > total || pct <= 0 || pct > 100 {
		t.Fatalf("used=%v total=%v pct=%v", used, total, pct)
	}
}

func TestSystemDriveIsReported(t *testing.T) {
	sys := strings.TrimSuffix(strings.ToUpper(strings.TrimSpace(getEnv("SystemDrive"))), "\\")
	if sys == "" {
		sys = "C:"
	}
	for _, d := range GetAllDrives() {
		if d.Mount == sys {
			if d.TotalGB <= 0 || d.UsedPercent <= 0 || d.UsedPercent > 100 {
				t.Fatalf("implausible system drive %+v", d)
			}
			return
		}
	}
	t.Fatalf("system drive %s not reported", sys)
}

func TestNetworkCountersAreCumulative(t *testing.T) {
	rx1, tx1, err := GetNetworkOctets()
	if err != nil {
		t.Fatal(err)
	}
	rx2, tx2, err := GetNetworkOctets()
	if err != nil {
		t.Fatal(err)
	}
	if rx2 < rx1 || tx2 < tx1 {
		t.Fatal("network counters went backwards")
	}
}

func TestUptimeAndTheme(t *testing.T) {
	if GetSystemUptimeSeconds() == 0 {
		t.Fatal("uptime should be positive")
	}
	if th := GetSystemTheme(); th != "dark" && th != "light" {
		t.Fatalf("theme = %q", th)
	}
	if m := GetActiveMediaInfo(); m.Status == "" {
		t.Fatal("media status should never be empty")
	}
}

func getEnv(k string) string {
	v, _ := syscall.Getenv(k)
	return v
}
