//go:build windows

package main

import (
	"errors"
	"fmt"
	"image"
	"log"
	"sync"
	"syscall"
	"unsafe"
)

// ErrNoNewFrame DXGI 超时未获取到新帧（屏幕无变化），不是真正的错误
var ErrNoNewFrame = errors.New("no new frame")

// ─── DXGI Desktop Duplication 截图引擎 ───
// GPU 加速桌面复制（Windows 8+），比 BitBlt 更可靠

var (
	modD3D11              = syscall.NewLazyDLL("d3d11.dll")
	procD3D11CreateDevice = modD3D11.NewProc("D3D11CreateDevice")
)

// COM helper
func comCall(obj uintptr, method int, args ...uintptr) uintptr {
	vtbl := *(*uintptr)(unsafe.Pointer(obj))
	fn := *(*uintptr)(unsafe.Pointer(vtbl + uintptr(method)*unsafe.Sizeof(uintptr(0))))
	all := append([]uintptr{obj}, args...)
	r, _, _ := syscall.SyscallN(fn, all...)
	return r
}

func comRelease(obj uintptr) {
	if obj != 0 {
		comCall(obj, 2) // IUnknown::Release
	}
}

// GUIDs
type _GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var (
	iidIDXGIDevice     = _GUID{0x54ec77fa, 0x1377, 0x44e6, [8]byte{0x8c, 0x32, 0x88, 0xfd, 0x5f, 0x44, 0xc8, 0x4c}}
	iidIDXGIOutput1    = _GUID{0x00cddea8, 0x939b, 0x4b83, [8]byte{0xa3, 0x40, 0xa6, 0x85, 0x22, 0x66, 0x66, 0xcc}}
	iidID3D11Texture2D = _GUID{0x6f15aaf2, 0xd208, 0x4e89, [8]byte{0x9a, 0xb4, 0x48, 0x95, 0x35, 0xd3, 0x4f, 0x9c}}
)

// D3D11/DXGI constants
const (
	driverTypeHardware   = 1
	driverTypeWarp       = 5
	d3d11SDKVersion      = 7
	dxgiFormatB8G8R8A8   = 87
	d3d11UsageStaging    = 3
	d3d11CPUAccessRead   = 0x20000
	d3d11MapRead         = 1
	dxgiErrorWaitTimeout = 0x887A0027
	dxgiErrorAccessLost  = 0x887A0026
)

// D3D11 structures
type d3d11Texture2DDesc struct {
	Width, Height  uint32
	MipLevels      uint32
	ArraySize      uint32
	Format         uint32
	SampleCount    uint32
	SampleQuality  uint32
	Usage          uint32
	BindFlags      uint32
	CPUAccessFlags uint32
	MiscFlags      uint32
}

type dxgiOutduplFrameInfo struct {
	LastPresentTime     int64
	LastMouseUpdateTime int64
	AccumulatedFrames   uint32
	_pad                [28]byte // RectsCoalesced(4)+ProtectedContentMaskedOut(4)+PointerPosition(12)+TotalMetadata(4)+PointerShape(4)=28
}

type d3d11MappedSubresource struct {
	Data       uintptr
	RowPitch   uint32
	DepthPitch uint32
}

type dxgiOutputDesc struct {
	DeviceName        [32]uint16
	DesktopCoords     [4]int32 // RECT: left, top, right, bottom
	AttachedToDesktop int32
	Rotation          uint32
	Monitor           uintptr
}

// DXGICapturer 持久化的 DXGI 截图器，跨帧复用资源
type DXGICapturer struct {
	mu          sync.Mutex
	device      uintptr
	context     uintptr
	duplication uintptr
	staging     uintptr
	width       uint32
	height      uint32
	initialized bool
}

// NewDXGICapturer 创建 DXGI 截图器
func NewDXGICapturer() (*DXGICapturer, error) {
	c := &DXGICapturer{}
	if err := c.init(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *DXGICapturer) init() error {
	// 1. D3D11CreateDevice
	var device, context uintptr
	featureLevels := []uint32{0xb000, 0xa100, 0xa000} // 11.0, 10.1, 10.0
	drivers := []uint32{driverTypeHardware, driverTypeWarp}

	var hr uintptr
	for _, dt := range drivers {
		hr, _, _ = procD3D11CreateDevice.Call(
			0, uintptr(dt), 0, 0,
			uintptr(unsafe.Pointer(&featureLevels[0])),
			uintptr(len(featureLevels)),
			d3d11SDKVersion,
			uintptr(unsafe.Pointer(&device)),
			0,
			uintptr(unsafe.Pointer(&context)),
		)
		if hr == 0 {
			break
		}
	}
	if hr != 0 {
		return fmt.Errorf("D3D11CreateDevice failed: 0x%x", hr)
	}
	c.device = device
	c.context = context

	// 2. device → IDXGIDevice
	var dxgiDevice uintptr
	hr = comCall(device, 0, uintptr(unsafe.Pointer(&iidIDXGIDevice)), uintptr(unsafe.Pointer(&dxgiDevice)))
	if hr != 0 {
		return fmt.Errorf("QueryInterface IDXGIDevice: 0x%x", hr)
	}
	defer comRelease(dxgiDevice)

	// 3. IDXGIDevice → GetAdapter (method 7: GetParent is IDXGIObject, GetAdapter is IDXGIDevice method 10 from IUnknown base)
	// IDXGIDevice vtable: IUnknown(0-2) + IDXGIObject(3-6) + IDXGIDevice(7-10)
	// GetAdapter = method 7
	var adapter uintptr
	hr = comCall(dxgiDevice, 7, uintptr(unsafe.Pointer(&adapter)))
	if hr != 0 {
		return fmt.Errorf("GetAdapter: 0x%x", hr)
	}
	defer comRelease(adapter)

	// 4. adapter → EnumOutputs(0)
	// IDXGIAdapter vtable: IUnknown(0-2) + IDXGIObject(3-6) + IDXGIAdapter(7-8)
	// EnumOutputs = method 7
	var output uintptr
	hr = comCall(adapter, 7, 0, uintptr(unsafe.Pointer(&output)))
	if hr != 0 {
		return fmt.Errorf("EnumOutputs: 0x%x", hr)
	}
	defer comRelease(output)

	// 5. output → IDXGIOutput1
	var output1 uintptr
	hr = comCall(output, 0, uintptr(unsafe.Pointer(&iidIDXGIOutput1)), uintptr(unsafe.Pointer(&output1)))
	if hr != 0 {
		return fmt.Errorf("QueryInterface IDXGIOutput1: 0x%x", hr)
	}
	defer comRelease(output1)

	// Get output desc for dimensions
	var desc dxgiOutputDesc
	// IDXGIOutput vtable: IUnknown(0-2) + IDXGIObject(3-6) + IDXGIOutput(7-16)
	// GetDesc = method 7
	hr = comCall(output, 7, uintptr(unsafe.Pointer(&desc)))
	if hr != 0 {
		return fmt.Errorf("GetDesc: 0x%x", hr)
	}
	c.width = uint32(desc.DesktopCoords[2] - desc.DesktopCoords[0])
	c.height = uint32(desc.DesktopCoords[3] - desc.DesktopCoords[1])

	// 6. DuplicateOutput
	// IDXGIOutput1 vtable: ... + DuplicateOutput = method 22
	hr = comCall(output1, 22, uintptr(device), uintptr(unsafe.Pointer(&c.duplication)))
	if hr != 0 {
		return fmt.Errorf("DuplicateOutput: 0x%x", hr)
	}

	// 7. Create staging texture
	stagingDesc := d3d11Texture2DDesc{
		Width:          c.width,
		Height:         c.height,
		MipLevels:      1,
		ArraySize:      1,
		Format:         dxgiFormatB8G8R8A8,
		SampleCount:    1,
		SampleQuality:  0,
		Usage:          d3d11UsageStaging,
		CPUAccessFlags: d3d11CPUAccessRead,
	}
	// ID3D11Device::CreateTexture2D = method 5
	hr = comCall(device, 5,
		uintptr(unsafe.Pointer(&stagingDesc)),
		0,
		uintptr(unsafe.Pointer(&c.staging)),
	)
	if hr != 0 {
		return fmt.Errorf("CreateTexture2D staging: 0x%x", hr)
	}

	c.initialized = true
	return nil
}

// CaptureFrame 捕获一帧桌面，返回 BGRA 图像
func (c *DXGICapturer) CaptureFrame() (*image.RGBA, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized {
		return nil, fmt.Errorf("capturer not initialized")
	}

	// AcquireNextFrame (timeout 200ms)
	// IDXGIOutputDuplication vtable: IUnknown(0-2) + IDXGIObject(3-6) + methods(7...)
	// AcquireNextFrame = method 8
	var frameInfo dxgiOutduplFrameInfo
	var resource uintptr
	hr := comCall(c.duplication, 8,
		200, // timeout ms
		uintptr(unsafe.Pointer(&frameInfo)),
		uintptr(unsafe.Pointer(&resource)),
	)
	if hr == uintptr(dxgiErrorWaitTimeout) {
		return nil, ErrNoNewFrame
	}
	if hr == uintptr(dxgiErrorAccessLost) {
		c.reinit()
		return nil, fmt.Errorf("access lost, reinitializing")
	}
	if hr != 0 {
		return nil, fmt.Errorf("AcquireNextFrame: 0x%x", hr)
	}
	defer comCall(c.duplication, 14) // ReleaseFrame = method 14

	// resource → ID3D11Texture2D
	var tex uintptr
	hr = comCall(resource, 0, uintptr(unsafe.Pointer(&iidID3D11Texture2D)), uintptr(unsafe.Pointer(&tex)))
	comRelease(resource)
	if hr != 0 {
		return nil, fmt.Errorf("QueryInterface Texture2D: 0x%x", hr)
	}
	defer comRelease(tex)

	// CopyResource: context.CopyResource(staging, tex)
	// ID3D11DeviceContext::CopyResource = method 47
	comCall(c.context, 47, c.staging, tex)

	// Map staging texture
	// ID3D11DeviceContext::Map = method 14
	var mapped d3d11MappedSubresource
	hr = comCall(c.context, 14,
		c.staging, 0, d3d11MapRead, 0,
		uintptr(unsafe.Pointer(&mapped)),
	)
	if hr != 0 {
		return nil, fmt.Errorf("Map: 0x%x", hr)
	}

	// Copy BGRA pixels to RGBA image
	img := image.NewRGBA(image.Rect(0, 0, int(c.width), int(c.height)))
	for y := uint32(0); y < c.height; y++ {
		srcRow := unsafe.Pointer(mapped.Data + uintptr(y)*uintptr(mapped.RowPitch))
		dstOff := y * c.width * 4
		src := unsafe.Slice((*byte)(srcRow), c.width*4)
		dst := img.Pix[dstOff : dstOff+c.width*4]
		// BGRA → RGBA
		for x := uint32(0); x < c.width; x++ {
			i := x * 4
			dst[i+0] = src[i+2] // R ← B
			dst[i+1] = src[i+1] // G
			dst[i+2] = src[i+0] // B ← R
			dst[i+3] = src[i+3] // A
		}
	}

	// Unmap: ID3D11DeviceContext::Unmap = method 15
	comCall(c.context, 15, c.staging, 0)

	return img, nil
}

func (c *DXGICapturer) reinit() {
	c.Close()
	if err := c.init(); err != nil {
		log.Printf("DXGI reinit 失败: %v", err)
	}
}

// Close 释放所有 COM 资源
func (c *DXGICapturer) Close() {
	comRelease(c.staging)
	comRelease(c.duplication)
	comRelease(c.context)
	comRelease(c.device)
	c.staging = 0
	c.duplication = 0
	c.context = 0
	c.device = 0
	c.initialized = false
}

// Width 返回桌面宽度
func (c *DXGICapturer) Width() int { return int(c.width) }

// Height 返回桌面高度
func (c *DXGICapturer) Height() int { return int(c.height) }
