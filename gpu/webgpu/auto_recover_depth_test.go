//go:build linux && !nogpu && !js

package webgpu_test

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	rendgpu "github.com/energye/gpui/render/gpu"
)

// TestAutoRecover_SessionDepthAfterForceLost exercises the full minimize-equivalent
// path: allocate session-like depth → MarkLost → BeginFrame AutoRecover →
// CreateTexture session_depth_stencil must succeed (no "Not enough memory left").
func TestAutoRecover_SessionDepthAfterForceLost(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		_ = os.Setenv("DISPLAY", ":1")
	}
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		if st, err := os.Stat("lib/libwgpu_native.so"); err == nil && !st.IsDir() {
			_ = os.Setenv("WGPU_NATIVE_PATH", "lib/libwgpu_native.so")
			_ = os.Setenv("LD_LIBRARY_PATH", "lib:"+os.Getenv("LD_LIBRARY_PATH"))
		}
	}
	_ = os.Setenv("GPUI_SURFACE_SAMPLE_COUNT", "1")

	dpy, win, closeX, err := openTinyX11Window(320, 240)
	if err != nil {
		t.Skip(err)
	}
	defer closeX()

	inst, err := webgpu.CreateInstance(&webgpu.InstanceDescriptor{Backends: webgpu.BackendsPrimary})
	if err != nil {
		t.Skip(err)
	}
	defer inst.Release()

	surf, err := inst.CreateSurface(dpy, win)
	if err != nil {
		t.Fatalf("CreateSurface: %v", err)
	}
	defer surf.Release()

	adpt, err := inst.RequestAdapter(&webgpu.RequestAdapterOptions{
		PowerPreference:   webgpu.PowerPreferenceHighPerformance,
		CompatibleSurface: surf,
	})
	if err != nil {
		t.Skip(err)
	}
	defer adpt.Release()

	dev, err := adpt.RequestDevice(&webgpu.DeviceDescriptor{Label: "auto-recover-old"})
	if err != nil {
		t.Fatal(err)
	}

	if err := rendgpu.SetDeviceProvider(&webgpu.SimpleDeviceProvider{Dev: dev, Adpt: adpt}); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rendgpu.ResetAccelerator() }()

	sc := webgpu.NewSwapchain(surf, dev, 320, 240)
	sc.Usage = types.TextureUsageRenderAttachment
	sc.SetPreferVSync()
	if err := sc.ConfigureFromCapabilities(adpt); err != nil {
		t.Fatal(err)
	}
	defer sc.Release()

	sc.OnDeviceAbandon = func(_ *webgpu.Device) {
		rendgpu.AbandonDevice()
	}
	sc.EnableAutoRecover(adpt, "auto-recover-test", func(d *webgpu.Device) {
		if err := rendgpu.SetDeviceProvider(&webgpu.SimpleDeviceProvider{Dev: d, Adpt: adpt}); err != nil {
			t.Errorf("SetDeviceProvider after recover: %v", err)
		}
	})

	// Warm configure path with one acquire (session not required yet).
	fb0, err := sc.BeginFrame()
	if err != nil {
		t.Logf("warm BeginFrame: %v", err)
		sc.MarkNeedsReconfigure()
	} else {
		_ = sc.EndFrame(fb0)
	}

	// Force lost (minimize/TDR equivalent).
	dev.MarkLost()

	// First BeginFrames after recover return ErrRecovered (grace).
	var recovered bool
	for i := 0; i < 8; i++ {
		fb, err := sc.BeginFrame()
		if err != nil {
			if errors.Is(err, webgpu.ErrRecovered) {
				recovered = true
				t.Logf("frame %d: ErrRecovered (grace)", i)
				continue
			}
			if errors.Is(err, webgpu.ErrDeviceLost) {
				t.Logf("frame %d: still lost/recovering: %v", i, err)
				time.Sleep(20 * time.Millisecond)
				sc.ClearRecoverCooldown()
				continue
			}
			t.Fatalf("BeginFrame %d: %v", i, err)
		}
		// Successful acquire after recover grace — full Present path
		// (FlushGPUWithView → session ensureSurfaceTextures → session_depth_stencil).
		newDev := sc.Device
		if newDev == nil || newDev.IsLost() {
			sc.DiscardFrame(fb)
			t.Fatal("recovered device still nil/lost")
		}
		dc := render.NewContext(320, 240)
		defer dc.Close()
		dc.BeginFrame()
		dc.SetRGB(0.1, 0.2, 0.3)
		dc.DrawRectangle(0, 0, 320, 240)
		_ = dc.Fill()
		if err := dc.PresentFrameFull(fb.Handle, fb.Width, fb.Height, func() error {
			return sc.EndFrame(fb)
		}); err != nil {
			sc.DiscardFrame(fb)
			t.Fatalf("PresentFrameFull after AutoRecover: %v (recoveries=%d)", err, sc.Recoveries())
		}
		if sc.Recoveries() < 1 {
			t.Fatalf("expected recoveries>=1, got %d", sc.Recoveries())
		}
		// Drop recovered device/surface so -count=N does not starve VRAM.
		if sc.Device != nil {
			sc.Device.Release()
			sc.Device = nil
		}
		if sc.Surface != nil {
			sc.Surface.Release()
			sc.Surface = nil
		}
		t.Logf("PASS recoveries=%d recoveredGrace=%v", sc.Recoveries(), recovered)
		return
	}
	t.Fatalf("no successful frame after force lost (recoveries=%d)", sc.Recoveries())
}

func openTinyX11Window(w, h int) (dpy, win uintptr, close func(), err error) {
	lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return 0, 0, nil, err
	}
	var (
		xOpen    func(name *byte) uintptr
		xClose   func(dpy uintptr) int
		xScreen  func(dpy uintptr) int
		xRoot    func(dpy uintptr, screen int) uintptr
		xCreate  func(dpy, parent uintptr, x, y int, width, height, border uint, borderPx, bg uint64) uintptr
		xMap     func(dpy, win uintptr) int
		xFlush   func(dpy uintptr) int
		xDestroy func(dpy, win uintptr) int
	)
	purego.RegisterLibFunc(&xOpen, lib, "XOpenDisplay")
	purego.RegisterLibFunc(&xClose, lib, "XCloseDisplay")
	purego.RegisterLibFunc(&xScreen, lib, "XDefaultScreen")
	purego.RegisterLibFunc(&xRoot, lib, "XRootWindow")
	purego.RegisterLibFunc(&xCreate, lib, "XCreateSimpleWindow")
	purego.RegisterLibFunc(&xMap, lib, "XMapWindow")
	purego.RegisterLibFunc(&xFlush, lib, "XFlush")
	purego.RegisterLibFunc(&xDestroy, lib, "XDestroyWindow")

	d := xOpen(nil)
	if d == 0 {
		_ = purego.Dlclose(lib)
		return 0, 0, nil, fmt.Errorf("XOpenDisplay failed DISPLAY=%q", os.Getenv("DISPLAY"))
	}
	scr := xScreen(d)
	root := xRoot(d, scr)
	wnd := xCreate(d, root, 0, 0, uint(w), uint(h), 0, 0, 0)
	if wnd == 0 {
		xClose(d)
		_ = purego.Dlclose(lib)
		return 0, 0, nil, fmt.Errorf("XCreateSimpleWindow failed")
	}
	xMap(d, wnd)
	xFlush(d)
	time.Sleep(30 * time.Millisecond)
	return d, wnd, func() {}, nil // X cleanup skipped: race with recreated wgpu surface
}

// silence unused import if purego only
var _ = unsafe.Sizeof(0)
