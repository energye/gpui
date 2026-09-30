//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

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
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/energye/gpui/gpu/gwgpu/gles/egl"
	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

//
// 显存对照见文档（整卡 nvidia-smi：闲时个位数 M，窗开后仍个位数增量，
// 对 WebGPU 空白窗 300M+ 量级）；测试内只留 1s 窗供外部采样。
// 无 X11/EGL 时 t.Skipf（缺真机数据不假绿）。

type h4e3X11Win struct {
	lib     uintptr
	display uintptr
	window  uintptr
}

func h4e3OpenX11Window(t *testing.T, w, h int) *h4e3X11Win {
	// 与 e2 同序独立成份，单文件可跑。
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
	name := append([]byte("gpui-h4e3-pelican-x11"), 0)
	xStoreName(dpy, win, &name[0])
	xMapWindow(dpy, win)
	xFlush(dpy)

	return &h4e3X11Win{lib: lib, display: dpy, window: win}
}

func (w *h4e3X11Win) close(t *testing.T) {
	t.Helper()
	var xDestroyWindow func(dpy uintptr, w uintptr) int
	purego.RegisterLibFunc(&xDestroyWindow, w.lib, "XDestroyWindow")
	xDestroyWindow(w.display, w.window)
	_ = purego.Dlclose(w.lib)
}

func TestH4E3_PelicanWindowPresentX11(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	vsWGSL := h4cReadWGSL(t, "h4c_triangle_vertex.wgsl")
	fsWGSL := h4cReadWGSL(t, "h4c_triangle_fragment.wgsl")
	rawGolden, err := os.ReadFile(filepath.Join("testdata", "h4e3_pelican_golden.json"))
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

	xw := h4e3OpenX11Window(t, w, h)
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
		t.Fatalf("Surface.Configure(X11 pelican-size window): %v", err)
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

	// 第一帧：画三角 → swapchainFBO 读回做鹈鹕级对照 → Present 换屏。
	firstTex := drawFrame()

	glCtx := surf.ctx.Lock()
	glCtx.Finish()
	glCtx.BindFramebuffer(gl.FRAMEBUFFER, surf.swapchainFBO)
	pixels := make([]byte, w*h*4)
	glCtx.ReadPixels(0, 0, int32(w), int32(h), gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&pixels[0]))
	glCtx.BindFramebuffer(gl.FRAMEBUFFER, 0)
	surf.ctx.Unlock()

	at := func(x, y int) []int {
		o := (y*w + x) * 4
		return []int{int(pixels[o]), int(pixels[o+1]), int(pixels[o+2]), int(pixels[o+3])}
	}
	eq := func(a, b []int) bool {
		return len(a) == 4 && len(b) == 4 && a[0] == b[0] && a[1] == b[1] && a[2] == b[2] && a[3] == b[3]
	}
	if got := at(w/2, h/2); !eq(got, golden.Center) {
		t.Errorf("pelican-size frame center = %v, want %v", got, golden.Center)
	}
	for _, p := range [][2]int{{0, 0}, {w - 1, 0}, {0, h - 1}, {w - 1, h - 1}} {
		if got := at(p[0], p[1]); !eq(got, golden.Corner) {
			t.Errorf("pelican-size frame corner %v = %v, want %v", p, got, golden.Corner)
		}
	}
	sum := md5.Sum(pixels)
	if gotMD5 := hex.EncodeToString(sum[:]); gotMD5 != golden.PixelsMD5 {
		t.Errorf("pelican-size frame pixels md5 = %s, want %s (golden testdata/h4e3_pelican_golden.json)", gotMD5, golden.PixelsMD5)
	} else {
		t.Logf("pelican-size frame pixels md5: %s", gotMD5)
	}

	if err := queue.Present(surfHal, firstTex, nil); err != nil {
		t.Fatalf("Queue.Present(frame1): %v", err)
	}
	if e := egl.GetError(); e != egl.Success {
		t.Fatalf("egl error after Present(frame1) = 0x%x, want 0x%x (eglSwapBuffers zero-error)", uint32(e), uint32(egl.Success))
	}
	if surf.current != nil {
		t.Error("in-flight frame not dropped after Present(frame1)")
	}
	t.Logf("H4-e3 X11 pelican-size frame1 presented (window 0x%x, %dx%d)", xw.window, w, h)

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
	t.Logf("H4-e3 X11 pelican-size frame2 presented, eglSwapBuffers zero-error x2")

	// 留窗 1s 供外部 nvidia-smi 采样显存对照（测试本身不断言显存数值）。
	time.Sleep(time.Second)
}
