//go:build linux && !nogpu

package render_test

// T4: real X11 window + swapchain resize + complex dynamic frames.

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ebitengine/purego"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	rendgpu "github.com/energye/gpui/render/gpu"
)

func TestMem_T4_WindowComplex_ResizeChurn(t *testing.T) {
	if os.Getenv("GPUI_FORCE_NO_X11") == "1" {
		t.Skip("GPUI_FORCE_NO_X11=1")
	}
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no DISPLAY")
	}
	memRequireGPU(t)

	_ = os.Setenv("GPUI_SURFACE_SAMPLE_COUNT", "1")
	_ = rendgpu.ResetAccelerator()

	frames := memEnvIters(48)
	rng := newMemRNG(memSeed() + 99)
	font := memFindFont(t)
	img := memMakeCheckerImage(t, 28)

	winW, winH := 320, 240
	xw := memOpenX11(t, winW, winH)

	inst, err := webgpu.CreateInstance(&webgpu.InstanceDescriptor{Backends: webgpu.BackendsPrimary})
	if err != nil {
		xw.close()
		t.Skipf("CreateInstance: %v", err)
	}

	surf, err := inst.CreateSurface(xw.display, xw.window)
	if err != nil {
		inst.Release()
		xw.close()
		t.Fatalf("CreateSurface: %v", err)
	}

	adapter, err := inst.RequestAdapter(&webgpu.RequestAdapterOptions{
		PowerPreference:   webgpu.PowerPreferenceHighPerformance,
		CompatibleSurface: surf,
	})
	if err != nil {
		surf.Release()
		inst.Release()
		xw.close()
		t.Fatalf("RequestAdapter: %v", err)
	}

	device, err := adapter.RequestDevice(rendgpu.DeviceDescriptor("mem-t4-x11"))
	if err != nil {
		adapter.Release()
		surf.Release()
		inst.Release()
		xw.close()
		t.Skipf("RequestDevice: %v", err)
	}

	sc := webgpu.NewSwapchain(surf, device, uint32(winW), uint32(winH))
	sc.Usage = types.TextureUsageRenderAttachment
	sc.SetPreferVSync()
	if err := sc.ConfigureFromCapabilities(adapter); err != nil {
		device.Release()
		adapter.Release()
		surf.Release()
		inst.Release()
		xw.close()
		t.Fatalf("Configure: %v", err)
	}

	if err := rendgpu.SetDeviceProvider(&webgpu.SimpleDeviceProvider{
		Dev: device, Adpt: adapter, Format: sc.Format,
	}); err != nil {
		sc.Release()
		device.Release()
		adapter.Release()
		surf.Release()
		inst.Release()
		xw.close()
		t.Fatalf("SetDeviceProvider: %v", err)
	}

	dc := render.NewContext(winW, winH)
	// Cleanup order (LIFO registration → reverse run):
	// 1) close draw context + reset accelerator (may still touch device)
	// 2) unconfigure swapchain, wait idle, drop device/adapter
	// 3) release surface while X11 window still alive
	// 4) close X11 last
	// Defers must NOT close X11 before surface: that SIGSEGVs wgpuSurfaceRelease.
	t.Cleanup(func() { xw.close() })
	t.Cleanup(func() {
		sc.Release()
		_ = device.WaitIdle()
		device.Release()
		adapter.Release()
		surf.Release()
		inst.Release()
	})
	t.Cleanup(func() {
		_ = dc.Close()
		_ = rendgpu.ResetAccelerator()
	})
	if err := dc.LoadFontFace(font, 13); err != nil {
		t.Fatalf("font: %v", err)
	}

	sizes := [][2]int{
		{280, 200}, {320, 240}, {400, 280}, {360, 320}, {480, 300},
		{240, 240}, {512, 320}, {300, 400},
	}
	rss := make([]int64, 0, frames)

	for i := 0; i < frames; i++ {
		// Random size change every few frames
		if i%5 == 0 || i%7 == 0 {
			sz := sizes[rng.intn(len(sizes))]
			winW, winH = sz[0], sz[1]
			xw.resize(winW, winH)
			xw.flush()
			if err := sc.Resize(uint32(winW), uint32(winH)); err != nil {
				t.Fatalf("sc.Resize: %v", err)
			}
			if err := dc.Resize(winW, winH); err != nil {
				t.Fatalf("dc.Resize: %v", err)
			}
		}

		dc.BeginFrame()
		dc.ResetRenderPathStats()
		lvl := memSceneComplex
		if i%4 == 0 {
			lvl = memSceneMedium
		}
		memDrawScene(t, dc, winW, winH, i, lvl, rng, img)

		frame, err := sc.BeginFrame()
		if err != nil {
			t.Fatalf("BeginFrame: %v", err)
		}
		present := func() error { return sc.EndFrame(frame) }
		// Prefer Auto for damage-ish; full when large resize frame
		if i%5 == 0 {
			if err := dc.PresentFrameFull(frame.Handle, frame.Width, frame.Height, present); err != nil {
				sc.DiscardFrame(frame)
				t.Fatalf("PresentFrameFull: %v", err)
			}
		} else {
			if _, err := dc.PresentFrameAuto(frame.Handle, frame.Width, frame.Height, present); err != nil {
				sc.DiscardFrame(frame)
				t.Fatalf("PresentFrameAuto: %v", err)
			}
		}
		if dc.RenderPathStats().GPUOps == 0 && i%5 == 0 {
			// full frame must have GPU ops
			t.Fatalf("GPUOps==0 on full frame: %s", dc.RenderPathStats().LogLine())
		}
		if dc.RenderPathStats().CPUFallbackOps != 0 {
			t.Fatalf("cpu_fb=%d", dc.RenderPathStats().CPUFallbackOps)
		}
		memHardRSSCheck(t)
		if i >= frames/10 {
			rss = append(rss, memRSSKB())
		}
	}

	st := sc.Stats()
	t.Logf("T4 window frames=%d reconfig=%d acquires=%d presents=%d rss=%dKB",
		frames, st.Reconfigures, st.Acquires, st.Presents, memRSSKB())
	if st.Presents < uint64(frames/2) {
		t.Fatalf("too few presents: %d", st.Presents)
	}
	delta := memEnvInt64("GPUI_MEM_RSS_DELTA_KB", 96*1024)
	memAssertSteadyRSS(t, rss, delta, "T4")
}

// --- X11 helper with resize ---

type memX11 struct {
	lib     uintptr
	display uintptr
	window  uintptr
	close   func()
	flush   func()
	resize  func(w, h int)
}

func memOpenX11(t *testing.T, w, h int) *memX11 {
	t.Helper()
	lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		lib, err = purego.Dlopen("libX11.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	}
	if err != nil {
		t.Skipf("libX11: %v", err)
	}
	var (
		xOpenDisplay   func(name *byte) uintptr
		xCloseDisplay  func(dpy uintptr) int
		xDefaultScreen func(dpy uintptr) int
		xRootWindow    func(dpy uintptr, screen int) uintptr
		xCreateSimple  func(dpy uintptr, parent uintptr, x, y int, width, height, borderWidth uint, border, background uint64) uintptr
		xMapWindow     func(dpy uintptr, win uintptr) int
		xFlush         func(dpy uintptr) int
		xDestroyWindow func(dpy uintptr, win uintptr) int
		xStoreName     func(dpy uintptr, win uintptr, name *byte) int
		xResizeWindow  func(dpy uintptr, win uintptr, width, height uint) int
	)
	purego.RegisterLibFunc(&xOpenDisplay, lib, "XOpenDisplay")
	purego.RegisterLibFunc(&xCloseDisplay, lib, "XCloseDisplay")
	purego.RegisterLibFunc(&xDefaultScreen, lib, "XDefaultScreen")
	purego.RegisterLibFunc(&xRootWindow, lib, "XRootWindow")
	purego.RegisterLibFunc(&xCreateSimple, lib, "XCreateSimpleWindow")
	purego.RegisterLibFunc(&xMapWindow, lib, "XMapWindow")
	purego.RegisterLibFunc(&xFlush, lib, "XFlush")
	purego.RegisterLibFunc(&xDestroyWindow, lib, "XDestroyWindow")
	purego.RegisterLibFunc(&xStoreName, lib, "XStoreName")
	purego.RegisterLibFunc(&xResizeWindow, lib, "XResizeWindow")

	dpy := xOpenDisplay(nil)
	if dpy == 0 {
		_ = purego.Dlclose(lib)
		t.Skip("XOpenDisplay failed")
	}
	screen := xDefaultScreen(dpy)
	root := xRootWindow(dpy, screen)
	win := xCreateSimple(dpy, root, 40, 40, uint(w), uint(h), 1, 0, 0)
	if win == 0 {
		xCloseDisplay(dpy)
		_ = purego.Dlclose(lib)
		t.Skip("XCreateSimpleWindow failed")
	}
	name := append([]byte(fmt.Sprintf("gpui-mem-t4-%d", time.Now().UnixNano()%100000)), 0)
	xStoreName(dpy, win, &name[0])
	xMapWindow(dpy, win)
	xFlush(dpy)

	return &memX11{
		lib: lib, display: dpy, window: win,
		close: func() {
			xDestroyWindow(dpy, win)
			xCloseDisplay(dpy)
			_ = purego.Dlclose(lib)
		},
		flush: func() { xFlush(dpy) },
		resize: func(nw, nh int) {
			if nw < 64 {
				nw = 64
			}
			if nh < 64 {
				nh = 64
			}
			xResizeWindow(dpy, win, uint(nw), uint(nh))
		},
	}
}
