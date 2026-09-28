//go:build linux && !nogpu

package main

// P1-1b 全场景离屏特征门：同一棵鹈鹕树（static+stage，与 renderGolden 同画法，
// PELICAN_T=5 冻结）在盒装 GL 设备与纯 CPU 下各画一帧，对比差异率。
//
// 立门依据（2026-09-28）：CPU 是软件光栅，GL 是 GPU 光栅，两者抗锯齿与混合
// 天生有像素差（基线约 15%，大差异约 6%），CPU 零差立不住。逐位零差留给
// P1-2 的 WebGPU 快照（同为 GPU 光栅才可比）。本门只锁：无整块缺失、
// 零回退、差异率低于基线加余量。
//
// 无 DISPLAY/X11/EGL => Skip（与 P1 离屏探针同纪律，不假绿）。
// PNG 双写 t.TempDir() 供人工看图。

import (
	"image"
	"image/png"
	"os"
	"runtime"
	"testing"

	"github.com/ebitengine/purego"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"

	// Register the pure-Go GL backend (P1 temporary path).
	_ "github.com/energye/gpui/gpu/gwgpu/gles"
	// Register the render GPU accelerator (provider injection target).
	_ "github.com/energye/gpui/render/gpu"
)

const (
	// p1PelicanDiffCap 是 GL vs CPU 差异率上限：基线约 15%（2026-09-28
	// Mesa 实测 128908/840000），余量放到 25%。超了说明有整块缺失或回归。
	p1PelicanDiffCap = 0.25
	// p1PelicanHugeCap 是大差异（通道和>100）像素占比上限：基线约 6%，
	// 放到 10%。整块黑色/透明会把这个数字顶穿。
	p1PelicanHugeCap = 0.10
)

// p1X11Window is a minimal X11 window used only as a GL context provider.
// Pixels are validated offscreen; the window is never presented to.
type p1X11Window struct {
	display uintptr
	window  uintptr
	close   func()
}

func p1OpenX11(t *testing.T, w, h int) *p1X11Window {
	t.Helper()
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no DISPLAY")
	}
	lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		lib, err = purego.Dlopen("libX11.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	}
	if err != nil {
		t.Skipf("libX11: %v", err)
	}
	var (
		xOpenDisplay   func(name *byte) uintptr
		xDefaultScreen func(dpy uintptr) int
		xRootWindow    func(dpy uintptr, screen int) uintptr
		xCreateSimple  func(dpy uintptr, parent uintptr, x, y int, width, height, borderWidth uint, border, background uint64) uintptr
		xMapWindow     func(dpy uintptr, win uintptr) int
		xFlush         func(dpy uintptr) int
		xDestroyWindow func(dpy uintptr, win uintptr) int
	)
	purego.RegisterLibFunc(&xOpenDisplay, lib, "XOpenDisplay")
	purego.RegisterLibFunc(&xDefaultScreen, lib, "XDefaultScreen")
	purego.RegisterLibFunc(&xRootWindow, lib, "XRootWindow")
	purego.RegisterLibFunc(&xCreateSimple, lib, "XCreateSimpleWindow")
	purego.RegisterLibFunc(&xMapWindow, lib, "XMapWindow")
	purego.RegisterLibFunc(&xFlush, lib, "XFlush")
	purego.RegisterLibFunc(&xDestroyWindow, lib, "XDestroyWindow")

	dpy := xOpenDisplay(nil)
	if dpy == 0 {
		t.Skip("XOpenDisplay failed")
	}
	screen := xDefaultScreen(dpy)
	root := xRootWindow(dpy, screen)
	win := xCreateSimple(dpy, root, 40, 40, uint(w), uint(h), 1, 0, 0)
	if win == 0 {
		t.Skip("XCreateSimpleWindow failed")
	}
	xMapWindow(dpy, win)
	xFlush(dpy)

	var closed bool
	return &p1X11Window{
		display: dpy,
		window:  win,
		close: func() {
			if closed {
				return
			}
			closed = true
			// Destroy the window only; XCloseDisplay/Dlclose have SIGSEGV'd
			// in cleanup after surface release (purego + X11 race). The
			// Display leak is reclaimed on process exit.
			xDestroyWindow(dpy, win)
			xFlush(dpy)
		},
	}
}

// p1ProbeProvider carries a hal.Device through the boxed encoding.
type p1ProbeProvider struct {
	dev hal.Device
	ad  hal.Adapter
}

func (p *p1ProbeProvider) Device() gpucontext.Device {
	if p == nil || p.dev == nil {
		return gpucontext.Device{}
	}
	return gpucontext.PackDevice(p.dev)
}

func (p *p1ProbeProvider) Queue() gpucontext.Queue {
	if p == nil || p.dev == nil {
		return gpucontext.Queue{}
	}
	return gpucontext.PackQueue(p.dev.Queue())
}

func (p *p1ProbeProvider) SurfaceFormat() gputypes.TextureFormat {
	return gputypes.TextureFormatBGRA8Unorm
}

func (p *p1ProbeProvider) Adapter() gpucontext.Adapter {
	if p == nil || p.ad == nil {
		return gpucontext.Adapter{}
	}
	return gpucontext.PackAdapter(p.ad)
}

func (p *p1ProbeProvider) AdapterInfo() gpucontext.AdapterInfo {
	if p == nil || p.ad == nil {
		return gpucontext.AdapterInfo{Type: gpucontext.AdapterTypeUnknown}
	}
	info := p.ad.Info()
	ai := gpucontext.AdapterInfo{Name: info.Name}
	switch info.DeviceType {
	case gputypes.DeviceTypeDiscreteGPU:
		ai.Type = gpucontext.AdapterTypeDiscrete
	case gputypes.DeviceTypeIntegratedGPU:
		ai.Type = gpucontext.AdapterTypeIntegrated
	case gputypes.DeviceTypeCPU:
		ai.Type = gpucontext.AdapterTypeSoftware
	default:
		ai.Type = gpucontext.AdapterTypeUnknown
	}
	return ai
}

// p1GLDevice opens a boxed GL device using a minimal X11 window as the
// context provider. Pinned to one OS thread (EGL thread affinity: unpinned
// rapid device churn intermittently negotiates GLSL 330 and fails SDF
// compile with an empty info log).
func p1GLDevice(t *testing.T) (hal.Device, hal.Adapter) {
	t.Helper()
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	if render.Accelerator() == nil {
		t.Skip("GPU accelerator not registered")
	}
	be, ok := hal.GetBackend(gputypes.BackendGL)
	if !ok || be == nil {
		t.Skip("BackendGL not registered")
	}
	inst, err := be.CreateInstance(&hal.InstanceDescriptor{Backends: gputypes.BackendsPrimary})
	if err != nil {
		t.Skipf("GL CreateInstance unavailable: %v", err)
	}
	t.Cleanup(inst.Release)

	xw := p1OpenX11(t, 64, 64)
	t.Cleanup(xw.close)
	surf, err := inst.CreateSurface(hal.SurfaceTarget{
		Kind:          hal.SurfaceTargetXlibWindow,
		DisplayHandle: xw.display,
		WindowHandle:  xw.window,
	})
	if err != nil {
		t.Skipf("GL CreateSurface(Xlib) unavailable: %v", err)
	}
	t.Cleanup(surf.Destroy)

	ad, err := inst.RequestAdapter(&hal.RequestAdapterOptions{
		PowerPreference:   gputypes.PowerPreferenceHighPerformance,
		CompatibleSurface: surf,
	})
	if err != nil {
		t.Skipf("GL RequestAdapter unavailable: %v", err)
	}
	t.Cleanup(ad.Release)

	dev, err := ad.RequestDevice(render.DeviceDescriptor("p1-pelican"))
	if err != nil {
		t.Skipf("GL RequestDevice unavailable: %v", err)
	}
	t.Cleanup(dev.Release)
	return dev, ad
}

// renderFrozenFrame builds a fresh scene at PELICAN_T and paints static+stage
// (same calls as renderGolden) into a 1200x700 context. Bake and frame share
// the active backend so tiles stay consistent within one pass.
func renderFrozenFrame(t *testing.T) (image.Image, render.RenderPathStats, string) {
	t.Helper()
	t.Setenv("PELICAN_T", "5")
	sc := newPelicanScene(winW, winH)
	dc := render.NewContext(int(stageW), int(stageH))
	defer dc.Close()
	dc.SetRGBA(0.47, 0.78, 0.96, 1)
	dc.Clear()
	pc := rendering.NewPaintContext(dc, 1)
	size := rendering.Size{Width: stageW, Height: stageH}
	sc.paintStatic(pc, size)
	sc.paintStage(pc, size)
	img := dc.Image()
	return img, dc.RenderPathStats(), dc.LastCPUFallbackReason()
}

func savePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
}

func TestP1OffscreenPelicanGLvsCPU(t *testing.T) {
	dev, ad := p1GLDevice(t)

	// GL pass first (CPU pass touches no GPU session, so no cross pollution).
	if err := render.SetAcceleratorDeviceProvider(&p1ProbeProvider{dev: dev, ad: ad}); err != nil {
		t.Fatalf("SetAcceleratorDeviceProvider(boxed GL): %v", err)
	}
	glImg, glStats, glFallback := renderFrozenFrame(t)
	t.Logf("gl frame %s fallback=%q", glStats.LogLine(), glFallback)
	if glStats.GPUOps == 0 {
		t.Fatalf("GL frame drew nothing: %s", glStats.LogLine())
	}
	if glStats.CPUFallbackOps != 0 {
		t.Fatalf("GL frame must not CPU-fallback: %s reason=%q",
			glStats.LogLine(), glFallback)
	}
	render.AbandonAcceleratorDevice()

	// CPU reference: fresh scene, same frozen frame, no accelerator.
	cpuImg, _, _ := renderFrozenFrame(t)

	dir := t.TempDir()
	savePNG(t, dir+"/pelican_gl.png", glImg)
	savePNG(t, dir+"/pelican_cpu.png", cpuImg)
	t.Logf("pngs: %s/pelican_gl.png %s/pelican_cpu.png", dir, dir)

	b := glImg.Bounds()
	if !b.Eq(cpuImg.Bounds()) {
		t.Fatalf("bounds differ: gl=%v cpu=%v", b, cpuImg.Bounds())
	}
	px := func(img image.Image, x, y int) (int, int, int, int) {
		r, g, b, a := img.At(x, y).RGBA()
		return int(r >> 8), int(g >> 8), int(b >> 8), int(a >> 8)
	}
	// Sky anchor: readback orientation lock. A Y-flip anywhere shows up
	// here first (top sky vs bottom grass differ strongly).
	gr, gg, gb, ga := px(glImg, 0, 0)
	cr, cg, cb, ca := px(cpuImg, 0, 0)
	t.Logf("(0,0) gl=(%d,%d,%d,%d) cpu=(%d,%d,%d,%d)", gr, gg, gb, ga, cr, cg, cb, ca)
	if gr != cr || gg != cg || gb != cb || ga != ca {
		t.Fatalf("sky anchor (0,0) differs: gl=(%d,%d,%d,%d) cpu=(%d,%d,%d,%d)",
			gr, gg, gb, ga, cr, cg, cb, ca)
	}

	var diffCount, hugeCount int
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r1, g1, b1, a1 := glImg.At(x, y).RGBA()
			r2, g2, b2, a2 := cpuImg.At(x, y).RGBA()
			d := absDiff(int(r1>>8), int(r2>>8)) + absDiff(int(g1>>8), int(g2>>8)) +
				absDiff(int(b1>>8), int(b2>>8)) + absDiff(int(a1>>8), int(a2>>8))
			if d > 0 {
				diffCount++
				if d > 100 {
					hugeCount++
				}
			}
		}
	}
	total := b.Dx() * b.Dy()
	diffRate := float64(diffCount) / float64(total)
	hugeRate := float64(hugeCount) / float64(total)
	t.Logf("gl-vs-cpu: total=%d diff=%d (%.3f) huge=%d (%.3f) %s",
		total, diffCount, diffRate, hugeCount, hugeRate, glStats.LogLine())
	if diffRate > p1PelicanDiffCap {
		t.Fatalf("gl-vs-cpu diff rate %.3f above cap %.2f (%d/%d)",
			diffRate, p1PelicanDiffCap, diffCount, total)
	}
	if hugeRate > p1PelicanHugeCap {
		t.Fatalf("gl-vs-cpu huge-diff rate %.3f above cap %.2f (%d/%d)",
			hugeRate, p1PelicanHugeCap, hugeCount, total)
	}
}

func absDiff(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}
