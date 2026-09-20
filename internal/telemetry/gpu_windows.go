//go:build windows

package telemetry

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type GPUInfo struct {
	Index            int      `json:"index"`
	Name             string   `json:"name"`
	Vendor           string   `json:"vendor"`
	CoreUsagePercent *float64 `json:"core_usage_percent,omitempty"`
	MemoryUsedMB     *float64 `json:"memory_used_mb,omitempty"`
	MemoryTotalMB    *float64 `json:"memory_total_mb,omitempty"`
	MemoryPercent    *float64 `json:"memory_percent,omitempty"`
	TemperatureC     *float64 `json:"temperature_c,omitempty"`
	PowerWatts       *float64 `json:"power_watts,omitempty"`
	FanSpeedPercent  *float64 `json:"fan_speed_percent,omitempty"`
}

type dxgiAdapterInfo struct {
	index     int
	name      string
	vendor    string
	vramTotal uint64
	luidStr   string
}

type dxgiLuid struct {
	LowPart  uint32
	HighPart int32
}

type dxgiAdapterDesc1 struct {
	Description           [128]uint16
	VendorId              uint32
	DeviceId              uint32
	SubSysId              uint32
	Revision              uint32
	DedicatedVideoMemory  uintptr
	DedicatedSystemMemory uintptr
	SharedSystemMemory    uintptr
	AdapterLuid           dxgiLuid
	Flags                 uint32
}

type pdhCounterValueDouble struct {
	CStatus     uint32
	DoubleValue float64
}

type pdhCounterItemDouble struct {
	SzName   *uint16
	FmtValue pdhCounterValueDouble
}

type pdhCounterValueLarge struct {
	CStatus    uint32
	LargeValue int64
}

type pdhCounterItemLarge struct {
	SzName   *uint16
	FmtValue pdhCounterValueLarge
}

type nvmlUtilization struct {
	GPU    uint32
	Memory uint32
}

type nvmlMemory struct {
	Total uint64
	Free  uint64
	Used  uint64
}

type nvmlDeviceStats struct {
	index       uint32
	name        string
	loadPercent float64
	memUsedMB   float64
	memTotalMB  float64
	tempC       float64
	powerW      float64
	fanPct      float64
	hasTemp     bool
	hasPower    bool
	hasFan      bool
}

const (
	pdhFmtDouble = 0x00000200
	pdhFmtLarge  = 0x00000400
)

var (
	guidIDXGIFactory1 = windows.GUID{Data1: 0x770aae78, Data2: 0xf26f, Data3: 0x4dba, Data4: [8]byte{0xa8, 0x29, 0x25, 0x3c, 0x83, 0xd1, 0xb3, 0x87}}
	globalPdhGpu      *pdhGpuCollector
	globalPdhOnce     sync.Once
	globalNvml        *nvmlCollector
	globalNvmlOnce    sync.Once
)

func vendorFromID(vendorID uint32) string {
	switch vendorID {
	case 0x10DE:
		return "NVIDIA"
	case 0x1002:
		return "AMD"
	case 0x8086:
		return "Intel"
	case 0x1414:
		return "Microsoft"
	default:
		return "Unknown"
	}
}

func getDXGIAdapters() []dxgiAdapterInfo {
	dxgi := windows.NewLazySystemDLL("dxgi.dll")
	createFactory1 := dxgi.NewProc("CreateDXGIFactory1")
	if createFactory1.Find() != nil {
		return nil
	}

	var factory uintptr
	ret, _, _ := createFactory1.Call(
		uintptr(unsafe.Pointer(&guidIDXGIFactory1)),
		uintptr(unsafe.Pointer(&factory)),
	)
	if ret != 0 {
		return nil
	}

	factoryVtbl := *(**[14]uintptr)(unsafe.Pointer(factory))
	enumAdapters1 := factoryVtbl[12]
	releaseFactory := factoryVtbl[2]
	defer syscall.SyscallN(releaseFactory, factory)

	var list []dxgiAdapterInfo
	for i := uintptr(0); ; i++ {
		var adapter uintptr
		ret, _, _ = syscall.SyscallN(enumAdapters1, factory, i, uintptr(unsafe.Pointer(&adapter)))
		if ret != 0 {
			break
		}

		adapterVtbl := *(**[11]uintptr)(unsafe.Pointer(adapter))
		releaseAdapter := adapterVtbl[2]
		getDesc1 := adapterVtbl[10]

		var desc dxgiAdapterDesc1
		ret, _, _ = syscall.SyscallN(getDesc1, adapter, uintptr(unsafe.Pointer(&desc)))
		if ret == 0 {
			isSoftware := (desc.Flags & 2) != 0
			if !isSoftware {
				name := strings.TrimRight(windows.UTF16ToString(desc.Description[:]), "\x00")
				luidStr := strings.ToLower(fmt.Sprintf("luid_0x%08x_0x%08x", uint32(desc.AdapterLuid.HighPart), desc.AdapterLuid.LowPart))
				list = append(list, dxgiAdapterInfo{
					index:     len(list),
					name:      name,
					vendor:    vendorFromID(desc.VendorId),
					vramTotal: uint64(desc.DedicatedVideoMemory),
					luidStr:   luidStr,
				})
			}
		}
		syscall.SyscallN(releaseAdapter, adapter)
	}
	return list
}

type pdhGpuCollector struct {
	mu           sync.Mutex
	query        windows.Handle
	engineCnt    windows.Handle
	memDedCnt    windows.Handle
	hasCollected bool
}

func initPDHGpuCollector() *pdhGpuCollector {
	pdh := windows.NewLazySystemDLL("pdh.dll")
	openQuery := pdh.NewProc("PdhOpenQueryW")
	addCounter := pdh.NewProc("PdhAddEnglishCounterW")

	var query windows.Handle
	ret, _, _ := openQuery.Call(0, 0, uintptr(unsafe.Pointer(&query)))
	if ret != 0 {
		return nil
	}

	var engineCnt, memDedCnt windows.Handle
	enginePath, _ := windows.UTF16PtrFromString(`\GPU Engine(*)\Utilization Percentage`)
	addCounter.Call(uintptr(query), uintptr(unsafe.Pointer(enginePath)), 0, uintptr(unsafe.Pointer(&engineCnt)))

	memDedPath, _ := windows.UTF16PtrFromString(`\GPU Adapter Memory(*)\Dedicated Usage`)
	addCounter.Call(uintptr(query), uintptr(unsafe.Pointer(memDedPath)), 0, uintptr(unsafe.Pointer(&memDedCnt)))

	return &pdhGpuCollector{
		query:     query,
		engineCnt: engineCnt,
		memDedCnt: memDedCnt,
	}
}

func (p *pdhGpuCollector) collect(trackedLuids []string) (map[string]float64, map[string]int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	engineMap := make(map[string]float64)
	memMap := make(map[string]int64)

	for _, luid := range trackedLuids {
		engineMap[luid] = 0
	}

	if p == nil || p.query == 0 {
		return engineMap, memMap
	}

	pdh := windows.NewLazySystemDLL("pdh.dll")
	collectProc := pdh.NewProc("PdhCollectQueryData")
	getArrayProc := pdh.NewProc("PdhGetFormattedCounterArrayW")

	collectProc.Call(uintptr(p.query))
	if !p.hasCollected {
		p.hasCollected = true
		return engineMap, memMap
	}

	if p.engineCnt != 0 {
		var bufSize, count uint32
		getArrayProc.Call(uintptr(p.engineCnt), uintptr(pdhFmtDouble), uintptr(unsafe.Pointer(&bufSize)), uintptr(unsafe.Pointer(&count)), 0)
		if bufSize > 0 {
			buf := make([]byte, bufSize)
			ret, _, _ := getArrayProc.Call(uintptr(p.engineCnt), uintptr(pdhFmtDouble), uintptr(unsafe.Pointer(&bufSize)), uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&buf[0])))
			if ret == 0 && count > 0 {
				items := (*[16384]pdhCounterItemDouble)(unsafe.Pointer(&buf[0]))[:count:count]
				for _, it := range items {
					name := strings.ToLower(windows.UTF16PtrToString(it.SzName))
					val := it.FmtValue.DoubleValue
					if val > 0 {
						for _, luid := range trackedLuids {
							if strings.Contains(name, luid) {
								engineMap[luid] += val
								break
							}
						}
					}
				}
			}
		}
	}

	if p.memDedCnt != 0 {
		var bufSize, count uint32
		getArrayProc.Call(uintptr(p.memDedCnt), uintptr(pdhFmtLarge), uintptr(unsafe.Pointer(&bufSize)), uintptr(unsafe.Pointer(&count)), 0)
		if bufSize > 0 {
			buf := make([]byte, bufSize)
			ret, _, _ := getArrayProc.Call(uintptr(p.memDedCnt), uintptr(pdhFmtLarge), uintptr(unsafe.Pointer(&bufSize)), uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&buf[0])))
			if ret == 0 && count > 0 {
				items := (*[1024]pdhCounterItemLarge)(unsafe.Pointer(&buf[0]))[:count:count]
				for _, it := range items {
					name := strings.ToLower(windows.UTF16PtrToString(it.SzName))
					memMap[name] = it.FmtValue.LargeValue
				}
			}
		}
	}

	return engineMap, memMap
}

type nvmlCollector struct {
	dll           *syscall.LazyDLL
	initialized   bool
	getHandleProc *syscall.LazyProc
	getNameProc   *syscall.LazyProc
	getUtilProc   *syscall.LazyProc
	getMemProc    *syscall.LazyProc
	getTempProc   *syscall.LazyProc
	getPowerProc  *syscall.LazyProc
	getFanProc    *syscall.LazyProc
	count         uint32
}

func initNVMLCollector() *nvmlCollector {
	dll := syscall.NewLazyDLL("nvml.dll")
	if err := dll.Load(); err != nil {
		return nil
	}

	initProc := dll.NewProc("nvmlInit_v2")
	if initProc.Find() != nil {
		initProc = dll.NewProc("nvmlInit")
	}
	ret, _, _ := initProc.Call()
	if ret != 0 {
		return nil
	}

	getCountProc := dll.NewProc("nvmlDeviceGetCount_v2")
	if getCountProc.Find() != nil {
		getCountProc = dll.NewProc("nvmlDeviceGetCount")
	}
	var count uint32
	ret, _, _ = getCountProc.Call(uintptr(unsafe.Pointer(&count)))
	if ret != 0 {
		return nil
	}

	getHandleProc := dll.NewProc("nvmlDeviceGetHandleByIndex_v2")
	if getHandleProc.Find() != nil {
		getHandleProc = dll.NewProc("nvmlDeviceGetHandleByIndex")
	}

	return &nvmlCollector{
		dll:           dll,
		initialized:   true,
		count:         count,
		getHandleProc: getHandleProc,
		getNameProc:   dll.NewProc("nvmlDeviceGetName"),
		getUtilProc:   dll.NewProc("nvmlDeviceGetUtilizationRates"),
		getMemProc:    dll.NewProc("nvmlDeviceGetMemoryInfo"),
		getTempProc:   dll.NewProc("nvmlDeviceGetTemperature"),
		getPowerProc:  dll.NewProc("nvmlDeviceGetPowerUsage"),
		getFanProc:    dll.NewProc("nvmlDeviceGetFanSpeed"),
	}
}

func (n *nvmlCollector) getStats() []nvmlDeviceStats {
	if n == nil || !n.initialized {
		return nil
	}

	var results []nvmlDeviceStats
	for i := uint32(0); i < n.count; i++ {
		var handle uintptr
		ret, _, _ := n.getHandleProc.Call(uintptr(i), uintptr(unsafe.Pointer(&handle)))
		if ret != 0 {
			continue
		}

		nameBuf := make([]byte, 64)
		n.getNameProc.Call(handle, uintptr(unsafe.Pointer(&nameBuf[0])), uintptr(len(nameBuf)))
		name := string(bytes.TrimRight(nameBuf, "\x00"))

		var util nvmlUtilization
		n.getUtilProc.Call(handle, uintptr(unsafe.Pointer(&util)))

		var mem nvmlMemory
		n.getMemProc.Call(handle, uintptr(unsafe.Pointer(&mem)))

		var temp uint32
		rTemp, _, _ := n.getTempProc.Call(handle, 0, uintptr(unsafe.Pointer(&temp)))

		var power uint32
		rPower, _, _ := n.getPowerProc.Call(handle, uintptr(unsafe.Pointer(&power)))

		var fan uint32
		rFan, _, _ := n.getFanProc.Call(handle, uintptr(unsafe.Pointer(&fan)))

		stats := nvmlDeviceStats{
			index:       i,
			name:        name,
			loadPercent: float64(util.GPU),
			memUsedMB:   math.Round(float64(mem.Used) / 1024 / 1024),
			memTotalMB:  math.Round(float64(mem.Total) / 1024 / 1024),
		}
		if rTemp == 0 {
			stats.hasTemp = true
			stats.tempC = float64(temp)
		}
		if rPower == 0 {
			stats.hasPower = true
			stats.powerW = math.Round((float64(power)/1000.0)*10) / 10
		}
		if rFan == 0 {
			stats.hasFan = true
			stats.fanPct = float64(fan)
		}
		results = append(results, stats)
	}
	return results
}

func GetGPUInfo() []GPUInfo {
	globalPdhOnce.Do(func() {
		globalPdhGpu = initPDHGpuCollector()
	})
	globalNvmlOnce.Do(func() {
		globalNvml = initNVMLCollector()
	})

	adapters := getDXGIAdapters()
	if len(adapters) == 0 {
		return nil
	}

	trackedLuids := make([]string, 0, len(adapters))
	for _, a := range adapters {
		trackedLuids = append(trackedLuids, a.luidStr)
	}

	var engineMap map[string]float64
	var memMap map[string]int64
	if globalPdhGpu != nil {
		engineMap, memMap = globalPdhGpu.collect(trackedLuids)
	}

	var nvmlStats []nvmlDeviceStats
	if globalNvml != nil {
		nvmlStats = globalNvml.getStats()
	}

	var gpus []GPUInfo
	for _, a := range adapters {
		gpu := GPUInfo{
			Index:  a.index,
			Name:   a.name,
			Vendor: a.vendor,
		}

		if a.vramTotal > 0 {
			totMB := math.Round(float64(a.vramTotal) / 1024 / 1024)
			gpu.MemoryTotalMB = &totMB
		}

		for k, v := range memMap {
			if strings.Contains(k, a.luidStr) {
				usedMB := math.Round(float64(v) / 1024 / 1024)
				gpu.MemoryUsedMB = &usedMB
				if gpu.MemoryTotalMB != nil && *gpu.MemoryTotalMB > 0 {
					pct := math.Round((usedMB / *gpu.MemoryTotalMB) * 1000) / 10
					gpu.MemoryPercent = &pct
				}
				break
			}
		}

		if val, ok := engineMap[a.luidStr]; ok {
			corePct := math.Min(100.0, math.Round(val*10)/10)
			gpu.CoreUsagePercent = &corePct
		}

		if a.vendor == "NVIDIA" && len(nvmlStats) > 0 {
			for _, ns := range nvmlStats {
				gpu.CoreUsagePercent = &ns.loadPercent
				if ns.hasTemp {
					gpu.TemperatureC = &ns.tempC
				}
				if ns.hasPower {
					gpu.PowerWatts = &ns.powerW
				}
				if ns.hasFan {
					gpu.FanSpeedPercent = &ns.fanPct
				}
				if ns.memTotalMB > 0 && gpu.MemoryTotalMB == nil {
					gpu.MemoryTotalMB = &ns.memTotalMB
				}
				if ns.memUsedMB > 0 && gpu.MemoryUsedMB == nil {
					gpu.MemoryUsedMB = &ns.memUsedMB
				}
				if gpu.MemoryUsedMB != nil && gpu.MemoryTotalMB != nil && *gpu.MemoryTotalMB > 0 {
					pct := math.Round((*gpu.MemoryUsedMB / *gpu.MemoryTotalMB) * 1000) / 10
					gpu.MemoryPercent = &pct
				}
				break
			}
		}

		gpus = append(gpus, gpu)
	}

	return gpus
}
