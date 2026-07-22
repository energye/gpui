//go:build linux && !nogpu

package exboot

import (
	"errors"
	"log"
	"time"

	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/app"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/platform"
)

// UIDemandConfig drives a gogpu-aligned demand frame loop for UI smokes.
type UIDemandConfig struct {
	Host   platform.Host
	Tree   *core.Tree
	SC     *webgpu.Swapchain
	DC     *render.Context
	Device *webgpu.Device
	Theme  *core.Theme
	// Clear is the per-frame clear color (logical).
	Clear render.RGBA
	// Seconds is how long to run before Quit; <=0 means unlimited (default).
	// Set GPUI_ANIM_SECONDS>0 in callers for timed CI smokes.
	Seconds float64
	// Continuous forces every-tick paint (animations / gallery demos).
	Continuous bool
	// OnUpdate runs on the main (Run) goroutine each active tick.
	OnUpdate func(dt float64)
	// BeforeDispatch optional; return true to skip platform.Dispatch.
	BeforeDispatch func(tree *core.Tree, ev platform.Event) (skip bool)
	// OnResize is called after session size update (swapchain/dc resize here).
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

// RunUIDemand attaches host+tree to ui/app, presents via PresentFrame*, and
// runs until Seconds elapse (if >0), window close, or error.
//
// Present runs on the app render OS thread (default); events stay on Run.
func RunUIDemand(cfg UIDemandConfig) UIDemandResult {
	if cfg.Host == nil || cfg.Tree == nil || cfg.SC == nil || cfg.DC == nil {
		log.Fatal("exboot.RunUIDemand: Host, Tree, SC, DC required")
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

		dc.BeginFrame()
		dc.ClearWithColor(cfg.Clear)
		pc := &core.PaintContext{
			DC:    dc,
			Scale: host.ScaleFactor(),
			Theme: cfg.Theme,
		}
		if s.Theme != nil {
			pc.Theme = s.Theme
		}
		s.Tree.Frame(pc, core.Size{Width: float64(w), Height: float64(h)})

		if device != nil {
			device.FlushCallbacks()
		}
		frame, err := sc.BeginFrame()
		if err != nil {
			if errors.Is(err, webgpu.ErrDeviceLost) || (device != nil && device.IsLost()) {
				time.Sleep(16 * time.Millisecond)
				return nil
			}
			log.Printf("BeginFrame: %v", err)
			return nil
		}
		if _, err := dc.PresentFrameAuto(frame.Handle, frame.Width, frame.Height, func() error {
			return sc.EndFrame(frame)
		}); err != nil {
			sc.DiscardFrame(frame)
			if errors.Is(err, webgpu.ErrDeviceLost) || (device != nil && device.IsLost()) {
				time.Sleep(16 * time.Millisecond)
				return nil
			}
			log.Printf("PresentFrameAuto: %v", err)
			return nil
		}
		if cfg.Flush != nil {
			cfg.Flush()
		}
		return nil
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
		if cfg.OnResize != nil {
			cfg.OnResize(w, h)
		}
	}

	// Optional wall-clock stop for CI smokes; Quit wakes IDLE WaitEvents.
	// Seconds<=0 → unlimited (close window / signal to exit).
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
