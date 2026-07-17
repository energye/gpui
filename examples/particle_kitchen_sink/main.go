//go:build linux && !nogpu

// Particle kitchen-sink stress: non-fullscreen stage, isolatable feature toggles.
// Purpose: surface CPU / submit / glow-export / blend / memory issues with gates.
//
//	cd examples/particle_kitchen_sink && go run .
//	GPUI_TIER=L2 GPUI_ANIM_SECONDS=8 GPUI_RESULT_FILE=/tmp/pks_L2.json go run .
//	GPUI_PROBE=P_BLEND_LAYER GPUI_ANIM_SECONDS=7 GPUI_RESULT_FILE=/tmp/p.json go run .
//	GPUI_LIST_PROBES=1 go run .   # print isolation matrix catalog
//
// See README.md for bisect switches and matrix runner.
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	rendgpu "github.com/energye/gpui/render/gpu"
)

const (
	defaultW         = 800
	defaultH         = 600
	defaultTargetFPS = 60
)

func main() {
	if envBool("GPUI_LIST_PROBES", false) {
		printProbeCatalog()
		return
	}
	if path := os.Getenv("GPUI_CPUPROFILE"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			log.Fatalf("cpuprofile: %v", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatalf("cpuprofile start: %v", err)
		}
		defer func() {
			pprof.StopCPUProfile()
			_ = f.Close()
		}()
	}
	if path := os.Getenv("GPUI_MEMPROFILE"); path != "" {
		defer func() {
			f, err := os.Create(path)
			if err != nil {
				log.Printf("memprofile create: %v", err)
				return
			}
			runtime.GC()
			if err := pprof.WriteHeapProfile(f); err != nil {
				log.Printf("memprofile write: %v", err)
			}
			_ = f.Close()
		}()
	}

	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		for _, p := range []string{"../../lib/libwgpu_native.so", "lib/libwgpu_native.so"} {
			if _, err := os.Stat(p); err == nil {
				_ = os.Setenv("WGPU_NATIVE_PATH", p)
				break
			}
		}
	}
	if os.Getenv("GPUI_SURFACE_SAMPLE_COUNT") == "" {
		_ = os.Setenv("GPUI_SURFACE_SAMPLE_COUNT", "1")
	}

	cfg := loadConfig()
	targetFPS := envInt("GPUI_TARGET_FPS", defaultTargetFPS)
	if targetFPS < 15 {
		targetFPS = 15
	}
	if targetFPS > 120 {
		targetFPS = 120
	}
	frameBudget := time.Second / time.Duration(targetFPS)
	animSeconds := envInt("GPUI_ANIM_SECONDS", 0)
	if animSeconds <= 0 && cfg.MemSoakSec > 0 {
		animSeconds = cfg.MemSoakSec
	}
	resultPath := os.Getenv("GPUI_RESULT_FILE")
	logEvery := envInt("GPUI_ANIM_LOG_EVERY", 60)
	// Resize oscillate must unlock size.
	lockSize := envBool("GPUI_LOCK_SIZE", !cfg.ResizeOscillate)

	winW, winH := defaultW, defaultH
	title := fmt.Sprintf("gpui particle-sink %s %s", cfg.Tier, cfg.NameCN)
	xw, err := openX11Window(winW, winH, title)
	if err != nil {
		log.Fatalf("X11: %v", err)
	}
	defer xw.Close()

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
	if lockSize {
		xw.LockSize(winW, winH)
	}

	inst, err := webgpu.CreateInstance(&webgpu.InstanceDescriptor{Backends: webgpu.BackendsPrimary})
	if err != nil {
		log.Fatalf("CreateInstance: %v", err)
	}
	defer inst.Release()

	surf, err := inst.CreateSurface(xw.Display, xw.Window)
	if err != nil {
		log.Fatalf("CreateSurface: %v", err)
	}
	defer surf.Release()

	adapter, err := inst.RequestAdapter(&webgpu.RequestAdapterOptions{
		PowerPreference:   webgpu.PowerPreferenceHighPerformance,
		CompatibleSurface: surf,
	})
	if err != nil {
		log.Fatalf("RequestAdapter: %v", err)
	}
	defer adapter.Release()

	device, err := adapter.RequestDevice(rendgpu.DeviceDescriptor("particle-kitchen-sink"))
	if err != nil {
		log.Fatalf("RequestDevice: %v", err)
	}
	defer device.Release()

	sc := webgpu.NewSwapchain(surf, device, uint32(winW), uint32(winH))
	sc.Usage = types.TextureUsageRenderAttachment
	sc.SetPreferVSync()
	if err := sc.ConfigureFromCapabilities(adapter); err != nil {
		log.Fatalf("Configure: %v", err)
	}
	defer sc.Release()

	if err := rendgpu.SetDeviceProvider(&webgpu.SimpleDeviceProvider{
		Dev: device, Adpt: adapter, Format: sc.Format,
	}); err != nil {
		log.Fatalf("SetDeviceProvider: %v", err)
	}
	defer func() { _ = rendgpu.ResetAccelerator() }()

	dc := render.NewContext(winW, winH)
	defer dc.Close()
	fonts := loadFonts(dc)

	sx, sy, sw, sh := stageRect(float64(winW), float64(winH), cfg.Region)
	_ = sx
	_ = sy
	sim := newSim(cfg.ParticleN, sw, sh)

	stopSig := make(chan os.Signal, 1)
	signal.Notify(stopSig, os.Interrupt, syscall.SIGTERM)

	log.Printf("probe=%s tier=%s (%s) %s window=%dx%d target=%dfps seconds=%d",
		cfg.ProbeID, cfg.Tier, cfg.NameCN, cfg.String(), winW, winH, targetFPS, animSeconds)
	log.Printf("expect: %s", cfg.Expect)
	if cfg.BisectHint != "" {
		log.Printf("bisect_hint: %s", cfg.BisectHint)
	}
	log.Printf("bisect: GPUI_ENABLE_BLEND/GLOW/MESH/ATLAS/TEXT/LAYER/TRAILS=0|1  GPUI_PARTICLE_N=  GPUI_REGION=  GPUI_PROBE=")

	start := time.Now()
	frame := 0
	rssStart := rssKB()
	exitReason := "window_close"
	var (
		fpsEMA           float64
		fpsMin           float64
		fpsMax           float64
		lastFrameEnd     time.Time
		cpuPctEMA        float64
		prevCPU          cpuSample
		havePrevCPU      bool
		cpuSum           float64
		cpuSamples       int
		rssSamples       []int64
		lastRSS          int64
		gpuOpsLast       int
		cpuFBLast        int
		lastFB           string
		presents         int
		presentErrors    int
		presentErrResize int
		presentErrSteady int
		lastPresentErr   string
		resizeEvents     int
		recoverFails     int
		resizeGrace      int // frames remaining that count as resize-side errors
		postResizeOK     int // successful presents after last resize
		awaitRecover     bool
		probeOKFlag      bool
		probeNote        string
		probeChecked     bool
		contentOKFlag    bool
		contentNote      string
		contentChecked   bool
		pixelOKFlag      bool
		pixelNote        string
		pixelSamples     string
		pixelChecked     bool
		stageSigOK       bool
		stageSigNote     string
		stageSigChecked  bool
		sigSamples       int
		sigFails         int
		markersLast      int
		steadyStart      time.Time
		steadyFrame0     int
		haveSteady       bool
		resizePhase      int
		lowFPSCount      int
		instFPSCount     int
	)

	running := true
	for running {
		select {
		case <-stopSig:
			running = false
			exitReason = "signal"
			continue
		default:
		}
		if animSeconds > 0 && time.Since(start) >= time.Duration(animSeconds)*time.Second {
			running = false
			exitReason = "timeout"
			continue
		}

		deadline := time.Now().Add(frameBudget)
		for xw.Pending() {
			ev := xw.NextEvent()
			if xw.IsDelete(ev) {
				running = false
				exitReason = "window_close"
				break
			}
			if ev.Type == xConfigureNotify && !lockSize {
				nw, nh := ev.Width, ev.Height
				if nw < 64 {
					nw = 64
				}
				if nh < 64 {
					nh = 64
				}
				if nw != winW || nh != winH {
					winW, winH = nw, nh
					_ = dc.Close()
					dc = render.NewContext(winW, winH)
					fonts = loadFonts(dc)
					if err := sc.Resize(uint32(winW), uint32(winH)); err != nil {
						log.Printf("sc.Resize: %v", err)
						presentErrors++
						presentErrResize++
						lastPresentErr = err.Error()
					}
					sx, sy, sw, sh = stageRect(float64(winW), float64(winH), cfg.Region)
					_ = sx
					_ = sy
					sim.resize(cfg.ParticleN, sw, sh)
					resizeEvents++
					resizeGrace = 8
					postResizeOK = 0
					awaitRecover = true
				}
			}
		}
		if !running {
			break
		}

		// Real X11 resize oscillate (diagnostic): resize window; ConfigureNotify
		// path reconfigures swapchain/context. Avoid size-mismatched GetCurrentTexture.
		if cfg.ResizeOscillate && frame > 0 && frame%45 == 0 {
			resizePhase = (resizePhase + 1) % 4
			sizes := [][2]int{
				{800, 600},
				{900, 640},
				{760, 560},
				{860, 620},
			}
			nw, nh := sizes[resizePhase][0], sizes[resizePhase][1]
			if nw != winW || nh != winH {
				if awaitRecover && postResizeOK == 0 && resizeEvents > 0 {
					recoverFails++
				}
				xw.Resize(nw, nh)
				// Do not force sc.Resize here — wait for ConfigureNotify so
				// surface extent matches the real window.
				resizeEvents++
				resizeGrace = 12
				postResizeOK = 0
				awaitRecover = true
			}
		}

		// Grow particle density over time (mem / buffer pressure) — never shrink below MinN.
		if cfg.GrowN && frame > 0 && frame%90 == 0 {
			next := cfg.ParticleN + 400
			if next > 2500 {
				next = 2500
			}
			if next > cfg.ParticleN {
				cfg.ParticleN = next
				_, _, sw, sh = stageRect(float64(winW), float64(winH), cfg.Region)
				sim.resize(cfg.ParticleN, sw, sh)
				log.Printf("grow_n -> %d", cfg.ParticleN)
			}
		}

		fw, fh := float64(winW), float64(winH)
		t := time.Since(start).Seconds()
		_, _, sw, sh = stageRect(fw, fh, cfg.Region)
		sim.step(1.0/float64(targetFPS), sw, sh)

		dc.BeginFrame()
		note, markers := drawFrame(dc, fonts, cfg, sim, fw, fh, t, frame)
		markersLast = markers
		drawHUD(dc, fonts, cfg, fw, fh, note, fpsEMA, cpuPctEMA, lastRSS, frame)

		fb, err := sc.BeginFrame()
		if err != nil {
			log.Printf("BeginFrame: %v — skip", err)
			presentErrors++
			lastPresentErr = err.Error()
			if resizeGrace > 0 {
				presentErrResize++
				resizeGrace--
			} else {
				presentErrSteady++
			}
			time.Sleep(2 * time.Millisecond)
			continue
		}
		present := func() error { return sc.EndFrame(fb) }
		if err := dc.PresentFrameFull(fb.Handle, fb.Width, fb.Height, present); err != nil {
			sc.DiscardFrame(fb)
			presentErrors++
			lastPresentErr = err.Error()
			log.Printf("PresentFrameFull: %v", err)
			if resizeGrace > 0 {
				presentErrResize++
				resizeGrace--
			} else {
				presentErrSteady++
			}
			if presentErrSteady > 30 {
				log.Fatalf("too many steady present errors: %v", err)
			}
			continue
		}
		presents++
		if resizeGrace > 0 {
			resizeGrace--
		}
		if awaitRecover {
			postResizeOK++
			if postResizeOK >= 3 {
				awaitRecover = false
			}
		}

		st := dc.RenderPathStats()
		gpuOpsLast = st.GPUOps
		cpuFBLast = st.CPUFallbackOps
		lastFB = st.LastCPUFallbackReason
		if frame >= 10 {
			ok, pn := probeOK(dc)
			if !probeChecked {
				probeOKFlag, probeNote = ok, pn
				probeChecked = true
			} else if !ok {
				probeOKFlag, probeNote = ok, pn
			}
			cok, cn := contentProbe(dc, cfg, markers)
			if !contentChecked {
				contentOKFlag, contentNote = cok, cn
				contentChecked = true
			} else if !cok {
				contentOKFlag, contentNote = cok, cn
			}
		}
		// Pixel evidence once after warm-up (expensive-ish export; not every frame).
		if !pixelChecked && frame >= 45 {
			pixelOKFlag, pixelNote, pixelSamples = pixelFingerprint()
			pixelChecked = true
			if cfg.TextBi {
				okBi, noteBi := textBiFingerprint(fonts)
				if !okBi {
					pixelOKFlag = false
					pixelNote = "text_bi:" + noteBi
				} else {
					pixelNote = pixelNote + "; text_bi:" + noteBi
				}
			}
			stageSigOK, stageSigNote = stageContentSignature()
			stageSigChecked = true
			sigSamples++
			if !stageSigOK {
				sigFails++
			}
			log.Printf("pixel_probe ok=%v note=%s | stage_sig ok=%v note=%s",
				pixelOKFlag, pixelNote, stageSigOK, stageSigNote)
		} else if frame >= 45 && frame%30 == 0 {
			// Intermittent content sampling — catches flicker/dropouts without full-frame readback.
			okSig, noteSig := stageContentSignature()
			sigSamples++
			if !okSig {
				sigFails++
				stageSigOK = false
				stageSigNote = noteSig
			}
		}

		now := time.Now()
		if !lastFrameEnd.IsZero() {
			dt := now.Sub(lastFrameEnd).Seconds()
			if dt > 1e-6 {
				instFPS := 1.0 / dt
				if fpsEMA <= 0 {
					fpsEMA = instFPS
				} else {
					fpsEMA = fpsEMA*0.9 + instFPS*0.1
				}
				// Track span after warm-up.
				// Ignore first ~1s scheduling noise; track steady hitch ratio.
				if frame >= 60 {
					instFPSCount++
					if fpsMin <= 0 || instFPS < fpsMin {
						fpsMin = instFPS
					}
					if instFPS > fpsMax {
						fpsMax = instFPS
					}
					if instFPS < float64(targetFPS)-15 {
						lowFPSCount++
					}
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
		if !haveSteady && (time.Since(start) >= time.Second || frame >= 45) {
			haveSteady = true
			steadyStart = time.Now()
			steadyFrame0 = frame
		}
		if logEvery > 0 && frame%logEvery == 0 {
			log.Printf("%s frame=%d fps=%.1f cpu=%.1f%% rss=%dKB gpu_ops=%d cpu_fb=%d n=%d feats=%s probe=%v content=%v markers=%d",
				cfg.ProbeID, frame, fpsEMA, cpuPctEMA, lastRSS, gpuOpsLast, cpuFBLast, cfg.ParticleN, cfg.featuresSummary(), probeOKFlag, contentOKFlag, markersLast)
		}

		if sleep := time.Until(deadline); sleep > 0 {
			time.Sleep(sleep)
		}
	}

	elapsed := time.Since(start).Seconds()
	if elapsed < 1e-6 {
		elapsed = 1e-6
	}
	fpsAvg := float64(frame) / elapsed
	if haveSteady {
		se := time.Since(steadyStart).Seconds()
		if se > 1e-3 {
			sf := frame - steadyFrame0
			if sf > 0 {
				fpsAvg = float64(sf) / se
			}
		}
	}
	cpuAvg := 0.0
	if cpuSamples > 0 {
		cpuAvg = cpuSum / float64(cpuSamples)
	}
	if lastRSS == 0 {
		lastRSS = rssKB()
	}
	if !probeChecked {
		probeOKFlag, probeNote = probeOK(dc)
	}
	if !contentChecked {
		contentOKFlag, contentNote = contentProbe(dc, cfg, markersLast)
	}
	if !pixelChecked {
		pixelOKFlag, pixelNote, pixelSamples = pixelFingerprint()
		pixelChecked = true
	}
	if !stageSigChecked {
		stageSigOK, stageSigNote = stageContentSignature()
		stageSigChecked = true
	}

	probeID := cfg.ProbeID
	if probeID == "" {
		probeID = cfg.Tier
	}
	res := runResult{
		Tier: cfg.Tier, ProbeID: probeID, ProbeClass: cfg.ProbeClass, Name: cfg.NameCN,
		Seconds: elapsed, Frames: frame,
		FPSEma: fpsEMA, FPSAvg: fpsAvg, FPSMin: fpsMin, FPSMax: fpsMax, FPSJitter: fpsSpan(fpsMin, fpsMax),
		LowFPSRatio: func() float64 {
			if instFPSCount <= 0 {
				return 0
			}
			return float64(lowFPSCount) / float64(instFPSCount)
		}(),
		CPUAvg:     cpuAvg,
		RSSStartKB: rssStart, RSSEndKB: lastRSS,
		RSSSteadyDeltaKB: rssSteadyDelta(rssSamples),
		GPUOps:           gpuOpsLast, CPUFallback: cpuFBLast, LastFB: lastFB,
		Presents: presents, PresentErrors: presentErrors,
		PresentErrResize: presentErrResize, PresentErrSteady: presentErrSteady,
		LastPresentErr: lastPresentErr, ResizeEvents: resizeEvents, RecoverFails: recoverFails,
		ParticleN: cfg.ParticleN, MinParticleN: cfg.MinParticleN, Region: cfg.Region,
		EnableSolid: cfg.Solid, EnableBlend: cfg.Blend, EnableGlow: cfg.Glow,
		EnableMesh: cfg.Mesh, EnableAtlas: cfg.Atlas, EnableText: cfg.Text,
		EnableLayer: cfg.Layer, EnableTrails: cfg.Trails,
		PerCircleBlend: cfg.PerCircleBlend, ResizeOscillate: cfg.ResizeOscillate,
		PathSubmitHeavy: cfg.PathSubmitHeavy, MultiLayer: cfg.MultiLayer,
		AltClear: cfg.AltClear, GrowN: cfg.GrowN, MaxCPUPct: cfg.MaxCPUPct, MaxJitter: cfg.MaxJitter,
		BlendCircles: cfg.BlendCircles,
		ContentOK:    contentOKFlag, ContentNote: contentNote,
		PixelOK: pixelOKFlag, PixelNote: pixelNote, PixelSamples: pixelSamples,
		StageSigOK: stageSigOK, StageSigNote: stageSigNote,
		SigSamples: sigSamples, SigFails: sigFails,
		SigFailRatio: func() float64 {
			if sigSamples <= 0 {
				return 0
			}
			return float64(sigFails) / float64(sigSamples)
		}(),
		ProbeOK: probeOKFlag, ProbeNote: probeNote,
		AllowLowFPS: cfg.AllowLowFPS, ExitReason: exitReason,
		Features: cfg.featuresSummary(), BisectHint: cfg.BisectHint, Expect: cfg.Expect,
	}
	// Final resize without recovery
	if awaitRecover && postResizeOK == 0 && resizeEvents > 0 {
		res.RecoverFails++
	}
	res.Status, res.FailReason = judgeResult(res, targetFPS)
	collectWarnings(&res, targetFPS)
	writeResult(resultPath, res)
	log.Printf("DONE probe=%s status=%s fps_ema=%.1f fps_avg=%.1f jit=%.1f cpu=%.1f cpu_fb=%d n=%d present_err=%d/%d feats=%s reason=%s warn=%s exit=%s",
		probeID, res.Status, res.FPSEma, res.FPSAvg, res.FPSJitter, res.CPUAvg, res.CPUFallback, res.ParticleN, res.PresentErrSteady, res.PresentErrResize, res.Features, res.FailReason, joinWarn(res.Warnings), res.ExitReason)
	if res.Status != "PASS" {
		os.Exit(1)
	}
}

func drawHUD(dc *render.Context, fonts fontPack, cfg featureConfig, fw, fh float64, note string, fps, cpu float64, rss int64, frame int) {
	dc.SetRGBA(0.05, 0.06, 0.09, 0.82)
	dc.DrawRectangle(0, 0, fw, 56)
	_ = dc.Fill()
	ensureFont(dc, fonts, 14)
	dc.SetRGBA(0.95, 0.97, 1, 1)
	id := cfg.ProbeID
	if id == "" {
		id = cfg.Tier
	}
	dc.DrawString(fmt.Sprintf("%s  %s  n=%d  region=%.0f%%  feats[%s]  class=%s",
		id, cfg.NameCN, cfg.ParticleN, cfg.Region*100, cfg.featuresSummary(), cfg.ProbeClass), 10, 18)
	dc.SetRGBA(0.55, 0.9, 0.7, 1)
	dc.DrawString(fmt.Sprintf("FPS %.1f  CPU %.0f%%  RSS %dKB  frame %d  (本进程)", fps, cpu, rss, frame), 10, 40)

	dc.SetRGBA(0.08, 0.1, 0.14, 0.78)
	dc.DrawRectangle(0, fh-44, fw, 44)
	_ = dc.Fill()
	ensureFont(dc, fonts, 12)
	dc.SetRGBA(0.9, 0.92, 0.98, 1)
	if note == "" {
		note = cfg.Expect
	}
	dc.DrawString(note, 10, fh-28)
	if cfg.BisectHint != "" {
		dc.SetRGBA(0.7, 0.75, 0.85, 1)
		dc.DrawString("二分: "+cfg.BisectHint, 10, fh-12)
	}
}
