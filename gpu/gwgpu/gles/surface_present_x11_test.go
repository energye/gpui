//go:build linux && !(js && wasm)

package gles

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/ebitengine/purego"
	"github.com/energye/gpui/gpu/gwgpu/gles/egl"
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

//
// 无 X11/EGL 时 t.Skipf（缺真机数据不假绿）。

type h4e2X11Win struct {
	lib     uintptr
	display uintptr
	window  uintptr
}

func h4e2OpenX11Window(t *testing.T, w, h int) *h4e2X11Win {
	t.Helper()
	if os.Getenv("GPUI_FORCE_NO_X11") == "1" {
		t.Skip("GPUI_FORCE_NO_X11=1")
	}
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no DISPLAY")
	}
	lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		lib, err = purego.Dlopen("libX11.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	}
	if err != nil {
		t.Skipf("libX11 not available: %v", err)
	}

	var (
		xOpenDisplay   func(name *byte) uintptr
		xDefaultScreen func(dpy uintptr) int
		xRootWindow    func(dpy uintptr, screen int) uintptr
		xCreateSimple  func(dpy uintptr, parent uintptr, x, y int, width, height, borderWidth uint, border, background uint64) uintptr
		xMapWindow     func(dpy uintptr, w uintptr) int
		xFlush         func(dpy uintptr) int
		xDestroyWindow func(dpy uintptr, w uintptr) int
		xStoreName     func(dpy uintptr, w uintptr, name *byte) int
	)
	purego.RegisterLibFunc(&xOpenDisplay, lib, "XOpenDisplay")
	purego.RegisterLibFunc(&xDefaultScreen, lib, "XDefaultScreen")
	purego.RegisterLibFunc(&xRootWindow, lib, "XRootWindow")
	purego.RegisterLibFunc(&xCreateSimple, lib, "XCreateSimpleWindow")
	purego.RegisterLibFunc(&xMapWindow, lib, "XMapWindow")
	purego.RegisterLibFunc(&xFlush, lib, "XFlush")
	purego.RegisterLibFunc(&xDestroyWindow, lib, "XDestroyWindow")
	purego.RegisterLibFunc(&xStoreName, lib, "XStoreName")

	installXErrorHandler(lib)
	dpy := xOpenDisplay(nil)
	if dpy == 0 {
		_ = purego.Dlclose(lib)
		t.Skip("XOpenDisplay failed (no usable DISPLAY)")
	}
	screen := xDefaultScreen(dpy)
	root := xRootWindow(dpy, screen)
	win := xCreateSimple(dpy, root, 0, 0, uint(w), uint(h), 1, 0, 0)
	if win == 0 {
		_ = purego.Dlclose(lib)
		t.Skip("XCreateSimpleWindow failed")
	}
	name := append([]byte("gpui-h4e2-triangle-x11"), 0)
	xStoreName(dpy, win, &name[0])
	xMapWindow(dpy, win)
	xFlush(dpy)
	waitUntilViewable(t, lib, dpy, win, 5*time.Second)

	return &h4e2X11Win{lib: lib, display: dpy, window: win}
}

func (w *h4e2X11Win) close(t *testing.T) {
	t.Helper()
	var xDestroyWindow func(dpy uintptr, w uintptr) int
	purego.RegisterLibFunc(&xDestroyWindow, w.lib, "XDestroyWindow")
	xDestroyWindow(w.display, w.window)
	_ = purego.Dlclose(w.lib)
}

func TestH4E2_TriangleWindowPresentX11(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	vsWGSL := h4cReadWGSL(t, "h4c_triangle_vertex.wgsl")
	fsWGSL := h4cReadWGSL(t, "h4c_triangle_fragment.wgsl")
	rawGolden, err := os.ReadFile(filepath.Join("testdata", "h4c_triangle_golden.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var golden h4cTriangleGolden
	if err := json.Unmarshal(rawGolden, &golden); err != nil {
		t.Fatalf("parse golden: %v", err)
	}
	if golden.Tolerance != 0 {
		t.Fatalf("golden tolerance = %d, want 0 (bit-exact)", golden.Tolerance)
	}
	w, h := golden.Width, golden.Height
	if w <= 0 || h <= 0 {
		t.Fatalf("golden size = %dx%d, want positive", w, h)
	}

	xw := h4e2OpenX11Window(t, w, h)
	defer xw.close(t)

	// 纯 Go GL 临时路：Backend → Instance → Surface(Xlib) → Adapter → Open。
	// 不碰 render/present_target，不碰 SelectBackend。
	backend := NewBackend()
	inst, err := backend.CreateInstance(nil)
	if err != nil {
		t.Skipf("CreateInstance unavailable (no EGL?): %v", err)
	}
	defer inst.Release()

	surfHal, err := inst.CreateSurface(hal.SurfaceTarget{
		Kind:          hal.SurfaceTargetXlibWindow,
		DisplayHandle: xw.display,
		WindowHandle:  xw.window,
	})
	if err != nil {
		t.Fatalf("CreateSurface(Xlib): %v", err)
	}
	surf, ok := surfHal.(*Surface)
	if !ok {
		t.Fatalf("surface type = %T, want *gles.Surface", surfHal)
	}
	defer surf.Destroy()

	adapter, err := inst.RequestAdapter(&hal.RequestAdapterOptions{CompatibleSurface: surfHal})
	if err != nil {
		t.Fatalf("RequestAdapter: %v", err)
	}
	opened, err := adapter.Open(0, gputypes.DefaultLimits())
	if err != nil {
		t.Fatalf("Adapter.Open: %v", err)
	}
	dev, queue := opened.Device, opened.Queue

	cfg := &hal.SurfaceConfiguration{
		Width:       uint32(w),
		Height:      uint32(h),
		Format:      gputypes.TextureFormatRGBA8Unorm,
		Usage:       gputypes.TextureUsageRenderAttachment,
		PresentMode: hal.PresentModeFifo,
		AlphaMode:   gputypes.CompositeAlphaModeOpaque,
	}
	if err := surf.Configure(nil, cfg); err != nil {
		t.Fatalf("Surface.Configure(X11 window): %v", err)
	}
	if surf.swapchainFBO == 0 {
		t.Fatal("swapchainFBO == 0 after Configure")
	}
	if surf.eglSurface == 0 {
		t.Fatal("eglSurface == 0 after Configure (X11 window surface not created)")
	}

	vsMod, err := dev.CreateShaderModule(&ShaderModuleDescriptor{WGSL: vsWGSL})
	if err != nil {
		t.Fatalf("CreateShaderModule(vertex): %v", err)
	}
	fsMod, err := dev.CreateShaderModule(&ShaderModuleDescriptor{WGSL: fsWGSL})
	if err != nil {
		t.Fatalf("CreateShaderModule(fragment): %v", err)
	}
	pipe, err := dev.CreateRenderPipeline(&RenderPipelineDescriptor{
		Vertex: hal.VertexState{Module: vsMod, EntryPoint: "vs_main"},
		Fragment: &hal.FragmentState{
			Module:     fsMod,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{{
				Format:    cfg.Format,
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
	if err != nil {
		t.Fatalf("CreateRenderPipeline: %v", err)
	}

	// drawFrame draws one frame into the acquired surface texture: blue clear
	// with the triangle on top.
	drawFrame := func() hal.SurfaceTexture {
		t.Helper()
		acq, err := surf.AcquireTexture(nil)
		if err != nil {
			t.Fatalf("AcquireTexture: %v", err)
		}
		if acq == nil || acq.Texture == nil {
			t.Fatal("AcquireTexture returned nil texture")
		}
		if surf.current == nil {
			t.Fatal("in-flight frame not tracked after Acquire")
		}
		view, err := dev.CreateTextureView(acq.Texture, nil)
		if err != nil {
			t.Fatalf("CreateTextureView(surface): %v", err)
		}
		enc, err := dev.CreateCommandEncoder(nil)
		if err != nil {
			t.Fatalf("CreateCommandEncoder: %v", err)
		}
		rp, err := enc.BeginRenderPass(&hal.RenderPassDescriptor{
			ColorAttachments: []hal.RenderPassColorAttachment{{
				View:       view,
				LoadOp:     gputypes.LoadOpClear,
				StoreOp:    gputypes.StoreOpStore,
				ClearValue: gputypes.ColorBlue,
			}},
		})
		if err != nil {
			t.Fatalf("BeginRenderPass: %v", err)
		}
		rp.SetPipeline(pipe)
		rp.Draw(3, 1, 0, 0)
		if err := rp.End(); err != nil {
			t.Fatalf("RenderPass.End: %v", err)
		}
		cmd, err := enc.Finish()
		if err != nil {
			t.Fatalf("Encoder.Finish: %v", err)
		}
		if _, err := queue.Submit(cmd); err != nil {
			t.Fatalf("Queue.Submit: %v", err)
		}
		return acq.Texture
	}

	// 第一帧：画三角 → Present 换屏 → 从 X 服务器读窗口内容做三角级对照。
	// 直渲路下窗口默认帧缓冲就是被换上的那张，离屏路下窗口内容是生产 blit
	// 的输出；两条路断言的都是「窗上真显示的那张」。
	firstTex := drawFrame()

	if err := queue.Present(surfHal, firstTex, nil); err != nil {
		t.Fatalf("Queue.Present(frame1): %v", err)
	}
	if e := egl.GetError(); e != egl.Success {
		t.Fatalf("egl error after Present(frame1) = 0x%x, want 0x%x (eglSwapBuffers zero-error)", uint32(e), uint32(egl.Success))
	}
	if surf.current != nil {
		t.Error("in-flight frame not dropped after Present(frame1)")
	}

	// 换屏后的窗口内容由 X 服务器在下一个 vblank 取上新缓冲，等到本帧到位再断言。
	pixels := pollWindowPixels(t, xw.lib, xw.display, xw.window, w, h, func(p []byte) bool {
		s := md5.Sum(p)
		return hex.EncodeToString(s[:]) == golden.PixelsMD5
	})
	const source = "X server window copy"

	at := func(x, y int) []int {
		return pixelsAt(pixels, w, x, y)
	}
	eq := pixelsEqual
	if got := at(w/2, h/2); !eq(got, golden.Center) {
		t.Errorf("window frame center = %v, want %v (source %s)", got, golden.Center, source)
	}
	for _, p := range [][2]int{{0, 0}, {w - 1, 0}, {0, h - 1}, {w - 1, h - 1}} {
		if got := at(p[0], p[1]); !eq(got, golden.Corner) {
			t.Errorf("window frame corner %v = %v, want %v (source %s)", p, got, golden.Corner, source)
		}
	}
	sum := md5.Sum(pixels)
	if gotMD5 := hex.EncodeToString(sum[:]); gotMD5 != golden.PixelsMD5 {
		t.Errorf("window frame pixels md5 = %s, want %s (golden testdata/h4c_triangle_golden.json, source %s)", gotMD5, golden.PixelsMD5, source)
	} else {
		t.Logf("window frame pixels md5: %s (source %s)", gotMD5, source)
	}

	t.Logf("H4-e2 X11 triangle frame1 presented (window 0x%x, %dx%d)", xw.window, w, h)

	// 第二帧：再换一次屏，证连续 Present 零报错（换屏亮灯不断）。
	time.Sleep(100 * time.Millisecond)
	secondTex := drawFrame()
	if err := queue.Present(surfHal, secondTex, nil); err != nil {
		t.Fatalf("Queue.Present(frame2): %v", err)
	}
	if e := egl.GetError(); e != egl.Success {
		t.Fatalf("egl error after Present(frame2) = 0x%x, want 0x%x", uint32(e), uint32(egl.Success))
	}
	if surf.current != nil {
		t.Error("in-flight frame not dropped after Present(frame2)")
	}
	// 第二帧同样对照 golden：连续换屏后窗上内容仍是那张，中途不残留上一帧碎片。
	secondPixels := pollWindowPixels(t, xw.lib, xw.display, xw.window, w, h, func(p []byte) bool {
		s := md5.Sum(p)
		return hex.EncodeToString(s[:]) == golden.PixelsMD5
	})
	if got := pixelsAt(secondPixels, w, w/2, h/2); !eq(got, golden.Center) {
		t.Errorf("window frame2 center = %v, want %v (source %s)", got, golden.Center, source)
	}
	secondSum := md5.Sum(secondPixels)
	if gotMD5 := hex.EncodeToString(secondSum[:]); gotMD5 != golden.PixelsMD5 {
		t.Errorf("window frame2 pixels md5 = %s, want %s (golden testdata/h4c_triangle_golden.json, source %s)", gotMD5, golden.PixelsMD5, source)
	}
	t.Logf("H4-e2 X11 triangle frame2 presented, eglSwapBuffers zero-error x2")
}
