package app

import (
	"errors"
	"log"
	"os"
	"sync"
	"time"

	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/layer"
)

// MinPresentSize is the floor for paint/configure dimensions (avoids zero surfaces).
const MinPresentSize = 64

// UseCompositor reports whether retained dual-band composition is enabled.
// Default ON (required for Modal/Drawer above ScrollViewport layers; MAP §4.1).
// Opt out with GPUI_COMPOSITOR=0|false|off (debug only).
func UseCompositor() bool {
	v := os.Getenv("GPUI_COMPOSITOR")
	return v != "0" && v != "false" && v != "off"
}

// OwnedPresentConfig configures the framework-owned Present path
// (compositor dual-band → blit → PresentFrame*).
//
// See docs/UI_FOUNDATION_P0.md P0.5 and docs/UI_APP_SHELL_PLAN.md.
type OwnedPresentConfig struct {
	// SC / DC are required for GPU present.
	SC *webgpu.Swapchain
	DC *render.Context
	// Device optional: FlushCallbacks + device-lost recovery.
	Device *webgpu.Device
	// Clear is the main-band background (base RT / surface).
	Clear render.RGBA
	// Theme fallback when Session.Theme is nil.
	Theme *core.Theme
	// Flush optional (e.g. LinuxHost.Flush after present).
	Flush func()
	// OnLayoutSize is called when the paint size changes (layout viewport).
	// Do not call SC.Resize here — OwnedPresenter configures in Present.
	OnLayoutSize func(w, h int)
	// DisableCompositor forces direct Tree.Frame into the surface DC.
	// When false, UseCompositor() env decides (default ON).
	DisableCompositor bool
	// MinSize overrides MinPresentSize when > 0.
	MinSize int
}

// OwnedPresenter owns a Compositor and produces a PresentFunc for Application.Attach.
type OwnedPresenter struct {
	cfg           OwnedPresentConfig
	comp          *layer.Compositor
	lastLayoutW   int
	lastLayoutH   int
	logScaleOnce  sync.Once
	minSize       int
	useCompositor bool
}

// NewOwnedPresenter builds a presenter. Call Release when the session ends.
func NewOwnedPresenter(cfg OwnedPresentConfig) *OwnedPresenter {
	min := cfg.MinSize
	if min < 1 {
		min = MinPresentSize
	}
	p := &OwnedPresenter{
		cfg:           cfg,
		minSize:       min,
		useCompositor: !cfg.DisableCompositor && UseCompositor(),
	}
	if p.useCompositor {
		p.comp = layer.NewCompositor()
		p.comp.BG = cfg.Clear
	}
	return p
}

// Release drops retained compositor surfaces. Safe to call multiple times.
func (p *OwnedPresenter) Release() {
	if p == nil {
		return
	}
	if p.comp != nil {
		p.comp.ReleaseAll()
		p.comp = nil
	}
}

// Func returns a PresentFunc suitable for Application.Attach.
func (p *OwnedPresenter) Func() PresentFunc {
	if p == nil {
		return nil
	}
	return p.Present
}

// Present implements PresentFunc (dual-band compositor or direct).
func (p *OwnedPresenter) Present(s *Session) error {
	if p == nil || s == nil {
		return errors.New("app: nil presenter or session")
	}
	cfg := p.cfg
	if cfg.SC == nil || cfg.DC == nil {
		return errors.New("app: OwnedPresent requires SC and DC")
	}
	dc, sc, device := cfg.DC, cfg.SC, cfg.Device

	// Live client size from session (app.runFrame keeps Width/Height in sync).
	liveW, liveH := s.Width, s.Height
	if s.Host != nil {
		if hw, hh := s.Host.Size(); hw > 0 && hh > 0 {
			liveW, liveH = hw, hh
		}
	}
	if liveW < p.minSize {
		liveW = p.minSize
	}
	if liveH < p.minSize {
		liveH = p.minSize
	}

	surfW, surfH := int(sc.Width), int(sc.Height)
	if surfW < 1 {
		surfW = liveW
	}
	if surfH < 1 {
		surfH = liveH
	}

	// FOUNDATION: UI demand loop uses logical scale=1 (hit == layout == paint).
	scale := 1.0
	hostScale := 1.0
	if s.Host != nil {
		hostScale = s.Host.ScaleFactor()
		if hostScale <= 0 {
			hostScale = 1
		}
	}
	p.logScaleOnce.Do(func() {
		log.Printf("ui/app: UI logical scale=1 hostDPR=%.2f compositor=%v", hostScale, p.useCompositor && p.comp != nil)
	})

	paintW, paintH := liveW, liveH
	doConfigure := paintW != surfW || paintH != surfH

	if p.lastLayoutW != paintW || p.lastLayoutH != paintH {
		s.Width, s.Height = paintW, paintH
		p.lastLayoutW, p.lastLayoutH = paintW, paintH
		if cfg.OnLayoutSize != nil {
			cfg.OnLayoutSize(paintW, paintH)
		}
		if s.Tree != nil {
			s.Tree.MarkFullPaintRequired()
		}
	}

	theme := themeOf(s, cfg.Theme)

	if p.useCompositor && p.comp != nil {
		p.comp.BG = cfg.Clear
		p.comp.Resize(paintW, paintH, 1)
		full := true
		if s.Tree != nil {
			full = s.Tree.FullPaintRequired() || doConfigure || s.Tree.NonBoundaryPaintDirty()
		}
		if !p.comp.HasBase() {
			full = true
		}
		if !p.comp.Frame(s.Tree, theme, full) || !p.comp.HasBase() {
			if s.Tree != nil && s.Tree.HasOverlays() {
				log.Printf("ui/app: compositor base failed with overlays — direct present may mis-order Modal vs Scroll (MAP §4.1)")
			}
			if doConfigure {
				_ = sc.Resize(uint32(paintW), uint32(paintH))
				_ = dc.Resize(paintW, paintH)
			}
			return p.presentDirect(s, dc, sc, device, paintW, paintH, scale, theme)
		}
		if doConfigure {
			if err := sc.Resize(uint32(paintW), uint32(paintH)); err != nil {
				log.Printf("ui/app: swapchain resize %dx%d: %v", paintW, paintH, err)
			}
			if err := dc.Resize(paintW, paintH); err != nil {
				log.Printf("ui/app: dc resize %dx%d: %v", paintW, paintH, err)
			}
			if s.Tree != nil {
				s.Tree.MarkFullPaintRequired()
			}
		}
		return p.blitAndPresent(s, dc, sc, device, paintW, paintH, liveW, liveH, full)
	}

	if doConfigure {
		if err := sc.Resize(uint32(paintW), uint32(paintH)); err != nil {
			log.Printf("ui/app: swapchain resize %dx%d: %v", paintW, paintH, err)
		}
		if err := dc.Resize(paintW, paintH); err != nil {
			log.Printf("ui/app: dc resize %dx%d: %v", paintW, paintH, err)
		}
	}
	return p.presentDirect(s, dc, sc, device, paintW, paintH, scale, theme)
}

func (p *OwnedPresenter) blitAndPresent(
	s *Session,
	dc *render.Context,
	sc *webgpu.Swapchain,
	device *webgpu.Device,
	paintW, paintH, liveW, liveH int,
	_ bool,
) error {
	if dc.Width() != paintW || dc.Height() != paintH {
		_ = dc.Resize(paintW, paintH)
	}
	dc.BeginFrame()
	p.comp.BlitTo(dc)
	if !p.comp.HasBase() {
		return errors.New("ui/app: compositor base missing after blit")
	}
	if device != nil {
		device.FlushCallbacks()
	}
	frame, err := sc.BeginFrame()
	if err != nil {
		if errors.Is(err, webgpu.ErrDeviceLost) || (device != nil && device.IsLost()) {
			p.comp.ReleaseAll()
			if s.Tree != nil {
				s.Tree.MarkFullPaintRequired()
			}
			time.Sleep(16 * time.Millisecond)
			return nil
		}
		if liveW >= p.minSize && liveH >= p.minSize {
			_ = sc.Resize(uint32(liveW), uint32(liveH))
			_ = dc.Resize(liveW, liveH)
			if p.comp != nil {
				p.comp.Resize(liveW, liveH, 1)
			}
		}
		if s.Tree != nil {
			s.Tree.MarkFullPaintRequired()
		}
		log.Printf("ui/app: BeginFrame: %v (will retry)", err)
		return nil
	}
	if err := dc.PresentFrameFull(frame.Handle, frame.Width, frame.Height, func() error {
		return sc.EndFrame(frame)
	}); err != nil {
		sc.DiscardFrame(frame)
		if errors.Is(err, webgpu.ErrDeviceLost) || (device != nil && device.IsLost()) {
			p.comp.ReleaseAll()
			if s.Tree != nil {
				s.Tree.MarkFullPaintRequired()
			}
			time.Sleep(16 * time.Millisecond)
			return nil
		}
		log.Printf("ui/app: PresentFrameFull: %v", err)
		if s.Tree != nil {
			s.Tree.MarkFullPaintRequired()
		}
		return nil
	}
	if p.cfg.Flush != nil {
		p.cfg.Flush()
	}
	return nil
}

func (p *OwnedPresenter) presentDirect(
	s *Session,
	dc *render.Context,
	sc *webgpu.Swapchain,
	device *webgpu.Device,
	w, h int,
	scale float64,
	theme *core.Theme,
) error {
	if w < p.minSize {
		w = p.minSize
	}
	if h < p.minSize {
		h = p.minSize
	}
	_ = scale
	if dc.Width() != w || dc.Height() != h {
		_ = dc.Resize(w, h)
	}
	if dc.DeviceScale() != 1 {
		dc.SetDeviceScale(1)
	}

	clear := p.cfg.Clear
	dc.BeginFrame()
	dc.SetRGBA(clear.R, clear.G, clear.B, clear.A)
	if clear.A <= 0 {
		dc.SetRGBA(0.1, 0.1, 0.1, 1)
	}
	dc.DrawRectangle(0, 0, float64(w), float64(h))
	_ = dc.Fill()
	dc.MarkFullRedraw()

	if s.Tree != nil {
		pc := &core.PaintContext{
			DC:            dc,
			Scale:         1,
			Theme:         theme,
			CompositeOnly: false,
		}
		s.Tree.Frame(pc, core.Size{Width: float64(w), Height: float64(h)})
	}

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
		log.Printf("ui/app: BeginFrame: %v", err)
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
		log.Printf("ui/app: PresentFrameFull: %v", err)
		if s.Tree != nil {
			s.Tree.MarkFullPaintRequired()
		}
		return nil
	}
	if p.cfg.Flush != nil {
		p.cfg.Flush()
	}
	return nil
}

func themeOf(s *Session, fallback *core.Theme) *core.Theme {
	if s != nil && s.Theme != nil {
		return s.Theme
	}
	return fallback
}

// PaintCompositorFrame runs dual-band composition into an offscreen path
// (no swapchain). Returns false when GPU offscreen raster is unavailable.
// Used by tests and non-window hosts that only need Frame+Blit semantics.
func PaintCompositorFrame(tree *core.Tree, theme *core.Theme, w, h int, clear render.RGBA, full bool) (comp *layer.Compositor, ok bool) {
	if tree == nil || w < 1 || h < 1 {
		return nil, false
	}
	comp = layer.NewCompositor()
	comp.BG = clear
	comp.Resize(w, h, 1)
	if !comp.Frame(tree, theme, full) || !comp.HasBase() {
		comp.ReleaseAll()
		return nil, false
	}
	return comp, true
}
