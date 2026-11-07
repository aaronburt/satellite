//go:build windows && amd64

package icue

import (
	"os"
	"testing"
	"time"
	"unsafe"
)

func TestSDKLayouts(t *testing.T) {
	if unsafe.Sizeof(sdkDevice{}) != 396 || unsafe.Offsetof(sdkDevice{}.LEDCount) != 388 {
		t.Fatal("device layout does not match SDK v4")
	}
	if unsafe.Sizeof(sdkPosition{}) != 24 || unsafe.Offsetof(sdkPosition{}.X) != 8 {
		t.Fatal("position layout does not match SDK v4")
	}
	if unsafe.Sizeof(LED{}) != 8 || unsafe.Offsetof(LED{}.R) != 4 {
		t.Fatal("colour layout does not match SDK v4")
	}
}

func TestMissingSDKIsSafe(t *testing.T) {
	s := &sdk{nextLoad: time.Now().Add(time.Hour)}
	if state := s.read(); state.Status != "sdk_missing" || len(state.Devices) != 0 {
		t.Fatalf("unexpected state: %+v", state)
	}
	s.close()
	if s.load([]string{`C:\nonexistent-satellite-sdk\missing.dll`}) {
		t.Fatal("loaded nonexistent SDK")
	}
}

func TestLiveSDKReadOnly(t *testing.T) {
	path := os.Getenv("SATELLITE_TEST_ICUE_DLL")
	if path == "" {
		t.Skip("set SATELLITE_TEST_ICUE_DLL to test against a local iCUE installation")
	}
	s := &sdk{}
	if !s.load([]string{path}) {
		t.Fatal("could not load the official SDK")
	}
	defer s.close()
	deadline := time.Now().Add(10 * time.Second)
	for {
		state := s.read()
		if state.Status == "connected" || state.Status == "partial" {
			t.Logf("read %d RGB devices (%s)", len(state.Devices), state.Status)
			for _, d := range state.Devices {
				t.Logf("%s: %d LEDs", d.Model, len(d.LEDs))
			}
			return
		}
		if state.Status != "connecting" || time.Now().After(deadline) {
			t.Fatalf("iCUE state: %s", state.Status)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
