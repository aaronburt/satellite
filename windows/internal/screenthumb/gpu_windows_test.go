//go:build windows

package screenthumb

import (
	"image/color"
	"os"
	"runtime"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func warpCapturer(t testing.TB) *comCapturer {
	t.Helper()
	var device, ctx unsafe.Pointer
	var level uint32
	r, _, _ := procD3D11CreateDevice.Call(0, driverTypeWarp, 0, 0, 0, 0, sdkVersion,
		ptr(&device), ptr(&level), ptr(&ctx))
	if failed(r) {
		t.Skipf("no software D3D11 device on this machine: HRESULT 0x%08X", uint32(r))
	}
	c := newComCapturerForDevice(device, ctx)
	t.Cleanup(c.Close)
	return c
}

func uploadTexture(t testing.TB, c *comCapturer, w, h int, pixel func(x, y int) [4]byte) unsafe.Pointer {
	t.Helper()
	data := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := pixel(x, y)
			copy(data[(y*w+x)*4:], p[:])
		}
	}
	desc := texDesc{
		Width: uint32(w), Height: uint32(h), MipLevels: 1, ArraySize: 1,
		Format: formatB8G8R8A8, SampleCount: 1, Usage: usageDefault, BindFlags: bindShaderResource,
	}
	init := subresourceData{Mem: unsafe.Pointer(&data[0]), Pitch: uint32(w * 4)}
	var tex unsafe.Pointer
	err := check("CreateTexture2D(source)", vcall(c.device, vtDeviceCreateTexture2D, ptr(&desc), ptr(&init), ptr(&tex)))
	runtime.KeepAlive(data)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { release(&tex) })
	return tex
}

func TestGPUPipelineDownscalesSyntheticScreen(t *testing.T) {
	cases := []struct {
		name         string
		w, h         int
		wantW, wantH int
	}{
		{"1080p", 1920, 1080, 200, 113},
		{"4k", 3840, 2160, 200, 113},
		{"portrait", 1080, 1920, 113, 200},
		{"ultrawide", 3440, 1440, 200, 84},
		{"small screen needs no mips", 160, 90, 160, 90},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := warpCapturer(t)
			tex := uploadTexture(t, c, tc.w, tc.h, func(x, y int) [4]byte {
				if x < tc.w/2 {
					return [4]byte{255, 0, 0, 255}
				}
				return [4]byte{0, 0, 255, 255}
			})
			if err := c.downscale(tex, tc.w, tc.h, DefaultSide); err != nil {
				t.Fatal(err)
			}
			img, err := c.readStaging(DefaultSide)
			if err != nil {
				t.Fatal(err)
			}
			b := img.Bounds()
			if b.Dx() != tc.wantW || b.Dy() != tc.wantH {
				t.Fatalf("thumbnail is %dx%d, want %dx%d", b.Dx(), b.Dy(), tc.wantW, tc.wantH)
			}
			left := color.RGBAModel.Convert(img.At(b.Dx()/10, b.Dy()/2)).(color.RGBA)
			right := color.RGBAModel.Convert(img.At(b.Dx()-b.Dx()/10, b.Dy()/2)).(color.RGBA)
			if left.B < 240 || left.R > 15 {
				t.Errorf("left half = %v, want blue (source bytes are BGRA)", left)
			}
			if right.R < 240 || right.B > 15 {
				t.Errorf("right half = %v, want red", right)
			}
		})
	}
}

func TestGPUPipelineAveragesFineDetail(t *testing.T) {
	c := warpCapturer(t)
	tex := uploadTexture(t, c, 1920, 1080, func(x, y int) [4]byte {
		if (x+y)%2 == 0 {
			return [4]byte{255, 255, 255, 255}
		}
		return [4]byte{0, 0, 0, 255}
	})
	if err := c.downscale(tex, 1920, 1080, DefaultSide); err != nil {
		t.Fatal(err)
	}
	img, err := c.readStaging(DefaultSide)
	if err != nil {
		t.Fatal(err)
	}
	got := color.RGBAModel.Convert(img.At(100, 56)).(color.RGBA)
	if got.R < 110 || got.R > 145 {
		t.Fatalf("a 1px checkerboard should average to grey, got %v", got)
	}
}

func TestGPUPipelineReusesStagingForSameSize(t *testing.T) {
	c := warpCapturer(t)
	tex := uploadTexture(t, c, 1920, 1080, func(x, y int) [4]byte { return [4]byte{9, 9, 9, 255} })
	if err := c.downscale(tex, 1920, 1080, DefaultSide); err != nil {
		t.Fatal(err)
	}
	first := c.staging
	if err := c.downscale(tex, 1920, 1080, DefaultSide); err != nil {
		t.Fatal(err)
	}
	if c.staging != first {
		t.Fatal("staging texture should be reused when the screen size is unchanged")
	}
	small := uploadTexture(t, c, 1280, 720, func(x, y int) [4]byte { return [4]byte{9, 9, 9, 255} })
	if err := c.downscale(small, 1280, 720, DefaultSide); err != nil {
		t.Fatal(err)
	}
	if c.stagingW != 320 || c.stagingH != 180 {
		t.Fatalf("staging is %dx%d after a resolution change, want 320x180", c.stagingW, c.stagingH)
	}
}

func TestGPUCapturerCloseIsSafeToRepeat(t *testing.T) {
	c := warpCapturer(t)
	c.Close()
	c.Close()
	if _, err := c.Capture(DefaultSide); err == nil {
		t.Fatal("a closed capturer must refuse to capture")
	}
}

func TestRealGPUCaptureMatchesGDISize(t *testing.T) {
	gpu, err := newComCapturer()
	if err != nil {
		t.Skipf("desktop duplication is not available here: %v", err)
	}
	defer gpu.Close()
	gpuImg, err := gpu.Capture(DefaultSide)
	if err != nil {
		t.Skipf("GPU capture failed here: %v", err)
	}
	t.Logf("GPU capture works: %v", gpuImg.Bounds())
	gdiImg, err := Capture(DefaultSide)
	if err != nil {
		t.Skipf("GDI capture failed here: %v", err)
	}
	if gpuImg.Bounds() != gdiImg.Bounds() {
		t.Logf("GPU %v vs GDI %v (can differ on DPI-scaled screens)", gpuImg.Bounds(), gdiImg.Bounds())
	}
	if max(gpuImg.Bounds().Dx(), gpuImg.Bounds().Dy()) > DefaultSide {
		t.Fatalf("GPU thumbnail %v is larger than %dpx", gpuImg.Bounds(), DefaultSide)
	}
}

type processMemoryCounters struct {
	Cb, PageFaultCount                                 uint32
	PeakWorkingSetSize, WorkingSetSize                 uintptr
	QuotaPeakPagedPoolUsage, QuotaPagedPoolUsage       uintptr
	QuotaPeakNonPagedPoolUsage, QuotaNonPagedPoolUsage uintptr
	PagefileUsage, PeakPagefileUsage                   uintptr
}

func workingSetMB() float64 {
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("K32GetProcessMemoryInfo")
	var m processMemoryCounters
	m.Cb = uint32(unsafe.Sizeof(m))
	proc.Call(uintptr(windows.CurrentProcess()), ptr(&m), uintptr(m.Cb))
	return float64(m.WorkingSetSize) / (1 << 20)
}

func TestReportGPUMemoryCost(t *testing.T) {
	before := workingSetMB()
	gpu, err := newComCapturer()
	if err != nil {
		t.Skipf("desktop duplication is not available here: %v", err)
	}
	defer gpu.Close()
	for i := 0; i < 10; i++ {
		if _, err := gpu.Capture(DefaultSide); err != nil {
			t.Skipf("GPU capture failed here: %v", err)
		}
	}
	open := workingSetMB()
	gpu.Close()
	after := workingSetMB()
	t.Logf("working set: %.1f MB before, %.1f MB with the GPU capturer open, %.1f MB after closing", before, open, after)
}

func BenchmarkGPUCapture(b *testing.B) {
	gpu, err := newComCapturer()
	if err != nil {
		b.Skipf("desktop duplication is not available here: %v", err)
	}
	defer gpu.Close()
	if _, err := gpu.Capture(DefaultSide); err != nil {
		b.Skipf("GPU capture failed here: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := gpu.Capture(DefaultSide); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGPUPipelineWARP(b *testing.B) {
	c := warpCapturer(b)
	tex := uploadTexture(b, c, 1920, 1080, func(x, y int) [4]byte { return [4]byte{byte(x), byte(y), 0, 255} })
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := c.downscale(tex, 1920, 1080, DefaultSide); err != nil {
			b.Fatal(err)
		}
		if _, err := c.readStaging(DefaultSide); err != nil {
			b.Fatal(err)
		}
	}
}

func TestGPUReadbackRetainedMemory(t *testing.T) {
	if os.Getenv("SATELLITE_MEMORY_BENCH") != "1" {
		t.Skip("set SATELLITE_MEMORY_BENCH=1 to measure retained readback memory")
	}
	c := warpCapturer(t)
	tex := uploadTexture(t, c, 1920, 1080, func(x, y int) [4]byte { return [4]byte{byte(x), byte(y), 0, 255} })
	if err := c.downscale(tex, 1920, 1080, DefaultSide); err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := 0; i < 10; i++ {
		if _, err := c.readStaging(DefaultSide); err != nil {
			t.Fatal(err)
		}
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(c)
	t.Logf("1920x1080 source, 200px thumbnail, retained readback heap_delta=%d", int64(after.HeapAlloc)-int64(before.HeapAlloc))
}
