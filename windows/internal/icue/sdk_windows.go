//go:build windows && amd64

package icue

import (
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const dllName = "iCUESDK.x64_2019.dll"

type sdkDevice struct {
	Type         uint32
	ID           [128]byte
	Serial       [128]byte
	Model        [128]byte
	LEDCount     int32
	ChannelCount int32
}

type sdkPosition struct {
	ID uint32
	_  uint32
	X  float64
	Y  float64
}

type sdk struct {
	dll                                             *windows.DLL
	connect, disconnect, devices, positions, colors *windows.Proc
	state                                           atomic.Uint32
	callback                                        uintptr
	started                                         bool
	nextLoad                                        time.Time
}

func sdkPaths() []string {
	var paths []string
	if dir, err := os.UserConfigDir(); err == nil {
		paths = append(paths, filepath.Join(dir, "satellite", "sdk", dllName))
	}
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), dllName))
	}
	return paths
}

func Detected() bool {
	for _, path := range sdkPaths() {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return true
		}
	}
	for _, name := range []string{"iCUE.exe", "iCUE Launcher.exe"} {
		key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\`+name, registry.QUERY_VALUE)
		if err == nil {
			key.Close()
			return true
		}
	}
	for _, version := range []string{"Corsair iCUE5 Software", "CORSAIR iCUE 4 Software"} {
		if base := os.Getenv("ProgramFiles"); base != "" {
			if _, err := os.Stat(filepath.Join(base, "Corsair", version, "iCUE.exe")); err == nil {
				return true
			}
		}
	}
	return false
}

func newBackend() backend { return &sdk{} }

func (s *sdk) load(paths []string) bool {
	for _, path := range paths {
		handle, err := windows.LoadLibraryEx(path, 0, windows.LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR|windows.LOAD_LIBRARY_SEARCH_SYSTEM32)
		if err != nil {
			continue
		}
		dll := &windows.DLL{Name: path, Handle: handle}
		var procs []*windows.Proc
		for _, name := range []string{"CorsairConnect", "CorsairDisconnect", "CorsairGetDevices", "CorsairGetLedPositions", "CorsairGetLedColors"} {
			proc, err := dll.FindProc(name)
			if err != nil {
				break
			}
			procs = append(procs, proc)
		}
		if len(procs) != 5 {
			dll.Release()
			continue
		}
		s.dll = dll
		s.connect, s.disconnect, s.devices, s.positions, s.colors = procs[0], procs[1], procs[2], procs[3], procs[4]
		return true
	}
	return false
}

func (s *sdk) read() State {
	if s.dll == nil {
		if time.Now().Before(s.nextLoad) {
			return unavailable("sdk_missing")
		}
		s.nextLoad = time.Now().Add(30 * time.Second)
		if !s.load(sdkPaths()) {
			return unavailable("sdk_missing")
		}
	}
	if !s.started {
		if s.callback == 0 {
			s.callback = syscall.NewCallbackCDecl(func(_ uintptr, event *uint32) uintptr {
				if event != nil {
					s.state.Store(*event)
				}
				return 0
			})
		}
		result, _, _ := s.connect.Call(s.callback, 0)
		if result != 0 {
			return unavailable("connection_failed")
		}
		s.started = true
	}
	if s.state.Load() != 6 {
		status := map[uint32]string{0: "connecting", 1: "disconnected", 2: "connecting", 3: "icue_unavailable", 4: "access_denied", 5: "disconnected"}[s.state.Load()]
		if status == "" {
			status = "disconnected"
		}
		return unavailable(status)
	}
	filter := uint32(0xffffffff)
	var devices [64]sdkDevice
	var count int32
	result, _, _ := s.devices.Call(uintptr(unsafe.Pointer(&filter)), uintptr(len(devices)), uintptr(unsafe.Pointer(&devices[0])), uintptr(unsafe.Pointer(&count)))
	if result != 0 || count < 0 || count > int32(len(devices)) {
		return unavailable("read_failed")
	}
	state := unavailable("connected")
	for i := int32(0); i < count; i++ {
		d := &devices[i]
		if d.LEDCount <= 0 {
			continue
		}
		var positions [512]sdkPosition
		var size int32
		result, _, _ = s.positions.Call(uintptr(unsafe.Pointer(&d.ID[0])), uintptr(len(positions)), uintptr(unsafe.Pointer(&positions[0])), uintptr(unsafe.Pointer(&size)))
		if result != 0 || size <= 0 || size > int32(len(positions)) {
			state.Status = "partial"
			continue
		}
		leds := make([]LED, size)
		for j := range leds {
			leds[j].ID = positions[j].ID
		}
		result, _, _ = s.colors.Call(uintptr(unsafe.Pointer(&d.ID[0])), uintptr(size), uintptr(unsafe.Pointer(&leds[0])))
		if result != 0 {
			state.Status = "partial"
			continue
		}
		state.Devices = append(state.Devices, device(windows.ByteSliceToString(d.ID[:]), windows.ByteSliceToString(d.Model[:]), d.Type, leds))
	}
	sort.Slice(state.Devices, func(i, j int) bool { return state.Devices[i].ID < state.Devices[j].ID })
	return state
}

func (s *sdk) close() {
	if s.started {
		s.disconnect.Call()
		s.started = false
	}
	s.state.Store(0)
}
