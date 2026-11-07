//go:build windows

package screenthumb

import (
	"errors"
	"fmt"
	"image"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modD3D11 = windows.NewLazySystemDLL("d3d11.dll")
	modDXGI  = windows.NewLazySystemDLL("dxgi.dll")

	procD3D11CreateDevice  = modD3D11.NewProc("D3D11CreateDevice")
	procCreateDXGIFactory1 = modDXGI.NewProc("CreateDXGIFactory1")

	iidFactory1, _   = windows.GUIDFromString("{770aae78-f26f-4dba-a829-253c83d1b387}")
	iidOutput1, _    = windows.GUIDFromString("{00cddea8-939b-4b83-a340-a685226666cc}")
	iidDXGIDevice, _ = windows.GUIDFromString("{54ec77fa-1377-44e6-8c32-88fd5f44c84c}")
	iidTexture2D, _  = windows.GUIDFromString("{6f15aaf2-d208-4e89-9ab4-489535d34f9c}")
)

const (
	vtQueryInterface = 0
	vtRelease        = 2

	vtFactory1EnumAdapters1 = 12
	vtAdapterEnumOutputs    = 7
	vtOutputGetDesc         = 7
	vtOutput1DuplicateOut   = 22
	vtDuplAcquireNextFrame  = 8
	vtDuplReleaseFrame      = 14

	vtDeviceCreateTexture2D = 5
	vtDeviceCreateSRV       = 7
	vtContextMap            = 14
	vtContextUnmap          = 15
	vtContextCopySubregion  = 46
	vtContextGenerateMips   = 54
	vtTexture2DGetDesc      = 10

	formatB8G8R8A8 = 87

	usageDefault = 0
	usageStaging = 3

	bindShaderResource = 0x8
	bindRenderTarget   = 0x20
	cpuAccessRead      = 0x20000
	miscGenerateMips   = 0x1
	mapRead            = 1

	driverTypeUnknown = 0
	driverTypeWarp    = 5
	sdkVersion        = 7

	hrWaitTimeout uint32 = 0x887A0027
)

type texDesc struct {
	Width, Height, MipLevels, ArraySize, Format uint32
	SampleCount, SampleQuality                  uint32
	Usage, BindFlags, CPUAccessFlags, MiscFlags uint32
}

type subresourceData struct {
	Mem        unsafe.Pointer
	Pitch      uint32
	SlicePitch uint32
}

type mappedSubresource struct {
	Data       unsafe.Pointer
	RowPitch   uint32
	DepthPitch uint32
}

type outputDesc struct {
	DeviceName               [32]uint16
	Left, Top, Right, Bottom int32
	Attached                 uint32
	Rotation                 uint32
	Monitor                  uintptr
}

type hresultError struct {
	op   string
	code uint32
}

func (e hresultError) Error() string {
	return fmt.Sprintf("%s failed: HRESULT 0x%08X", e.op, e.code)
}

func vcall(obj unsafe.Pointer, index int, args ...uintptr) uintptr {
	vtbl := *(*unsafe.Pointer)(obj)
	fn := *(*uintptr)(unsafe.Add(vtbl, uintptr(index)*unsafe.Sizeof(uintptr(0))))
	all := make([]uintptr, 0, len(args)+1)
	all = append(all, uintptr(obj))
	all = append(all, args...)
	r, _, _ := syscall.SyscallN(fn, all...)
	return r
}

func failed(hr uintptr) bool { return int32(uint32(hr)) < 0 }

func check(op string, hr uintptr) error {
	if failed(hr) {
		return hresultError{op: op, code: uint32(hr)}
	}
	return nil
}

func ptr[T any](p *T) uintptr { return uintptr(unsafe.Pointer(p)) }

func release(p *unsafe.Pointer) {
	if *p != nil {
		vcall(*p, vtRelease)
		*p = nil
	}
}

type comCapturer struct {
	target                    string
	device, ctx, dup, staging unsafe.Pointer
	stagingW, stagingH        int
	srcW, srcH                int
	haveFrame                 bool

	factory, adapter, output, out1, dxgiDev unsafe.Pointer
	resource, tex, mipTex, srv              unsafe.Pointer

	frameInfo    [6]uint64
	outDesc      outputDesc
	desc         texDesc
	mipDesc      texDesc
	mapped       mappedSubresource
	featureLevel uint32
	buf          []byte
}

func platformNewGPU() (gpuCapturer, error) {
	c, err := newComCapturer()
	if err != nil {
		return nil, err
	}
	return c, nil
}

func platformNewDisplayGPU(display Display) (gpuCapturer, error) {
	c := &comCapturer{target: display.Device}
	if err := c.open(); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func newComCapturer() (*comCapturer, error) {
	c := &comCapturer{}
	if err := c.open(); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func newComCapturerForDevice(device, ctx unsafe.Pointer) *comCapturer {
	return &comCapturer{device: device, ctx: ctx}
}

func (c *comCapturer) open() error {
	r, _, _ := procCreateDXGIFactory1.Call(ptr(&iidFactory1), ptr(&c.factory))
	if err := check("CreateDXGIFactory1", r); err != nil {
		return err
	}

	found := false
	for a := 0; !found; a++ {
		if failed(vcall(c.factory, vtFactory1EnumAdapters1, uintptr(a), ptr(&c.adapter))) {
			break
		}
		for o := 0; ; o++ {
			if failed(vcall(c.adapter, vtAdapterEnumOutputs, uintptr(o), ptr(&c.output))) {
				break
			}
			if !failed(vcall(c.output, vtOutputGetDesc, ptr(&c.outDesc))) &&
				c.outDesc.Attached != 0 && c.matchesOutput() {
				found = true
				break
			}
			release(&c.output)
		}
		if !found {
			release(&c.adapter)
		}
	}
	if !found {
		return fmt.Errorf("no GPU output found for display %q", c.target)
	}
	if c.outDesc.Rotation > 1 {
		return errors.New("rotated displays are not supported by GPU capture")
	}

	levels := [...]uint32{0xb100, 0xb000, 0xa100, 0xa000}
	r, _, _ = procD3D11CreateDevice.Call(uintptr(c.adapter), driverTypeUnknown, 0, 0,
		ptr(&levels[0]), uintptr(len(levels)), sdkVersion,
		ptr(&c.device), ptr(&c.featureLevel), ptr(&c.ctx))
	if err := check("D3D11CreateDevice", r); err != nil {
		return err
	}

	if err := check("QueryInterface(IDXGIOutput1)", vcall(c.output, vtQueryInterface, ptr(&iidOutput1), ptr(&c.out1))); err != nil {
		return err
	}
	if err := check("QueryInterface(IDXGIDevice)", vcall(c.device, vtQueryInterface, ptr(&iidDXGIDevice), ptr(&c.dxgiDev))); err != nil {
		return err
	}
	if err := check("DuplicateOutput", vcall(c.out1, vtOutput1DuplicateOut, uintptr(c.dxgiDev), ptr(&c.dup))); err != nil {
		return err
	}

	release(&c.out1)
	release(&c.dxgiDev)
	release(&c.output)
	release(&c.adapter)
	release(&c.factory)
	return nil
}

func (c *comCapturer) matchesOutput() bool {
	if c.target != "" {
		return syscall.UTF16ToString(c.outDesc.DeviceName[:]) == c.target
	}
	return c.outDesc.Left == 0 && c.outDesc.Top == 0
}

func (c *comCapturer) Close() {
	release(&c.srv)
	release(&c.mipTex)
	release(&c.tex)
	release(&c.resource)
	release(&c.staging)
	release(&c.dup)
	release(&c.out1)
	release(&c.dxgiDev)
	release(&c.output)
	release(&c.adapter)
	release(&c.factory)
	release(&c.ctx)
	release(&c.device)
	c.haveFrame = false
	c.buf = nil
}

func (c *comCapturer) Capture(maxSide int) (image.Image, error) {
	if c.dup == nil {
		return nil, errors.New("GPU capturer is closed")
	}
	timeout := uintptr(0)
	if !c.haveFrame {
		timeout = 250
	}
	hr := vcall(c.dup, vtDuplAcquireNextFrame, timeout, ptr(&c.frameInfo[0]), ptr(&c.resource))
	switch {
	case !failed(hr):
		if err := c.takeFrame(maxSide); err != nil {
			return nil, err
		}
	case uint32(hr) == hrWaitTimeout && c.haveFrame:
	default:
		return nil, hresultError{op: "AcquireNextFrame", code: uint32(hr)}
	}
	return c.readStaging(maxSide)
}

func (c *comCapturer) takeFrame(maxSide int) error {
	defer func() {
		release(&c.tex)
		release(&c.resource)
		vcall(c.dup, vtDuplReleaseFrame)
	}()
	if err := check("QueryInterface(ID3D11Texture2D)", vcall(c.resource, vtQueryInterface, ptr(&iidTexture2D), ptr(&c.tex))); err != nil {
		return err
	}
	vcall(c.tex, vtTexture2DGetDesc, ptr(&c.desc))
	if c.desc.Format != formatB8G8R8A8 || c.desc.SampleCount != 1 {
		return fmt.Errorf("unsupported desktop format %d", c.desc.Format)
	}
	return c.downscale(c.tex, int(c.desc.Width), int(c.desc.Height), maxSide)
}

func (c *comCapturer) downscale(src unsafe.Pointer, w, h, maxSide int) error {
	level := PickMipLevel(w, h, maxSide)
	mw, mh := MipSize(w, h, level)
	if err := c.ensureStaging(mw, mh); err != nil {
		return err
	}

	if level == 0 {
		vcall(c.ctx, vtContextCopySubregion, uintptr(c.staging), 0, 0, 0, 0, uintptr(src), 0, 0)
	} else {
		c.mipDesc = texDesc{
			Width: uint32(w), Height: uint32(h), MipLevels: uint32(level + 1), ArraySize: 1,
			Format: formatB8G8R8A8, SampleCount: 1, Usage: usageDefault,
			BindFlags: bindShaderResource | bindRenderTarget, MiscFlags: miscGenerateMips,
		}
		if err := check("CreateTexture2D(mips)", vcall(c.device, vtDeviceCreateTexture2D, ptr(&c.mipDesc), 0, ptr(&c.mipTex))); err != nil {
			return err
		}
		defer release(&c.mipTex)
		vcall(c.ctx, vtContextCopySubregion, uintptr(c.mipTex), 0, 0, 0, 0, uintptr(src), 0, 0)
		if err := check("CreateShaderResourceView", vcall(c.device, vtDeviceCreateSRV, uintptr(c.mipTex), 0, ptr(&c.srv))); err != nil {
			return err
		}
		vcall(c.ctx, vtContextGenerateMips, uintptr(c.srv))
		release(&c.srv)
		vcall(c.ctx, vtContextCopySubregion, uintptr(c.staging), 0, 0, 0, 0, uintptr(c.mipTex), uintptr(level), 0)
	}
	c.srcW, c.srcH = w, h
	c.haveFrame = true
	return nil
}

func (c *comCapturer) ensureStaging(w, h int) error {
	if c.staging != nil && c.stagingW == w && c.stagingH == h {
		return nil
	}
	release(&c.staging)
	c.mipDesc = texDesc{
		Width: uint32(w), Height: uint32(h), MipLevels: 1, ArraySize: 1,
		Format: formatB8G8R8A8, SampleCount: 1, Usage: usageStaging, CPUAccessFlags: cpuAccessRead,
	}
	if err := check("CreateTexture2D(staging)", vcall(c.device, vtDeviceCreateTexture2D, ptr(&c.mipDesc), 0, ptr(&c.staging))); err != nil {
		return err
	}
	c.stagingW, c.stagingH = w, h
	return nil
}

func (c *comCapturer) readStaging(maxSide int) (image.Image, error) {
	if err := check("Map", vcall(c.ctx, vtContextMap, uintptr(c.staging), 0, mapRead, 0, ptr(&c.mapped))); err != nil {
		return nil, err
	}
	mw, mh := c.stagingW, c.stagingH
	row := mw * 4
	pitch := int(c.mapped.RowPitch)
	if pitch < row {
		vcall(c.ctx, vtContextUnmap, uintptr(c.staging), 0)
		return nil, fmt.Errorf("unexpected row pitch %d for width %d", pitch, mw)
	}
	if cap(c.buf) < row*mh {
		c.buf = make([]byte, row*mh)
	}
	c.buf = c.buf[:row*mh]
	mapped := unsafe.Slice((*byte)(c.mapped.Data), (mh-1)*pitch+row)
	for y := 0; y < mh; y++ {
		copy(c.buf[y*row:(y+1)*row], mapped[y*pitch:y*pitch+row])
	}
	vcall(c.ctx, vtContextUnmap, uintptr(c.staging), 0)

	img, err := FromBGRA(c.buf, mw, mh)
	if err != nil {
		return nil, err
	}
	tw, th, err := ThumbSize(c.srcW, c.srcH, maxSide)
	if err != nil {
		return nil, err
	}
	return BoxResize(img, tw, th), nil
}
