//go:build linux && !nogpu

package exboot

import (
	"errors"
	"log"
	"os"
	"sync"
	"time"

	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/app"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/layer"
	"github.com/energye/gpui/ui/platform"
)

// UIDemandConfig drives a gogpu-aligned demand frame loop for UI smokes.
//
// Present policy:
//
//	Default ON — compositor base RT (recommended):
//	  full tree → offscreen base texture (GPU vector)
//	  swapchain ← DrawGPUTexture(base) only (G2.b blit-only)
//
//	Opt-out — surface direct vector (G2.a):
//	  GPUI_COMPOSITOR=0
//
// Resize policy (framework — not per-example):
//
//  1. Every ConfigureNotify updates live client size (coalesced by ui/app).
//  2. Each present paints at the LIVE window size so content tracks the window
//     (no black margins mid-drag).
//  3. Surface.Configure is deferred to present and applied at most once per
//     present with the latest size (event drain + coalesce). Content is painted
//     into the base RT before Configure when using the compositor, then blit
//     + Present in the same call so the first frame after recreate is complete.
//  4. X11 LinuxHost uses back_pixmap=None + NW gravity so the server does not
//     solid-fill enlarged regions (device_lost_redraw anti-flash).
//  5. BeginFrame Outdated/Lost: force reconfigure to live size and retry.
//
// Demand: Continuous=false → paint when Dirty() or size/present mismatch.
type UIDemandConfig struct {
	Host   platform.Host
	Tree   *core.Tree
	SC     *webgpu.Swapchain
	DC     *render.Context
	Device *webgpu.Device
	Theme  *core.Theme
	// Clear is the frame background color (base RT / surface).
	Clear render.RGBA
	// Seconds is how long to run before Quit; <=0 means unlimited (default).
	Seconds float64
	// Continuous forces every-tick paint (game loop only).
	Continuous bool
	// OnUpdate runs on the main (Run) goroutine each active tick.
	OnUpdate func(dt float64)
	// BeforeDispatch optional; return true to skip platform.Dispatch.
	BeforeDispatch func(tree *core.Tree, ev platform.Event) (skip bool)
	// OnResize is layout-only (root width/height / viewport). Called with the
	// size that will be painted this frame (surface size mid-drag, live size when quiet).
	// Do not call SC.Resize here.
	OnResize func(w, h int)
	// Flush is optional (LinuxHost.Flush after present).
	Flush func()
}

// UIDemandResult summarizes a demand run.
type UIDemandResult struct {
	Paints int64
	Loops  int64
	Hops   int64
}

// ResizeConfigureIdle is how long ConfigureNotify must be quiet before we
// Surface.Configure to the live size (sharp pixels). Matches device_lost_redraw.
const ResizeConfigureIdle = 32 * time.Millisecond

var logPaintScaleOnce sync.Once

func useCompositor() bool {
	v := os.Getenv("GPUI_COMPOSITOR")
	return v != "0" && v != "false" && v != "off"
}

const minPresentSize = 64

// resizeTracker records live client size + last ConfigureNotify time.
type resizeTracker struct {
	mu        sync.Mutex
	liveW     int
	liveH     int
	lastCfgAt time.Time
}

func (r *resizeTracker) noteLive(w, h int) {
	if r == nil {
		return
	}
	if w < minPresentSize {
		w = minPresentSize
	}
	if h < minPresentSize {
		h = minPresentSize
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastCfgAt = time.Now()
	r.liveW, r.liveH = w, h
}

func (r *resizeTracker) snapshot() (liveW, liveH int, quiet bool) {
	if r == nil {
		return 0, 0, true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	liveW, liveH = r.liveW, r.liveH
	if liveW < 1 || liveH < 1 {
		return liveW, liveH, true
	}
	if r.lastCfgAt.IsZero() {
		return liveW, liveH, true
	}
	quiet = time.Since(r.lastCfgAt) >= ResizeConfigureIdle
	return liveW, liveH, quiet
}

// RunUIDemand attaches host+tree to ui/app and runs the demand loop.
func RunUIDemand(cfg UIDemandConfig) UIDemandResult {
	if cfg.Host == nil || cfg.Tree == nil || cfg.SC == nil || cfg.DC == nil {
		log.Fatal("exboot.RunUIDemand: Host, Tree, SC, DC required")
	}

	// Mouse cursor: map core.CursorKind → platform.CursorHost when available.
	if ch, ok := cfg.Host.(platform.CursorHost); ok {
		cfg.Tree.SetOnCursor(func(k core.CursorKind) {
			ch.SetCursor(platform.CursorKind(k))
		})
	}

	var comp *layer.Compositor
	if useCompositor() {
		comp = layer.NewCompositor()
		comp.BG = cfg.Clear
		defer comp.ReleaseAll()
	}

	rt := &resizeTracker{}
	if w, h := cfg.Host.Size(); w > 0 && h > 0 {
		rt.noteLive(w, h)
	}

	a := app.New(app.Options{
		ContinuousRender: cfg.Continuous,
		OnUpdate:         cfg.OnUpdate,
		BeforeDispatch:   cfg.BeforeDispatch,
	})
	defer a.Close()

	lastLayoutW, lastLayoutH := 0, 0

	present := func(s *app.Session) error {
		dc := cfg.DC
		sc := cfg.SC
		device := cfg.Device

		// Live client size (window).
		liveW, liveH, _ := rt.snapshot()
		if liveW < 1 || liveH < 1 {
			liveW, liveH = s.Width, s.Height
		}
		if liveW < minPresentSize {
			liveW = minPresentSize
		}
		if liveH < minPresentSize {
			liveH = minPresentSize
		}

		// Current surface buffer size.
		surfW, surfH := int(sc.Width), int(sc.Height)
		if surfW < 1 {
			surfW = liveW
		}
		if surfH < 1 {
			surfH = liveH
		}

		scale := 1.0
		if cfg.Host != nil {
			scale = cfg.Host.ScaleFactor()
		}
		if scale <= 0 {
			scale = 1
		}
		// UI supersample: paint compositor base at ≥2× when host DPR is 1 so
		// 1px borders / small circles get soft edges (match ui_ant_compare).
		// Disable with GPUI_UI_SUPERSAMPLE=0.
		if v := os.Getenv("GPUI_UI_SUPERSAMPLE"); v != "0" && v != "false" && v != "off" {
			if scale < 2 {
				scale = 2
			}
		}

		logPaintScaleOnce.Do(func() {
			log.Printf("exboot: UI paint scale=%.2f (host=%.2f supersample env=%q sample_count env=%q)",
				scale, func() float64 {
					if cfg.Host != nil {
						return cfg.Host.ScaleFactor()
					}
					return 1
				}(), os.Getenv("GPUI_UI_SUPERSAMPLE"), os.Getenv("GPUI_SURFACE_SAMPLE_COUNT"))
		})
		// Always paint at LIVE window size so content tracks the window.
		// Reconfigure surface when it lags (at most once per present after event drain).
		paintW, paintH := liveW, liveH
		doConfigure := paintW != surfW || paintH != surfH

		// Layout to the size we will paint this frame.
		if lastLayoutW != paintW || lastLayoutH != paintH {
			s.Width, s.Height = paintW, paintH
			lastLayoutW, lastLayoutH = paintW, paintH
			if cfg.OnResize != nil {
				cfg.OnResize(paintW, paintH)
			}
			if cfg.Tree != nil {
				cfg.Tree.MarkFullPaintRequired()
			}
		}

		if comp != nil {
			// 1) Paint full content into base RT at the new size first.
			comp.BG = cfg.Clear
			comp.Resize(paintW, paintH, scale)
			full := true // resize/drag always needs a complete base frame
			if !comp.Frame(s.Tree, themeOf(cfg, s), full) || !comp.HasBase() {
				log.Printf("exboot: compositor base failed, direct present")
				// Direct path must Configure before drawing into the surface.
				if doConfigure {
					_ = sc.Resize(uint32(paintW), uint32(paintH))
					_ = dc.Resize(paintW, paintH)
				}
				return presentDirect(cfg, s, dc, sc, device, paintW, paintH, scale)
			}

			// 2) Reconfigure surface only after base content is ready, then blit+present.
			if doConfigure {
				if err := sc.Resize(uint32(paintW), uint32(paintH)); err != nil {
					log.Printf("exboot: swapchain resize %dx%d: %v", paintW, paintH, err)
				}
				if err := dc.Resize(paintW, paintH); err != nil {
					log.Printf("exboot: dc resize %dx%d: %v", paintW, paintH, err)
				}
				if cfg.Tree != nil {
					cfg.Tree.MarkFullPaintRequired()
				}
			}

			return blitAndPresent(cfg, s, comp, dc, sc, device, paintW, paintH, liveW, liveH, rt)
		}

		// Direct: Configure then paint into surface in the same present call.
		if doConfigure {
			if err := sc.Resize(uint32(paintW), uint32(paintH)); err != nil {
				log.Printf("exboot: swapchain resize %dx%d: %v", paintW, paintH, err)
			}
			if err := dc.Resize(paintW, paintH); err != nil {
				log.Printf("exboot: dc resize %dx%d: %v", paintW, paintH, err)
			}
		}
		return presentDirect(cfg, s, dc, sc, device, paintW, paintH, scale)
	}

	sess := a.Attach(cfg.Host, cfg.Tree, present)
	if cfg.Theme != nil {
		sess.Theme = cfg.Theme
	}
	if w, h := cfg.Host.Size(); w > 0 && h > 0 {
		sess.Width, sess.Height = w, h
	}

	// Layout + track live size; never SC.Resize here.
	sess.OnResize = func(w, h int) {
		if w < minPresentSize {
			w = minPresentSize
		}
		if h < minPresentSize {
			h = minPresentSize
		}
		// Track live client size for quiet-configure; layout applied in present.
		rt.noteLive(w, h)
		sess.Width, sess.Height = w, h
		if cfg.Tree != nil {
			cfg.Tree.MarkFullPaintRequired()
		}
	}

	if cfg.Seconds > 0 {
		go func() {
			time.Sleep(time.Duration(cfg.Seconds * float64(time.Second)))
			a.Quit()
		}()
	}

	a.Run()
	return UIDemandResult{
		Paints: a.PaintCount.Load(),
		Loops:  a.LoopCount.Load(),
		Hops:   a.RenderThreadHops.Load(),
	}
}

func blitAndPresent(
	cfg UIDemandConfig,
	s *app.Session,
	comp *layer.Compositor,
	dc *render.Context,
	sc *webgpu.Swapchain,
	device *webgpu.Device,
	paintW, paintH, liveW, liveH int,
	rt *resizeTracker,
) error {
	// Ensure DC matches surface (mid-drag surface size).
	if dc.Width() != paintW || dc.Height() != paintH {
		_ = dc.Resize(paintW, paintH)
	}

	dc.BeginFrame()
	comp.BlitTo(dc)
	if !comp.HasBase() {
		return errors.New("compositor: base missing after blit")
	}
	if device != nil {
		device.FlushCallbacks()
	}
	frame, err := sc.BeginFrame()
	if err != nil {
		if errors.Is(err, webgpu.ErrDeviceLost) || (device != nil && device.IsLost()) {
			comp.ReleaseAll()
			if s.Tree != nil {
				s.Tree.MarkFullPaintRequired()
			}
			time.Sleep(16 * time.Millisecond)
			return nil
		}
		// Outdated: must reconfigure to live size (cannot present old surface).
		if liveW >= minPresentSize && liveH >= minPresentSize {
			_ = sc.Resize(uint32(liveW), uint32(liveH))
			_ = dc.Resize(liveW, liveH)
			if comp != nil {
				comp.Resize(liveW, liveH, 1)
			}
			if rt != nil {
				// Allow immediate quiet reconfigure next frame.
				rt.mu.Lock()
				rt.lastCfgAt = time.Time{}
				rt.mu.Unlock()
			}
		}
		if s.Tree != nil {
			s.Tree.MarkFullPaintRequired()
		}
		log.Printf("BeginFrame: %v (will retry)", err)
		return nil
	}
	if err := dc.PresentFrameFull(frame.Handle, frame.Width, frame.Height, func() error {
		return sc.EndFrame(frame)
	}); err != nil {
		sc.DiscardFrame(frame)
		if errors.Is(err, webgpu.ErrDeviceLost) || (device != nil && device.IsLost()) {
			comp.ReleaseAll()
			if s.Tree != nil {
				s.Tree.MarkFullPaintRequired()
			}
			time.Sleep(16 * time.Millisecond)
			return nil
		}
		log.Printf("PresentFrameFull: %v", err)
		if s.Tree != nil {
			s.Tree.MarkFullPaintRequired()
		}
		return nil
	}
	if cfg.Flush != nil {
		cfg.Flush()
	}
	return nil
}

func presentDirect(
	cfg UIDemandConfig,
	s *app.Session,
	dc *render.Context,
	sc *webgpu.Swapchain,
	device *webgpu.Device,
	w, h int,
	scale float64,
) error {
	if w < minPresentSize {
		w = minPresentSize
	}
	if h < minPresentSize {
		h = minPresentSize
	}
	if dc.Width() != w || dc.Height() != h {
		_ = dc.Resize(w, h)
	}

	dc.BeginFrame()
	dc.SetRGBA(cfg.Clear.R, cfg.Clear.G, cfg.Clear.B, cfg.Clear.A)
	if cfg.Clear.A <= 0 {
		dc.SetRGBA(0.1, 0.1, 0.1, 1)
	}
	dc.DrawRectangle(0, 0, float64(w), float64(h))
	_ = dc.Fill()
	dc.MarkFullRedraw()

	pc := &core.PaintContext{
		DC:            dc,
		Scale:         scale,
		Theme:         themeOf(cfg, s),
		CompositeOnly: false,
	}
	s.Tree.Frame(pc, core.Size{Width: float64(w), Height: float64(h)})

	if device != nil {
		device.FlushCallbacks()
	}
	frame, err := sc.BeginFrame()
	if err != nil {
		if errors.Is(err, webgpu.ErrDeviceLost) || (device != nil && device.IsLost()) {
			if s.Tree != nil {
				s.Tree.MarkFullPaintRequired()
			}
			time.Sleep(16 * time.Millisecond)
			return nil
		}
		log.Printf("BeginFrame: %v", err)
		if s.Tree != nil {
			s.Tree.MarkFullPaintRequired()
		}
		return nil
	}
	if err := dc.PresentFrameFull(frame.Handle, frame.Width, frame.Height, func() error {
		return sc.EndFrame(frame)
	}); err != nil {
		sc.DiscardFrame(frame)
		if errors.Is(err, webgpu.ErrDeviceLost) || (device != nil && device.IsLost()) {
			if s.Tree != nil {
				s.Tree.MarkFullPaintRequired()
			}
			time.Sleep(16 * time.Millisecond)
			return nil
		}
		log.Printf("PresentFrameFull: %v", err)
		if s.Tree != nil {
			s.Tree.MarkFullPaintRequired()
		}
		return nil
	}
	if cfg.Flush != nil {
		cfg.Flush()
	}
	return nil
}

func themeOf(cfg UIDemandConfig, s *app.Session) *core.Theme {
	if s != nil && s.Theme != nil {
		return s.Theme
	}
	return cfg.Theme
}
