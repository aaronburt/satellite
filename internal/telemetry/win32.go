//go:build windows

package telemetry

import (
	"fmt"
	"math"
	"net"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

var (
	modUser32   = syscall.NewLazyDLL("user32.dll")
	modKernel32 = syscall.NewLazyDLL("kernel32.dll")
	modIphlpapi = syscall.NewLazyDLL("iphlpapi.dll")
	modOle32    = syscall.NewLazyDLL("ole32.dll")
	modWlanapi  = syscall.NewLazyDLL("wlanapi.dll")

	procGetLastInputInfo                 = modUser32.NewProc("GetLastInputInfo")
	procGetForegroundWindow              = modUser32.NewProc("GetForegroundWindow")
	procGetWindowTextW                   = modUser32.NewProc("GetWindowTextW")
	procGetWindowThreadProcessId         = modUser32.NewProc("GetWindowThreadProcessId")
	procGetWindowRect                    = modUser32.NewProc("GetWindowRect")
	procGetSystemMetrics                 = modUser32.NewProc("GetSystemMetrics")
	procMonitorFromWindow                = modUser32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW                  = modUser32.NewProc("GetMonitorInfoW")
	procOpenInputDesktop                 = modUser32.NewProc("OpenInputDesktop")
	procCloseDesktop                     = modUser32.NewProc("CloseDesktop")
	procSendMessageW                     = modUser32.NewProc("SendMessageW")
	procRegisterPowerSettingNotification = modUser32.NewProc("RegisterPowerSettingNotification")

	procCoInitializeEx   = modOle32.NewProc("CoInitializeEx")
	procCoUninitialize   = modOle32.NewProc("CoUninitialize")
	procCoCreateInstance = modOle32.NewProc("CoCreateInstance")
	procPropVariantClear = modOle32.NewProc("PropVariantClear")

	procWlanOpenHandle     = modWlanapi.NewProc("WlanOpenHandle")
	procWlanCloseHandle    = modWlanapi.NewProc("WlanCloseHandle")
	procWlanEnumInterfaces = modWlanapi.NewProc("WlanEnumInterfaces")
	procWlanQueryInterface = modWlanapi.NewProc("WlanQueryInterface")
	procWlanFreeMemory     = modWlanapi.NewProc("WlanFreeMemory")

	procGetTickCount64             = modKernel32.NewProc("GetTickCount64")
	procGetSystemTimes             = modKernel32.NewProc("GetSystemTimes")
	procGlobalMemoryStatusEx       = modKernel32.NewProc("GlobalMemoryStatusEx")
	procGetSystemPowerStatus       = modKernel32.NewProc("GetSystemPowerStatus")
	procOpenProcess                = modKernel32.NewProc("OpenProcess")
	procCloseHandle                = modKernel32.NewProc("CloseHandle")
	procQueryFullProcessImageNameW = modKernel32.NewProc("QueryFullProcessImageNameW")
	procGetLogicalDrives           = modKernel32.NewProc("GetLogicalDrives")
	procGetDiskFreeSpaceExW        = modKernel32.NewProc("GetDiskFreeSpaceExW")
	procSetProcessWorkingSetSize   = modKernel32.NewProc("SetProcessWorkingSetSize")
	procGetCurrentProcess          = modKernel32.NewProc("GetCurrentProcess")

	procGetIfTable2  = modIphlpapi.NewProc("GetIfTable2")
	procFreeMibTable = modIphlpapi.NewProc("FreeMibTable")
)

type lastInputInfo struct {
	cbSize uint32
	dwTime uint32
}

type systemPowerStatus struct {
	ACLineStatus        byte
	BatteryFlag         byte
	BatteryLifePercent  byte
	SystemStatusFlag    byte
	BatteryLifeTime     uint32
	BatteryFullLifeTime uint32
}

type rect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type memoryStatusEx struct {
	cbSize                  uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

type filetime struct {
	dwLowDateTime  uint32
	dwHighDateTime uint32
}

func (ft filetime) toUint64() uint64 {
	return uint64(ft.dwHighDateTime)<<32 | uint64(ft.dwLowDateTime)
}

type MIB_IF_ROW2 struct {
	InterfaceLuid               uint64
	InterfaceIndex              uint32
	InterfaceGuid               [16]byte
	Alias                       [257]uint16
	Description                 [257]uint16
	PhysicalAddressLength       uint32
	PhysicalAddress             [32]byte
	PermanentPhysicalAddress    [32]byte
	Mtu                         uint32
	Type                        uint32
	TunnelType                  uint32
	MediaType                   uint32
	PhysicalMediumType          uint32
	AccessType                  uint32
	DirectionType               uint32
	InterfaceAndOperStatusFlags byte
	OperStatus                  uint32
	AdminStatus                 uint32
	MediaConnectState           uint32
	NetworkGuid                 [16]byte
	ConnectionType              uint32
	Padding                     [4]byte
	TransmitLinkSpeed           uint64
	ReceiveLinkSpeed            uint64
	InOctets                    uint64
	InUcastPkts                 uint64
	InNUcastPkts                uint64
	InDiscards                  uint64
	InErrors                    uint64
	InUnknownProtos             uint64
	InUcastOctets               uint64
	InMulticastOctets           uint64
	InBroadcastOctets           uint64
	OutOctets                   uint64
	OutUcastPkts                uint64
	OutNUcastPkts               uint64
	OutDiscards                 uint64
	OutErrors                   uint64
	OutUcastOctets              uint64
	OutMulticastOctets          uint64
	OutBroadcastOctets          uint64
	OutQLen                     uint64
}

type MIB_IF_TABLE2 struct {
	NumEntries uint32
	Padding    [4]byte
	Table      [1]MIB_IF_ROW2
}

const (
	PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
	SM_CXSCREEN                       = 0
	SM_CYSCREEN                       = 1
	DESKTOP_SWITCHDESKTOP             = 0x0100
)

func TrimWorkingSet() {
	hProc, _, _ := procGetCurrentProcess.Call()
	if hProc != 0 {
		_, _, _ = procSetProcessWorkingSetSize.Call(hProc, ^uintptr(0), ^uintptr(0))
	}
}

func GetCPUTimes() (idle, kernel, user uint64, err error) {
	var ftIdle, ftKernel, ftUser filetime
	r, _, err := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&ftIdle)),
		uintptr(unsafe.Pointer(&ftKernel)),
		uintptr(unsafe.Pointer(&ftUser)),
	)
	if r == 0 {
		return 0, 0, 0, err
	}
	return ftIdle.toUint64(), ftKernel.toUint64(), ftUser.toUint64(), nil
}

func GetRAMUsage() (usedGB, totalGB, percent float64, err error) {
	var ms memoryStatusEx
	ms.cbSize = uint32(unsafe.Sizeof(ms))
	r, _, err := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ms)))
	if r == 0 {
		return 0, 0, 0, err
	}

	totalGB = math.Round((float64(ms.ullTotalPhys)/1024/1024/1024)*100) / 100
	usedGB = math.Round((float64(ms.ullTotalPhys-ms.ullAvailPhys)/1024/1024/1024)*100) / 100
	percent = math.Round(float64(ms.dwMemoryLoad)*10) / 10
	return usedGB, totalGB, percent, nil
}

func GetNetworkOctets() (rx, tx uint64, err error) {
	var pTable *MIB_IF_TABLE2
	r, _, err := procGetIfTable2.Call(uintptr(unsafe.Pointer(&pTable)))
	if r != 0 || pTable == nil {
		return 0, 0, err
	}
	defer procFreeMibTable.Call(uintptr(unsafe.Pointer(pTable)))

	basePtr := unsafe.Pointer(&pTable.Table[0])
	rowSize := unsafe.Sizeof(MIB_IF_ROW2{})

	for i := uint32(0); i < pTable.NumEntries; i++ {
		row := (*MIB_IF_ROW2)(unsafe.Add(basePtr, uintptr(i)*rowSize))
		if row.Type != 24 {
			rx += row.InOctets
			tx += row.OutOctets
		}
	}
	return rx, tx, nil
}

func GetSystemUptimeSeconds() uint64 {
	tick, _, _ := procGetTickCount64.Call()
	return uint64(tick) / 1000
}

func GetUserIdleSeconds() (uint64, bool) {
	var lii lastInputInfo
	lii.cbSize = uint32(unsafe.Sizeof(lii))

	r, _, _ := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&lii)))
	if r == 0 {
		return 0, false
	}

	tick, _, _ := procGetTickCount64.Call()
	elapsedMs := uint64(uint32(tick) - lii.dwTime)
	idleSec := elapsedMs / 1000
	return idleSec, true
}

func GetBatteryAndPower() (hasBattery bool, batteryPercent int, acPlugged bool) {
	var sps systemPowerStatus
	r, _, _ := procGetSystemPowerStatus.Call(uintptr(unsafe.Pointer(&sps)))
	if r == 0 {
		return false, 0, false
	}

	if sps.BatteryFlag&128 != 0 || sps.BatteryLifePercent == 255 {
		return false, 0, sps.ACLineStatus == 1
	}

	pct := int(sps.BatteryLifePercent)
	if pct > 100 {
		pct = 100
	}

	return true, pct, sps.ACLineStatus == 1
}

func GetActiveWindowAndProcess() (processName, windowTitle string) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return "", ""
	}

	var buf [256]uint16
	lenTitle, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if lenTitle > 0 {
		windowTitle = syscall.UTF16ToString(buf[:lenTitle])
	}

	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return "", windowTitle
	}

	hProcess, _, _ := procOpenProcess.Call(PROCESS_QUERY_LIMITED_INFORMATION, 0, uintptr(pid))
	if hProcess != 0 {
		defer procCloseHandle.Call(hProcess)

		var pathBuf [1024]uint16
		size := uint32(len(pathBuf))
		ret, _, _ := procQueryFullProcessImageNameW.Call(hProcess, 0, uintptr(unsafe.Pointer(&pathBuf[0])), uintptr(unsafe.Pointer(&size)))
		if ret != 0 {
			fullPath := syscall.UTF16ToString(pathBuf[:size])
			processName = filepath.Base(fullPath)
		}
	}

	if processName == "" && windowTitle != "" {
		parts := strings.Split(windowTitle, " - ")
		processName = parts[len(parts)-1]
	}

	return processName, windowTitle
}

func IsSessionLocked() bool {
	hDesk, _, _ := procOpenInputDesktop.Call(0, 0, DESKTOP_SWITCHDESKTOP)
	if hDesk == 0 {
		return true
	}
	procCloseDesktop.Call(hDesk)
	return false
}

const monitorDefaultToNearest = 2

type monitorInfo struct {
	cbSize    uint32
	rcMonitor rect
	rcWork    rect
	dwFlags   uint32
}

func IsFullscreenActive() bool {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return false
	}

	var r rect
	ret, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	if ret == 0 {
		return false
	}

	hMonitor, _, _ := procMonitorFromWindow.Call(hwnd, monitorDefaultToNearest)
	if hMonitor == 0 {
		return false
	}

	var mi monitorInfo
	mi.cbSize = uint32(unsafe.Sizeof(mi))
	ret, _, _ = procGetMonitorInfoW.Call(hMonitor, uintptr(unsafe.Pointer(&mi)))
	if ret == 0 {
		return false
	}

	return r.Left <= mi.rcMonitor.Left &&
		r.Top <= mi.rcMonitor.Top &&
		r.Right >= mi.rcMonitor.Right &&
		r.Bottom >= mi.rcMonitor.Bottom
}

func IsMicrophoneInUse() bool {
	checkConsentKey := func(root registry.Key, path string) bool {
		k, err := registry.OpenKey(root, path, registry.ENUMERATE_SUB_KEYS|registry.READ)
		if err != nil {
			return false
		}
		defer k.Close()

		subkeys, err := k.ReadSubKeyNames(-1)
		if err != nil {
			return false
		}

		for _, sub := range subkeys {
			sk, err := registry.OpenKey(k, sub, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			val, _, err := sk.GetIntegerValue("LastUsedTimeStop")
			sk.Close()
			if err == nil && val == 0 {
				return true
			}
		}
		return false
	}

	if checkConsentKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\CapabilityAccessManager\ConsentStore\microphone\NonPackaged`) {
		return true
	}
	if checkConsentKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\CapabilityAccessManager\ConsentStore\microphone`) {
		return true
	}
	return false
}

func GetWindowsTheme() string {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err == nil {
		defer k.Close()
		val, _, err := k.GetIntegerValue("AppsUseLightTheme")
		if err == nil {
			if val == 1 {
				return "light"
			}
			return "dark"
		}
	}
	return "dark"
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

	var fallbackIP string
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			ip := ipnet.IP.To4()
			if ip != nil {
				ipStr := ip.String()
				if strings.HasPrefix(ipStr, "169.254.") {
					continue
				}
				if ip.IsPrivate() {
					return ipStr
				}
				if fallbackIP == "" {
					fallbackIP = ipStr
				}
			}
		}
	}
	return fallbackIP
}

func GetAllDrives() []DiskInfo {
	bitmask, _, _ := procGetLogicalDrives.Call()
	var drives []DiskInfo

	for i := 0; i < 26; i++ {
		if (bitmask & (1 << i)) != 0 {
			driveLetter := fmt.Sprintf("%c:\\", 'A'+i)
			ptr, err := syscall.UTF16PtrFromString(driveLetter)
			if err != nil {
				continue
			}

			var freeBytes, totalBytes, totalFreeBytes uint64
			r, _, _ := procGetDiskFreeSpaceExW.Call(
				uintptr(unsafe.Pointer(ptr)),
				uintptr(unsafe.Pointer(&freeBytes)),
				uintptr(unsafe.Pointer(&totalBytes)),
				uintptr(unsafe.Pointer(&totalFreeBytes)),
			)

			if r != 0 && totalBytes > 0 {
				usedBytes := totalBytes - freeBytes
				usedGB := math.Round((float64(usedBytes)/1024/1024/1024)*10) / 10
				totalGB := math.Round((float64(totalBytes)/1024/1024/1024)*10) / 10
				usedPct := math.Round((float64(usedBytes)/float64(totalBytes))*1000) / 10

				drives = append(drives, DiskInfo{
					Mount:       strings.TrimSuffix(driveLetter, "\\"),
					UsedGB:      usedGB,
					TotalGB:     totalGB,
					UsedPercent: usedPct,
				})
			}
		}
	}
	return drives
}

func IsRebootPending() bool {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\WindowsUpdate\Auto Update\RebootRequired`, registry.QUERY_VALUE)
	if err == nil {
		k.Close()
		return true
	}

	k2, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Component Based Servicing\RebootPending`, registry.QUERY_VALUE)
	if err == nil {
		k2.Close()
		return true
	}

	return false
}

var (
	modCombase = syscall.NewLazyDLL("combase.dll")

	procRoInitialize              = modCombase.NewProc("RoInitialize")
	procRoUninitialize            = modCombase.NewProc("RoUninitialize")
	procRoGetActivationFactory    = modCombase.NewProc("RoGetActivationFactory")
	procWindowsCreateString       = modCombase.NewProc("WindowsCreateString")
	procWindowsDeleteString       = modCombase.NewProc("WindowsDeleteString")
	procWindowsGetStringRawBuffer = modCombase.NewProc("WindowsGetStringRawBuffer")
)

type winGUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var (
	guidIAsyncInfo   = winGUID{0x00000036, 0x0000, 0x0000, [8]byte{0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	guidGSMTCStatics = winGUID{0x2050c4ee, 0x11a0, 0x57de, [8]byte{0xae, 0xd7, 0xc9, 0x7c, 0x70, 0x33, 0x82, 0x45}}
)

func hstringFromString(s string) (uintptr, error) {
	u16, err := syscall.UTF16FromString(s)
	if err != nil {
		return 0, err
	}
	var hs uintptr
	r, _, _ := procWindowsCreateString.Call(
		uintptr(unsafe.Pointer(&u16[0])),
		uintptr(len(u16)-1),
		uintptr(unsafe.Pointer(&hs)),
	)
	if int32(r) < 0 {
		return 0, fmt.Errorf("WindowsCreateString: 0x%08x", uint32(r))
	}
	return hs, nil
}

func toUnsafePointer(p uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&p))
}

func getVtableFunc(obj unsafe.Pointer, index int) uintptr {
	vtblPtr := *(*uintptr)(obj)
	entryPtr := vtblPtr + uintptr(index)*unsafe.Sizeof(uintptr(0))
	return *(*uintptr)(toUnsafePointer(entryPtr))
}

func stringFromHstring(hs uintptr) string {
	if hs == 0 {
		return ""
	}
	defer procWindowsDeleteString.Call(hs)

	var length uint32
	ptr, _, _ := procWindowsGetStringRawBuffer.Call(hs, uintptr(unsafe.Pointer(&length)))
	if ptr == 0 || length == 0 {
		return ""
	}
	buf := unsafe.Slice((*uint16)(toUnsafePointer(ptr)), length)
	return syscall.UTF16ToString(buf)
}

func comRelease(obj unsafe.Pointer) {
	if obj == nil {
		return
	}
	fn := getVtableFunc(obj, 2)
	syscall.SyscallN(fn, uintptr(obj))
}

func comQueryInterface(obj unsafe.Pointer, riid *winGUID) (unsafe.Pointer, error) {
	if obj == nil {
		return nil, fmt.Errorf("null com object")
	}
	fn := getVtableFunc(obj, 0)
	var out unsafe.Pointer
	r, _, _ := syscall.SyscallN(fn, uintptr(obj), uintptr(unsafe.Pointer(riid)), uintptr(unsafe.Pointer(&out)))
	if int32(r) < 0 {
		return nil, fmt.Errorf("QueryInterface: 0x%08x", uint32(r))
	}
	return out, nil
}

func awaitAsyncOperation(asyncOp unsafe.Pointer, resultIfaceIdx int) (unsafe.Pointer, error) {
	asyncInfo, err := comQueryInterface(asyncOp, &guidIAsyncInfo)
	if err != nil {
		return nil, err
	}
	defer comRelease(asyncInfo)

	fnGetStatus := getVtableFunc(asyncInfo, 7)

	for i := 0; i < 50; i++ {
		var status int32
		syscall.SyscallN(fnGetStatus, uintptr(asyncInfo), uintptr(unsafe.Pointer(&status)))
		if status == 1 {
			break
		}
		if status == 2 || status == 3 {
			return nil, fmt.Errorf("async operation error: %d", status)
		}
		time.Sleep(10 * time.Millisecond)
	}

	fnGetResults := getVtableFunc(asyncOp, resultIfaceIdx)
	var result unsafe.Pointer
	r, _, _ := syscall.SyscallN(fnGetResults, uintptr(asyncOp), uintptr(unsafe.Pointer(&result)))
	if int32(r) < 0 {
		return nil, fmt.Errorf("GetResults: 0x%08x", uint32(r))
	}
	return result, nil
}

func GetActiveMediaInfo() MediaInfo {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	rInit, _, _ := procRoInitialize.Call(1)
	if int32(rInit) >= 0 {
		defer procRoUninitialize.Call()
	}

	media := MediaInfo{
		Status: "idle",
	}

	classHs, err := hstringFromString("Windows.Media.Control.GlobalSystemMediaTransportControlsSessionManager")
	if err != nil {
		return media
	}
	defer procWindowsDeleteString.Call(classHs)

	var statics unsafe.Pointer
	r, _, _ := procRoGetActivationFactory.Call(
		classHs,
		uintptr(unsafe.Pointer(&guidGSMTCStatics)),
		uintptr(unsafe.Pointer(&statics)),
	)
	if int32(r) < 0 || statics == nil {
		return media
	}
	defer comRelease(statics)

	fnRequestAsync := getVtableFunc(statics, 6)
	var asyncOp unsafe.Pointer
	r, _, _ = syscall.SyscallN(fnRequestAsync, uintptr(statics), uintptr(unsafe.Pointer(&asyncOp)))
	if int32(r) < 0 || asyncOp == nil {
		return media
	}
	defer comRelease(asyncOp)

	mgr, err := awaitAsyncOperation(asyncOp, 8)
	if err != nil || mgr == nil {
		return media
	}
	defer comRelease(mgr)

	fnGetCurrentSession := getVtableFunc(mgr, 6)
	var session unsafe.Pointer
	r, _, _ = syscall.SyscallN(fnGetCurrentSession, uintptr(mgr), uintptr(unsafe.Pointer(&session)))
	if int32(r) < 0 || session == nil {
		return media
	}
	defer comRelease(session)

	var appHs uintptr
	fnGetSourceAppId := getVtableFunc(session, 6)
	syscall.SyscallN(fnGetSourceAppId, uintptr(session), uintptr(unsafe.Pointer(&appHs)))
	rawApp := stringFromHstring(appHs)
	if rawApp != "" {
		parts := strings.Split(rawApp, "!")
		appName := parts[len(parts)-1]
		media.AppID = strings.TrimSuffix(appName, ".exe")
	}

	var playbackInfo unsafe.Pointer
	fnGetPlaybackInfo := getVtableFunc(session, 9)
	syscall.SyscallN(fnGetPlaybackInfo, uintptr(session), uintptr(unsafe.Pointer(&playbackInfo)))
	if playbackInfo != nil {
		fnGetPlaybackStatus := getVtableFunc(playbackInfo, 7)
		var pbStatus int32
		syscall.SyscallN(fnGetPlaybackStatus, uintptr(playbackInfo), uintptr(unsafe.Pointer(&pbStatus)))
		comRelease(playbackInfo)

		switch pbStatus {
		case 4:
			media.Status = "playing"
		case 5:
			media.Status = "paused"
		case 3:
			media.Status = "stopped"
		default:
			media.Status = "idle"
		}
	}

	var propAsyncOp unsafe.Pointer
	fnTryGetMediaPropertiesAsync := getVtableFunc(session, 7)
	r, _, _ = syscall.SyscallN(fnTryGetMediaPropertiesAsync, uintptr(session), uintptr(unsafe.Pointer(&propAsyncOp)))
	if int32(r) >= 0 && propAsyncOp != nil {
		defer comRelease(propAsyncOp)

		props, err := awaitAsyncOperation(propAsyncOp, 8)
		if err == nil && props != nil {
			defer comRelease(props)

			var titleHs, artistHs, albumHs uintptr
			syscall.SyscallN(getVtableFunc(props, 6), uintptr(props), uintptr(unsafe.Pointer(&titleHs)))
			syscall.SyscallN(getVtableFunc(props, 9), uintptr(props), uintptr(unsafe.Pointer(&artistHs)))
			syscall.SyscallN(getVtableFunc(props, 10), uintptr(props), uintptr(unsafe.Pointer(&albumHs)))

			media.Title = stringFromHstring(titleHs)
			media.Artist = stringFromHstring(artistHs)
			media.Album = stringFromHstring(albumHs)
		}
	}

	return media
}

const (
	HWND_BROADCAST         = 0xffff
	WM_SYSCOMMAND          = 0x0112
	SC_MONITORPOWER        = 0xf170
	DEVICE_NOTIFY_CALLBACK = 2
)

var (
	clsidMMDeviceEnumerator = winGUID{0xBCDE0395, 0xE52F, 0x467C, [8]byte{0x8E, 0x3D, 0xC4, 0x57, 0x92, 0x91, 0x69, 0x2E}}
	iidIMMDeviceEnumerator  = winGUID{0xA95664D2, 0x9614, 0x4F35, [8]byte{0xA7, 0x46, 0xDE, 0x8D, 0xB6, 0x36, 0x17, 0xE6}}
	pkeyDeviceFriendlyName  = propertyKey{
		fmtid: winGUID{0xA45C254E, 0xDF1C, 0x4EFD, [8]byte{0x80, 0x20, 0x67, 0xD1, 0x46, 0xA8, 0x50, 0xE0}},
		pid:   14,
	}
	guidConsoleDisplayState = winGUID{0x6FE69556, 0x704A, 0x47A0, [8]byte{0xA2, 0xA2, 0x67, 0x75, 0x8D, 0xA0, 0x63, 0x19}}
)

type propertyKey struct {
	fmtid winGUID
	pid   uint32
}

type propVariant struct {
	vt       uint16
	reserved [6]byte
	data     [2]uintptr
}

type deviceNotifySubscribeParameters struct {
	callback uintptr
	context  uintptr
}

type powerBroadcastSetting struct {
	PowerSetting winGUID
	DataLength   uint32
	Data         [1]byte
}

var (
	displayStateInitOnce sync.Once
	displayPowerMu       sync.RWMutex
	displayPowerActive   = true
)

func displayPowerCallback(context uintptr, typ uint32, setting uintptr) uintptr {
	if setting == 0 {
		return 0
	}
	pbs := (*powerBroadcastSetting)(toUnsafePointer(setting))
	if pbs.PowerSetting == guidConsoleDisplayState && pbs.DataLength >= 1 {
		displayPowerMu.Lock()
		displayPowerActive = pbs.Data[0] != 0
		displayPowerMu.Unlock()
	}
	return 0
}

func initDisplayPowerTracking() {
	displayStateInitOnce.Do(func() {
		cb := syscall.NewCallback(displayPowerCallback)
		params := deviceNotifySubscribeParameters{
			callback: cb,
			context:  0,
		}
		procRegisterPowerSettingNotification.Call(
			uintptr(unsafe.Pointer(&params)),
			uintptr(unsafe.Pointer(&guidConsoleDisplayState)),
			DEVICE_NOTIFY_CALLBACK,
		)
	})
}

func IsDisplayPowered() bool {
	initDisplayPowerTracking()
	displayPowerMu.RLock()
	defer displayPowerMu.RUnlock()
	return displayPowerActive
}

func SleepDisplays() error {
	procSendMessageW.Call(HWND_BROADCAST, WM_SYSCOMMAND, SC_MONITORPOWER, 2)
	displayPowerMu.Lock()
	displayPowerActive = false
	displayPowerMu.Unlock()
	return nil
}

func GetWebcamStatus() (bool, string) {
	checkKey := func(root registry.Key, path string) (bool, string) {
		k, err := registry.OpenKey(root, path, registry.ENUMERATE_SUB_KEYS|registry.READ)
		if err != nil {
			return false, ""
		}
		defer k.Close()

		subkeys, err := k.ReadSubKeyNames(-1)
		if err != nil {
			return false, ""
		}

		for _, sub := range subkeys {
			sk, err := registry.OpenKey(k, sub, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			val, _, err := sk.GetIntegerValue("LastUsedTimeStop")
			sk.Close()
			if err == nil && val == 0 {
				normalized := strings.ReplaceAll(sub, "#", `\`)
				base := filepath.Base(normalized)
				appName := sub
				if base != "" && base != "." {
					appName = strings.TrimSuffix(base, ".exe")
				}
				return true, appName
			}
		}
		return false, ""
	}

	if inUse, app := checkKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\CapabilityAccessManager\ConsentStore\webcam\NonPackaged`); inUse {
		return true, app
	}
	if inUse, app := checkKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\CapabilityAccessManager\ConsentStore\webcam`); inUse {
		return true, app
	}
	return false, ""
}

func utf16PtrToString(ptr uintptr, maxLen int) string {
	if ptr == 0 {
		return ""
	}
	buf := unsafe.Slice((*uint16)(toUnsafePointer(ptr)), maxLen)
	for i, v := range buf {
		if v == 0 {
			return syscall.UTF16ToString(buf[:i])
		}
	}
	return syscall.UTF16ToString(buf)
}

func GetActiveAudioOutputDevice() string {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	const hresultSOK = 0
	rInit, _, _ := procCoInitializeEx.Call(0, 0)
	if int32(rInit) == hresultSOK {
		defer procCoUninitialize.Call()
	}

	var enumerator unsafe.Pointer
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidMMDeviceEnumerator)),
		0,
		1,
		uintptr(unsafe.Pointer(&iidIMMDeviceEnumerator)),
		uintptr(unsafe.Pointer(&enumerator)),
	)
	if int32(hr) < 0 || enumerator == nil {
		return ""
	}
	defer comRelease(enumerator)

	fnGetDefaultEndpoint := getVtableFunc(enumerator, 4)
	var device unsafe.Pointer
	r, _, _ := syscall.SyscallN(fnGetDefaultEndpoint, uintptr(enumerator), 0, 1, uintptr(unsafe.Pointer(&device)))
	if int32(r) < 0 || device == nil {
		return ""
	}
	defer comRelease(device)

	fnOpenPropertyStore := getVtableFunc(device, 4)
	var propStore unsafe.Pointer
	r, _, _ = syscall.SyscallN(fnOpenPropertyStore, uintptr(device), 0, uintptr(unsafe.Pointer(&propStore)))
	if int32(r) < 0 || propStore == nil {
		return ""
	}
	defer comRelease(propStore)

	fnGetValue := getVtableFunc(propStore, 5)
	var pv propVariant
	r, _, _ = syscall.SyscallN(fnGetValue, uintptr(propStore), uintptr(unsafe.Pointer(&pkeyDeviceFriendlyName)), uintptr(unsafe.Pointer(&pv)))
	name := ""
	if int32(r) >= 0 && pv.vt == 31 && pv.data[0] != 0 {
		name = utf16PtrToString(pv.data[0], 256)
	}
	procPropVariantClear.Call(uintptr(unsafe.Pointer(&pv)))
	return name
}

type wlanInterfaceInfo struct {
	InterfaceGuid           winGUID
	strInterfaceDescription [256]uint16
	isState                 uint32
}

type wlanInterfaceInfoList struct {
	dwNumberOfItems uint32
	dwIndex         uint32
	InterfaceInfo   [1]wlanInterfaceInfo
}

type dot11Ssid struct {
	uSSIDLength uint32
	ucSSID      [32]byte
}

type wlanAssociationAttributes struct {
	dot11Ssid         dot11Ssid
	dot11BssType      uint32
	dot11Bssid        [6]byte
	_                 [2]byte
	dot11PhyType      uint32
	uDot11PhyIndex    uint32
	wlanSignalQuality uint32
	ulRxRate          uint32
	ulTxRate          uint32
}

type wlanConnectionAttributes struct {
	isState                   uint32
	wlanConnectionMode        uint32
	strProfileName            [256]uint16
	wlanAssociationAttributes wlanAssociationAttributes
}

func GetWifiStatus() (string, *int) {
	var handle uintptr
	var negVer uint32
	r, _, _ := procWlanOpenHandle.Call(2, 0, uintptr(unsafe.Pointer(&negVer)), uintptr(unsafe.Pointer(&handle)))
	if r != 0 || handle == 0 {
		return detectWiredOrDisconnected()
	}
	defer procWlanCloseHandle.Call(handle, 0)

	var pList unsafe.Pointer
	rList, _, _ := procWlanEnumInterfaces.Call(handle, 0, uintptr(unsafe.Pointer(&pList)))
	if rList != 0 || pList == nil {
		return detectWiredOrDisconnected()
	}
	defer procWlanFreeMemory.Call(uintptr(pList))

	list := (*wlanInterfaceInfoList)(pList)
	if list.dwNumberOfItems == 0 {
		return detectWiredOrDisconnected()
	}

	items := unsafe.Slice(&list.InterfaceInfo[0], list.dwNumberOfItems)
	for _, item := range items {
		if item.isState != 1 {
			continue
		}
		var dataSize uint32
		var pData unsafe.Pointer
		var opcodeType uint32
		rQuery, _, _ := procWlanQueryInterface.Call(
			handle,
			uintptr(unsafe.Pointer(&item.InterfaceGuid)),
			7,
			0,
			uintptr(unsafe.Pointer(&dataSize)),
			uintptr(unsafe.Pointer(&pData)),
			uintptr(unsafe.Pointer(&opcodeType)),
		)
		if rQuery != 0 || pData == nil {
			continue
		}
		conn := (*wlanConnectionAttributes)(pData)
		ssidLen := conn.wlanAssociationAttributes.dot11Ssid.uSSIDLength
		if ssidLen > 32 {
			ssidLen = 32
		}
		ssid := string(conn.wlanAssociationAttributes.dot11Ssid.ucSSID[:ssidLen])
		quality := int(conn.wlanAssociationAttributes.wlanSignalQuality)
		procWlanFreeMemory.Call(uintptr(pData))
		if ssid != "" {
			return ssid, &quality
		}
	}

	return detectWiredOrDisconnected()
}

func detectWiredOrDisconnected() (string, *int) {
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
				if ipNet.IP.To4() != nil {
					return "Wired / Ethernet", nil
				}
			}
		}
	}
	return "Disconnected", nil
}
