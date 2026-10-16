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

// Window default framebuffer route evidence (default_fb_linux.go).
//
// H4F proves the direct route shows the golden image: the pass renders into
// FBO 0, which IS the presented buffer. The offscreen route leg renders the
// same frame through the swapchain FBO and the production present blit, so
// both routes are compared against the same golden.
//
// H4G measures what a frame starts from when it loads its target instead of
// clearing it (LoadOp=Load + damage scissor, the engine's steady state): which
// image the untouched pixels carry once the frame reaches the screen.
//
// The window image comes from the X server (captureWindowPixels): a
// glReadPixels of a window's FBO 0 is not a reliable oracle on a composited
// desktop. Every assertion here is on a presented frame, so what is compared
// is what the window shows.
//
// 无 X11/EGL 时 t.Skipf（缺真机数据不假绿）。

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

type h4fX11Win struct {
	lib     uintptr
	display uintptr
	window  uintptr
}

func h4fOpenX11Window(t *testing.T, w, h int, title string) *h4fX11Win {
	// 与 e2/e3 同序独立成份，单文件可跑。
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
		xCreateSimple  func(dpy uintptr, parent uintptr, x, y int, width, height uint, borderWidth uint, border, background uint64) uintptr
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
	name := append([]byte(title), 0)
	xStoreName(dpy, win, &name[0])
	xMapWindow(dpy, win)
	xFlush(dpy)
	waitUntilViewable(t, lib, dpy, win, 5*time.Second)

	return &h4fX11Win{lib: lib, display: dpy, window: win}
}

func (w *h4fX11Win) close(t *testing.T) {
	t.Helper()
	var xDestroyWindow func(dpy uintptr, w uintptr) int
	purego.RegisterLibFunc(&xDestroyWindow, w.lib, "XDestroyWindow")
	xDestroyWindow(w.display, w.window)
	_ = purego.Dlclose(w.lib)
}

// TestH4F_DirectDefaultFBWindowPixelX11 renders the H4C triangle through both
// routes on a real X11 window and compares the image the window shows against
// the tolerance-0 golden: directly (the pass wrote FBO 0, the presented
// buffer) and offscreen (swapchain FBO + the production present blit).
func TestH4F_DirectDefaultFBWindowPixelX11(t *testing.T) {
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

	xw := h4fOpenX11Window(t, w, h, "gpui-h4f-direct-fbo0-x11")
	defer xw.close(t)

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

	// Route guards: the test is only meaningful on the direct route, and the
	// route only exists when the window config carries the depth/stencil bits
	// a surface pass writes (default_fb_linux.go).
	savedDisabled := directDefaultFBDisabled
	defer func() {
		directDefaultFBDisabled = savedDisabled
		surf.enableDirectDefaultFB(surf.ctx.EGL())
	}()
	if directDefaultFBDisabled {
		t.Skip("GPUI_GLES_NO_DIRECT_FBO0 set: direct route disabled by env")
	}
	depth, stencil, ok := eglConfigHasDepthStencil(surf.ctx.EGL())
	if !ok || depth < 24 || stencil < 8 {
		t.Fatalf("window config depth/stencil = %d/%d ok=%v, want >=24/>=8 (the direct route's precondition)", depth, stencil, ok)
	}
	if !surf.directDefaultFB {
		t.Fatal("directDefaultFB = false on a 24/8 window config, want true (silent fallback to the offscreen route)")
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

	drawTriangle := func() hal.SurfaceTexture {
		t.Helper()
		acq, err := surf.AcquireTexture(nil)
		if err != nil {
			t.Fatalf("AcquireTexture: %v", err)
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

	// windowImage presents a frame and returns what the window shows for it,
	// from the X server (the reliable oracle for window content): the golden is
	// what tells the frame has landed, so the read waits for it.
	goldenLanded := func(pixels []byte) bool {
		sum := md5.Sum(pixels)
		return hex.EncodeToString(sum[:]) == golden.PixelsMD5
	}
	windowImage := func(leg string, tex hal.SurfaceTexture) []byte {
		t.Helper()
		if err := queue.Present(surfHal, tex, nil); err != nil {
			t.Fatalf("Queue.Present(%s): %v", leg, err)
		}
		if e := egl.GetError(); e != egl.Success {
			t.Fatalf("egl error after Present(%s) = 0x%x, want 0x%x", leg, uint32(e), uint32(egl.Success))
		}
		return pollWindowPixels(t, xw.lib, xw.display, xw.window, w, h, goldenLanded)
	}

	checkGolden := func(leg string, pixels []byte, source string) {
		t.Helper()
		at := func(x, y int) []int { return pixelsAt(pixels, w, x, y) }
		eq := func(a, b []int) bool {
			return len(a) == 4 && len(b) == 4 && a[0] == b[0] && a[1] == b[1] && a[2] == b[2] && a[3] == b[3]
		}
		if got := at(w/2, h/2); !eq(got, golden.Center) {
			t.Errorf("%s (%s): center = %v, want %v", leg, source, got, golden.Center)
		}
		for _, p := range [][2]int{{0, 0}, {w - 1, 0}, {0, h - 1}, {w - 1, h - 1}} {
			if got := at(p[0], p[1]); !eq(got, golden.Corner) {
				t.Errorf("%s (%s): corner %v = %v, want %v", leg, source, p, got, golden.Corner)
			}
		}
		sum := md5.Sum(pixels)
		if gotMD5 := hex.EncodeToString(sum[:]); gotMD5 != golden.PixelsMD5 {
			t.Errorf("%s (%s): pixels md5 = %s, want %s (golden testdata/h4c_triangle_golden.json)",
				leg, source, gotMD5, golden.PixelsMD5)
		} else {
			t.Logf("%s (%s): pixels md5 %s", leg, source, gotMD5)
		}
	}

	// --- Leg 1: direct route. The pass wrote FBO 0, so the swap put this
	// frame on screen as it was drawn.
	directPixels := windowImage("direct frame1", drawTriangle())
	checkGolden("direct route", directPixels, "X server window copy")
	// A second frame proves the route survives the swap and its depth/stencil
	// reset (ClearDefaultDepthStencilCommand) — the first frame after the swap
	// is the one that clears.
	secondPixels := windowImage("direct frame2", drawTriangle())
	checkGolden("direct route frame2", secondPixels, "X server window copy")

	// --- Leg 2: offscreen route, resolved through the production kill switch
	// at a frame boundary (no frame in flight).
	directDefaultFBDisabled = true
	surf.enableDirectDefaultFB(surf.ctx.EGL())
	if surf.directDefaultFB {
		t.Fatal("directDefaultFB still true after the kill switch (route re-resolution broken)")
	}

	offTex := drawTriangle()
	offPixels := readSwapchainPixels(t, surf, w, h)
	checkGolden("offscreen route target", offPixels, "swapchainFBO(offscreen)")
	// The window image is the present blit's output. Comparing it against the
	// swapchain readback — the engine's own pixels, exact on this route — is
	// also what validates the X-server oracle the direct leg is measured with.
	offWindowPixels := windowImage("offscreen frame", offTex)
	checkGolden("offscreen route window", offWindowPixels, "X server window copy")

	if len(offPixels) != len(offWindowPixels) {
		t.Fatalf("window image and swapchain image sizes differ: %d vs %d", len(offWindowPixels), len(offPixels))
	}
	for i := range offPixels {
		if offPixels[i] != offWindowPixels[i] {
			t.Fatalf("X server window copy differs from the swapchain readback at byte %d (row %d, col %d): window %d, swapchain %d",
				i, i/(w*4), (i%(w*4))/4, offWindowPixels[i], offPixels[i])
		}
	}
	t.Log("offscreen route: X server window copy is byte-identical to the engine's swapchain image")

	// Bit-exact route equality: the two routes' window images are the same
	// pixels, so the direct route's mirrored geometry plus the mirrored
	// fragment read cancel out against the present blit's flip.
	if len(directPixels) != len(offWindowPixels) {
		t.Fatalf("route window image sizes differ: %d vs %d", len(directPixels), len(offWindowPixels))
	}
	for i := range directPixels {
		if directPixels[i] != offWindowPixels[i] {
			t.Fatalf("route window images differ at byte %d (row %d, col %d): direct %d, offscreen %d",
				i, i/(w*4), (i%(w*4))/4, directPixels[i], offWindowPixels[i])
		}
	}
	t.Log("direct route and offscreen present blit produced bit-identical window pixels")
}

// TestH4G_DefaultFBPersistenceProbeX11 measures what a frame that loads its
// target instead of clearing it shows in the pixels it does not draw — the
// direct route's steady state (LoadOp=Load + damage scissor).
//
// The measurement is screen-level: the window copy after each present. Three
// full-target clears to distinct colours first (red, green, blue), so the load
// frame's untouched pixels name the image its buffer carried: the last
// presented one (blue) means the swap preserves the window back buffer, which
// is what drawing only this frame's damage needs; the one before last (green)
// means it does not. A fifth frame that loads and draws nothing must leave the
// window showing the loaded image unchanged.
//
// 无 X11/EGL 时 t.Skipf。
func TestH4G_DefaultFBPersistenceProbeX11(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	const (
		w = 64
		h = 64
	)
	xw := h4fOpenX11Window(t, w, h, "gpui-h4g-fbo0-persistence-x11")
	defer xw.close(t)

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

	if err := surf.Configure(nil, &hal.SurfaceConfiguration{
		Width:       w,
		Height:      h,
		Format:      gputypes.TextureFormatRGBA8Unorm,
		Usage:       gputypes.TextureUsageRenderAttachment,
		PresentMode: hal.PresentModeFifo,
		AlphaMode:   gputypes.CompositeAlphaModeOpaque,
	}); err != nil {
		t.Fatalf("Surface.Configure(X11 window): %v", err)
	}
	if !surf.directDefaultFB {
		t.Skip("offscreen route active (no 24/8 window config or kill switch): nothing to measure")
	}

	pipe := h4cTrianglePipeline(t, dev)

	// drawFrame clears to clearColor (LoadOpClear) and, when drawTri is set,
	// draws the red triangle on top (LoadOpLoad keeps the clear instead).
	drawFrame := func(loadOp gputypes.LoadOp, clearColor gputypes.Color, drawTri bool) hal.SurfaceTexture {
		t.Helper()
		acq, err := surf.AcquireTexture(nil)
		if err != nil {
			t.Fatalf("AcquireTexture: %v", err)
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
				LoadOp:     loadOp,
				StoreOp:    gputypes.StoreOpStore,
				ClearValue: clearColor,
			}},
		})
		if err != nil {
			t.Fatalf("BeginRenderPass: %v", err)
		}
		if drawTri {
			rp.SetPipeline(pipe)
			rp.Draw(3, 1, 0, 0)
		}
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

	red := gputypes.Color{R: 1, G: 0, B: 0, A: 1}
	green := gputypes.Color{R: 0, G: 1, B: 0, A: 1}
	blue := gputypes.Color{R: 0, G: 0, B: 1, A: 1}
	redRGBA := []int{255, 0, 0, 255}
	greenRGBA := []int{0, 255, 0, 255}
	blueRGBA := []int{0, 0, 255, 255}
	// uniform reports whether every pixel of img carries want.
	uniform := func(img []byte, want []int) bool {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if !pixelsEqual(pixelsAt(img, w, x, y), want) {
					return false
				}
			}
		}
		return true
	}
	// The frames of this test differ from each other, so each one is waited for
	// by its own content: a frame that never reaches the window fails the
	// assertions below on the image the last read returned.
	windowImage := func(leg string, tex hal.SurfaceTexture, landed func([]byte) bool) []byte {
		t.Helper()
		if err := queue.Present(surfHal, tex, nil); err != nil {
			t.Fatalf("Queue.Present(%s): %v", leg, err)
		}
		return pollWindowPixels(t, xw.lib, xw.display, xw.window, w, h, landed)
	}

	// Three full-target clears in three distinct colours: a loaded frame that
	// reads a buffer preserved across the swap shows the last one (blue), while
	// one that reads a buffer last written two presents ago shows green. The
	// steady state this route ships needs the first.
	for _, step := range []struct {
		leg  string
		col  gputypes.Color
		want []int
	}{
		{"red frame", red, redRGBA},
		{"green frame", green, greenRGBA},
		{"blue frame", blue, blueRGBA},
	} {
		want := step.want
		img := windowImage(step.leg, drawFrame(gputypes.LoadOpClear, step.col, false), func(p []byte) bool {
			return uniform(p, want)
		})
		for _, p := range [][2]int{{w / 2, h / 2}, {0, 0}, {w - 1, h - 1}} {
			if got := pixelsAt(img, w, p[0], p[1]); !pixelsEqual(got, step.want) {
				t.Errorf("%s: pixel %v = %v, want %v (full-target clear)", step.leg, p, got, step.want)
			}
		}
	}

	// Frame 4: load what is there, draw the red triangle — the direct route's
	// steady-state shape (draw only this frame's content, keep the rest).
	loaded := func(p []byte) bool {
		return pixelsEqual(pixelsAt(p, w, w/2, h/2), redRGBA) && pixelsEqual(pixelsAt(p, w, 0, 0), blueRGBA)
	}
	img := windowImage("LoadOpLoad + triangle", drawFrame(gputypes.LoadOpLoad, blue, true), loaded)
	center := pixelsAt(img, w, w/2, h/2)
	corner := pixelsAt(img, w, 0, 0)
	t.Logf("H4G Load+triangle frame: center = %v (want the red triangle), corner = %v (want the last presented image)", center, corner)
	// The triangle carries its colour from the vertex stage (red); the blue
	// clear value above is only the LoadOpClear argument the Load frame ignores.
	if !pixelsEqual(center, redRGBA) {
		t.Errorf("after Load+draw frame: center = %v, want the red triangle drawn this frame", center)
	}
	// The untouched pixels must carry the image the previous frame presented:
	// that is what makes drawing only this frame's damage correct. A frame of a
	// loaded pass that shows anything else (the frame before last, or content
	// no frame wrote) means the steady state is not safe on this swap.
	if !pixelsEqual(corner, blueRGBA) {
		t.Errorf("after Load+draw frame: untouched corner = %v, want %v (the last presented image the loaded buffer must hold)",
			corner, blueRGBA)
	}

	// Frame 5: load and draw nothing. Loading, storing and presenting the
	// buffer as it stands must leave the window showing the same image — an
	// empty frame is not a black or garbage frame.
	prev := img
	drawFrame(gputypes.LoadOpLoad, blue, false)
	time.Sleep(60 * time.Millisecond)
	img = captureWindowPixels(t, xw.lib, xw.display, xw.window, w, h)
	if len(img) != len(prev) {
		t.Fatalf("load-only frame size = %d, want %d", len(img), len(prev))
	}
	for i := range prev {
		if img[i] != prev[i] {
			t.Fatalf("load-only frame differs from the loaded image at byte %d (row %d, col %d): %d, want %d",
				i, i/(w*4), (i%(w*4))/4, img[i], prev[i])
		}
	}
	t.Log("H4G load-only frame: window still shows the loaded image byte for byte")
}

// TestH4H_DirectDefaultFBScissorX11 proves the direct route clips in the same
// place the offscreen route does.
//
// Scissor rects arrive in WebGPU's top-left pixel space. The offscreen route
// flips the scene in the vertex shader, so its rect passes into GL unchanged
// (SetScissorCommand); the direct route has no vertex flip
// (target_adjust.go), so GL's bottom-up rows need the rect mirrored
// (RenderPassEncoder.SetScissorRect). Both routes must clip the same pixels —
// a missing mirror shows up as the opposite half of the target being drawn.
//
// 无 X11/EGL 时 t.Skipf。
func TestH4H_DirectDefaultFBScissorX11(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	const (
		w = 64
		h = 64
	)
	xw := h4fOpenX11Window(t, w, h, "gpui-h4h-direct-fbo0-scissor-x11")
	defer xw.close(t)

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

	if err := surf.Configure(nil, &hal.SurfaceConfiguration{
		Width:       w,
		Height:      h,
		Format:      gputypes.TextureFormatRGBA8Unorm,
		Usage:       gputypes.TextureUsageRenderAttachment,
		PresentMode: hal.PresentModeFifo,
		AlphaMode:   gputypes.CompositeAlphaModeOpaque,
	}); err != nil {
		t.Fatalf("Surface.Configure(X11 window): %v", err)
	}

	savedDisabled := directDefaultFBDisabled
	defer func() {
		directDefaultFBDisabled = savedDisabled
		surf.enableDirectDefaultFB(surf.ctx.EGL())
	}()
	if directDefaultFBDisabled {
		t.Skip("GPUI_GLES_NO_DIRECT_FBO0 set: direct route disabled by env")
	}
	if !surf.directDefaultFB {
		t.Skip("offscreen route active (no 24/8 window config): nothing to compare")
	}

	pipe := h4cTrianglePipeline(t, dev)

	// The scissor covers the target's top half in WebGPU pixel space. The H4C
	// triangle spans rows 16..47, so a correct clip keeps its wide rows 16..31
	// and drops the tip at row 47.
	const scissorH = h / 2

	// drawScissoredFrame records the scissored frame and submits it.
	drawScissoredFrame := func() hal.SurfaceTexture {
		t.Helper()
		acq, err := surf.AcquireTexture(nil)
		if err != nil {
			t.Fatalf("AcquireTexture: %v", err)
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
		rp.SetScissorRect(0, 0, w, scissorH)
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

	// Half-split red counts in the image the window shows (row 0 = logical top,
	// the order both routes' images are normalised to).
	redCount := func(pixels []byte) (top, bottom int) {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if pixelsEqual(pixelsAt(pixels, w, x, y), []int{255, 0, 0, 255}) {
					if y < scissorH {
						top++
					} else {
						bottom++
					}
				}
			}
		}
		return top, bottom
	}
	redHalves := func(tag string, pixels []byte) (top, bottom int) {
		t.Helper()
		top, bottom = redCount(pixels)
		t.Logf("%s: red pixels above the scissor line = %d, below = %d", tag, top, bottom)
		return top, bottom
	}

	windowImage := func(leg string, tex hal.SurfaceTexture) []byte {
		t.Helper()
		if err := queue.Present(surfHal, tex, nil); err != nil {
			t.Fatalf("Queue.Present(%s): %v", leg, err)
		}
		// Every frame of this test carries the same scissored image, so the
		// frame has landed once the clip shows — the counts below are the
		// actual assertion.
		return pollWindowPixels(t, xw.lib, xw.display, xw.window, w, h, func(p []byte) bool {
			top, _ := redCount(p)
			return top > 0
		})
	}

	// Frame measured on the direct route: the pass wrote FBO 0, so the window
	// shows the image the scissor produced.
	directPixels := windowImage("direct scissored frame", drawScissoredFrame())
	directTop, directBottom := redHalves("direct route", directPixels)
	if directTop == 0 {
		t.Error("direct route: no triangle pixels in the scissored half, want the clip to keep them")
	}
	if directBottom != 0 {
		t.Error("direct route: triangle pixels below the scissor line, want them clipped away")
	}

	// Offscreen route with the same rect: its clip semantics are the shipped
	// ones, so its window image is the reference the direct route has to match.
	directDefaultFBDisabled = true
	surf.enableDirectDefaultFB(surf.ctx.EGL())
	if surf.directDefaultFB {
		t.Fatal("directDefaultFB still true after the kill switch (route re-resolution broken)")
	}

	offscreenPixels := windowImage("offscreen scissored frame", drawScissoredFrame())
	if offTop, offBottom := redHalves("offscreen route", offscreenPixels); offTop == 0 || offBottom != 0 {
		t.Errorf("offscreen route clipped red to %d/%d pixels (top/bottom), want the same half the direct route kept", offTop, offBottom)
	}

	if len(directPixels) != len(offscreenPixels) {
		t.Fatalf("scissored image sizes differ: %d vs %d", len(directPixels), len(offscreenPixels))
	}
	for i := range directPixels {
		if directPixels[i] != offscreenPixels[i] {
			t.Fatalf("scissored images differ at byte %d (row %d, col %d): direct %d, offscreen %d",
				i, i/(w*4), (i%(w*4))/4, directPixels[i], offscreenPixels[i])
		}
	}
	t.Log("scissored frame: direct route and offscreen route produced bit-identical window pixels")
}
