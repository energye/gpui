//go:build linux && !nogpu

package exboot

import (
	"log"
	"time"

	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/app"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/platform"
)

// UIDemandConfig drives a gogpu-aligned demand frame loop for UI smokes.
//
// Present is owned by ui/app.OwnedPresenter (compositor dual-band by default;
// GPUI_COMPOSITOR=0 for direct Tree.Frame). See docs/UI_FOUNDATION_P0.md P0.5.
//
// Resize policy (framework — not per-example):
//
//  1. Every ConfigureNotify updates live client size (coalesced by ui/app).
//  2. Each present paints at the LIVE window size so content tracks the window.
//  3. Surface.Configure is applied inside OwnedPresenter with the latest size.
//  4. X11 LinuxHost uses back_pixmap=None + NW gravity (anti-flash).
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
	// OnResize is layout-only (root width/height / viewport). Called when paint size changes.
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

// ResizeConfigureIdle is retained for callers that still reference the constant.
// Quiet-configure is handled inside the present path via live size vs surface size.
const ResizeConfigureIdle = 32 * time.Millisecond

// RunUIDemand attaches host+tree to ui/app and runs the demand loop.
// Present uses app.NewOwnedPresenter (framework default dual-band path).
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

	presenter := app.NewOwnedPresenter(app.OwnedPresentConfig{
		SC:     cfg.SC,
		DC:     cfg.DC,
		Device: cfg.Device,
		Clear:  cfg.Clear,
		Theme:  cfg.Theme,
		Flush:  cfg.Flush,
		OnLayoutSize: func(w, h int) {
			if cfg.OnResize != nil {
				cfg.OnResize(w, h)
			}
		},
	})
	defer presenter.Release()

	a := app.New(app.Options{
		ContinuousRender: cfg.Continuous,
		OnUpdate:         cfg.OnUpdate,
		BeforeDispatch:   cfg.BeforeDispatch,
	})
	defer a.Close()

	sess := a.Attach(cfg.Host, cfg.Tree, presenter.Func())
	if cfg.Theme != nil {
		sess.Theme = cfg.Theme
	}
	if w, h := cfg.Host.Size(); w > 0 && h > 0 {
		sess.Width, sess.Height = w, h
	}

	// Track live size for app session; Present paints at Host.Size / Session size.
	sess.OnResize = func(w, h int) {
		if w < app.MinPresentSize {
			w = app.MinPresentSize
		}
		if h < app.MinPresentSize {
			h = app.MinPresentSize
		}
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
