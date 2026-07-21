//go:build linux && !nogpu

// Capability matrix window: real X11 + webgpu present path for SKIA_2D_CAPABILITY_MATRIX IDs.
//
// One process = one scenario (GPUI_SCENARIO=C0x|C2x). Metrics gate GPU-first correctness.
// See docs/CAPABILITY_MATRIX_WINDOW.md.
package main

import (
	"errors"
	"fmt"
	"image"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/energye/gpui/examples/exboot"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
)

const (
	defaultW         = 800
	defaultH         = 600
	defaultTargetFPS = 60
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

	scenarioID := os.Getenv("GPUI_SCENARIO")
	spec, ok := applyScenario(scenarioID)
	if !ok {
		// Default to C01 for interactive smoke.
		spec, _ = applyScenario("C01")
		log.Printf("GPUI_SCENARIO unset/unknown → default C01")
	}

	targetFPS := envInt("GPUI_TARGET_FPS", defaultTargetFPS)
	if targetFPS < 15 {
		targetFPS = 15
	}
	if targetFPS > 120 {
		targetFPS = 120
	}
	frameBudget := time.Second / time.Duration(targetFPS)
	animSeconds := envInt("GPUI_ANIM_SECONDS", 0)
	resultPath := os.Getenv("GPUI_RESULT_FILE")
	logEvery := envInt("GPUI_ANIM_LOG_EVERY", 60)

	winW, winH := defaultW, defaultH
	title := fmt.Sprintf("gpui capability %s %s", spec.ID, spec.NameCN)
	xw, err := openX11Window(winW, winH, title)
	if err != nil {
		log.Fatalf("X11: %v", err)
	}
	defer xw.Close()

	// Drain initial configure
	for i := 0; i < 32; i++ {
		got := false
		for xw.Pending() {
			ev := xw.NextEvent()
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
					got = true
				}
			}
		}
		if got {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	xw.LockSize(winW, winH)

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

	adapter, device, err := exboot.OpenDevice(inst, surf, "capability-matrix")
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

	// Library auto-recover: DeviceLostCallback → recreate device + reconfigure surface.
	dc := render.NewContext(winW, winH)
	defer dc.Close()
	exboot.WireAutoRecover(sc, adapter, "capability-matrix",
		func(dev *webgpu.Device) { device = dev },
		func() { dc.DropGPURenderContext() },
		nil,
	)
	fonts := loadFonts(dc)
	pixelScratch := make([]byte, 64*48*4)

	stopSig := make(chan os.Signal, 1)
	signal.Notify(stopSig, os.Interrupt, syscall.SIGTERM)

	log.Printf("scenario=%s (%s) matrix=[%s] window=%dx%d target=%dfps seconds=%d",
		spec.ID, spec.NameCN, spec.MatrixIDs, winW, winH, targetFPS, animSeconds)
	log.Printf("expect: %s", spec.Expect)

	start := time.Now()
	frame := 0
	rssStart := rssKB()
	exitReason := "window_close"
	var (
		fpsEMA       float64
		lastFrameEnd time.Time
		cpuPctEMA    float64
		prevCPU      cpuSample
		havePrevCPU  bool
		cpuSum       float64
		cpuSamples   int
		rssSamples   []int64
		lastRSS      int64
		gpuOpsLast   int
		cpuFBLast    int
		lastFB       string
		presents     int
		probeOK      bool
		probeNote    string
		probeChecked bool
		// Steady-state FPS avg excludes warm-up (shader/pipeline/mask first hits).
		steadyStart  time.Time
		steadyFrame0 int
		haveSteady   bool
	)

	running := true
	windowMinimized := false
	windowFullyObscured := false
	wasHidden := false
	for running {
		deadline := time.Now().Add(frameBudget)
		// Events
		for xw.Pending() {
			ev := xw.NextEvent()
			if xw.IsDelete(ev) {
				running = false
				exitReason = "window_close"
				break
			}
			if ev.Type == xConfigureNotify {
				// fixed size soak — ignore maximize
			}
			if ev.Type == xVisibilityNotify {
				windowFullyObscured = ev.Visibility == xVisibilityFullyObscured
			}
			// GNOME minimize often only updates WM_STATE.
			windowMinimized = xw.IsIconic()
		}
		// Pause present when unpresentable (docs/GPU_修复_device_lost.md).
		hidden := windowMinimized || windowFullyObscured
		if hidden {
			if !wasHidden {
				wasHidden = true
				if sc.Surface != nil && sc.Device != nil && !sc.Device.IsLost() {
					sc.Surface.Unconfigure()
					sc.MarkNeedsReconfigure()
				}
				log.Printf("window hidden (minimized=%v obscured=%v) — pause present + unconfigure",
					windowMinimized, windowFullyObscured)
			}
			if device != nil {
				device.FlushCallbacks()
			}
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if wasHidden {
			wasHidden = false
			sc.MarkNeedsReconfigure()
			sc.ClearRecoverCooldown()
			if sc.Device != nil {
				device = sc.Device
			}
			log.Printf("window visible again — resume present recoveries=%d", sc.Recoveries())
		}
		select {
		case <-stopSig:
			running = false
			exitReason = "signal"
		default:
		}
		if !running {
			break
		}
		if animSeconds > 0 && time.Since(start) >= time.Duration(animSeconds)*time.Second {
			running = false
			exitReason = "timeout"
			break
		}

		t := time.Since(start).Seconds()
		if envBool("GPUI_DETERMINISTIC", false) {
			// Fixed 60Hz timeline for golden/regression pixel captures.
			t = float64(frame) / 60.0
		}
		if ft := os.Getenv("GPUI_FIXED_T"); ft != "" {
			if v, err := strconv.ParseFloat(ft, 64); err == nil {
				t = v
			}
		}
		fw, fh := float64(winW), float64(winH)

		dc.BeginFrame()
		note := drawCapability(dc, fonts, spec.DrawKind, fw, fh, t, frame, pixelScratch)
		skipHUD := envBool("GPUI_GOLDEN_NO_HUD", false)
		if !skipHUD {
			drawOverlayHUD(dc, fonts, fw, fh, spec, note, fpsEMA, cpuPctEMA, lastRSS, frame)
		}
		// Optional golden/capture dump (content surface after draw).
		if capDir := os.Getenv("GPUI_CAPTURE_DIR"); capDir != "" {
			capFrame := envInt("GPUI_CAPTURE_FRAME", 90)
			if frame == capFrame {
				_ = os.MkdirAll(capDir, 0o755)
				out := filepath.Join(capDir, spec.ID+".png")
				if err := dc.SavePNG(out); err != nil {
					log.Printf("capture %s: %v", out, err)
				} else {
					log.Printf("captured %s frame=%d t=%.3f", out, frame, t)
				}
			}
		}

		// Present via swapchain BeginFrame/EndFrame.
		// Device lost is recovered inside sc.BeginFrame when EnableAutoRecover is armed.
		if device != nil {
			device.FlushCallbacks()
		}
		fb, err := sc.BeginFrame()
		if err != nil {
			log.Printf("BeginFrame: %v — skip frame", err)
			if errors.Is(err, webgpu.ErrDeviceLost) || (device != nil && device.IsLost()) {
				log.Printf("BeginFrame: device lost/recovering (recoveries=%d) — skip", sc.Recoveries())
				time.Sleep(16 * time.Millisecond)
				continue
			}
			time.Sleep(2 * time.Millisecond)
			continue
		}
		present := func() error { return sc.EndFrame(fb) }
		if spec.DamageMode {
			dc.SetDamageTracking(true)
			dx, dy, dw, dh := damageRect(fw, fh)
			rect := image.Rect(dx-4, dy-4, dx+dw+4, dy+dh+4)
			if err := dc.PresentFrameDamage(fb.Handle, fb.Width, fb.Height, rect, present); err != nil {
				if err2 := dc.PresentFrameFull(fb.Handle, fb.Width, fb.Height, present); err2 != nil {
					sc.DiscardFrame(fb)
					if errors.Is(err2, webgpu.ErrDeviceLost) || errors.Is(err, webgpu.ErrDeviceLost) || (device != nil && device.IsLost()) {
						log.Printf("Present: device lost/recovering (recoveries=%d) — skip", sc.Recoveries())
						time.Sleep(16 * time.Millisecond)
						continue
					}
					log.Printf("Present: %v / fallback %v — skip frame", err, err2)
					time.Sleep(2 * time.Millisecond)
					continue
				}
			}
		} else {
			if err := dc.PresentFrameFull(fb.Handle, fb.Width, fb.Height, present); err != nil {
				sc.DiscardFrame(fb)
				if errors.Is(err, webgpu.ErrDeviceLost) || (device != nil && device.IsLost()) {
					log.Printf("PresentFrameFull: device lost/recovering (recoveries=%d) — skip", sc.Recoveries())
					time.Sleep(16 * time.Millisecond)
					continue
				}
				log.Printf("PresentFrameFull: %v — skip frame", err)
				time.Sleep(2 * time.Millisecond)
				continue
			}
		}
		presents++

		st := dc.RenderPathStats()
		gpuOpsLast = st.GPUOps
		cpuFBLast = st.CPUFallbackOps
		lastFB = st.LastCPUFallbackReason
		// Probe after a few warm-up frames
		if frame >= 10 && !probeChecked {
			probeOK, probeNote = probeCapability(dc, spec.DrawKind)
			probeChecked = true
		} else if frame >= 10 {
			// continuous recheck: any cpu_fb fails
			ok, note2 := probeCapability(dc, spec.DrawKind)
			if !ok {
				probeOK, probeNote = ok, note2
			} else if probeOK {
				probeNote = note2
			}
		}

		// Metrics
		now := time.Now()
		if !lastFrameEnd.IsZero() {
			dt := now.Sub(lastFrameEnd).Seconds()
			if dt > 1e-4 {
				instFPS := 1.0 / dt
				if fpsEMA <= 0 {
					fpsEMA = instFPS
				} else {
					fpsEMA = fpsEMA*0.9 + instFPS*0.1
				}
			}
		}
		lastFrameEnd = now
		if cur, ok := readCPUSample(); ok {
			if havePrevCPU {
				if pct, ok2 := cpuPercent(prevCPU, cur); ok2 {
					if cpuPctEMA <= 0 {
						cpuPctEMA = pct
					} else {
						cpuPctEMA = cpuPctEMA*0.85 + pct*0.15
					}
					cpuSum += pct
					cpuSamples++
				}
			}
			prevCPU = cur
			havePrevCPU = true
		}
		if frame%30 == 0 {
			lastRSS = rssKB()
			rssSamples = append(rssSamples, lastRSS)
		}
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
				}
			}
		}
		// Begin steady window after ~1s of frames (or frame 45 min) so avg FPS
		// is not poisoned by first-frame pipeline/mask compile cost.
		if !haveSteady && (time.Since(start) >= time.Second || frame >= 45) {
			steadyStart = time.Now()
			steadyFrame0 = frame
			haveSteady = true
		}

		if logEvery > 0 && frame%logEvery == 0 {
			log.Printf("%s frame=%d fps=%.1f cpu=%.1f%% rss=%dKB gpu_ops=%d cpu_fb=%d probe=%v",
				spec.ID, frame, fpsEMA, cpuPctEMA, lastRSS, gpuOpsLast, cpuFBLast, probeOK)
		}

		// Pace
		if d := time.Until(deadline); d > 200*time.Microsecond {
			pad := 900 * time.Microsecond
			if d > pad {
				time.Sleep(d - pad)
			}
		}
	}

	elapsed := time.Since(start).Seconds()
	fpsAvg := 0.0
	if haveSteady {
		se := time.Since(steadyStart).Seconds()
		sf := frame - steadyFrame0
		if se > 0 && sf > 0 {
			fpsAvg = float64(sf) / se
		}
	}
	if fpsAvg <= 0 && elapsed > 0 {
		fpsAvg = float64(frame) / elapsed
	}
	cpuAvg := 0.0
	if cpuSamples > 0 {
		cpuAvg = cpuSum / float64(cpuSamples)
	}
	rssEnd := rssKB()
	res := runResult{
		Scenario:         spec.ID,
		Name:             spec.NameCN,
		MatrixIDs:        spec.MatrixIDs,
		Seconds:          elapsed,
		Frames:           frame,
		FPSEma:           fpsEMA,
		FPSAvg:           fpsAvg,
		CPUAvg:           cpuAvg,
		RSSStartKB:       rssStart,
		RSSEndKB:         rssEnd,
		RSSSteadyDeltaKB: rssSteadyDelta(rssSamples),
		GPUOps:           gpuOpsLast,
		CPUFallback:      cpuFBLast,
		LastFB:           lastFB,
		Presents:         presents,
		ProbeOK:          probeOK,
		ProbeNote:        probeNote,
		AllowLowFPS:      spec.AllowLowFPS,
		ExitReason:       exitReason,
	}
	status, reason := judgeResult(res, targetFPS)
	res.Status = status
	res.FailReason = reason
	writeResult(resultPath, res)
	log.Printf("DONE %s status=%s fps_ema=%.1f fps_avg=%.1f cpu=%.1f cpu_fb=%d gpu_ops=%d probe=%v reason=%s exit=%s",
		spec.ID, status, fpsEMA, fpsAvg, cpuAvg, cpuFBLast, gpuOpsLast, probeOK, reason, exitReason)
	if status != "PASS" {
		os.Exit(1)
	}
}

func drawOverlayHUD(dc *render.Context, fonts fontPack, fw, fh float64, spec scenarioSpec, note string, fps, cpu float64, rss int64, frame int) {
	// Top bar
	dc.SetRGBA(0.05, 0.06, 0.09, 0.78)
	dc.DrawRectangle(0, 0, fw, 52)
	_ = dc.Fill()
	ensureFontPack(dc, fonts, 14)
	dc.SetRGBA(0.95, 0.97, 1, 1)
	dc.DrawString(fmt.Sprintf("%s  %s  matrix[%s]", spec.ID, spec.NameCN, spec.MatrixIDs), 10, 18)
	dc.SetRGBA(0.55, 0.9, 0.7, 1)
	dc.DrawString(fmt.Sprintf("FPS %.1f  CPU %.0f%%  RSS %dKB  frame %d", fps, cpu, rss, frame), 10, 38)
	// Expect strip
	dc.SetRGBA(0.08, 0.1, 0.14, 0.72)
	dc.DrawRectangle(0, fh-40, fw, 40)
	_ = dc.Fill()
	ensureFontPack(dc, fonts, 13)
	dc.SetRGBA(0.9, 0.92, 0.98, 1)
	msg := note
	if msg == "" {
		msg = spec.Expect
	}
	dc.DrawString("应看到: "+msg, 10, fh-16)
}
