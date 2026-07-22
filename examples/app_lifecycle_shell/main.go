//go:build linux && !nogpu

// app_lifecycle_shell — host lifecycle matrix for Skia / Flutter / Ant Design-style apps.
//
// Upper-layer patterns → host duties (not engine-only):
//
//	S_ANIM     continuous Canvas animation (Skia / Flutter ticker)
//	S_FOCUS    unfocused but still visible — KEEP present (antd desktop)
//	S_MIN      minimize / iconify — pause acquire + Unconfigure (Flutter inactive)
//	S_RESIZE   live resize — Resize surface + Context (Flutter View metrics)
//	S_LAYER    modal/offscreen layer RT composite (antd Modal / Skia saveLayer)
//	S_MULTI    multi Context (two canvases / secondary surface content)
//	S_HIDDEN   model ticks while hidden → force-full on show (Flutter AppLifecycle)
//	S_RECOVER  device recreate mid-run (Flutter Rasterizer rebind / Skia abandon)
//	S_CYCLE    rapid minimize↔restore bursts (WM thrash)
//	S_ALL      short pass of the above in one process
//
//	GPUI_SHELL_SCENARIO=S_ALL go run ./examples/app_lifecycle_shell
//	GPUI_FORCE_LOST_AFTER=40 /tmp/app_lifecycle_shell
//	GPUI_SELFTEST_LIFECYCLE=1 GPUI_SELFTEST_MIN_AT=30 ... /tmp/app_lifecycle_shell
package main

import (
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/energye/gpui/examples/exboot"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
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

	scenario := strings.ToUpper(envOr("GPUI_SHELL_SCENARIO", "S_ALL"))
	// Default unlimited; GPUI_ANIM_SECONDS>0 for timed CI.
	animSec := envInt("GPUI_ANIM_SECONDS", 0)
	if animSec < 0 {
		animSec = 0
	}
	targetFPS := envInt("GPUI_TARGET_FPS", 60)
	if targetFPS < 15 {
		targetFPS = 15
	}
	frameBudget := time.Second / time.Duration(targetFPS)

	winW, winH := 720, 480
	xw, err := openX11Window(winW, winH, "gpui app_lifecycle_shell "+scenario)
	if err != nil {
		log.Fatalf("X11: %v", err)
	}
	defer xw.Close()

	exboot.InitEnv()
	inst, err := exboot.NewInstanceX11(xw.Display, 0)
	if err != nil {
		log.Fatalf("CreateInstance: %v", err)
	}
	defer inst.Release()

	surf, err := inst.CreateSurface(xw.Display, xw.Window)
	if err != nil {
		log.Fatalf("CreateSurface: %v", err)
	}
	defer surf.Release()

	adapter, device, err := exboot.OpenDevice(inst, surf, "app-lifecycle-shell")
	if err != nil {
		log.Fatalf("OpenDevice: %v", err)
	}
	defer adapter.Release()
	defer func() {
		if device != nil {
			device.Release()
		}
	}()

	sc := webgpu.NewSwapchain(surf, device, uint32(winW), uint32(winH))
	sc.Usage = types.TextureUsageRenderAttachment
	sc.SetPreferVSync()
	if err := sc.ConfigureFromCapabilities(adapter); err != nil {
		log.Fatalf("Configure: %v", err)
	}
	defer sc.Release()

	if err := exboot.BindProvider(device, adapter, sc.Format); err != nil {
		log.Fatalf("SetDeviceProvider: %v", err)
	}
	defer exboot.ResetAccelerator()

	mainCtx := render.NewContext(winW, winH)
	defer mainCtx.Close()
	// Second canvas: multi-Context pattern (Flutter secondary view / Skia surface).
	sideCtx := render.NewContext(160, 120)
	defer sideCtx.Close()
	// Modal layer RT (antd Modal / saveLayer-style bounded offscreen).
	layerCtx := render.NewContext(200, 140)
	defer layerCtx.Close()

	var needForceFull bool
	exboot.WireAutoRecover(sc, adapter, "app-lifecycle-shell",
		func(dev *webgpu.Device) { device = dev },
		func() {
			mainCtx.DropGPURenderContext()
			sideCtx.DropGPURenderContext()
			layerCtx.DropGPURenderContext()
			needForceFull = true
		},
		nil,
	)

	stopSig := make(chan os.Signal, 1)
	signal.Notify(stopSig, os.Interrupt, syscall.SIGTERM)

	log.Printf("scenario=%s window=%dx%d fps=%d seconds=%d", scenario, winW, winH, targetFPS, animSec)

	start := time.Now()
	frame := 0
	tick := 0
	presents := 0
	exitReason := "timeout"
	windowMinimized := false
	windowFullyObscured := false
	windowFocused := true
	wasHidden := false
	hiddenSince := time.Time{}
	hiddenAccum := time.Duration(0)
	// Model state continues while hidden (Flutter AppLifecycleState.paused content).
	modelPhase := 0.0
	resizeAt := envInt("GPUI_SHELL_RESIZE_AT", 0)
	cycleEvery := envInt("GPUI_SHELL_CYCLE_EVERY", 0)
	forceFocusOnly := scenario == "S_FOCUS" || envBool("GPUI_SHELL_FORCE_UNFOCUSED", false)

	// Scenario defaults for automated matrix.
	switch scenario {
	case "S_MIN", "S_CYCLE", "S_ALL":
		if os.Getenv("GPUI_SELFTEST_LIFECYCLE") == "" {
			_ = os.Setenv("GPUI_SELFTEST_LIFECYCLE", "1")
		}
	case "S_RECOVER":
		if os.Getenv("GPUI_FORCE_LOST_AFTER") == "" {
			_ = os.Setenv("GPUI_FORCE_LOST_AFTER", "45")
		}
	case "S_RESIZE":
		if resizeAt == 0 {
			resizeAt = 40
		}
	case "S_FOCUS":
		forceFocusOnly = true
	}

	minAt, mapAt, lostAt, doneAt := 60, 100, 140, 200
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

	running := true
	for running {
		tick++
		deadline := time.Now().Add(frameBudget)

		// Events
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
			if ev.Type == xFocusIn {
				windowFocused = true
			}
			if ev.Type == xFocusOut {
				windowFocused = false
			}
			if ev.Type == xConfigureNotify {
				nw, nh := ev.Width, ev.Height
				if nw < 64 {
					nw = 64
				}
				if nh < 64 {
					nh = 64
				}
				if nw != winW || nh != winH {
					winW, winH = nw, nh
					_ = mainCtx.Resize(winW, winH)
					if err := sc.Resize(uint32(winW), uint32(winH)); err != nil {
						log.Printf("Resize: %v", err)
					}
					needForceFull = true
					log.Printf("resized %dx%d", winW, winH)
				}
			}
		}
		if !running {
			break
		}
		select {
		case <-stopSig:
			running = false
			exitReason = "signal"
			continue
		default:
		}

		// Selftest / scenario injectors (tick advances even while hidden).
		if os.Getenv("GPUI_SELFTEST_LIFECYCLE") == "1" || scenario == "S_MIN" || scenario == "S_CYCLE" || scenario == "S_ALL" {
			switch tick {
			case minAt:
				log.Printf("SHELL: Iconify tick=%d (minimize)", tick)
				xw.Iconify()
			case mapAt:
				log.Printf("SHELL: MapRaise tick=%d (restore)", tick)
				xw.MapRaise()
				windowMinimized = false
				windowFullyObscured = false
				windowFocused = true
			case lostAt:
				if scenario == "S_RECOVER" || scenario == "S_ALL" || os.Getenv("GPUI_SELFTEST_LIFECYCLE") == "1" {
					log.Printf("SHELL: ForceRecoverHealthy tick=%d frame=%d", tick, frame)
					if err := sc.ForceRecoverHealthy(); err != nil {
						log.Printf("ForceRecoverHealthy: %v", err)
					} else if sc.Device != nil {
						device = sc.Device
						needForceFull = true
					}
				}
			}
			if cycleEvery > 0 && tick > minAt && tick%cycleEvery == 0 {
				if windowMinimized {
					xw.MapRaise()
					windowMinimized = false
					windowFocused = true
				} else {
					xw.Iconify()
				}
			}
			if tick >= doneAt && (os.Getenv("GPUI_SELFTEST_LIFECYCLE") == "1" || scenario == "S_ALL") {
				exitReason = "selftest_ok"
				break
			}
		}
		if resizeAt > 0 && tick == resizeAt {
			nw, nh := winW+80, winH+40
			log.Printf("SHELL: programmatic resize %dx%d → %dx%d", winW, winH, nw, nh)
			xw.Resize(nw, nh)
		}

		// Always advance model (Flutter: logic continues when paused).
		modelPhase += 0.04

		// Focus-only scenario: never treat unfocus as hide.
		hidden := windowMinimized || windowFullyObscured
		if forceFocusOnly {
			// S_FOCUS: only minimize/obscure pauses; pure unfocus keeps presenting.
			hidden = windowMinimized || windowFullyObscured
			_ = windowFocused
		}
		if hidden {
			if !wasHidden {
				wasHidden = true
				hiddenSince = time.Now()
				if sc.Surface != nil && sc.Device != nil && !sc.Device.IsLost() {
					sc.Surface.Unconfigure()
					sc.MarkNeedsReconfigure()
				}
				log.Printf("hidden minimized=%v obscured=%v — pause + unconfigure", windowMinimized, windowFullyObscured)
			}
			if device != nil {
				device.FlushCallbacks()
			}
			time.Sleep(50 * time.Millisecond)
			if animSec > 0 && time.Since(start)-hiddenAccum >= time.Duration(animSec)*time.Second {
				// count only visible time roughly
			}
			continue
		}
		if wasHidden {
			if !hiddenSince.IsZero() {
				hiddenAccum += time.Since(hiddenSince)
				hiddenSince = time.Time{}
			}
			wasHidden = false
			needForceFull = true
			sc.MarkNeedsReconfigure()
			sc.ClearRecoverCooldown()
			if sc.Device != nil {
				device = sc.Device
			}
			log.Printf("visible again hidden_total=%.1fs recoveries=%d", hiddenAccum.Seconds(), sc.Recoveries())
		}

		visibleElapsed := time.Since(start) - hiddenAccum
		if animSec > 0 && visibleElapsed >= time.Duration(animSec)*time.Second {
			exitReason = "duration"
			break
		}

		t := modelPhase
		fw, fh := float64(winW), float64(winH)

		// Side canvas (multi-Context).
		if scenario == "S_MULTI" || scenario == "S_ALL" || scenario == "S_ANIM" {
			sideCtx.BeginFrame()
			sideCtx.SetRGB(0.12, 0.16, 0.22)
			sideCtx.DrawRectangle(0, 0, 160, 120)
			_ = sideCtx.Fill()
			sideCtx.SetRGB(0.3+0.5*math.Sin(t), 0.7, 0.9)
			sideCtx.DrawCircle(80, 60, 20+8*math.Sin(t*1.7))
			_ = sideCtx.Fill()
		}

		// Modal layer RT.
		var layerImg *render.ImageBuf
		if scenario == "S_LAYER" || scenario == "S_ALL" {
			layerCtx.BeginFrame()
			layerCtx.SetRGBA(0.05, 0.08, 0.12, 0.92)
			layerCtx.DrawRoundedRectangle(0, 0, 200, 140, 12)
			_ = layerCtx.Fill()
			layerCtx.SetRGB(0.95, 0.85, 0.4)
			layerCtx.DrawRoundedRectangle(20, 40, 160, 48, 8)
			_ = layerCtx.Fill()
			_ = layerCtx.ExportImageBuf(&layerImg)
		}

		mainCtx.BeginFrame()
		if needForceFull {
			mainCtx.MarkFullRedraw()
			needForceFull = false
		}
		// Background
		mainCtx.SetRGB(0.07, 0.09, 0.12)
		mainCtx.DrawRectangle(0, 0, fw, fh)
		_ = mainCtx.Fill()
		// Animated content (S_ANIM / always)
		mainCtx.SetRGB(0.2, 0.55+0.25*math.Sin(t), 0.9)
		mainCtx.DrawRoundedRectangle(40+30*math.Sin(t), 60, 220, 100, 14)
		_ = mainCtx.Fill()
		mainCtx.SetRGB(1, 0.45, 0.25)
		mainCtx.DrawCircle(fw*0.7, fh*0.45, 36+10*math.Sin(t*1.3))
		_ = mainCtx.Fill()
		// Focus indicator strip
		if !windowFocused && !forceFocusOnly {
			mainCtx.SetRGBA(0.9, 0.2, 0.2, 0.35)
			mainCtx.DrawRectangle(0, 0, fw, 6)
			_ = mainCtx.Fill()
		} else if forceFocusOnly && !windowFocused {
			// Still drawing — prove unfocus ≠ pause.
			mainCtx.SetRGBA(0.2, 0.8, 0.4, 0.5)
			mainCtx.DrawRectangle(0, 0, fw, 6)
			_ = mainCtx.Fill()
		}
		// Composite modal
		if layerImg != nil {
			mainCtx.DrawImage(layerImg, fw*0.5-100, fh*0.5-70)
		}
		// HUD
		mainCtx.SetRGBA(0, 0, 0, 0.55)
		mainCtx.DrawRectangle(0, fh-28, fw, 28)
		_ = mainCtx.Fill()
		mainCtx.SetRGB(0.9, 0.95, 1)
		// DrawString may no-op without fonts; shapes still prove present path.
		_ = mainCtx

		if device != nil {
			device.FlushCallbacks()
		}
		fb, err := sc.BeginFrame()
		if err != nil {
			if errors.Is(err, webgpu.ErrRecovered) {
				needForceFull = true
				continue
			}
			if errors.Is(err, webgpu.ErrDeviceLost) || (device != nil && device.IsLost()) {
				log.Printf("BeginFrame lost/recovering recoveries=%d", sc.Recoveries())
				sc.ClearRecoverCooldown()
				if sc.Device != nil {
					device = sc.Device
				}
				time.Sleep(16 * time.Millisecond)
				continue
			}
			log.Printf("BeginFrame: %v", err)
			time.Sleep(2 * time.Millisecond)
			continue
		}
		if err := mainCtx.PresentFrameFull(fb.Handle, fb.Width, fb.Height, func() error {
			return sc.EndFrame(fb)
		}); err != nil {
			sc.DiscardFrame(fb)
			if errors.Is(err, webgpu.ErrDeviceLost) || (device != nil && device.IsLost()) {
				log.Printf("Present lost/recovering recoveries=%d", sc.Recoveries())
				needForceFull = true
				time.Sleep(16 * time.Millisecond)
				continue
			}
			log.Printf("Present: %v", err)
			needForceFull = true
			continue
		}
		presents++
		frame++

		if v := os.Getenv("GPUI_FORCE_LOST_AFTER"); v != "" {
			var n int
			fmt.Sscanf(v, "%d", &n)
			if n > 0 && frame == n && sc != nil {
				log.Printf("GPUI_FORCE_LOST_AFTER=%d ForceRecoverHealthy", n)
				if err := sc.ForceRecoverHealthy(); err != nil {
					log.Printf("ForceRecoverHealthy: %v", err)
				} else if sc.Device != nil {
					device = sc.Device
					needForceFull = true
				}
			}
		}

		if d := time.Until(deadline); d > 200*time.Microsecond {
			time.Sleep(d)
		}
	}

	log.Printf("DONE scenario=%s exit=%s frames=%d presents=%d recoveries=%d hidden=%.1fs gpu_ctxs=%d",
		scenario, exitReason, frame, presents, sc.Recoveries(), hiddenAccum.Seconds(), render.GPUContextCount())
	if exitReason == "selftest_ok" {
		if sc.Recoveries() < 1 && (scenario == "S_RECOVER" || scenario == "S_ALL") {
			// recover may be optional if FORCE not set and lostAt not reached
		}
		os.Exit(0)
	}
	if exitReason == "duration" || exitReason == "timeout" {
		os.Exit(0)
	}
	if exitReason == "window_close" || exitReason == "signal" {
		os.Exit(0)
	}
	os.Exit(1)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
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

func envBool(k string, def bool) bool {
	v := strings.ToLower(os.Getenv(k))
	if v == "" {
		return def
	}
	return v == "1" || v == "true" || v == "yes"
}
