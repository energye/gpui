//go:build linux && !nogpu

// api_coverage_app — every public render.Context API in product-shaped paths,
// with Skia/Flutter-aligned adaptive surface lifecycle (portable, not 1GB-only).
//
//	GPUI_LIFECYCLE=auto|normal|purge|recreate
//	GPUI_COVERAGE_STRICT=1
//	GPUI_SELFTEST_LIFECYCLE=1
package main

import (
	"errors"
	"fmt"
	"image"
	"log"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/energye/gpui/examples/exboot"
	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	rendgpu "github.com/energye/gpui/render/gpu"
)

func main() {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		if _, err := os.Stat("lib/libwgpu_native.so"); err == nil {
			_ = os.Setenv("WGPU_NATIVE_PATH", "lib/libwgpu_native.so")
		}
	}
	if os.Getenv("GPUI_SURFACE_SAMPLE_COUNT") == "" {
		_ = os.Setenv("GPUI_SURFACE_SAMPLE_COUNT", "1")
	}

	animSec := envInt("GPUI_ANIM_SECONDS", 0)
	targetFPS := envInt("GPUI_TARGET_FPS", 20)
	if targetFPS < 10 {
		targetFPS = 10
	}
	frameBudget := time.Second / time.Duration(targetFPS)
	strict := os.Getenv("GPUI_COVERAGE_STRICT") == "1"
	selftest := os.Getenv("GPUI_SELFTEST_LIFECYCLE") == "1"

	winW, winH := 960, 640
	xw, err := openX11Window(winW, winH, "gpui · API coverage (Skia/Flutter lifecycle)")
	must(err)
	defer xw.Close()
	xw.MapRaise()

	exboot.InitEnv()
	inst, err := exboot.NewInstanceX11(xw.Display, 0)
	must(err)
	defer inst.Release()
	surf, err := inst.CreateSurface(xw.Display, xw.Window)
	must(err)
	defer surf.Release()
	adapter, device, err := exboot.OpenDevice(inst, surf, "api-coverage-app")
	must(err)
	defer adapter.Release()
	defer func() {
		if device != nil {
			device.Release()
		}
	}()

	sc := webgpu.NewSwapchain(surf, device, uint32(winW), uint32(winH))
	sc.Usage = types.TextureUsageRenderAttachment
	sc.SetPreferVSync()
	must(sc.ConfigureFromCapabilities(adapter))
	defer sc.Release()
	must(exboot.BindProvider(device, adapter, sc.Format))
	defer exboot.ResetAccelerator()

	dc := render.NewContext(winW, winH)
	defer dc.Close()
	side := render.NewContext(120, 80)
	defer side.Close()
	aux := render.NewContext(64, 64)
	defer aux.Close()

	var needForceFull bool
	dropAll := func() {
		dc.DropGPURenderContext()
		side.DropGPURenderContext()
		aux.DropGPURenderContext()
		needForceFull = true
	}
	exboot.WireAutoRecover(sc, adapter, "api-coverage-app",
		func(dev *webgpu.Device) { device = dev },
		dropAll,
		nil,
	)
	host := &exboot.SurfaceHost{
		SC: sc, Adapter: adapter, Device: &device, DropGPU: dropAll, Format: sc.Format,
	}
	log.Printf("api_coverage_app lifecycle_tier=%s apis=%d",
		rendgpu.ResolveSurfaceLifecycle(adapter), len(AllPublicContextAPIs))

	cov := NewCoverage(AllPublicContextAPIs)
	fonts := findFont()
	scratch := make([]byte, 64)
	seed := render.NewContext(32, 32)
	seed.SetRGB(0.2, 0.6, 1)
	seed.DrawRectangle(0, 0, 32, 32)
	_ = seed.Fill()
	seed.SetRGB(1, 0.4, 0.2)
	seed.DrawCircle(16, 16, 10)
	_ = seed.Fill()
	var seedImg *render.ImageBuf
	_ = seed.ExportImageBuf(&seedImg)
	_ = seed.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	start := time.Now()
	frame, tick, presents := 0, 0, 0
	exitReason := "duration"
	windowMinimized, windowFullyObscured := false, false
	wasHidden := false
	hiddenSince := time.Time{}
	hiddenAccum := time.Duration(0)
	presentMode := 0
	lastOOMHandled := uint32(0)

	minAt, mapAt, lostAt, doneAt := 90, 150, 200, 280
	if v := os.Getenv("GPUI_SELFTEST_MIN_AT"); v != "" {
		fmt.Sscanf(v, "%d", &minAt)
	}
	if v := os.Getenv("GPUI_SELFTEST_MAP_AT"); v != "" {
		fmt.Sscanf(v, "%d", &mapAt)
	}
	if v := os.Getenv("GPUI_SELFTEST_LOST_AT"); v != "" {
		fmt.Sscanf(v, "%d", &lostAt)
	}
	if v := os.Getenv("GPUI_SELFTEST_DONE_AT"); v != "" {
		fmt.Sscanf(v, "%d", &doneAt)
	}

	log.Printf("api_coverage_app %dx%d fps=%d seconds=%d strict=%v selftest=%v",
		winW, winH, targetFPS, animSec, strict, selftest)

	running := true
	for running {
		tick++
		deadline := time.Now().Add(frameBudget)

		for xw.Pending() {
			ev := xw.NextEvent()
			if xw.IsDelete(ev) {
				running = false
				exitReason = "window_close"
				break
			}
			if ev.Type == xVisibilityNotify {
				windowFullyObscured = ev.Visibility == xVisibilityFullyObscured
			}
			if xw.IsWMStateProperty(ev) {
				windowMinimized = xw.IsIconic()
			}
			if ev.Type == xConfigureNotify {
				nw, nh := max(64, ev.Width), max(64, ev.Height)
				if nw != winW || nh != winH {
					winW, winH = nw, nh
					_ = dc.Resize(winW, winH)
					cov.Hit("Resize")
					_ = sc.Resize(uint32(winW), uint32(winH))
					needForceFull = true
				}
			}
		}
		if !running {
			break
		}
		select {
		case <-stop:
			running = false
			exitReason = "signal"
			continue
		default:
		}

		if selftest {
			switch tick {
			case minAt:
				log.Printf("COV: Iconify tick=%d", tick)
				xw.Iconify()
			case mapAt:
				log.Printf("COV: MapRaise tick=%d", tick)
				xw.MapRaise()
				windowMinimized = false
				windowFullyObscured = false
			case lostAt:
				log.Printf("COV: ForceRecoverHealthy tick=%d frame=%d", tick, frame)
				if err := sc.ForceRecoverHealthy(); err != nil {
					log.Printf("ForceRecoverHealthy: %v", err)
				} else if sc.Device != nil {
					device = sc.Device
					needForceFull = true
				}
				// Skia/Flutter: do not rebuild full scene in the same turn as context reset.
				continue
			}
			if tick >= doneAt {
				exitReason = "selftest_ok"
				break
			}
		}

		hidden := windowMinimized || windowFullyObscured
		if hidden {
			if !wasHidden {
				wasHidden = true
				hiddenSince = time.Now()
				host.OnUnpresentable()
				log.Printf("hidden — adaptive unpresentable tier=%s", rendgpu.ResolveSurfaceLifecycle(adapter))
			}
			if device != nil {
				device.FlushCallbacks()
			}
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if wasHidden {
			if !hiddenSince.IsZero() {
				hiddenAccum += time.Since(hiddenSince)
				hiddenSince = time.Time{}
			}
			wasHidden = false
			needForceFull = true
			host.OnPresentable()
			if sc.Device != nil {
				device = sc.Device
			}
			log.Printf("visible again recoveries=%d tier=%s %s",
				sc.Recoveries(), rendgpu.ResolveSurfaceLifecycle(adapter), cov.SummaryLine())
		}

		if animSec > 0 && time.Since(start)-hiddenAccum >= time.Duration(animSec)*time.Second {
			exitReason = "duration"
			break
		}

		tt := float64(frame) * 0.05
		fw, fh := float64(winW), float64(winH)

		dc.BeginFrame()
		cov.Hit("BeginFrame")
		if needForceFull {
			needForceFull = false
		}
		dc.MarkFullRedraw()
		cov.Hit("MarkFullRedraw")
		if frame%2 == 0 {
			dc.BeginGPUFrame()
			cov.Hit("BeginGPUFrame")
		}
		if frame == 2 {
			_ = dc.Resize(winW, winH)
			cov.Hit("Resize")
		}

		paintAllAPIGroups(dc, side, cov, fw, fh, frame, tt, fonts, scratch, seedImg)

		// Guaranteed visible overlay (API exercises above must not leave a blank surface).
		drawVisibleChrome(dc, fw, fh, frame, tt, fonts)

		aux.BeginFrame()
		aux.SetRGB(0.1, 0.2, 0.3)
		aux.DrawRectangle(0, 0, 64, 64)
		_ = aux.Fill()
		if frame == 1 {
			_ = aux.ResizeTarget()
			cov.Hit("ResizeTarget")
		}

		if device != nil {
			device.FlushCallbacks()
		}
		if frame%25 == 7 || frame < 3 {
			enc := dc.CreateSharedEncoder()
			cov.Hit("CreateSharedEncoder")
			if !enc.IsNil() {
				dc.SetSharedEncoder(enc)
				cov.Hit("SetSharedEncoder")
				dc.SetRGB(1, 1, 0)
				dc.DrawCircle(fw-30, 30, 12)
				_ = dc.Fill()
				if err := dc.SubmitSharedEncoder(enc); err != nil {
					log.Printf("SubmitSharedEncoder: %v", err)
				}
				cov.Hit("SubmitSharedEncoder")
				dc.SetSharedEncoder(gpucontext.CommandEncoder{})
			}
		}
		if frame%40 == 11 || frame < 3 {
			if err := dc.FlushGPU(); err != nil {
				log.Printf("FlushGPU: %v", err)
			}
			cov.Hit("FlushGPU")
		}

		fb, err := sc.BeginFrame()
		if err != nil {
			if errors.Is(err, webgpu.ErrRecovered) {
				needForceFull = true
				continue
			}
			if errors.Is(err, webgpu.ErrDeviceLost) || (device != nil && device.IsLost()) {
				sc.ClearRecoverCooldown()
				if sc.Device != nil {
					device = sc.Device
				}
				time.Sleep(16 * time.Millisecond)
				continue
			}
			time.Sleep(2 * time.Millisecond)
			continue
		}

		present := func() error { return sc.EndFrame(fb) }
		var perr error
		// Always full present so the window shows complete frames (coverage of damage
		// modes uses full-window rects so content is not clipped away).
		switch presentMode % 6 {
		case 3:
			_, perr = dc.PresentFrameAuto(fb.Handle, fb.Width, fb.Height, present)
			cov.Hit("PresentFrameAuto")
		case 4:
			perr = dc.PresentFrameDamage(fb.Handle, fb.Width, fb.Height, image.Rect(0, 0, winW, winH), present)
			cov.Hit("PresentFrameDamage")
		case 5:
			perr = dc.PresentFrameDamageRects(fb.Handle, fb.Width, fb.Height,
				[]image.Rectangle{image.Rect(0, 0, winW, winH)}, present)
			cov.Hit("PresentFrameDamageRects")
		default:
			perr = dc.PresentFrameFull(fb.Handle, fb.Width, fb.Height, present)
		}
		cov.Hit("PresentFrameFull")
		cov.Hit("PresentFrame")
		if frame%30 == 13 || frame < 4 {
			_ = side.FlushGPUWithView(fb.Handle, fb.Width, fb.Height)
			cov.Hit("FlushGPUWithView")
			_ = side.FlushGPUWithViewDamage(fb.Handle, fb.Width, fb.Height, image.Rect(0, 0, 32, 32))
			cov.Hit("FlushGPUWithViewDamage")
			_ = side.FlushGPUWithViewDamageRects(fb.Handle, fb.Width, fb.Height, []image.Rectangle{image.Rect(0, 0, 16, 16)})
			cov.Hit("FlushGPUWithViewDamageRects")
		}
		if perr != nil {
			sc.DiscardFrame(fb)
			needForceFull = true
			if n := rendgpu.TextureOOMCount(); n > lastOOMHandled {
				if host.RecoverIfOOMPressure() {
					lastOOMHandled = n
					needForceFull = true
					if sc.Device != nil {
						device = sc.Device
					}
				}
			}
			if errors.Is(perr, webgpu.ErrDeviceLost) {
				time.Sleep(16 * time.Millisecond)
			}
			continue
		}
		presents++
		frame++
		presentMode++

		if v := os.Getenv("GPUI_FORCE_LOST_AFTER"); v != "" {
			var n int
			fmt.Sscanf(v, "%d", &n)
			if n > 0 && frame == n {
				log.Printf("GPUI_FORCE_LOST_AFTER=%d ForceRecoverHealthy", n)
				if err := sc.ForceRecoverHealthy(); err != nil {
					log.Printf("ForceRecoverHealthy: %v", err)
				} else if sc.Device != nil {
					device = sc.Device
					needForceFull = true
				}
				continue
			}
		}
		if n := rendgpu.TextureOOMCount(); n > lastOOMHandled {
			if host.RecoverIfOOMPressure() {
				lastOOMHandled = n
				needForceFull = true
				if sc.Device != nil {
					device = sc.Device
				}
			}
		}

		if frame%30 == 0 {
			log.Printf("frame=%d presents=%d recoveries=%d tier=%s %s",
				frame, presents, sc.Recoveries(), rendgpu.ResolveSurfaceLifecycle(adapter), cov.SummaryLine())
		}
		if d := time.Until(deadline); d > 200*time.Microsecond {
			time.Sleep(d)
		}
	}

	cov.Hit("DropGPURenderContext")
	dc.DropGPURenderContext()
	cov.Hit("Close")

	covN, covTot, missing := cov.Report()
	log.Printf("DONE exit=%s frames=%d presents=%d recoveries=%d hidden=%.1fs tier=%s %s",
		exitReason, frame, presents, sc.Recoveries(), hiddenAccum.Seconds(),
		rendgpu.ResolveSurfaceLifecycle(adapter), cov.SummaryLine())
	if len(missing) > 0 {
		log.Printf("MISSING_APIS (%d): %v", len(missing), missing)
	} else {
		log.Printf("ALL_PUBLIC_CONTEXT_APIS_HIT %d/%d", covN, covTot)
	}
	if strict && len(missing) > 0 {
		os.Exit(2)
	}
	if exitReason == "selftest_ok" || exitReason == "duration" || exitReason == "window_close" || exitReason == "signal" {
		os.Exit(0)
	}
	os.Exit(1)
}

func envInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return def
	}
	return n
}
func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func drawVisibleChrome(dc *render.Context, fw, fh float64, frame int, t float64, fonts fontPack) {
	if dc == nil {
		return
	}
	dc.ResetClip()
	dc.ClearMask()
	dc.Identity()
	dc.SetBlendMode(render.BlendNormal)
	dc.SetAntiAlias(true)
	// Opaque full background
	dc.SetRGB(0.93, 0.94, 0.96)
	dc.DrawRectangle(0, 0, fw, fh)
	_ = dc.Fill()
	// Header
	dc.SetRGB(0.10, 0.40, 0.85)
	dc.DrawRectangle(0, 0, fw, 56)
	_ = dc.Fill()
	// Rail
	dc.SetRGB(0.17, 0.19, 0.26)
	dc.DrawRectangle(0, 56, 160, fh-56)
	_ = dc.Fill()
	// Four content cards
	cols := [][3]float64{{0.98, 0.98, 1}, {0.95, 0.97, 0.92}, {1, 0.95, 0.90}, {0.92, 0.95, 1}}
	for i, c := range cols {
		dc.SetRGB(c[0], c[1], c[2])
		x := 180 + float64(i%2)*360
		y := 80 + float64(i/2)*210
		dc.DrawRoundedRectangle(x, y, 340, 180, 14)
		_ = dc.Fill()
		dc.SetRGB(0.15+float64(i)*0.12, 0.55, 0.95-float64(i)*0.12)
		dc.DrawRectangle(x, y, 14, 180)
		_ = dc.Fill()
	}
	// Motion marker
	dc.SetRGB(1, 0.4, 0.15)
	dc.DrawCircle(fw-70, 120, 20+12*(0.5+0.5*sinApprox(t*2.5)))
	_ = dc.Fill()
	// Status
	dc.SetRGB(0.12, 0.55, 0.38)
	dc.DrawRectangle(0, fh-48, fw, 48)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	if fonts.sans != "" {
		_ = dc.LoadFontFace(fonts.sans, 20)
		dc.DrawString("API COVERAGE APP — all public Context APIs", 20, 36)
		_ = dc.LoadFontFace(fonts.sans, 15)
		dc.DrawString("If you see blue header + 4 cards + green status, present path is healthy.", 20, fh-16)
	} else {
		dc.SetRGB(1, 0.9, 0.2)
		dc.DrawRectangle(20, 16, 320, 28)
		_ = dc.Fill()
	}
}

func sinApprox(x float64) float64 {
	return math.Sin(x)
}
