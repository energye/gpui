//go:build linux && !nogpu

package exboot

import (
	"errors"
	"log"
	"os"
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
//	  Host kit smoke (4-core): ~3.05% CPU vs ~3.5% with GPUI_COMPOSITOR=0 (~0.5pp better).
//
//	Opt-out — surface direct vector (G2.a):
//	  GPUI_COMPOSITOR=0
//
// Demand: Continuous=false → paint only when Tree.Dirty().
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
	// OnResize is called after session size update.
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

func useCompositor() bool {
	v := os.Getenv("GPUI_COMPOSITOR")
	// Default ON. Explicit off for direct surface path.
	return v != "0" && v != "false" && v != "off"
}

// RunUIDemand attaches host+tree to ui/app and runs the demand loop.
func RunUIDemand(cfg UIDemandConfig) UIDemandResult {
	if cfg.Host == nil || cfg.Tree == nil || cfg.SC == nil || cfg.DC == nil {
		log.Fatal("exboot.RunUIDemand: Host, Tree, SC, DC required")
	}

	var comp *layer.Compositor
	if useCompositor() {
		comp = layer.NewCompositor()
		comp.BG = cfg.Clear
		defer comp.ReleaseAll()
	}

	a := app.New(app.Options{
		ContinuousRender: cfg.Continuous,
		OnUpdate:         cfg.OnUpdate,
		BeforeDispatch:   cfg.BeforeDispatch,
	})
	defer a.Close()

	present := func(s *app.Session) error {
		dc := cfg.DC
		sc := cfg.SC
		device := cfg.Device
		host := cfg.Host

		w, h := s.Width, s.Height
		if w < 1 || h < 1 {
			w, h = host.Size()
		}
		scale := host.ScaleFactor()
		if scale <= 0 {
			scale = 1
		}

		if comp != nil {
			if err := presentCompositor(cfg, s, comp, dc, sc, device, w, h, scale); err == nil {
				return nil
			}
			// Fall through to direct if compositor cannot produce a base RT.
			log.Printf("exboot: compositor unavailable, using direct present")
		}
		return presentDirect(cfg, s, dc, sc, device, w, h, scale)
	}

	sess := a.Attach(cfg.Host, cfg.Tree, present)
	if cfg.Theme != nil {
		sess.Theme = cfg.Theme
	}
	if w, h := cfg.Host.Size(); w > 0 && h > 0 {
		sess.Width, sess.Height = w, h
	}
	sess.OnResize = func(w, h int) {
		if w < 64 {
			w = 64
		}
		if h < 64 {
			h = 64
		}
		sess.Width, sess.Height = w, h
		_ = cfg.SC.Resize(uint32(w), uint32(h))
		_ = cfg.DC.Resize(w, h)
		if comp != nil {
			comp.ReleaseAll()
		}
		if cfg.Tree != nil {
			cfg.Tree.MarkFullPaintRequired()
		}
		if cfg.OnResize != nil {
			cfg.OnResize(w, h)
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

// presentCompositor paints full tree into base RT, then blit-only to surface.
func presentCompositor(
	cfg UIDemandConfig,
	s *app.Session,
	comp *layer.Compositor,
	dc *render.Context,
	sc *webgpu.Swapchain,
	device *webgpu.Device,
	w, h int,
	scale float64,
) error {
	comp.BG = cfg.Clear
	comp.Resize(w, h, scale)

	full := cfg.Continuous || s.Tree == nil || s.Tree.FullPaintRequired()
	if !comp.Frame(s.Tree, themeOf(cfg, s), full) || !comp.HasBase() {
		return errors.New("compositor: base RT not ready")
	}

	// Blit-only into the window context (no Clear / no Fill on surface).
	dc.BeginFrame()
	comp.BlitTo(dc)
	if !comp.HasBase() {
		return errors.New("compositor: base missing after blit setup")
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
		log.Printf("BeginFrame: %v", err)
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
		return nil
	}
	if cfg.Flush != nil {
		cfg.Flush()
	}
	return nil
}

// presentDirect: clear + full vector paint into the surface (G2.a).
func presentDirect(
	cfg UIDemandConfig,
	s *app.Session,
	dc *render.Context,
	sc *webgpu.Swapchain,
	device *webgpu.Device,
	w, h int,
	scale float64,
) error {
	if w < 1 {
		w = s.Width
	}
	if h < 1 {
		h = s.Height
	}
	dc.BeginFrame()
	// GPU-visible background (ClearWithColor is CPU pixmap only).
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
