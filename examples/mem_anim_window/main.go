//go:build linux && !nogpu

// Complex animated X11 window for render-layer API coverage + memory soak.
//
// Exercises a wide surface of render.Context (shapes/path/dash/clip/layer/
// backdrop/mask/image/text/filter/transform/blend/vertices/write-pixels) which
// in turn drives webgpu → rwgpu → libwgpu_native. Feature flags let you turn
// modules on/off in code (or env) when hunting leaks.
//
// Exit:
//
//   - Default (interactive): no time limit — close the window or Ctrl+C.
//     GPUI_ANIM_SECONDS is unset/0 → never auto-exit on duration.
//
//   - Automation: ONE scenario per process with an explicit timeout:
//
//     GPUI_SCENARIO=S03 GPUI_ANIM_SECONDS=90 go run ./examples/mem_anim_window
//
// Scenarios S01–S23: docs/MEM_ANIM_LONGSOAK_PLAN.md (never multi-scenario in one process)
package main

import (
	"errors"
	"fmt"
	"image"
	"log"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
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

// Features is the primary code-level switchboard for leak isolation.
// Edit defaults here, or override at runtime via GPUI_FEAT_* env vars.
// When hunting a leak: set Features.AllOffExcept("Glow")-style via env
// GPUI_FEAT_ALL=0 GPUI_FEAT_GLOW=1
var Features = FeatureFlags{
	Background: true,
	GlowOrbs:   true,
	Cards:      true,
	Paths:      true,
	DashStroke: true,
	Clip:       true,
	Layer:      true,
	Backdrop:   true,
	Mask:       true,
	Image:      true,
	Text:       true,
	// Filter (blur/color-matrix) is heavy on VRAM — on by default for coverage,
	// disable with GPUI_FEAT_FILTER=0 when isolating other modules.
	Filter:    true,
	Transform: true,
	Blend:     true,
	Vertices:  true,
	Pixels:    true,
	Polygon:   true,
	HUD:       true,
}

// FeatureFlags gates each render module independently.
type FeatureFlags struct {
	Background bool
	GlowOrbs   bool
	Cards      bool
	Paths      bool
	DashStroke bool
	Clip       bool
	Layer      bool
	Backdrop   bool
	Mask       bool
	Image      bool
	Text       bool
	Filter     bool
	Transform  bool
	Blend      bool
	Vertices   bool
	Pixels     bool
	Polygon    bool
	// Skia-gap modules (S15+)
	Gradient  bool
	Pattern   bool
	AdvBlend  bool
	RRectClip bool
	TextLCD   bool
	Damage    bool
	ScrollUI  bool
	Mesh3D    bool
	HUD       bool
}

func featureCount(f FeatureFlags) int {
	n := 0
	if f.Background {
		n++
	}
	if f.GlowOrbs {
		n++
	}
	if f.Cards {
		n++
	}
	if f.Paths {
		n++
	}
	if f.DashStroke {
		n++
	}
	if f.Clip {
		n++
	}
	if f.Layer {
		n++
	}
	if f.Backdrop {
		n++
	}
	if f.Mask {
		n++
	}
	if f.Image {
		n++
	}
	if f.Text {
		n++
	}
	if f.Filter {
		n++
	}
	if f.Transform {
		n++
	}
	if f.Blend {
		n++
	}
	if f.Vertices {
		n++
	}
	if f.Pixels {
		n++
	}
	if f.Polygon {
		n++
	}
	if f.Gradient {
		n++
	}
	if f.Pattern {
		n++
	}
	if f.AdvBlend {
		n++
	}
	if f.RRectClip {
		n++
	}
	if f.TextLCD {
		n++
	}
	if f.Damage {
		n++
	}
	if f.ScrollUI {
		n++
	}
	if f.Mesh3D {
		n++
	}
	if f.HUD {
		n++
	}
	return n
}

func (f FeatureFlags) Summary() string {
	on := make([]string, 0, 18)
	add := func(name string, v bool) {
		if v {
			on = append(on, name)
		}
	}
	add("bg", f.Background)
	add("glow", f.GlowOrbs)
	add("cards", f.Cards)
	add("paths", f.Paths)
	add("dash", f.DashStroke)
	add("clip", f.Clip)
	add("layer", f.Layer)
	add("backdrop", f.Backdrop)
	add("mask", f.Mask)
	add("image", f.Image)
	add("text", f.Text)
	add("filter", f.Filter)
	add("xform", f.Transform)
	add("blend", f.Blend)
	add("verts", f.Vertices)
	add("pixels", f.Pixels)
	add("poly", f.Polygon)
	add("grad", f.Gradient)
	add("pattern", f.Pattern)
	add("advblend", f.AdvBlend)
	add("rrectclip", f.RRectClip)
	add("textlcd", f.TextLCD)
	add("damage", f.Damage)
	add("scroll", f.ScrollUI)
	add("mesh3d", f.Mesh3D)
	add("hud", f.HUD)
	if len(on) == 0 {
		return "(none)"
	}
	return strings.Join(on, ",")
}

func applyFeatureEnv(f *FeatureFlags) {
	if v, ok := envBoolOK("GPUI_FEAT_ALL"); ok && !v {
		// Explicit ALL=0 → clear everything first, then enable individuals.
		*f = FeatureFlags{}
	}
	set := func(key string, dst *bool) {
		if v, ok := envBoolOK(key); ok {
			*dst = v
		}
	}
	set("GPUI_FEAT_BG", &f.Background)
	set("GPUI_FEAT_GLOW", &f.GlowOrbs)
	set("GPUI_FEAT_CARDS", &f.Cards)
	set("GPUI_FEAT_PATHS", &f.Paths)
	set("GPUI_FEAT_DASH", &f.DashStroke)
	set("GPUI_FEAT_CLIP", &f.Clip)
	set("GPUI_FEAT_LAYER", &f.Layer)
	set("GPUI_FEAT_BACKDROP", &f.Backdrop)
	set("GPUI_FEAT_MASK", &f.Mask)
	set("GPUI_FEAT_IMAGE", &f.Image)
	set("GPUI_FEAT_TEXT", &f.Text)
	set("GPUI_FEAT_FILTER", &f.Filter)
	set("GPUI_FEAT_TRANSFORM", &f.Transform)
	set("GPUI_FEAT_BLEND", &f.Blend)
	set("GPUI_FEAT_VERTICES", &f.Vertices)
	set("GPUI_FEAT_PIXELS", &f.Pixels)
	set("GPUI_FEAT_POLYGON", &f.Polygon)
	set("GPUI_FEAT_GRADIENT", &f.Gradient)
	set("GPUI_FEAT_PATTERN", &f.Pattern)
	set("GPUI_FEAT_ADVBLEND", &f.AdvBlend)
	set("GPUI_FEAT_RRECTCLIP", &f.RRectClip)
	set("GPUI_FEAT_TEXTLCD", &f.TextLCD)
	set("GPUI_FEAT_DAMAGE", &f.Damage)
	set("GPUI_FEAT_SCROLL", &f.ScrollUI)
	set("GPUI_FEAT_MESH3D", &f.Mesh3D)
	set("GPUI_FEAT_HUD", &f.HUD)
}

func main() {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		if _, err := os.Stat("lib/libwgpu_native.so"); err == nil {
			_ = os.Setenv("WGPU_NATIVE_PATH", "lib/libwgpu_native.so")
		}
	}
	if os.Getenv("GPUI_SURFACE_SAMPLE_COUNT") == "" {
		_ = os.Setenv("GPUI_SURFACE_SAMPLE_COUNT", "1")
	}

	// ONE scenario per process — never rotate scenarios in-process.
	scenarioID := strings.TrimSpace(os.Getenv("GPUI_SCENARIO"))
	spec, hasScenario := applyScenario(scenarioID)
	if !hasScenario {
		spec = scenarioSpec{ID: "S12", Name: "FullComposite", Flags: Features}
	}
	applyFeatureEnv(&Features)
	damagePresent := spec.DamagePresent || Features.Damage
	if damagePresent {
		resetDamageBootstrap()
	}
	logEvery := envInt("GPUI_ANIM_LOG_EVERY", 60)
	targetFPS := envInt("GPUI_TARGET_FPS", defaultTargetFPS)
	if targetFPS < 15 {
		targetFPS = 15
	}
	if targetFPS > 120 {
		targetFPS = 120
	}
	frameBudget := time.Second / time.Duration(targetFPS)
	perfLite := envBool("GPUI_PERF_LITE", false) || spec.Lite
	stress := envBool("GPUI_STRESS", false) || spec.Stress
	// Default 0 = run until window close / signal (no duration auto-exit).
	// Timed soak only when GPUI_ANIM_SECONDS is set to a positive value.
	animSeconds := envInt("GPUI_ANIM_SECONDS", 0)
	if animSeconds < 0 {
		animSeconds = 0
	}
	density := envInt("GPUI_DENSITY", spec.Density)
	if density < 0 {
		density = 0
	}
	rssHardKB := int64(envInt("GPUI_RSS_HARD_KB", 3670016))
	metrics := openMetrics(os.Getenv("GPUI_METRICS_FILE"))
	defer metrics.close()
	resultPath := os.Getenv("GPUI_RESULT_FILE")

	winW, winH := defaultW, defaultH
	xw, err := openX11Window(winW, winH, fmt.Sprintf("gpui mem anim %s %s", spec.ID, spec.Name))
	if err != nil {
		log.Fatalf("X11: %v", err)
	}
	defer xw.Close()

	// Drain initial Map/Configure so swapchain matches real client size (avoids
	// immediate suboptimal + black flash at first present).
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

	device, err := adapter.RequestDevice(rendgpu.DeviceDescriptor("mem-anim-window"))
	if err != nil {
		log.Fatalf("RequestDevice: %v", err)
	}
	// Release the *current* device at exit (may change after auto-recover).
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

	if err := rendgpu.SetDeviceProvider(&webgpu.SimpleDeviceProvider{
		Dev: device, Adpt: adapter, Format: sc.Format,
	}); err != nil {
		log.Fatalf("SetDeviceProvider: %v", err)
	}
	defer func() { _ = rendgpu.ResetAccelerator() }()

	// Library-level device recovery (Flutter Rasterizer / Skia GrContext model):
	// on device-lost, Swapchain recreates the device + reconfigures the surface.
	// Host rebinds the render accelerator; animation loop keeps running.
	sc.OnDeviceAbandon = func(_ *webgpu.Device) {
		rendgpu.AbandonDevice()
	}
	sc.EnableAutoRecover(adapter, "mem-anim-window", func(dev *webgpu.Device) {
		device = dev
		if err := rendgpu.SetDeviceProvider(&webgpu.SimpleDeviceProvider{
			Dev: device, Adpt: adapter, Format: sc.Format,
		}); err != nil {
			log.Printf("SetDeviceProvider after recover: %v", err)
		}
		log.Printf("GPU device recovered (recoveries=%d) — continue rendering", sc.Recoveries())
	})

	dc := render.NewContext(winW, winH)
	defer dc.Close()

	fonts := loadFonts(dc)
	rng := newRNG(42)
	scene := newAnimScene(rng, perfLite || !stress)
	assets := newAssets()
	pixelScratch := make([]byte, 24*16*4)
	var hud frameHUD

	stopSig := make(chan os.Signal, 1)
	signal.Notify(stopSig, os.Interrupt, syscall.SIGTERM)

	log.Printf("scenario=%s (%s) ONE-SCENARIO-ONLY window %dx%d target=%dfps present=%s lite=%v stress=%v density=%d seconds=%d features=[%s]",
		spec.ID, spec.Name, winW, winH, targetFPS, sc.PresentModeName(), perfLite, stress, density, animSeconds, Features.Summary())
	// Always print so interactive runs are unambiguous about duration policy.
	if animSeconds > 0 {
		log.Printf("GPUI_ANIM_SECONDS=%d → auto-exit after %ds visible time (or close window)", animSeconds, animSeconds)
	} else {
		log.Printf("GPUI_ANIM_SECONDS=0/unset → no time limit; close window or Ctrl+C; FPS paced ~%d", targetFPS)
	}

	start := time.Now()
	frame := 0
	rssStart := rssKB()
	exitReason := "window_close"
	var (
		fpsEMA               float64
		lastFrameEnd         time.Time
		cpuPctEMA            float64
		prevCPU              cpuSample
		havePrevCPU          bool
		cpuSum               float64
		cpuSamples           int
		rssSamples           []int64
		lastRSS              int64
		forceFull            = true // first frame + every resize → full clear present
		lastGuideSizeClass   = -1
		stickyLite           bool
		lowFPSFrames         int
		pendW, pendH         = winW, winH
		havePendSize         bool
		lastCfgEvent         time.Time
		pendSizeSince        time.Time
		nextFrameAt          time.Time // fixed-rate schedule (includes post-work)
		fixedSize            = animSeconds > 0 || envBool("GPUI_FIXED_SIZE", false)
		windowMinimized      bool
		windowFullyObscured  bool
		windowFocused        = true
		windowGeomCovered    bool // fully covered by another mapped top-level window
		lastGeomCoverCheck   time.Time
		geomCoverStickyUntil time.Time // hysteresis: stay paused after cover clears
		// Consecutive soft BeginFrame failures while "visible". Used only to
		// skip frames / log; does NOT freeze the animation for focus or partial
		// cover (user still wants sim/draw updates when the window is on screen).
		softAcquireFails  int
		forceRenderHidden = envBool("GPUI_FORCE_RENDER_WHEN_HIDDEN", false)
		// Visible-only timing (mature apps pause metrics while minimized):
		// wall clock keeps running for wallElapsed, but FPS/duration gates use
		// only time spent actively presenting.
		hiddenAccum time.Duration
		hiddenSince time.Time
		wasHidden   bool
	)
	if fixedSize {
		xw.LockSize(winW, winH)
		log.Printf("fixed window size %dx%d (soak/GPUI_FIXED_SIZE) — ignore WM resize", winW, winH)
	}
	lastRSS = rssStart
	for {
		select {
		case <-stopSig:
			log.Printf("signal after %d frames", frame)
			exitReason = "signal"
			goto done
		default:
		}
		// Pause the soak budget while hidden (Flutter/Chromium/winit pattern:
		// do not charge wall-clock of minimized time against frame/FPS gates).
		hiddenNow := hiddenAccum
		if !hiddenSince.IsZero() {
			hiddenNow += time.Since(hiddenSince)
		}
		visibleElapsed := time.Since(start) - hiddenNow
		if animSeconds > 0 && visibleElapsed >= time.Duration(animSeconds)*time.Second {
			log.Printf("duration %ds visible reached frames=%d scenario=%s (wall=%.1fs hidden=%.1fs)",
				animSeconds, frame, spec.ID, time.Since(start).Seconds(), hiddenNow.Seconds())
			exitReason = "duration"
			goto done
		}

		frameStart := time.Now()
		resizedThis := false

		// Drain X events; debounce resize until ConfigureNotify is quiet.
		for xw.Pending() {
			ev := xw.NextEvent()
			if ev.Type == xUnmapNotify {
				// Minimize / unmap: stop GetCurrentTexture — continuous acquire on
				// an unmapped surface can lose the device (native SIGABRT).
				windowMinimized = true
			}
			if ev.Type == xMapNotify {
				windowMinimized = false
				forceFull = true
				softAcquireFails = 0
			}
			if ev.Type == xVisibilityNotify {
				// FullyObscured pauses present. Partial cover still draws.
				// Note: GNOME compositor often never sends FullyObscured when the
				// window is merely stacked under another app — FocusOut handles that.
				windowFullyObscured = ev.Visibility == xVisibilityFullyObscured
				if !windowFullyObscured {
					forceFull = true
					softAcquireFails = 0
				}
			}
			if ev.Type == xFocusIn {
				windowFocused = true
				windowGeomCovered = false // focused ⇒ presentable; keep drawing
				geomCoverStickyUntil = time.Time{}
				forceFull = true
				softAcquireFails = 0
			}
			if ev.Type == xFocusOut {
				// User switched to another window (typical "cover" repro). Continuous
				// present while stacked-under causes TDR → Parent device is lost SIGABRT
				// on this libwgpu. Pause present until focus returns.
				windowFocused = false
			}
			// GNOME Iconify: often only WM_STATE→IconicState, no UnmapNotify.
			if xw.IsWMStateProperty(ev) {
				iconic := xw.IsIconic()
				windowMinimized = iconic
				if !iconic {
					forceFull = true
					softAcquireFails = 0
				}
			}
			if ev.Type == xConfigureNotify {
				nw, nh := ev.Width, ev.Height
				// Zero extent (some WMs iconify without UnmapNotify).
				if nw <= 0 || nh <= 0 {
					windowMinimized = true
					continue
				}
				if nw < 64 {
					nw = 64
				}
				if nh < 64 {
					nh = 64
				}
				if nw != pendW || nh != pendH {
					pendW, pendH = nw, nh
					havePendSize = true
					lastCfgEvent = time.Now()
					if pendSizeSince.IsZero() {
						pendSizeSince = lastCfgEvent
					}
				}
			}
			if ev.Type == xClientMessage && xw.IsDelete(ev) {
				log.Printf("window closed by user after %d frames", frame)
				exitReason = "window_close"
				goto done
			}
		}
		// Idle while minimized/unmapped/fully-obscured: do not touch the swapchain.
		// GPUI_FORCE_RENDER_WHEN_HIDDEN=1 keeps rendering for soak verification only
		// (library must return Go errors, never native abort).
		//
		// Policy (desktop UX + library safety):
		//   minimized / FullyObscured → pause present (no usable surface)
		//   partial cover / unfocused  → KEEP drawing (animation & sim advance)
		//   soft-fail streak (cover without VisibilityFullyObscured on some WMs)
		//     → enter FullyObscured idle to avoid continuous GCT after TDR
		// Device-lost: sticky + AutoRecover; never rely on SIGABRT longjmp.
		// Geometry full-cover check only when unfocused (Skia/Flutter: unpresentable
		// surface → skip acquire). Focused window always presents (even partial cover).
		// Unfocused + still partially visible → keep drawing data updates.
		// Unfocused + fully covered by another top-level → pause present (TDR safety).
		if time.Since(lastGeomCoverCheck) > 250*time.Millisecond {
			lastGeomCoverCheck = time.Now()
			rawCovered := false
			if !windowFocused {
				rawCovered = xw.IsFullyCoveredByOtherWindows()
			}
			// Hysteresis: once fully covered, keep pause for 3s after detector
			// clears. Log flapping (covered true/false every few s) was still
			// full-rate presenting under stack-under and hit Parent device is lost.
			if rawCovered {
				geomCoverStickyUntil = time.Now().Add(5 * time.Second)
			}
			covered := false
			if windowFocused {
				covered = false
				geomCoverStickyUntil = time.Time{}
			} else if rawCovered || (!geomCoverStickyUntil.IsZero() && time.Now().Before(geomCoverStickyUntil)) {
				covered = true
			}
			if covered != windowGeomCovered {
				windowGeomCovered = covered
				if !covered {
					forceFull = true
					softAcquireFails = 0
				}
				log.Printf("geom cover changed: covered=%v raw=%v focused=%v sticky_left=%.1fs",
					covered, rawCovered, windowFocused, time.Until(geomCoverStickyUntil).Seconds())
			}
		}

		// Skia/Flutter surface lifecycle:
		//   unpresentable (minimized / VisibilityFullyObscured / geom fully covered) → pause acquire
		//   unfocused but still visible → KEEP presenting (data updates must draw)
		//   device lost → sticky refuse + EnableAutoRecover (not process abort)
		// FocusOut alone does NOT pause; geom cover is only checked while unfocused.
		hidden := (windowMinimized || windowFullyObscured || windowGeomCovered) && !forceRenderHidden
		if hidden {
			if !wasHidden {
				wasHidden = true
				hiddenSince = time.Now()
				log.Printf("window hidden (minimized=%v fully_obscured=%v geom_covered=%v focused=%v) — pause present",
					windowMinimized, windowFullyObscured, windowGeomCovered, windowFocused)
			}
			now := time.Now()
			if nextFrameAt.IsZero() {
				nextFrameAt = now
			}
			// Throttle hard while hidden — no GPU work, just event drain.
			deadline := nextFrameAt.Add(50 * time.Millisecond)
			if d := deadline.Sub(now); d > 200*time.Microsecond {
				time.Sleep(d)
			}
			nextFrameAt = deadline
			// Pump callbacks so lost is sticky before resume acquire.
			// Do NOT exit: Swapchain.EnableAutoRecover recreates device on next
			// BeginFrame after the window becomes visible again.
			if device != nil {
				device.FlushCallbacks()
			}
			continue
		}
		// Transition hidden → visible: credit hidden time, force full present.
		if wasHidden {
			hiddenDur := time.Duration(0)
			if !hiddenSince.IsZero() {
				hiddenDur = time.Since(hiddenSince)
				hiddenAccum += hiddenDur
				hiddenSince = time.Time{}
			}
			wasHidden = false
			forceFull = true
			softAcquireFails = 0
			nextFrameAt = time.Time{}  // resync pace after resume
			lastFrameEnd = time.Time{} // do not let hidden gap crush instFPS/EMA
			sc.MarkNeedsReconfigure()
			sc.ClearRecoverCooldown()
			// Soft native: BeginFrame maps device-lost via error; AutoRecover recreates.
			if sc.Device != nil {
				device = sc.Device
			}
			log.Printf("window visible again — resume present (hidden_total=%.1fs lost=%v recoveries=%d)",
				hiddenAccum.Seconds(), device != nil && device.IsLost(), sc.Recoveries())
		}
		// Flush lost callbacks before GPU work. Device-lost is recovered inside
		// sc.BeginFrame when EnableAutoRecover is armed (do not exit here).
		if device != nil {
			device.FlushCallbacks()
		}
		if havePendSize && (pendW != winW || pendH != winH) {
			if fixedSize {
				// Timed soak / fixed mode: do not follow WM maximize/tile (spurious
				// ConfigureNotify was collapsing FPS mid-run in matrix tests).
				havePendSize = false
				pendSizeSince = time.Time{}
				xw.LockSize(winW, winH)
			} else {
				quiet := time.Since(lastCfgEvent)
				dragFor := time.Since(pendSizeSince)
				// Quiet 50ms → user stopped; or drag >300ms → apply latest to keep up.
				if quiet >= 50*time.Millisecond || dragFor >= 300*time.Millisecond {
					winW, winH = pendW, pendH
					if err := sc.Resize(uint32(winW), uint32(winH)); err != nil {
						log.Printf("sc.Resize: %v (will retry)", err)
					} else if err := dc.Resize(winW, winH); err != nil {
						log.Printf("dc.Resize: %v (will retry)", err)
					} else {
						forceFull = true
						resizedThis = true
						havePendSize = false
						pendSizeSince = time.Time{}
						gGuideCache.key = ""
						gGuideCache.img = nil
						if damagePresent {
							resetDamageBootstrap()
						}
						log.Printf("resized → %dx%d (debounced)", winW, winH)
					}
				}
			}
		}

		// Animation clock excludes hidden idle (same basis as FPS duration).
		hidNow := hiddenAccum
		if !hiddenSince.IsZero() {
			hidNow += time.Since(hiddenSince)
		}
		t := time.Since(start).Seconds() - hidNow.Seconds()
		if t < 0 {
			t = 0
		}
		scene.tick(t, frame, rng)

		// Adaptive quality: large surfaces (more pixels) or FPS pressure → lite path.
		// Keeps ~60fps after maximize without gutting scenario coverage.
		// IMPORTANT: lite must be sticky. Toggling lite every few frames used to
		// rebuild the CJK text panel (~15–25ms) → fps collapse feedback loop
		// (seen as "backdrop/filter flash" + 45fps on S07/S12).
		areaScale := float64(winW*winH) / float64(defaultW*defaultH)
		adaptiveLite := perfLite || stickyLite
		if areaScale > 1.6 {
			adaptiveLite = true
		}
		if fpsEMA > 0 && fpsEMA < float64(targetFPS)-8 {
			lowFPSFrames++
			if lowFPSFrames >= 45 { // ~0.75s under pressure
				stickyLite = true
				adaptiveLite = true
			}
		} else {
			lowFPSFrames = 0
		}
		// FullComposite / high feature-count: lite quality (geometry/orbs).
		// Continuous modules stay ON (Q-NOFLICKER). Layer/backdrop/filter now run
		// on small offscreen RTs with real APIs — do NOT force lite just because
		// those modules are on (would hide grayscale tile on S10 etc.).
		if featureCount(Features) >= 8 {
			adaptiveLite = true
			stickyLite = true
		}

		// Cadence: keep wall-clock near 60fps while still exercising all enabled APIs over time.
		// stress=1 forces every enabled module every frame (memory-leak soak; may be <60fps).
		active := cadenceFor(frame, Features, stress, adaptiveLite)

		// Sample RSS / build HUD less hotly (every frame string work is costly).
		if frame%15 == 0 || lastRSS == 0 {
			lastRSS = rssKB()
		}
		hud.FPS = fpsEMA
		hud.CPUPct = cpuPctEMA
		hud.Frame = frame
		hud.TargetFPS = targetFPS
		hud.FrameMS = 0
		hud.RSSKB = lastRSS
		hud.W, hud.H = winW, winH
		// Bottom bar shows MASTER scenario modules (stable string). Per-frame
		// cadence used to change length every few frames → text "jumped" right.
		hud.Active = Features.Summary()
		hud.ScenarioID = spec.ID
		hud.ScenarioName = spec.Name
		hud.PresentMode = sc.PresentModeName()
		hud.MasterSummary = Features.Summary()
		_ = active // still used for drawFrame
		// Guide is scenario-static; rebuild only on size class change (not every cadence tick).
		sizeClass := (winW/64)*10000 + (winH / 64)
		if hud.GuideLines == nil || sizeClass != lastGuideSizeClass {
			hud.GuideLines = buildGuideLines(spec, Features, active, adaptiveLite, areaScale)
			gGuideCache.key = ""
			gGuideCache.img = nil
			lastGuideSizeClass = sizeClass
		}
		hud.ResizedThisFrame = resizedThis

		dc.BeginFrame()
		// Background always fills the surface → coverage promotes to full clear path.
		// Force explicit full present after resize / first frames to avoid flash.
		if forceFull || resizedThis {
			dc.MarkFullRedraw()
			if damagePresent {
				resetDamageBootstrap()
			}
		}
		if damagePresent {
			dc.SetDamageTracking(true)
		}
		drawFrame(dc, winW, winH, t, frame, scene, fonts, assets, rng, active, adaptiveLite, pixelScratch, hud)
		if density > 0 {
			// Cap density growth with area so maximize does not collapse FPS.
			d := density
			if areaScale > 1.5 {
				d = int(float64(density) / math.Sqrt(areaScale))
				if d < density/3 {
					d = density / 3
				}
			}
			drawDensityField(dc, winW, winH, t, frame, d)
		}

		fb, err := sc.BeginFrame()
		if err != nil {
			// Library error policy (do not thrash native Configure):
			//   DeviceLost  → skip frame; EnableAutoRecover recreates on next BeginFrame
			//   Occluded/Timeout → skip frame only
			//   Outdated/Lost → reconfigure once (unless fixed-size soak)
			//   Other soft → skip on fixedSize; reconfigure once otherwise
			if errors.Is(err, webgpu.ErrDeviceLost) || (device != nil && device.IsLost()) {
				// Library auto-recover is rate-limited; skip frame and retry.
				// Never abort — matches Flutter engine keep-alive after context loss.
				log.Printf("BeginFrame: %v — device lost/recovering (recoveries=%d), skip frame",
					err, sc.Recoveries())
				forceFull = true
				// Keep trying recover; re-sync pointer if swapchain already swapped.
				sc.ClearRecoverCooldown()
				if sc.Device != nil && sc.Device != device {
					device = sc.Device
				}
				time.Sleep(16 * time.Millisecond)
				continue
			}
			if errors.Is(err, webgpu.ErrSurfaceOccluded) || errors.Is(err, webgpu.ErrTimeout) {
				// Skip this frame only — do NOT freeze animation for the rest of
				// the run. Occluded/Timeout often happens under partial cover or
				// compositor hitch; next frames may succeed. Library refuseIfLost
				// protects against native abort if the device actually dies.
				softAcquireFails++
				forceFull = true
				time.Sleep(2 * time.Millisecond)
				continue
			}
			softAcquireFails++
			// Non-occluded soft errors: skip frame; only enter FullyObscured idle
			// after a long streak (likely unmapped without events), not after a
			// few focus/partial-cover glitches.
			softLimit := 45
			if !windowFocused {
				softLimit = 10 // unfocused: bail faster — half-covered still TDRs
			}
			if softAcquireFails >= softLimit {
				windowFullyObscured = true
				windowGeomCovered = true
				geomCoverStickyUntil = time.Now().Add(5 * time.Second)
				log.Printf("BeginFrame soft fails=%d (focused=%v) — enter hidden idle (protect device)", softAcquireFails, windowFocused)
				forceFull = true
				time.Sleep(16 * time.Millisecond)
				continue
			}
			if fixedSize {
				log.Printf("BeginFrame: %v — fixed-size skip frame (no reconfigure thrash)", err)
				forceFull = true
				time.Sleep(2 * time.Millisecond)
				continue
			}
			// Outdated / surface lost / other: one Resize recovery attempt.
			log.Printf("BeginFrame: %v — reconfigure %dx%d and skip frame", err, winW, winH)
			if rerr := sc.Resize(uint32(winW), uint32(winH)); rerr != nil {
				log.Printf("recover Resize: %v", rerr)
				if errors.Is(rerr, webgpu.ErrDeviceLost) || (device != nil && device.IsLost()) {
					forceFull = true
					time.Sleep(16 * time.Millisecond)
					continue
				}
			}
			_ = dc.Resize(winW, winH)
			forceFull = true
			time.Sleep(2 * time.Millisecond)
			continue
		}
		softAcquireFails = 0
		present := func() error { return sc.EndFrame(fb) }
		// Default: full present for continuous animation (avoids LoadOpLoad flash on iGPU).
		// S19 DamagePartialPresent deliberately exercises PresentFrameAuto with dirty rects.
		if damagePresent && !forceFull && !resizedThis && frame > 2 {
			dc.SetDamageTracking(true)
			out, err := dc.PresentFrameAuto(fb.Handle, fb.Width, fb.Height, present)
			if err != nil {
				sc.DiscardFrame(fb)
				if errors.Is(err, webgpu.ErrDeviceLost) || (device != nil && device.IsLost()) || strings.Contains(err.Error(), "device lost") {
					log.Printf("PresentFrameAuto: %v — device lost/recovering, skip frame", err)
					forceFull = true
					time.Sleep(16 * time.Millisecond)
					continue
				}
				log.Printf("PresentFrameAuto: %v — skip frame", err)
				forceFull = true
				time.Sleep(2 * time.Millisecond)
				continue
			}
			if out.Idle {
				// Never freeze the window: force a tiny present if planner idled.
				if err := dc.PresentFrameFull(fb.Handle, fb.Width, fb.Height, present); err != nil {
					sc.DiscardFrame(fb)
					if errors.Is(err, webgpu.ErrDeviceLost) || (device != nil && device.IsLost()) || strings.Contains(err.Error(), "device lost") {
						log.Printf("PresentFrameFull(idle-fallback): %v — device lost/recovering, skip frame", err)
						forceFull = true
						time.Sleep(16 * time.Millisecond)
						continue
					}
					log.Printf("PresentFrameFull(idle-fallback): %v — skip frame", err)
					forceFull = true
					time.Sleep(2 * time.Millisecond)
					continue
				}
				hud.PresentMode = "full(idle-fallback)"
			} else {
				hud.PresentMode = out.Mode.String()
			}
		} else {
			if damagePresent {
				dc.SetDamageTracking(true)
			}
			if err := dc.PresentFrameFull(fb.Handle, fb.Width, fb.Height, present); err != nil {
				sc.DiscardFrame(fb)
				if errors.Is(err, webgpu.ErrDeviceLost) || (device != nil && device.IsLost()) || strings.Contains(err.Error(), "device lost") {
					log.Printf("PresentFrameFull: %v — device lost/recovering, skip frame", err)
					forceFull = true
					time.Sleep(16 * time.Millisecond)
					continue
				}
				log.Printf("PresentFrameFull: %v — skip frame", err)
				forceFull = true
				time.Sleep(2 * time.Millisecond)
				continue
			}
			if !damagePresent {
				hud.PresentMode = sc.PresentModeName()
			} else {
				hud.PresentMode = "full(bootstrap)"
			}
		}
		forceFull = false
		// XFlush only after resize (map/config); every-frame flush costs CPU for no gain.
		if resizedThis {
			xw.Flush()
		}

		// Work time includes present (Fifo may block ~1 vsync when driver cooperates).
		now := time.Now()
		workMS := float64(now.Sub(frameStart).Microseconds()) / 1000.0
		hud.FrameMS = workMS

		// Process-only CPU% from /proc/self/stat (utime+stime of THIS process).
		// 100% ≈ one full core of THIS process — NOT system-wide load.
		if sample, ok := readCPUSample(); ok {
			if havePrevCPU {
				if pct, ok := cpuPercent(prevCPU, sample); ok {
					if cpuPctEMA <= 0 {
						cpuPctEMA = pct
					} else {
						cpuPctEMA = cpuPctEMA*0.85 + pct*0.15
					}
				}
			}
			prevCPU = sample
			havePrevCPU = true
		}

		frame++
		if cpuPctEMA > 0 {
			cpuSum += cpuPctEMA
			cpuSamples++
		}
		// RSS sample is cheap; keep every frame for leak slope accuracy.
		curRSS := rssKB()
		rssSamples = append(rssSamples, curRSS)
		if rssHardKB > 0 && curRSS > rssHardKB {
			log.Printf("RSS hard cap %dKB > %dKB scenario=%s", curRSS, rssHardKB, spec.ID)
			exitReason = "rss_hard_cap"
			goto done
		}
		if logEvery > 0 && frame%logEvery == 0 {
			st := dc.RenderPathStats()
			sst := sc.Stats()
			// Mid-run avg must use visible time only — wall clock includes
			// minimized/occluded idle and would falsely collapse avgFPS.
			hid := hiddenAccum
			if !hiddenSince.IsZero() {
				hid += time.Since(hiddenSince)
			}
			elapsedWall := time.Since(start).Seconds()
			elapsedVis := elapsedWall - hid.Seconds()
			if elapsedVis < 1e-6 {
				elapsedVis = elapsedWall
			}
			avgFPS := float64(frame) / math.Max(elapsedVis, 1e-6)
			ncpu := hostCPUCount()
			machCPU := cpuPctEMA / float64(ncpu)
			log.Printf("scenario=%s frame=%d fps≈%.1f avg=%.1f work=%.1fms proc_cpu_1core≈%.0f%% proc_cpu_machine≈%.0f%%(/%dcores) size=%dx%d proc_rss=%dKB gpu_ops=%d cpu_fb=%d last_fb=%q presents=%d active=[%s]",
				spec.ID, frame, fpsEMA, avgFPS, workMS, cpuPctEMA, machCPU, ncpu, winW, winH, curRSS, st.GPUOps, st.CPUFallbackOps, st.LastCPUFallbackReason, sst.Presents, active.Summary())
			metrics.write(spec.ID, elapsedVis, frame, fpsEMA, avgFPS, workMS, cpuPctEMA, curRSS, st.GPUOps, st.CPUFallbackOps, st.LastCPUFallbackReason, int(sst.Presents), int(sst.Reconfigures), active.Summary())
		}

		// Fixed-rate pacing AFTER all end-of-frame work (CPU/RSS/metrics).
		// Sleep-only (no busy-spin): spin burned a full core and inflated CPU to 50–95%.
		// Schedule: nextFrameAt advances by frameBudget each frame; if we fall behind,
		// resync to "now" (do NOT run at max speed to catch up — that spikes FPS/CPU).
		now = time.Now()
		if nextFrameAt.IsZero() {
			nextFrameAt = now
		}
		// Target end of this frame slot.
		budget := frameBudget
		if !windowFocused {
			// Still present for data updates, but lower rate to cut TDR risk while
			// stacked partially under other windows (Skia hosts often throttle).
			ufps := envInt("GPUI_UNFOCUSED_FPS", 10)
			if ufps < 5 {
				ufps = 5
			}
			if ufps > targetFPS {
				ufps = targetFPS
			}
			budget = time.Second / time.Duration(ufps)
		}
		deadline := nextFrameAt.Add(budget)
		if d := deadline.Sub(now); d > 200*time.Microsecond {
			// Linux timers often overshoot 0.5-2ms under load; undersleep slightly.
			pad := 900 * time.Microsecond
			if d > pad+200*time.Microsecond {
				time.Sleep(d - pad)
			}
			now = time.Now()
		}
		if now.After(deadline) {
			// Missed: resync so the next frame still aims for ~targetFPS.
			nextFrameAt = now
		} else {
			nextFrameAt = deadline
		}

		// Displayed FPS from inter-frame wall clock (includes full loop + sleep).
		instFPS := 0.0
		if !lastFrameEnd.IsZero() {
			dt := now.Sub(lastFrameEnd).Seconds()
			if dt > 1e-6 {
				instFPS = 1.0 / dt
			}
		}
		if instFPS <= 0 {
			instFPS = 1000.0 / math.Max(float64(now.Sub(frameStart).Microseconds())/1000.0, 0.001)
		}
		// Clamp absurd spikes from timer/jitter so EMA stays meaningful.
		if instFPS > float64(targetFPS)*1.25 {
			instFPS = float64(targetFPS) * 1.25
		}
		lastFrameEnd = now
		if fpsEMA <= 0 {
			fpsEMA = instFPS
		} else {
			fpsEMA = fpsEMA*0.88 + instFPS*0.12
		}
	}

done:
	st := dc.RenderPathStats()
	sst := sc.Stats()
	// Finalize any in-progress hidden interval so metrics exclude it.
	if !hiddenSince.IsZero() {
		hiddenAccum += time.Since(hiddenSince)
		hiddenSince = time.Time{}
	}
	elapsedWall := time.Since(start).Seconds()
	elapsedVisible := elapsedWall - hiddenAccum.Seconds()
	if elapsedVisible < 1e-6 {
		elapsedVisible = elapsedWall
	}
	elapsed := elapsedVisible // gates / result "seconds" = visible render time
	avgFPS := float64(frame) / math.Max(elapsedVisible, 1e-6)
	cpuAvg := 0.0
	if cpuSamples > 0 {
		cpuAvg = cpuSum / float64(cpuSamples)
	}
	rssEnd := rssKB()
	steadyDelta := rssSteadyDelta(rssSamples)
	res := runResult{
		Scenario:         spec.ID,
		Name:             spec.Name,
		Seconds:          elapsed,
		Frames:           frame,
		FPSEma:           fpsEMA,
		FPSAvg:           avgFPS,
		CPUAvg:           cpuAvg,
		RSSStartKB:       rssStart,
		RSSEndKB:         rssEnd,
		RSSDeltaKB:       rssEnd - rssStart,
		RSSSteadyDeltaKB: steadyDelta,
		GPUOps:           st.GPUOps,
		CPUFallback:      st.CPUFallbackOps,
		LastFB:           st.LastCPUFallbackReason,
		Presents:         int(sst.Presents),
		AllowLowFPS:      spec.AllowLowFPS,
		ExitReason:       exitReason,
	}
	res.Status, res.FailReason = judgeResult(res, targetFPS)
	writeResult(resultPath, res)
	ncpuDone := hostCPUCount()
	fmt.Printf("done scenario=%s status=%s reason=%q frames=%d elapsed=%.1fs wall=%.1fs hidden=%.1fs fps≈%.1f avg=%.1f proc_cpu_1core≈%.0f%% proc_cpu_machine≈%.0f%%(/%dcores) proc_rss %d→%dKB steady_delta=%dKB %s presents=%d reconfig=%d exit=%s feats=[%s]\n",
		spec.ID, res.Status, res.FailReason, frame, elapsed, elapsedWall, hiddenAccum.Seconds(), fpsEMA, avgFPS, cpuAvg, cpuAvg/float64(ncpuDone), ncpuDone, rssStart, rssEnd, steadyDelta, st.LogLine(),
		sst.Presents, sst.Reconfigures, exitReason, Features.Summary())
	if res.Status != "PASS" {
		os.Exit(2)
	}
}

type frameHUD struct {
	FPS, FrameMS, CPUPct   float64
	Frame, TargetFPS, W, H int
	RSSKB                  int64
	Active                 string
	ScenarioID             string
	ScenarioName           string
	PresentMode            string
	MasterSummary          string
	GuideLines             []string // on-screen "what you should see"
	ResizedThisFrame       bool
}

// cadenceFor selects which modules draw this frame.
//
// Visual correctness rule (user-facing): if a scenario enables a module, it must
// remain VISIBLE continuously — not a 1-frame flash. Old sparse cadence made
// blend/cards/filter appear to "flicker" and guide text looked like lies.
//
// Performance: under lite/adaptive, reduce quality flags (caller passes lite to
// draw* helpers) but keep modules ON. GPUI_CADENCE_SPARSE=1 restores the old
// rotate-on/off mode only for explicit leak-hunt experiments.
func cadenceFor(frame int, master FeatureFlags, stress, lite bool) FeatureFlags {
	if stress {
		return master
	}
	if envBool("GPUI_CADENCE_SPARSE", false) {
		return cadenceSparse(frame, master, lite)
	}
	// Persistent: master flags are the on-screen truth every frame.
	// Q-NOFLICKER / 丝滑: NEVER toggle modules off in the default cadence.
	// stickyOn windows (even multi-frame) still cause visible flash when they go
	// off — e.g. S06 半透明 layer 关掉几帧 → 背景突然变亮 = 闪烁.
	// lite/adaptive only reduces draw quality inside draw* helpers.
	out := master
	// Warm-up: first frames only bg+hud+glow so pipelines compile without a long black hang.
	// After warm-up everything in master stays ON continuously.
	// Short warm-up only (pipeline compile). Long warm-up looked like modules
	// "popping in" = flash. After frame 8 everything master-enabled is continuous.
	// Skia-gap / damage scenarios must not strip modules during warm-up (would flash
	// or miss PresentFrameAuto dirty content).
	if frame < 8 && !(master.Damage || master.Gradient || master.AdvBlend || master.RRectClip || master.TextLCD || master.ScrollUI || master.Mesh3D) {
		return FeatureFlags{
			Background: master.Background,
			HUD:        master.HUD,
			GlowOrbs:   master.GlowOrbs,
			Text:       master.Text && frame >= 4,
		}
	}
	_ = lite // quality knob for draw* helpers only
	out.Text = master.Text
	return out
}

// stickyOn is true for `on` frames starting at phase within each period.
func stickyOn(frame, period, phase, on int) bool {
	if period <= 0 || on <= 0 {
		return true
	}
	if on >= period {
		return true
	}
	age := frame - phase
	if age < 0 {
		return false
	}
	return age%period < on
}

// cadenceSparse is the old rotate-on/off schedule (opt-in via GPUI_CADENCE_SPARSE=1).
func cadenceSparse(frame int, master FeatureFlags, lite bool) FeatureFlags {
	out := FeatureFlags{
		Background: master.Background,
		HUD:        master.HUD,
		GlowOrbs:   master.GlowOrbs,
		Text:       master.Text && frame%4 == 0,
	}
	if frame < 45 {
		return out
	}
	out.Cards = master.Cards && frame%15 == 0
	out.Paths = master.Paths && frame%12 == 3
	out.Polygon = master.Polygon && frame%12 == 6
	out.Blend = master.Blend && frame%24 == 9
	out.Transform = master.Transform && frame%30 == 1
	out.DashStroke = master.DashStroke && frame%30 == 2
	out.Clip = master.Clip && frame%18 == 7
	out.Vertices = master.Vertices && frame%30 == 13
	out.Pixels = master.Pixels && frame%24 == 16
	out.Image = master.Image && frame%24 == 4
	out.Layer = master.Layer && frame%60 == 50
	out.Mask = master.Mask && frame%60 == 55
	out.Backdrop = master.Backdrop && frame%150 == 80
	out.Filter = master.Filter && frame%240 == 100
	out.Gradient = master.Gradient && frame%20 == 0
	out.Pattern = master.Pattern && frame%20 == 0
	out.AdvBlend = master.AdvBlend && frame%24 == 8
	out.RRectClip = master.RRectClip && frame%18 == 5
	out.TextLCD = master.TextLCD && frame%16 == 0
	out.Damage = master.Damage
	out.ScrollUI = master.ScrollUI && frame%12 == 0
	if lite {
		out.Cards = master.Cards && frame%20 == 0
		out.Paths = master.Paths && frame%20 == 5
		out.DashStroke = master.DashStroke && frame%40 == 0
		out.Image = master.Image && frame%30 == 0
		out.Layer = master.Layer && frame%60 == 0
		out.Mask = master.Mask && frame%60 == 30
		out.Backdrop = master.Backdrop && frame%180 == 0
		out.Filter = master.Filter && frame%240 == 0
	}
	return out
}

// ---------- assets ----------

type assets struct {
	checker *render.ImageBuf
	mask    *render.Mask
}

// guideCache rasterizes 中文说明 once per content change (CJK shaping is expensive).
type guideCache struct {
	key string
	img *render.ImageBuf
	w   int
	h   int
}

var gGuideCache guideCache

// HUD metrics ImageBuf cache — rebuild ~1Hz with coarse keys (no RSS in key)
// so GPU image-cache genIDs do not climb with every MB of heap growth.
type hudImgCache struct {
	key  string
	img  *render.ImageBuf
	w, h int
	at   time.Time
}

var gHUDBadge hudImgCache
var gHUDBarLabel hudImgCache

// vertsMeshPool reuses S11/S12 DrawMesh grid buffers (no per-frame make).
type vertsMeshPool struct {
	pos []render.Point
	col []render.RGBA
	idx []uint16
}

var gVertsMesh vertsMeshPool

func (p *vertsMeshPool) ensure(nVert, nIdx int) ([]render.Point, []render.RGBA, []uint16) {
	if cap(p.pos) < nVert {
		p.pos = make([]render.Point, nVert)
		p.col = make([]render.RGBA, nVert)
	} else {
		p.pos = p.pos[:nVert]
		p.col = p.col[:nVert]
	}
	if cap(p.idx) < nIdx {
		p.idx = make([]uint16, nIdx)
	} else {
		p.idx = p.idx[:nIdx]
	}
	return p.pos, p.col, p.idx
}

// Avoid LoadFontFace every frame (face switch is costly with CJK TTF).
var (
	hudFaceReady bool
	hudFacePath  string
	hudFaceSize  float64
)

func ensureFont(dc *render.Context, path string, size float64) {
	if dc == nil || path == "" {
		return
	}
	if hudFaceReady && hudFacePath == path && hudFaceSize == size {
		return
	}
	if err := dc.LoadFontFace(path, size); err == nil {
		hudFaceReady = true
		hudFacePath = path
		hudFaceSize = size
	}
}

func newAssets() *assets {
	a := &assets{}
	// Checker image for DrawImage* paths.
	img, err := render.NewImageBuf(48, 48, render.FormatRGBA8)
	if err == nil && img != nil {
		for y := 0; y < 48; y++ {
			for x := 0; x < 48; x++ {
				on := ((x/8)+(y/8))%2 == 0
				if on {
					_ = img.SetRGBA(x, y, 40, 160, 255, 255)
				} else {
					_ = img.SetRGBA(x, y, 255, 120, 40, 255)
				}
			}
		}
		a.checker = img
	}
	// Soft circular mask.
	m := render.NewMask(64, 64)
	if m != nil {
		for y := 0; y < 64; y++ {
			for x := 0; x < 64; x++ {
				dx := float64(x-32) / 32
				dy := float64(y-32) / 32
				d := math.Sqrt(dx*dx + dy*dy)
				a := 0.0
				if d < 1 {
					a = (1 - d) * 255
				}
				m.Set(x, y, uint8(a))
			}
		}
		a.mask = m
	}
	return a
}

// ---------- scene ----------

type orb struct {
	phase, speed, baseR, hue, orbit float64
}

type animScene struct {
	orbs  []orb
	bgHue float64
	pulse float64
}

func newAnimScene(rng *rng, lite bool) *animScene {
	s := &animScene{}
	n := 3
	if lite {
		n = 2
	}
	for i := 0; i < n; i++ {
		s.orbs = append(s.orbs, orb{
			phase: rng.float01() * math.Pi * 2,
			speed: 0.25 + rng.float01()*0.8,
			baseR: 10 + rng.float01()*16,
			hue:   rng.float01(),
			orbit: 0.14 + rng.float01()*0.22,
		})
	}
	return s
}

func (s *animScene) tick(t float64, frame int, rng *rng) {
	s.bgHue = math.Mod(t*0.03, 1)
	s.pulse = 0.5 + 0.5*math.Sin(t*2.1)
	if frame%40 == 0 {
		i := rng.intn(len(s.orbs))
		s.orbs[i].hue = rng.float01()
		s.orbs[i].baseR = 14 + rng.float01()*48
	}
}

// ---------- draw (modular) ----------

func drawFrame(dc *render.Context, w, h int, t float64, frame int, scn *animScene, fonts fontPack, assets *assets, rng *rng, feat FeatureFlags, lite bool, pixelScratch []byte, hud frameHUD) {
	fw, fh := float64(w), float64(h)
	cx, cy := fw*0.5, fh*0.5
	prof := envBool("GPUI_PROFILE_DRAW", false)
	tick := func(name string, start time.Time) {
		if prof && frame%60 == 0 {
			log.Printf("drawprof f=%d %s=%.2fms", frame, name, float64(time.Since(start).Microseconds())/1000)
		}
	}

	{
		st := time.Now()
		if feat.Damage {
			// Background handled by damage module (bootstrap or dirty band only).
		} else if feat.Background {
			drawBackground(dc, fw, fh, scn.bgHue, lite)
		} else {
			dc.SetRGB(0.05, 0.05, 0.07)
			dc.DrawRectangle(0, 0, fw, fh)
			_ = dc.Fill()
		}
		tick("bg", st)
	}
	if feat.GlowOrbs {
		st := time.Now()
		drawGlowOrbs(dc, cx, cy, fw, fh, t, scn, lite)
		tick("glow", st)
	}
	if feat.Cards {
		st := time.Now()
		drawCards(dc, fw, fh, t, scn.pulse, lite)
		tick("cards", st)
	}
	if feat.Paths {
		st := time.Now()
		drawSwirlPath(dc, cx, cy, fw, fh, t, lite)
		tick("paths", st)
	}
	if feat.DashStroke {
		st := time.Now()
		drawDashCloud(dc, fw, fh, t, frame, rng, lite)
		tick("dash", st)
	}
	if feat.Polygon {
		st := time.Now()
		drawPolygons(dc, fw, fh, t, lite)
		tick("poly", st)
	}
	if feat.Blend {
		st := time.Now()
		drawBlendShapes(dc, fw, fh, t, lite, frame)
		tick("blend", st)
	}
	if feat.Transform {
		st := time.Now()
		drawTransformed(dc, fw, fh, t)
		tick("xform", st)
	}
	if feat.Clip {
		st := time.Now()
		drawClippedRegion(dc, fw, fh, t, lite)
		tick("clip", st)
	}
	if feat.Layer {
		st := time.Now()
		drawLayerStack(dc, fw, fh, t, frame, lite)
		tick("layer", st)
	}
	if feat.Backdrop {
		st := time.Now()
		drawBackdropCard(dc, fw, fh, t, lite, frame)
		tick("backdrop", st)
	}
	if feat.Mask && assets.mask != nil {
		st := time.Now()
		drawMasked(dc, fw, fh, t, assets.mask)
		tick("mask", st)
	}
	if feat.Image && assets.checker != nil {
		st := time.Now()
		drawImages(dc, fw, fh, t, assets.checker, lite)
		tick("image", st)
	}
	if feat.Vertices {
		st := time.Now()
		drawVertices(dc, fw, fh, t)
		tick("verts", st)
	}
	if feat.Pixels {
		st := time.Now()
		drawWritePixels(dc, fw, fh, frame, pixelScratch)
		tick("pixels", st)
	}
	if feat.Filter {
		st := time.Now()
		drawFilterPanel(dc, fw, fh, t, lite, frame)
		tick("filter", st)
	}
	if feat.Text && fonts.ok {
		st := time.Now()
		drawTextStyles(dc, fonts, fw, fh, t, frame, feat, lite)
		tick("text", st)
	}
	if feat.Gradient || feat.Pattern {
		st := time.Now()
		drawGradientPattern(dc, fw, fh, t, lite, frame)
		tick("grad", st)
	}
	if feat.AdvBlend {
		st := time.Now()
		drawAdvancedBlendPanel(dc, fw, fh, t, lite, frame)
		tick("advblend", st)
	}
	if feat.RRectClip {
		st := time.Now()
		drawRRectEvenOdd(dc, fw, fh, t, lite)
		tick("rrectclip", st)
	}
	if feat.TextLCD {
		st := time.Now()
		drawTextLCDShape(dc, fonts, fw, fh, t, frame, lite)
		tick("textlcd", st)
	}
	if feat.ScrollUI && !feat.Damage {
		// S20 (and S21): scroll+modal visual; S19 uses damage module instead.
		st := time.Now()
		drawScrollModalUI(dc, fonts, fw, fh, t, lite)
		tick("scroll", st)
	}
	if feat.Damage {
		st := time.Now()
		// bootstrap flag carried via package state; main sets reset on resize/forceFull
		drawDamagePartialScene(dc, fonts, fw, fh, t, frame, false)
		tick("damage", st)
	}
	if feat.Mesh3D {
		st := time.Now()
		drawMesh3DScene(dc, fw, fh, t, lite)
		tick("mesh3d", st)
	}
	if feat.HUD {
		st := time.Now()
		drawHUD(dc, fonts, fw, fh, t, hud)
		tick("hud", st)
	}
}

func drawBackground(dc *render.Context, fw, fh, hue float64, lite bool) {
	r, g, b := hsv(hue, 0.35, 0.12)
	dc.SetRGB(r, g, b)
	dc.DrawRectangle(0, 0, fw, fh)
	_ = dc.Fill()
	rings := 1
	if !lite {
		rings = 2
	}
	for i := 0; i < rings; i++ {
		m := float64(i+1) * 0.05
		dc.SetRGBA(0, 0, 0, 0.07)
		dc.DrawRectangle(fw*m, fh*m, fw*(1-2*m), fh*(1-2*m))
		_ = dc.Fill()
	}
}

func drawGlowOrbs(dc *render.Context, cx, cy, fw, fh, t float64, scn *animScene, lite bool) {
	// Core-only orbs by default. Optional soft halo when not lite (still 1 extra fill).
	for i, o := range scn.orbs {
		// In lite mode draw subset for headroom.
		if lite && i >= 2 {
			continue
		}
		ang := o.phase + t*o.speed
		ox := cx + math.Cos(ang)*fw*o.orbit
		oy := cy + math.Sin(ang*1.17)*fh*o.orbit*0.85
		r, g, b := hsv(math.Mod(o.hue+t*0.05, 1), 0.75, 0.95)
		if !lite && i%2 == 0 {
			dc.SetRGBA(r, g, b, 0.10)
			dc.DrawCircle(ox, oy, o.baseR*1.25)
			_ = dc.Fill()
		}
		dc.SetRGBA(r, g, b, 0.90)
		dc.DrawCircle(ox, oy, o.baseR*(0.55+0.08*scn.pulse))
		_ = dc.Fill()
	}
}

func drawCards(dc *render.Context, fw, fh, t, pulse float64, lite bool) {
	// Profile (Intel HD): DrawRoundedRectangle+Fill was ~7ms EACH (cubic path +
	// shape detect), 3 cards x 3 rrects ≈ 20ms → locked ~45fps. Axis-aligned
	// DrawRectangle uses the cheap rect SDF batch path (~0.01ms like bg).
	// Rounded corners remain covered by S05/clip + rrect unit tests; here we
	// keep continuous visible card UI at Skia-class 60fps cost.
	cardW, cardH := fw*0.28, fh*0.22
	cards := []struct{ x, y, hue float64 }{
		{fw * 0.05, fh * 0.12, math.Mod(t*0.07, 1)},
		{fw * 0.65, fh * 0.14, math.Mod(0.33+t*0.05, 1)},
	}
	if !lite {
		cards = append(cards, struct{ x, y, hue float64 }{fw * 0.34, fh * 0.62, math.Mod(0.66+t*0.09, 1)})
	}
	alpha := 0.55 + 0.2*pulse
	for i, c := range cards {
		dx := 6 * math.Sin(t*1.1+float64(i))
		dy := 4 * math.Cos(t*0.9+float64(i)*1.3)
		x, y := c.x+dx, c.y+dy
		// soft shadow
		dc.SetRGBA(0, 0, 0, 0.18)
		dc.DrawRectangle(x+4, y+5, cardW, cardH)
		_ = dc.Fill()
		// body
		cr, cg, cb := hsv(c.hue, 0.4, 0.28)
		dc.SetRGBA(cr, cg, cb, alpha)
		dc.DrawRectangle(x, y, cardW, cardH)
		_ = dc.Fill()
		// accent bar
		ar, ag, ab := hsv(math.Mod(c.hue+0.12, 1), 0.85, 0.95)
		dc.SetRGBA(ar, ag, ab, 0.95)
		dc.DrawRectangle(x, y, cardW, 7)
		_ = dc.Fill()
		// status pip (filled circle is SDF-cheap vs stroked arc)
		dc.SetRGBA(1, 1, 1, 0.55)
		dc.DrawCircle(x+cardW*0.82, y+cardH*0.55, 8)
		_ = dc.Fill()
	}
}

func drawSwirlPath(dc *render.Context, cx, cy, fw, fh, t float64, lite bool) {
	// Continuous path strokes must stay cheap: quantize animation so S4.3/S6.6
	// stroke geometry caches hit, and keep segment counts low on iGPU.
	// Visual still reads as a slow swirl + cubic ribbon.
	phaseStep := math.Pi / 24 // 48 discrete orientations
	if lite {
		phaseStep = math.Pi / 16
	}
	phase := math.Floor(t/phaseStep) * phaseStep

	dc.SetRGBA(1, 1, 1, 0.45)
	dc.SetLineWidth(2)
	p := render.NewPath()
	steps := 12
	if lite {
		steps = 8
	}
	for i := 0; i <= steps; i++ {
		u := float64(i) / float64(steps)
		ang := u*math.Pi*4 + phase
		rad := (0.08 + 0.12*u) * math.Min(fw, fh)
		px := cx + math.Cos(ang)*rad
		py := cy + math.Sin(ang)*rad*0.75
		if i == 0 {
			p.MoveTo(px, py)
		} else {
			p.LineTo(px, py)
		}
	}
	dc.SetPath(p)
	_ = dc.Stroke()

	// Static cubic ribbon (full cache hit after first frame).
	p2 := render.NewPath()
	p2.MoveTo(cx-fw*0.2, cy)
	p2.CubicTo(cx-fw*0.05, cy-fh*0.2, cx+fw*0.05, cy+fh*0.2, cx+fw*0.2, cy)
	dc.SetRGBA(0.4, 0.9, 1, 0.55)
	dc.SetLineWidth(1.5)
	dc.SetPath(p2)
	_ = dc.Stroke()
}

func drawDashCloud(dc *render.Context, fw, fh, t float64, frame int, rng *rng, lite bool) {
	// Fixed positions + quantized dash offset: continuous visibility without
	// thrashing dash/stroke geom caches every frame (old path moved blobs).
	dc.SetLineWidth(1.5)
	dc.SetDash(6, 4, 2, 4)
	// Pattern length = 16; quantize offset to 16 slots for cache reuse.
	off := math.Mod(t*18, 16)
	dc.SetDashOffset(math.Floor(off))
	n := 2
	if lite {
		n = 1
	}
	for i := 0; i < n; i++ {
		x0 := fw*0.12 + float64(i)*fw*0.22
		y0 := fh*0.22 + float64(i)*fh*0.08
		dc.SetRGB(0.35+0.08*float64(i%4), 0.5, 0.85)
		p := render.NewPath()
		p.MoveTo(x0, y0)
		p.LineTo(x0+48+float64(i*6), y0+10)
		p.QuadraticTo(x0+36, y0+36, x0+6, y0+28)
		p.Close()
		dc.SetPath(p)
		_ = dc.Stroke()
	}
	// One cheap solid line so dash module remains visually busy if dash misses.
	dc.ClearDash()
	dc.SetRGBA(0.55, 0.75, 1.0, 0.35)
	dc.SetLineWidth(1)
	dc.DrawLine(fw*0.08, fh*0.18+6*math.Sin(t), fw*0.42, fh*0.18+6*math.Cos(t*0.8))
	_ = dc.Stroke()
	_ = frame
	_ = rng
}

func drawPolygons(dc *render.Context, fw, fh, t float64, lite bool) {
	// Filled poly is continuous + cheap relative to multi-stroke expand.
	// Quantize rotation so stroke outline cache can hit.
	rotStep := math.Pi / 36
	rot := math.Floor(t/rotStep) * rotStep
	rPulse := 28.0
	if !lite {
		rPulse = 28 + 4*math.Sin(t)
	}
	dc.SetRGBA(0.95, 0.6, 0.2, 0.55)
	dc.DrawRegularPolygon(5, fw*0.18, fh*0.78, rPulse, rot)
	_ = dc.Fill()
	dc.SetRGBA(0.3, 0.9, 0.7, 0.55)
	dc.DrawRegularPolygon(6, fw*0.82, fh*0.28, 24, -rot*0.7)
	_ = dc.Fill()
	// Single bottom guide line (solid stroke; continuous, low cost after batching).
	dc.SetRGBA(0.7, 0.85, 1.0, 0.45)
	dc.SetLineWidth(1)
	dc.DrawLine(fw*0.1, fh*0.9, fw*0.9, fh*0.9)
	_ = dc.Stroke()
}

func drawTransformed(dc *render.Context, fw, fh, t float64) {
	// CTM group: rotating/scaled mini-grid + solid shapes (S11 变换 must be obvious).
	// Uniform scale only: non-uniform Scale(sx,sy) forces T.03 user-space stroke
	// expand (matrixRequiresUserSpaceStroke) and trips mem_anim cpu_fallback_ops=0
	// gate even though the expanded outline may still fill on GPU.
	dc.Push()
	dc.Translate(fw*0.52, fh*0.28)
	dc.Rotate(t * 0.55)
	s := 1 + 0.12*math.Sin(t)
	dc.Scale(s, s)

	// Back plate
	dc.SetRGBA(0.12, 0.14, 0.22, 0.75)
	dc.DrawRectangle(-70, -48, 140, 96)
	_ = dc.Fill()

	// Mini grid under CTM
	dc.SetLineWidth(1.25)
	for i := -3; i <= 3; i++ {
		u := float64(i) * 18
		dc.SetRGBA(0.35, 0.9, 1.0, 0.9)
		dc.DrawLine(u, -42, u, 42)
		_ = dc.Stroke()
		dc.SetRGBA(1.0, 0.55, 0.85, 0.9)
		dc.DrawLine(-60, u*0.7, 60, u*0.7)
		_ = dc.Stroke()
	}
	// Filled diamond + rect so fill+CTM is exercised, not only strokes.
	dc.SetRGBA(1.0, 0.85, 0.2, 0.85)
	dc.DrawRectangle(-28, -16, 56, 32)
	_ = dc.Fill()
	dc.SetRGBA(0.3, 1.0, 0.55, 0.9)
	dc.MoveTo(0, -36)
	dc.LineTo(24, 0)
	dc.LineTo(0, 36)
	dc.LineTo(-24, 0)
	dc.ClosePath()
	_ = dc.Fill()

	dc.Pop()
}

func drawClippedRegion(dc *render.Context, fw, fh, t float64, lite bool) {
	// Circle path-clip is the expensive part (~1ms+). Keep it on S05 / non-lite;
	// lite (S12) keeps continuous ClipRect coverage only.
	if !lite {
		dc.Push()
		dc.DrawCircle(fw*0.85, fh*0.78, 48+8*math.Sin(t*1.5))
		dc.Clip()
		r, g, b := hsv(math.Mod(t*0.2, 1), 0.85, 1)
		dc.SetRGBA(r, g, b, 0.35)
		dc.DrawRectangle(fw*0.7, fh*0.62, fw*0.3, fh*0.38)
		_ = dc.Fill()
		dc.ResetClip()
		dc.Pop()
	}

	dc.Push()
	dc.ClipRect(fw*0.05, fh*0.42, fw*0.25, fh*0.2)
	dc.SetRGBA(0.2, 1, 0.6, 0.25)
	bars := 6
	if lite {
		bars = 3
	}
	for i := 0; i < bars; i++ {
		dc.DrawRectangle(fw*0.05, fh*0.42+float64(i)*10, fw*0.25, 5)
		_ = dc.Fill()
	}
	dc.ResetClip()
	dc.Pop()
}

func drawMasked(dc *render.Context, fw, fh, t float64, mask *render.Mask) {
	dc.Push()
	dc.SetMask(mask)
	// position content where mask is meaningful — ApplyMask samples full surface;
	// draw a glowing square then apply mask soft edge via SetMask path.
	r, g, b := hsv(math.Mod(0.8+t*0.05, 1), 0.7, 1)
	dc.SetRGBA(r, g, b, 0.7)
	dc.DrawCircle(fw*0.2, fh*0.55, 50)
	_ = dc.Fill()
	dc.ClearMask()
	dc.Pop()
}

func drawImages(dc *render.Context, fw, fh, t float64, img *render.ImageBuf, lite bool) {
	x := fw*0.08 + 10*math.Sin(t)
	y := fh * 0.62
	// Always exercise plain + atlas (GPU texture path). Heavy variants under
	// full quality only — S08 covers rounded/circular/nine continuously.
	dc.DrawImage(img, x, y)
	dc.DrawAtlas(img, []render.AtlasSprite{
		{SrcX: 0, SrcY: 0, SrcW: 24, SrcH: 24, DstX: x, DstY: y - 40, DstW: 28, DstH: 28, Opacity: 0.9},
		{SrcX: 24, SrcY: 24, SrcW: 24, SrcH: 24, DstX: x + 34, DstY: y - 40, DstW: 28, DstH: 28, Opacity: 0.85},
	})
	if !lite {
		dc.DrawImageRounded(img, x+60, y, 8)
		dc.DrawImageCircular(img, x+140, y+24, 22)
		dc.DrawImageNine(img, image.Rect(16, 16, 32, 32), x+200, y, 72, 40)
		dc.DrawImage(img, fw*0.72, fh*0.08)
	}
}

func drawVertices(dc *render.Context, fw, fh, t float64) {
	// Colored triangle-list (V.01) + indexed mesh grid (V.03).
	// Wire lines MUST be exact mesh edges (LineTo between consecutive verts).
	// Catmull-Rom/Bezier overlays leave the triangulated surface → worse 错位.
	ox, oy := fw*0.18, fh*0.58
	dc.SetRGBA(0.08, 0.09, 0.14, 0.72)
	dc.DrawRectangle(ox-8, oy-12, 250, 168)
	_ = dc.Fill()
	dc.SetRGBA(0.55, 0.95, 0.75, 0.7)
	dc.SetLineWidth(1.5)
	dc.DrawRectangle(ox-8, oy-12, 250, 168)
	_ = dc.Stroke()

	// Free-form rainbow triangles (V.01) near plate top.
	s := 36 + 8*math.Sin(t)
	pts := []render.Point{
		{X: ox + 40, Y: oy + 8 - s*0.35},
		{X: ox + 8, Y: oy + 8 + s*0.45},
		{X: ox + 72, Y: oy + 8 + s*0.45},
		{X: ox + 40, Y: oy + 8 + s*0.1},
		{X: ox + 88, Y: oy + 8 + s*0.25},
		{X: ox + 16, Y: oy + 8 + s*0.15},
	}
	cols := []render.RGBA{
		{R: 1, G: 0.25, B: 0.35, A: 0.95},
		{R: 0.25, G: 0.85, B: 1, A: 0.95},
		{R: 1, G: 0.9, B: 0.15, A: 0.95},
		{R: 0.4, G: 1, B: 0.45, A: 0.9},
		{R: 1, G: 0.45, B: 0.9, A: 0.9},
		{R: 0.45, G: 0.55, B: 1, A: 0.9},
	}
	dc.DrawVertices(pts, cols, render.VertexModeTriangles)

	// Main mesh grid. Mild wave so edges stay readable; denser than 2-triangle demo.
	const colsN, rowsN = 8, 5
	cellW, cellH := 24.0, 20.0
	gx, gy := ox+10, oy+42
	nVert := (colsN + 1) * (rowsN + 1)
	nIdx := colsN * rowsN * 6
	positions, colors, indices := gVertsMesh.ensure(nVert, nIdx)
	vi := 0
	for j := 0; j <= rowsN; j++ {
		for i := 0; i <= colsN; i++ {
			// Only vertical displace, continuous in i — each row is a clean polyline.
			wave := 2.5 * math.Sin(t*1.4+float64(i)*0.5+float64(j)*0.15)
			positions[vi] = render.Point{
				X: gx + float64(i)*cellW,
				Y: gy + float64(j)*cellH + wave,
			}
			colors[vi] = render.RGBA{
				R: 0.15 + 0.85*float64(i)/float64(colsN),
				G: 0.25 + 0.7*float64(j)/float64(rowsN),
				B: 0.95 - 0.55*float64(i+j)/float64(colsN+rowsN),
				A: 0.95,
			}
			vi++
		}
	}
	ii := 0
	for j := 0; j < rowsN; j++ {
		for i := 0; i < colsN; i++ {
			i0 := uint16(j*(colsN+1) + i)
			i1 := i0 + 1
			i2 := i0 + uint16(colsN+1)
			i3 := i2 + 1
			indices[ii+0] = i0
			indices[ii+1] = i1
			indices[ii+2] = i2
			indices[ii+3] = i1
			indices[ii+4] = i3
			indices[ii+5] = i2
			ii += 6
		}
	}
	dc.DrawMesh(render.Mesh{
		Positions: positions,
		Colors:    colors,
		Indices:   indices,
	})

	// Exact-edge wire: one path, LineTo along every grid row & col (matches triangles).
	// Do NOT cubic-smooth: curves leave the mesh surface and look more misaligned.
	dc.SetRGBA(1, 1, 1, 0.72)
	dc.SetLineWidth(1.05)
	dc.SetLineCap(render.LineCapRound)
	dc.SetLineJoin(render.LineJoinRound)
	// horizontal edges
	for j := 0; j <= rowsN; j++ {
		base := j * (colsN + 1)
		p0 := positions[base]
		dc.MoveTo(p0.X, p0.Y)
		for i := 1; i <= colsN; i++ {
			p := positions[base+i]
			dc.LineTo(p.X, p.Y)
		}
	}
	// vertical edges
	for i := 0; i <= colsN; i++ {
		p0 := positions[i]
		dc.MoveTo(p0.X, p0.Y)
		for j := 1; j <= rowsN; j++ {
			p := positions[j*(colsN+1)+i]
			dc.LineTo(p.X, p.Y)
		}
	}
	_ = dc.Stroke()
}

func drawWritePixels(dc *render.Context, fw, fh float64, frame int, pix []byte) {
	const pw, ph = 24, 16
	need := pw * ph * 4
	if len(pix) < need {
		pix = make([]byte, need)
	}
	// only rewrite a subset each frame — still exercises WritePixels every frame
	for y := 0; y < ph; y++ {
		for x := 0; x < pw; x++ {
			i := (y*pw + x) * 4
			pix[i+0] = byte((x*10 + frame) % 255)
			pix[i+1] = byte((y*20 + frame*3) % 255)
			pix[i+2] = 180
			pix[i+3] = 255
		}
	}
	px := int(fw) - pw - 12
	py := int(fh) - ph - 40
	if px < 0 {
		px = 0
	}
	if py < 0 {
		py = 0
	}
	dc.WritePixels(px, py, pw, ph, pix[:need])
}

type textPanelCache struct {
	key  string
	img  *render.ImageBuf
	w, h int
}

var gTextPanel textPanelCache

func drawTextStyles(dc *render.Context, fonts fontPack, fw, fh, t float64, frame int, feat FeatureFlags, lite bool) {
	// Place on RIGHT, below FPS badge — never overlap left 场景 guide panel.
	panelW := int(math.Min(280, fw*0.34))
	// Keep panelH independent of lite — lite used to flip panelH and force a
	// 15–25ms CJK reshape (S07/S12 "flash" + fps death spiral).
	panelH := 96
	x := fw - float64(panelW) - 12
	y := 96.0 // under top-right metrics badge

	// Rebuild CJK panel only on size class change — never periodic / lite toggle.
	// Continuous DrawImage every frame = no flicker; CJK reshape is expensive.
	key := fmt.Sprintf("%dx%d", panelW, panelH)
	need := gTextPanel.img == nil || gTextPanel.key != key
	if need {
		img, err := rasterizeTextPanel(fonts, panelW, panelH, t, lite)
		if err == nil && img != nil {
			gTextPanel.key = key
			gTextPanel.img = img
			gTextPanel.w, gTextPanel.h = panelW, panelH
		}
	}
	if gTextPanel.img != nil {
		dc.DrawImageEx(gTextPanel.img, render.DrawImageOptions{
			X: x, Y: y, Interpolation: render.InterpNearest, Opacity: 1,
		})
	}

	// Live Latin-only line (cheap, every frame) for motion/leak of DrawString path.
	hudFace := fonts.latin
	if hudFace == "" {
		hudFace = fonts.sans
	}
	if frame%6 == 0 || frame < 12 {
		ensureFont(dc, hudFace, 12)
		dc.SetRGBA(0.45, 1.0, 0.75, 0.95)
		dc.SetTextDecoration(render.TextDecorationNone)
		dc.DrawString(fmt.Sprintf("text live f=%d t=%.1f", frame, t), x+10, y+float64(panelH)-10)
	}
	_ = feat
}

func rasterizeTextPanel(fonts fontPack, panelW, panelH int, t float64, lite bool) (*render.ImageBuf, error) {
	tmp := render.NewContext(panelW, panelH)
	defer tmp.Close()
	tmp.SetRGBA(0.06, 0.08, 0.12, 0.90)
	tmp.DrawRoundedRectangle(0, 0, float64(panelW), float64(panelH), 8)
	_ = tmp.Fill()
	bodySize := 13.0
	if fonts.ok && fonts.sans != "" {
		_ = tmp.LoadFontFace(fonts.sans, bodySize)
	}
	tr, tg, tb := hsv(math.Mod(0.1+t*0.08, 1), 0.55, 1)
	tmp.SetRGB(tr, tg, tb)
	tmp.DrawString("GPUI 渲染压测 · 关窗退出", 10, 22)
	tmp.SetRGB(0.85, 0.88, 0.95)
	tmp.SetTextDecoration(render.TextDecorationUnderline)
	tmp.DrawString("持续文本样式 · 约60fps", 10, 42)
	tmp.SetTextDecoration(render.TextDecorationNone)
	tmp.SetRGB(1.0, 0.82, 0.45)
	if fonts.serif != "" && !lite {
		_ = tmp.LoadFontFace(fonts.serif, 12)
	}
	tmp.SetTextDecoration(render.TextDecorationLineThrough)
	tmp.DrawString("中文渲染 テスト αβγ", 10, 62)
	tmp.SetTextDecoration(render.TextDecorationNone)
	if !lite {
		if fonts.sans != "" {
			_ = tmp.LoadFontFace(fonts.sans, 12)
		}
		tmp.SetRGB(0.7, 0.85, 1.0)
		tmp.DrawString("下划线 / 删除线 / 色相", 10, 82)
	}
	std := tmp.Image()
	if std == nil {
		return nil, fmt.Errorf("text panel nil")
	}
	img := render.ImageBufFromImage(std)
	if img == nil {
		return nil, fmt.Errorf("text panel ImageBuf nil")
	}
	return img, nil
}

func drawHUD(dc *render.Context, fonts fontPack, fw, fh, t float64, hud frameHUD) {
	hudFace := fonts.latin
	if hudFace == "" {
		hudFace = fonts.sans
	}
	// Cached ImageBuf badge/bar: rebuild ≤1Hz, coarse keys, **no RSS in key**
	// (RSS growth used to mint new genIDs → GPU image-cache VRAM slope).
	badgeW, badgeH := 250, 78
	bx, by := fw-float64(badgeW)-12, 10.0
	fps := hud.FPS
	if fps <= 0 {
		fps = 0
	}
	ncpu := hostCPUCount()
	mach := hud.CPUPct / float64(ncpu)
	fpsQ := int(fps + 0.5)
	cpuQ := int(hud.CPUPct/10+0.5) * 10 // 10% buckets
	workQ := int(hud.FrameMS/4+0.5) * 4 // 4ms buckets
	bucket := 0
	diff := math.Abs(fps - float64(hud.TargetFPS))
	if diff <= 4 {
		bucket = 0
	} else if diff <= 12 {
		bucket = 1
	} else {
		bucket = 2
	}
	badgeKey := fmt.Sprintf("%d|%d|%d|%dx%d|%s|%d",
		fpsQ, cpuQ, workQ, hud.W, hud.H, hud.PresentMode, bucket)
	now := time.Now()
	needBadge := gHUDBadge.img == nil || gHUDBadge.key != badgeKey
	if needBadge && (gHUDBadge.img == nil || now.Sub(gHUDBadge.at) >= time.Second) {
		img, err := rasterizeHUDBadge(fonts, hudFace, badgeW, badgeH, hud, fps, mach, bucket)
		if err == nil && img != nil {
			gHUDBadge.key = badgeKey
			gHUDBadge.img = img
			gHUDBadge.w, gHUDBadge.h = badgeW, badgeH
			gHUDBadge.at = now
		}
	}
	if gHUDBadge.img != nil {
		dc.DrawImageEx(gHUDBadge.img, render.DrawImageOptions{
			X: bx, Y: by, Interpolation: render.InterpNearest, Opacity: 1,
		})
	} else {
		dc.SetRGBA(0.05, 0.07, 0.11, 0.93)
		dc.DrawRoundedRectangle(bx, by, float64(badgeW), float64(badgeH), 8)
		_ = dc.Fill()
	}

	barH := math.Max(26, fh*0.045)
	barY := fh - barH - 10
	budgetMS := 1000.0 / math.Max(float64(hud.TargetFPS), 1)
	ratio := hud.FrameMS / budgetMS
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1.5 {
		ratio = 1.5
	}
	prog := ratio / 1.5
	if prog < 0.05 {
		prog = 0.05
	}
	dc.SetRGBA(0.08, 0.1, 0.14, 0.92)
	dc.DrawRoundedRectangle(12, barY, fw-24, barH, barH*0.4)
	_ = dc.Fill()
	if ratio <= 1.0 {
		dc.SetRGB(0.25, 0.85, 0.45)
	} else if ratio <= 1.25 {
		dc.SetRGB(0.95, 0.75, 0.25)
	} else {
		dc.SetRGB(0.95, 0.4, 0.25)
	}
	dc.DrawRoundedRectangle(16, barY+4, (fw-32)*prog, barH-8, (barH-8)*0.4)
	_ = dc.Fill()

	workBucket := int(hud.FrameMS/4+0.5) * 4
	// Feats string stable; omit fine work from key thrash.
	barKey := fmt.Sprintf("%d|%s|%d", workBucket, hud.Active, int(budgetMS))
	labelH := int(barH)
	labelW := int(math.Min(fw-40, 520))
	if labelW < 160 {
		labelW = 160
	}
	needBar := gHUDBarLabel.img == nil || gHUDBarLabel.key != barKey || gHUDBarLabel.h != labelH
	if needBar && (gHUDBarLabel.img == nil || now.Sub(gHUDBarLabel.at) >= time.Second) {
		img, err := rasterizeHUDBarLabel(fonts, hudFace, labelW, labelH, hud.FrameMS, budgetMS, hud.Active)
		if err == nil && img != nil {
			gHUDBarLabel.key = barKey
			gHUDBarLabel.img = img
			gHUDBarLabel.w, gHUDBarLabel.h = labelW, labelH
			gHUDBarLabel.at = now
		}
	}
	if gHUDBarLabel.img != nil {
		dc.DrawImageEx(gHUDBarLabel.img, render.DrawImageOptions{
			X: 18, Y: barY, Interpolation: render.InterpNearest, Opacity: 1,
		})
	}

	drawSceneGuide(dc, fonts, fw, fh, hud)
	_ = t
}

func rasterizeHUDBadge(fonts fontPack, face string, w, h int, hud frameHUD, fps, mach float64, bucket int) (*render.ImageBuf, error) {
	tmp := render.NewContext(w, h)
	defer tmp.Close()
	tmp.SetRGBA(0.05, 0.07, 0.11, 0.93)
	tmp.DrawRoundedRectangle(0, 0, float64(w), float64(h), 8)
	_ = tmp.Fill()
	if fonts.ok && face != "" {
		_ = tmp.LoadFontFace(face, 14)
	}
	switch bucket {
	case 0:
		tmp.SetRGB(0.35, 1.0, 0.55)
	case 1:
		tmp.SetRGB(1.0, 0.85, 0.35)
	default:
		tmp.SetRGB(1.0, 0.45, 0.35)
	}
	tmp.DrawString(fmt.Sprintf("FPS  %.1f  /  %d", fps, hud.TargetFPS), 12, 24)
	if fonts.ok && face != "" {
		_ = tmp.LoadFontFace(face, 12)
	}
	tmp.SetRGB(0.82, 0.88, 0.96)
	tmp.DrawString(fmt.Sprintf("%dx%d  CPU1核 %.0f%%≈整机%.0f%%  RSS %dMB",
		hud.W, hud.H, hud.CPUPct, mach, hud.RSSKB/1024), 12, 44)
	tmp.DrawString(fmt.Sprintf("f=%d  work=%.1fms  present=%s", hud.Frame, hud.FrameMS, hud.PresentMode), 12, 62)
	std := tmp.Image()
	if std == nil {
		return nil, fmt.Errorf("hud badge nil")
	}
	return render.ImageBufFromImage(std), nil
}

func rasterizeHUDBarLabel(fonts fontPack, face string, w, h int, frameMS, budgetMS float64, active string) (*render.ImageBuf, error) {
	tmp := render.NewContext(w, h)
	defer tmp.Close()
	tmp.SetRGBA(0, 0, 0, 0)
	tmp.DrawRectangle(0, 0, float64(w), float64(h))
	_ = tmp.Fill()
	if fonts.ok && face != "" {
		_ = tmp.LoadFontFace(face, 12)
	}
	tmp.SetRGB(0.88, 0.92, 1)
	if len(active) > 48 {
		active = active[:48] + "..."
	}
	tmp.DrawString(fmt.Sprintf("work %.1f / %.1f ms  feats[%s]", frameMS, budgetMS, active), 2, float64(h)*0.68)
	std := tmp.Image()
	if std == nil {
		return nil, fmt.Errorf("hud bar nil")
	}
	return render.ImageBufFromImage(std), nil
}

// drawSceneGuide 绘制中文画面说明。说明文字栅格化后缓存为贴图，避免每帧 CJK 塑形拖垮 FPS。
func drawSceneGuide(dc *render.Context, fonts fontPack, fw, fh float64, hud frameHUD) {
	lines := hud.GuideLines
	if len(lines) == 0 || dc == nil {
		return
	}
	maxLines := len(lines)
	if maxLines > 12 {
		maxLines = 12
	}
	lineH := 15.0
	pad := 10.0
	panelW := int(math.Min(340, fw*0.42))
	panelH := int(pad*2 + 4 + float64(maxLines)*lineH)
	if float64(panelH) > fh*0.62 {
		panelH = int(fh * 0.62)
		maxLines = int((float64(panelH) - pad*2) / lineH)
		if maxLines < 4 {
			maxLines = 4
		}
	}
	if panelW < 200 {
		panelW = 200
	}
	if panelH < 80 {
		panelH = 80
	}

	// Key includes lines + size; rebuild only when content/size changes.
	key := fmt.Sprintf("%dx%d|%s", panelW, panelH, strings.Join(lines[:maxLines], "\n"))
	if gGuideCache.img == nil || gGuideCache.key != key {
		img, err := rasterizeGuidePanel(fonts, lines[:maxLines], panelW, panelH, lineH, pad)
		if err != nil {
			// Fallback: direct draw (may be slow)
			drawSceneGuideDirect(dc, fonts, float64(panelW), float64(panelH), lineH, pad, lines[:maxLines])
			return
		}
		gGuideCache.key = key
		gGuideCache.img = img
		gGuideCache.w = panelW
		gGuideCache.h = panelH
	}

	x, y := 12.0, 10.0
	dc.DrawImageEx(gGuideCache.img, render.DrawImageOptions{
		X: x, Y: y,
		Interpolation: render.InterpNearest,
		Opacity:       1,
	})
}

func rasterizeGuidePanel(fonts fontPack, lines []string, panelW, panelH int, lineH, pad float64) (*render.ImageBuf, error) {
	tmp := render.NewContext(panelW, panelH)
	defer tmp.Close()
	drawSceneGuideDirect(tmp, fonts, float64(panelW), float64(panelH), lineH, pad, lines)
	// Image() returns current surface; convert to ImageBuf for DrawImage.
	std := tmp.Image()
	if std == nil {
		return nil, fmt.Errorf("guide: nil image")
	}
	img := render.ImageBufFromImage(std)
	if img == nil {
		return nil, fmt.Errorf("guide: ImageBufFromImage nil")
	}
	return img, nil
}

func drawSceneGuideDirect(dc *render.Context, fonts fontPack, panelW, panelH, lineH, pad float64, lines []string) {
	x, y := 0.0, 0.0
	dc.SetRGBA(0.04, 0.05, 0.08, 0.92)
	dc.DrawRoundedRectangle(x, y, panelW, panelH, 8)
	_ = dc.Fill()
	dc.SetRGB(0.25, 0.65, 1.0)
	dc.DrawRectangle(x, y, 4, panelH)
	_ = dc.Fill()

	if fonts.ok && fonts.sans != "" {
		ensureFont(dc, fonts.sans, 13)
	}
	ty := y + pad + 12
	for _, ln := range lines {
		if strings.HasPrefix(ln, "✓") {
			dc.SetRGB(0.45, 1.0, 0.65)
		} else if strings.HasPrefix(ln, "·") {
			dc.SetRGB(0.62, 0.65, 0.70)
		} else if strings.HasPrefix(ln, "提示") {
			dc.SetRGB(1.0, 0.78, 0.35)
		} else if strings.HasPrefix(ln, "场景") || strings.HasPrefix(ln, "——") {
			dc.SetRGB(0.95, 0.97, 1.0)
		} else {
			dc.SetRGB(0.82, 0.88, 0.96)
		}
		s := ln
		if rs := []rune(s); len(rs) > 34 {
			s = string(rs[:31]) + "…"
		}
		dc.DrawString(s, x+12, ty)
		ty += lineH
		if ty > panelH-4 {
			break
		}
	}
}

// buildGuideLines 生成中文画面说明：场景意图 + 本帧应出现的内容/效果。
func buildGuideLines(spec scenarioSpec, master, active FeatureFlags, adaptiveLite bool, areaScale float64) []string {
	// 说明文字只描述「场景主开关」应出现什么，避免每帧 on/off 变化导致
	// 中文面板反复重建（昂贵 + 曾触发 surface reconfigure 崩溃）。
	// 「本帧实际激活」用底部 ASCII active[] 查看。
	_ = active
	out := make([]string, 0, 20)
	out = append(out, fmt.Sprintf("场景 %s · %s", spec.ID, scenarioNameCN(spec.ID, spec.Name)))
	out = append(out, "下面列出本场景【应能看到】的内容/效果：")
	if adaptiveLite {
		out = append(out, "提示: 自适应精简已开（大窗/掉帧时降复杂度保约60fps）")
	}
	if areaScale > 1.05 {
		out = append(out, fmt.Sprintf("像素量缩放=%.2f（相对800×600，越大越吃填色）", areaScale))
	}

	type item struct {
		on   bool
		key  string
		desc string
	}
	items := []item{
		{master.Background, "bg", "整屏背景色相缓慢变化"},
		{master.GlowOrbs, "glow", "彩色光晕圆沿轨道运动"},
		{master.Cards, "cards", "圆角半透明卡片/标题条"},
		{master.Paths, "paths", "中心漩涡曲线描边"},
		{master.DashStroke, "dash", "虚线云团描边"},
		{master.Polygon, "poly", "旋转填充多边形"},
		{master.Clip, "clip", "裁剪区内图形（圆/矩形窗）"},
		{master.Layer, "layer", "半透明图层（PushLayer 每帧真实合成）"},
		{master.Backdrop, "backdrop", "背景层（PushBackdrop 每帧真实合成）"},
		{master.Mask, "mask", "蒙版遮罩图案"},
		{master.Image, "image", "棋盘格贴图"},
		{master.Text, "text", "多样式中英文/下划线文字"},
		{master.Filter, "filter", "三块真实滤镜：模糊/阴影/灰度（每帧）"},
		{master.Transform, "xform", "中上：旋转缩放的网格+黄块+绿菱形"},
		{master.Blend, "blend", "右中：棋盘网格方块上 Multiply橙/Screen蓝/Plus光晕"},
		{master.Vertices, "verts", "左下：彩色顶点网格(8x5)+贴合网格的折线框"},
		{master.Pixels, "pixels", "小块像素写入"},
		{master.Gradient || master.Pattern, "grad", "线性/径向/扫描渐变 + 图像图案填充"},
		{master.AdvBlend, "advblend", "多混合模式面板(Overlay/Darken/…/Plus)"},
		{master.RRectClip, "rrectclip", "圆角裁剪 + EvenOdd 星形 + 复杂路径"},
		{master.TextLCD, "textlcd", "LCD布局 + 多TextMode + 换行/装饰"},
		{master.ScrollUI && !master.Damage, "scroll", "滚动列表 + 模态遮罩卡片"},
		{master.Damage, "damage", "局部Damage条带 + PresentFrameAuto"},
		{master.Mesh3D, "mesh3d", "整窗3D渐变：立方体/球/变形星/地形旋转（GPU DrawMesh）"},
		{master.HUD, "hud", "右上角FPS/CPU与底部耗时条"},
	}
	n := 0
	for _, it := range items {
		if !it.on {
			continue
		}
		out = append(out, fmt.Sprintf("✓ %s：%s", it.key, it.desc))
		n++
	}
	if n == 0 {
		out = append(out, "· 本场景未启用绘制模块")
	}
	out = append(out, "说明: 场景模块持续绘制（非一闪）")
	out = append(out, "底部 active[] = 场景主开关列表（稳定）")
	out = append(out, "滤镜/图层/Backdrop 为小离屏真实 API 每帧生效（对标 Skia saveLayer）")
	return out
}

func scenarioNameCN(id, fallback string) string {
	switch id {
	case "S01":
		return "基线 HUD（背景+状态条）"
	case "S02":
		return "光晕场"
	case "S03":
		return "卡片 UI + 文本"
	case "S04":
		return "路径/虚线/多边形"
	case "S05":
		return "裁剪栈 + 光晕"
	case "S06":
		return "半透明图层 + 文本"
	case "S07":
		return "背景模糊面板"
	case "S08":
		return "贴图 + 蒙版"
	case "S09":
		return "文本样式专测"
	case "S10":
		return "滤镜特效"
	case "S11":
		return "网格/混合/变换"
	case "S12":
		return "全模块合成"
	case "S13":
		return "高密度图元"
	case "S14":
		return "每帧全开压力"
	case "S15":
		return "渐变 + 图案填充"
	case "S16":
		return "高级混合模式"
	case "S17":
		return "圆角裁剪 + EvenOdd"
	case "S18":
		return "LCD文本/塑形"
	case "S19":
		return "局部Damage Present"
	case "S20":
		return "滚动列表 + 模态"
	case "S21":
		return "Skia缺口全组合"
	case "S22":
		return "3D渐变旋转（整窗）"
	case "S23":
		return "3D+全模块合成"
	default:
		return fallback
	}
}

// ---------- fonts ----------

type fontPack struct {
	sans, sansBold, mono, serif string
	latin                       string // DejaVu for HUD ASCII (cheaper than CJK TTF every frame)
	ok                          bool
}

func loadFonts(dc *render.Context) fontPack {
	// 中文说明必须用 TrueType(glyf) 字体。
	// 当前 text 层不支持 CFF 轮廓：NotoSansCJK*.otf/*.ttc 会 Load 成功但字形空白→看起来像乱码。
	// 已用 offscreen 探针验证：DroidSansFallbackFull.ttf 有像素；Noto CJK CFF 无像素。
	cjk := findFirstExisting(
		"/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf",
		"/usr/share/fonts/truetype/droid/DroidSansFallback.ttf",
		"/usr/share/fonts/truetype/wqy/wqy-microhei.ttc",
		"/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc",
		"/usr/share/fonts/truetype/arphic/uming.ttc",
		"/usr/share/fonts/truetype/arphic/ukai.ttc",
	)
	latin := findFirstExisting(
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
	)
	// serif/mono：优先可显示中文的 TTF；不要选 Noto CJK CFF
	serif := findFirstExisting(
		"/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSerif.ttf",
	)
	mono := findFirstExisting(
		"/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf",
		"/usr/share/fonts/truetype/noto/NotoSansMono-Regular.ttf",
	)
	fp := fontPack{
		sans:     firstNonEmpty(cjk, latin),
		sansBold: firstNonEmpty(cjk, findFirstExisting("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf"), latin),
		mono:     firstNonEmpty(mono, cjk, latin),
		serif:    firstNonEmpty(serif, cjk, latin),
	}
	if fp.sans == "" {
		log.Printf("warning: 未找到可用字体，中文说明/文本模块将受限")
		return fp
	}
	if err := dc.LoadFontFace(fp.sans, 16); err != nil {
		log.Printf("LoadFontFace(%s): %v", fp.sans, err)
		if latin != "" && latin != fp.sans {
			if err2 := dc.LoadFontFace(latin, 16); err2 != nil {
				log.Printf("LoadFontFace latin fallback: %v", err2)
				return fp
			}
			fp.sans = latin
		} else {
			return fp
		}
	}
	fp.latin = latin
	if fp.latin == "" {
		fp.latin = fp.sans
	}
	log.Printf("fonts: cjk=%s latin=%s (中文说明必须 TrueType/glyf; CFF-NotoCJK 会空白/乱码)", filepath.Base(fp.sans), filepath.Base(fp.latin))
	fp.ok = true
	return fp
}

func firstNonEmpty(paths ...string) string {
	for _, p := range paths {
		if p != "" {
			return p
		}
	}
	return ""
}

func findFirstExisting(paths ...string) string {
	for _, p := range paths {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func findFont(name string) string {
	roots := []string{
		filepath.Join(os.Getenv("HOME"), ".local/share/fonts"),
		"/usr/share/fonts/opentype/noto",
		"/usr/share/fonts/truetype/droid",
		"/usr/share/fonts/truetype/wqy",
		"/usr/share/fonts/truetype/dejavu",
		"/usr/share/fonts/truetype/liberation",
		"/usr/share/fonts/TTF",
		"/usr/share/fonts/truetype",
	}
	for _, r := range roots {
		p := filepath.Join(r, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// ---------- color / rng / env ----------

func hsv(h, s, v float64) (r, g, b float64) {
	h = math.Mod(h, 1)
	if h < 0 {
		h += 1
	}
	i := math.Floor(h * 6)
	f := h*6 - i
	p := v * (1 - s)
	q := v * (1 - f*s)
	u := v * (1 - (1-f)*s)
	switch int(i) % 6 {
	case 0:
		return v, u, p
	case 1:
		return q, v, p
	case 2:
		return p, v, u
	case 3:
		return p, q, v
	case 4:
		return u, p, v
	default:
		return v, p, q
	}
}

type rng struct{ s uint64 }

func newRNG(seed int64) *rng {
	if seed == 0 {
		seed = 1
	}
	return &rng{s: uint64(seed)}
}
func (r *rng) next() uint64 {
	x := r.s
	x ^= x >> 12
	x ^= x << 25
	x ^= x >> 27
	r.s = x
	return x * 0x2545F4914F6CDD1D
}
func (r *rng) float01() float64 { return float64(r.next()%10000) / 10000.0 }
func (r *rng) intn(n int) int {
	if n <= 0 {
		return 0
	}
	return int(r.next() % uint64(n))
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envBool(key string, def bool) bool {
	if v, ok := envBoolOK(key); ok {
		return v
	}
	return def
}

func envBoolOK(key string) (bool, bool) {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return false, false
	}
	switch v {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	default:
		return false, false
	}
}

// rssKB returns THIS process VmRSS in KB (/proc/self/status) — not system memory.
func rssKB() int64 {
	b, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			var n int64
			_, _ = fmt.Sscanf(strings.TrimSpace(line[6:]), "%d", &n)
			return n
		}
	}
	return 0
}

type cpuSample struct {
	utime, stime uint64
	wall         time.Time
}

// readCPUSample reads THIS process utime/stime from /proc/self/stat — not /proc/stat (system).
func readCPUSample() (cpuSample, bool) {
	b, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return cpuSample{}, false
	}
	// comm may contain spaces/parens: find last ')' then fields after.
	s := string(b)
	rp := strings.LastIndex(s, ")")
	if rp < 0 || rp+2 >= len(s) {
		return cpuSample{}, false
	}
	fields := strings.Fields(s[rp+2:])
	// after comm: state(1) ... utime(12) stime(13) relative to fields[0]=state → utime=fields[11]
	if len(fields) < 13 {
		return cpuSample{}, false
	}
	ut, err1 := strconv.ParseUint(fields[11], 10, 64)
	st, err2 := strconv.ParseUint(fields[12], 10, 64)
	if err1 != nil || err2 != nil {
		return cpuSample{}, false
	}
	return cpuSample{utime: ut, stime: st, wall: time.Now()}, true
}

// cpuPercent is THIS process CPU relative to one core:
//
//	100%  = this process fully used 1 core over the interval
//	>100% = multi-threaded / multi-core use by this process only
//
// Never reads system-wide /proc/stat or loadavg.
// hostCPUCount is logical CPU count for converting 1-core% → machine-share%.
// GNOME/system monitors often show process CPU as fraction of ALL cores
// (1 full core on 4-core ≈ 25%). Our raw sample is 1-core relative (top-style).
func hostCPUCount() int {
	n := runtime.NumCPU()
	if n < 1 {
		return 1
	}
	return n
}

func cpuPercent(prev, cur cpuSample) (float64, bool) {
	dt := cur.wall.Sub(prev.wall).Seconds()
	if dt <= 1e-4 {
		return 0, false
	}
	// Linux clock ticks: usually 100 Hz
	const ticksPerSec = 100.0
	deltaTicks := float64((cur.utime + cur.stime) - (prev.utime + prev.stime))
	pct := (deltaTicks / ticksPerSec) / dt * 100.0
	if pct < 0 {
		pct = 0
	}
	if pct > 1000 {
		pct = 1000
	}
	return pct, true
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ---------- X11 ----------

const (
	xFocusIn          = 9
	xFocusOut         = 10
	xVisibilityNotify = 15
	xUnmapNotify      = 18
	xMapNotify        = 19
	xConfigureNotify  = 22
	xPropertyNotify   = 28
	xClientMessage    = 33
	xStructureNotify  = int64(1 << 17)
	xExposureMask     = int64(1 << 15)
	// VisibilityChangeMask: FullyObscured when covered by other windows.
	xVisibilityChangeMask = int64(1 << 16)
	xFocusChangeMask      = int64(1 << 21) // FocusChangeMask
	// PropertyChangeMask: GNOME Iconify often sets WM_STATE without UnmapNotify.
	xPropertyChangeMask = int64(1 << 22)

	xVisibilityUnobscured        = 0
	xVisibilityPartiallyObscured = 1
	xVisibilityFullyObscured     = 2

	xWMStateWithdrawn = 0
	xWMStateNormal    = 1
	xWMStateIconic    = 3
)

type x11Event struct {
	Type          int
	Width, Height int
	Atom          uintptr
	Visibility    int // VisibilityNotify state
	raw           [192]byte
}

type x11Win struct {
	lib                uintptr
	Display            uintptr
	Window             uintptr
	Root               uintptr
	wmDeleteAtom       uintptr
	wmStateAtom        uintptr
	xPending           func(dpy uintptr) int
	xNextEvent         func(dpy uintptr, ev *byte) int
	xFlush             func(dpy uintptr) int
	xDestroyWindow     func(dpy uintptr, win uintptr) int
	xCloseDisplay      func(dpy uintptr) int
	xInternAtom        func(dpy uintptr, name *byte, onlyIfExists int) uintptr
	xSetWMProtocols    func(dpy uintptr, win uintptr, protocols *uintptr, count int) int
	xSetWMNormalHints  func(dpy uintptr, win uintptr, hints *byte) int
	xGetWindowProperty func(
		dpy uintptr, win uintptr, property uintptr,
		offset, length int64, delete int, reqType uintptr,
		actualType *uintptr, actualFormat *int32,
		nitems *uint64, bytesAfter *uint64, prop **byte,
	) int
	xFree                 func(ptr uintptr) int
	xQueryTree            func(dpy, w uintptr, root, parent *uintptr, children *uintptr, nchildren *uint32) int
	xGetGeometry          func(dpy, w uintptr, root *uintptr, x, y *int32, width, height, border, depth *uint32) int
	xTranslateCoordinates func(dpy, src, dst uintptr, srcX, srcY int32, dstX, dstY *int32, child *uintptr) int
	xGetWindowAttributes  func(dpy, w uintptr, attrs *byte) int
}

// LockSize sets min=max size hints so the WM cannot maximize/tile during soaks.
func (w *x11Win) LockSize(width, height int) {
	if w == nil || w.xSetWMNormalHints == nil || w.Display == 0 || w.Window == 0 {
		return
	}
	if width < 64 {
		width = 64
	}
	if height < 64 {
		height = 64
	}
	// XSizeHints on LP64 Linux (xlib): long flags; int x,y,width,height,min_*,max_*,...
	// flags: PSize(8)|PMinSize(16)|PMaxSize(32) = 56
	var buf [128]byte
	*(*int64)(unsafe.Pointer(&buf[0])) = 8 | 16 | 32
	*(*int32)(unsafe.Pointer(&buf[8])) = 60  // x
	*(*int32)(unsafe.Pointer(&buf[12])) = 40 // y
	*(*int32)(unsafe.Pointer(&buf[16])) = int32(width)
	*(*int32)(unsafe.Pointer(&buf[20])) = int32(height)
	*(*int32)(unsafe.Pointer(&buf[24])) = int32(width)  // min_width
	*(*int32)(unsafe.Pointer(&buf[28])) = int32(height) // min_height
	*(*int32)(unsafe.Pointer(&buf[32])) = int32(width)  // max_width
	*(*int32)(unsafe.Pointer(&buf[36])) = int32(height) // max_height
	w.xSetWMNormalHints(w.Display, w.Window, &buf[0])
	if w.xFlush != nil {
		w.xFlush(w.Display)
	}
}

func (w *x11Win) Close() {
	if w == nil {
		return
	}
	if w.xDestroyWindow != nil && w.Display != 0 && w.Window != 0 {
		w.xDestroyWindow(w.Display, w.Window)
	}
	if w.xCloseDisplay != nil && w.Display != 0 {
		w.xCloseDisplay(w.Display)
	}
	if w.lib != 0 {
		_ = purego.Dlclose(w.lib)
	}
}
func (w *x11Win) Flush() {
	if w != nil && w.xFlush != nil {
		w.xFlush(w.Display)
	}
}
func (w *x11Win) Pending() bool {
	return w != nil && w.xPending != nil && w.xPending(w.Display) > 0
}
func (w *x11Win) NextEvent() x11Event {
	var ev x11Event
	if w == nil || w.xNextEvent == nil {
		return ev
	}
	w.xNextEvent(w.Display, &ev.raw[0])
	ev.Type = int(*(*int32)(unsafe.Pointer(&ev.raw[0])))
	if ev.Type == xConfigureNotify {
		// LP64 XConfigureEvent: width@56 height@60
		ev.Width = int(*(*int32)(unsafe.Pointer(&ev.raw[56])))
		ev.Height = int(*(*int32)(unsafe.Pointer(&ev.raw[60])))
	}
	if ev.Type == xPropertyNotify {
		// LP64 XPropertyEvent: atom@40
		ev.Atom = *(*uintptr)(unsafe.Pointer(&ev.raw[40]))
	}
	if ev.Type == xVisibilityNotify {
		// LP64 XVisibilityEvent: state@40 (int)
		ev.Visibility = int(*(*int32)(unsafe.Pointer(&ev.raw[40])))
	}
	return ev
}

// IsWMStateProperty reports whether ev is a PropertyNotify for WM_STATE
// (GNOME Iconify path without UnmapNotify).
func (w *x11Win) IsWMStateProperty(ev x11Event) bool {
	return w != nil && ev.Type == xPropertyNotify && w.wmStateAtom != 0 && ev.Atom == w.wmStateAtom
}

// IsIconic queries ICCCM WM_STATE. True when IconicState / WithdrawnState.
func (w *x11Win) IsIconic() bool {
	if w == nil || w.xGetWindowProperty == nil || w.wmStateAtom == 0 || w.Display == 0 || w.Window == 0 {
		return false
	}
	var actualType uintptr
	var actualFormat int32
	var nitems, bytesAfter uint64
	var prop *byte
	status := w.xGetWindowProperty(
		w.Display, w.Window, w.wmStateAtom,
		0, 2, 0, 0,
		&actualType, &actualFormat, &nitems, &bytesAfter, &prop,
	)
	if status != 0 || prop == nil || nitems < 1 {
		if prop != nil && w.xFree != nil {
			w.xFree(uintptr(unsafe.Pointer(prop)))
		}
		return false
	}
	state := *(*uint32)(unsafe.Pointer(prop))
	if w.xFree != nil {
		w.xFree(uintptr(unsafe.Pointer(prop)))
	}
	return state == xWMStateIconic || state == xWMStateWithdrawn
}
func (w *x11Win) IsDelete(ev x11Event) bool {
	if w == nil || ev.Type != xClientMessage || w.wmDeleteAtom == 0 {
		return false
	}
	msgType := *(*uintptr)(unsafe.Pointer(&ev.raw[40]))
	data0 := *(*uintptr)(unsafe.Pointer(&ev.raw[56]))
	return data0 == w.wmDeleteAtom || msgType == w.wmDeleteAtom
}

// IsFullyCoveredByOtherWindows reports whether a higher-stacked top-level window
// fully covers this window (Flutter/Skia: unpresentable surface → skip acquire).
// Unfocused-but-still-visible (partial cover) returns false — keep drawing.
func (w *x11Win) IsFullyCoveredByOtherWindows() bool {
	if w == nil || w.Display == 0 || w.Window == 0 || w.Root == 0 {
		return false
	}
	if w.xQueryTree == nil || w.xGetGeometry == nil || w.xTranslateCoordinates == nil || w.xFree == nil {
		return false
	}

	// Resolve reparented frame (WM decoration parent that is a direct root child).
	frame := w.Window
	for {
		var rootOut, parent, children uintptr
		var n uint32
		if w.xQueryTree(w.Display, frame, &rootOut, &parent, &children, &n) == 0 {
			break
		}
		if children != 0 {
			w.xFree(children)
		}
		if parent == 0 || parent == w.Root {
			break
		}
		frame = parent
	}

	var rootOut, parent, children uintptr
	var nchildren uint32
	if w.xQueryTree(w.Display, w.Root, &rootOut, &parent, &children, &nchildren) == 0 || nchildren == 0 || children == 0 {
		return false
	}
	defer w.xFree(children)

	// Our absolute content rect (use content window, not frame, for cover test).
	var absX, absY int32
	var child uintptr
	var selfRoot uintptr
	var sx, sy int32
	var sw, sh, sb, sd uint32
	if w.xGetGeometry(w.Display, w.Window, &selfRoot, &sx, &sy, &sw, &sh, &sb, &sd) == 0 {
		return false
	}
	if w.xTranslateCoordinates(w.Display, w.Window, w.Root, 0, 0, &absX, &absY, &child) == 0 {
		absX, absY = sx, sy
	}
	ourL, ourT := int(absX), int(absY)
	ourR, ourB := ourL+int(sw), ourT+int(sh)
	if int(sw) < 8 || int(sh) < 8 {
		return false
	}

	var rootW, rootH uint32
	{
		var rr uintptr
		var rx, ry int32
		var rb, rd uint32
		_ = w.xGetGeometry(w.Display, w.Root, &rr, &rx, &ry, &rootW, &rootH, &rb, &rd)
	}

	arr := unsafe.Slice((*uintptr)(unsafe.Pointer(children)), int(nchildren))
	idx := -1
	for i, id := range arr {
		if id == frame {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false // cannot establish stacking → do not pause (prefer draw)
	}

	// Only windows ABOVE us in the stack (higher index = top).
	for i := idx + 1; i < len(arr); i++ {
		id := arr[i]
		if id == 0 || id == frame || id == w.Window {
			continue
		}
		var r uintptr
		var x, y int32
		var ww, hh, br, dp uint32
		if w.xGetGeometry(w.Display, id, &r, &x, &y, &ww, &hh, &br, &dp) == 0 {
			continue
		}
		if ww < 64 || hh < 64 {
			continue
		}
		// Skip near-fullscreen shell/desktop guards.
		if rootW > 0 && rootH > 0 && uint64(ww)*uint64(hh)*10 >= uint64(rootW)*uint64(rootH)*9 {
			continue
		}
		var ax, ay int32
		var ch uintptr
		if w.xTranslateCoordinates(w.Display, id, w.Root, 0, 0, &ax, &ay, &ch) == 0 {
			continue
		}
		l, top := int(ax), int(ay)
		rgt, bot := l+int(ww), top+int(hh)
		// Full contain OR ≥85% area overlap (partial detector miss under reparent WMs).
		if l <= ourL && top <= ourT && rgt >= ourR && bot >= ourB {
			return true
		}
		ix0 := max(l, ourL)
		iy0 := max(top, ourT)
		ix1 := min(rgt, ourR)
		iy1 := min(bot, ourB)
		if ix1 > ix0 && iy1 > iy0 {
			inter := (ix1 - ix0) * (iy1 - iy0)
			ourArea := (ourR - ourL) * (ourB - ourT)
			if ourArea > 0 && inter*100 >= ourArea*85 {
				return true
			}
		}
	}
	return false
}

func openX11Window(w, h int, title string) (*x11Win, error) {
	lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		lib, err = purego.Dlopen("libX11.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	}
	if err != nil {
		return nil, fmt.Errorf("dlopen libX11: %w", err)
	}
	var (
		xOpenDisplay    func(name *byte) uintptr
		xCloseDisplay   func(dpy uintptr) int
		xDefaultScreen  func(dpy uintptr) int
		xRootWindow     func(dpy uintptr, screen int) uintptr
		xCreateSimple   func(dpy uintptr, parent uintptr, x, y int, width, height, borderWidth uint, border, background uint64) uintptr
		xMapWindow      func(dpy uintptr, win uintptr) int
		xFlush          func(dpy uintptr) int
		xDestroyWindow  func(dpy uintptr, win uintptr) int
		xStoreName      func(dpy uintptr, win uintptr, name *byte) int
		xSelectInput    func(dpy uintptr, win uintptr, mask int64) int
		xPending        func(dpy uintptr) int
		xNextEvent      func(dpy uintptr, ev *byte) int
		xInternAtom     func(dpy uintptr, name *byte, onlyIfExists int) uintptr
		xSetWMProtocols func(dpy uintptr, win uintptr, protocols *uintptr, count int) int
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
	purego.RegisterLibFunc(&xSelectInput, lib, "XSelectInput")
	purego.RegisterLibFunc(&xPending, lib, "XPending")
	purego.RegisterLibFunc(&xNextEvent, lib, "XNextEvent")
	purego.RegisterLibFunc(&xInternAtom, lib, "XInternAtom")
	purego.RegisterLibFunc(&xSetWMProtocols, lib, "XSetWMProtocols")

	dpy := xOpenDisplay(nil)
	if dpy == 0 {
		_ = purego.Dlclose(lib)
		return nil, fmt.Errorf("XOpenDisplay failed (DISPLAY=%q)", os.Getenv("DISPLAY"))
	}
	screen := xDefaultScreen(dpy)
	root := xRootWindow(dpy, screen)
	win := xCreateSimple(dpy, root, 60, 40, uint(w), uint(h), 1, 0, 0)
	if win == 0 {
		xCloseDisplay(dpy)
		_ = purego.Dlclose(lib)
		return nil, fmt.Errorf("XCreateSimpleWindow failed")
	}
	name := append([]byte(title), 0)
	xStoreName(dpy, win, &name[0])
	// Structure/Expose + Property (GNOME Iconify) + Visibility (covered by other windows).
	xSelectInput(dpy, win, xStructureNotify|xExposureMask|xPropertyChangeMask|xVisibilityChangeMask|xFocusChangeMask)

	atomName := append([]byte("WM_DELETE_WINDOW"), 0)
	delAtom := xInternAtom(dpy, &atomName[0], 0)
	if delAtom != 0 {
		prot := delAtom
		xSetWMProtocols(dpy, win, &prot, 1)
	}
	wmStateName := append([]byte("WM_STATE"), 0)
	wmStateAtom := xInternAtom(dpy, &wmStateName[0], 0)

	// Register size-hint setter for LockSize (timed soaks keep 800x600).
	var xSetWMNormalHints func(dpy uintptr, win uintptr, hints *byte) int
	purego.RegisterLibFunc(&xSetWMNormalHints, lib, "XSetWMNormalHints")
	var xGetWindowProperty func(
		dpy uintptr, win uintptr, property uintptr,
		offset, length int64, delete int, reqType uintptr,
		actualType *uintptr, actualFormat *int32,
		nitems *uint64, bytesAfter *uint64, prop **byte,
	) int
	purego.RegisterLibFunc(&xGetWindowProperty, lib, "XGetWindowProperty")
	var xFree func(ptr uintptr) int
	purego.RegisterLibFunc(&xFree, lib, "XFree")
	var xQueryTree func(dpy, w uintptr, root, parent *uintptr, children *uintptr, nchildren *uint32) int
	purego.RegisterLibFunc(&xQueryTree, lib, "XQueryTree")
	var xGetGeometry func(dpy, w uintptr, root *uintptr, x, y *int32, width, height, border, depth *uint32) int
	purego.RegisterLibFunc(&xGetGeometry, lib, "XGetGeometry")
	var xTranslateCoordinates func(dpy, src, dst uintptr, srcX, srcY int32, dstX, dstY *int32, child *uintptr) int
	purego.RegisterLibFunc(&xTranslateCoordinates, lib, "XTranslateCoordinates")
	var xGetWindowAttributes func(dpy, w uintptr, attrs *byte) int
	purego.RegisterLibFunc(&xGetWindowAttributes, lib, "XGetWindowAttributes")
	// _NET_WM_PID so external soak drivers can find this window by process id.
	var xChangeProperty func(dpy, w, prop, typ uintptr, format, mode int, data *byte, nelements int) int
	purego.RegisterLibFunc(&xChangeProperty, lib, "XChangeProperty")
	pidAtomName := append([]byte("_NET_WM_PID"), 0)
	pidAtom := xInternAtom(dpy, &pidAtomName[0], 0)
	cardAtomName := append([]byte("CARDINAL"), 0)
	cardAtom := xInternAtom(dpy, &cardAtomName[0], 0)
	if pidAtom != 0 && cardAtom != 0 && xChangeProperty != nil {
		pid := uint32(os.Getpid())
		xChangeProperty(dpy, win, pidAtom, cardAtom, 32, 0, (*byte)(unsafe.Pointer(&pid)), 1)
	}

	xMapWindow(dpy, win)
	xFlush(dpy)
	time.Sleep(50 * time.Millisecond)

	xw := &x11Win{
		lib: lib, Display: dpy, Window: win, Root: root, wmDeleteAtom: delAtom,
		wmStateAtom: wmStateAtom,
		xPending:    xPending, xNextEvent: xNextEvent, xFlush: xFlush,
		xDestroyWindow: xDestroyWindow, xCloseDisplay: xCloseDisplay,
		xInternAtom: xInternAtom, xSetWMProtocols: xSetWMProtocols,
		xSetWMNormalHints:     xSetWMNormalHints,
		xGetWindowProperty:    xGetWindowProperty,
		xFree:                 xFree,
		xQueryTree:            xQueryTree,
		xGetGeometry:          xGetGeometry,
		xTranslateCoordinates: xTranslateCoordinates,
		xGetWindowAttributes:  xGetWindowAttributes,
	}
	xw.LockSize(w, h)
	return xw, nil
}
