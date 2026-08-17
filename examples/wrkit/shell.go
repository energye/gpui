package wrkit

import (
	"fmt"

	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
)

// ShellChrome is the shared U17 top-bar + legend + HUD layout for ui_wr_* quality bars.
// Ability-specific body content is placed by the example after Attach.
type ShellChrome struct {
	Root   *rendering.AbsoluteBox
	Top    *Panel
	Legend *Panel
	Body   *Panel // main content band (example fills)
	HUD    *LiveHUD
	// WinW/WinH logical client.
	WinW, WinH float64
	HUDH       float64
}

// NewShell builds root + TopBar + Legend + empty Body panel + optional LiveHUD.
// bodyX is the left edge of the body panel (after legend); legendW default 260.
func NewShell(winW, winH float64, abilityTitle string, legendLines []string) *ShellChrome {
	const topH, legendW, hudH, gap = 48.0, 260.0, 72.0, 12.0
	s := &ShellChrome{WinW: winW, WinH: winH, HUDH: hudH}
	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: 0.08, G: 0.09, B: 0.11, A: 1}
	s.Root = root

	s.Top = NewPanel(winW, topH, 0.12, 0.14, 0.18, 1)
	s.Top.PlaceOn(root, 0, 0)
	s.Top.LabelAt(abilityTitle, 14, 16, 14, 0.88, 0.92, 0.98)

	legH := winH - topH - hudH - gap*2
	if legH < 200 {
		legH = 200
	}
	s.Legend = NewPanel(legendW, legH, 0.11, 0.12, 0.15, 1)
	s.Legend.PlaceOn(root, gap, topH+gap)
	s.Legend.LabelAt("LEGEND", 13, 12, 10, 0.55, 0.75, 0.95)
	for i, ln := range legendLines {
		if i >= 12 {
			break
		}
		s.Legend.LabelAt(ln, 12, 12, 36+float64(i)*22, 0.70, 0.78, 0.88)
	}

	bodyX := gap + legendW + gap
	bodyW := winW - bodyX - gap
	bodyH := legH
	s.Body = NewPanel(bodyW, bodyH, 0.10, 0.11, 0.13, 1)
	s.Body.PlaceOn(root, bodyX, topH+gap)

	if HUDEnabled() {
		s.HUD = NewLiveHUD(winW, hudH)
		root.Place(s.HUD.Box, 0, winH-hudH)
	}
	return s
}

// Resize re-lays the shell for a new window size (responsive layout): the root
// tracks the window, top/legend/body panels re-size, and the HUD is pinned to
// the new bottom edge. Call from the example's EventResize handler. Nodes whose
// size actually changed mark themselves dirty via MarkNeedsLayout, so only the
// affected panels re-layout/re-paint (no blanket full repaint).
func (s *ShellChrome) Resize(winW, winH float64) {
	if s == nil || s.Root == nil {
		return
	}
	const topH, legendW, hudH, gap = 48.0, 260.0, 72.0, 12.0
	if winW < legendW+3*gap {
		winW = legendW + 3*gap
	}
	if winH < topH+hudH+3*gap {
		winH = topH + hudH + 3*gap
	}
	s.WinW, s.WinH = winW, winH

	legH := winH - topH - hudH - gap*2
	if legH < 200 {
		legH = 200
	}
	bodyX := gap + legendW + gap
	bodyW := winW - bodyX - gap
	if bodyW < 100 {
		bodyW = 100
	}

	s.Root.FixedWidth, s.Root.FixedHeight = winW, winH
	s.Root.MarkNeedsLayout()

	s.Top.W, s.Top.H = winW, topH
	s.Top.Box.FixedWidth, s.Top.Box.FixedHeight = winW, topH
	s.Top.Box.MarkNeedsLayout()

	s.Legend.W, s.Legend.H = legendW, legH
	s.Legend.Box.FixedWidth, s.Legend.Box.FixedHeight = legendW, legH
	s.Legend.Box.MarkNeedsLayout()

	s.Body.W, s.Body.H = bodyW, legH
	s.Body.Box.FixedWidth, s.Body.Box.FixedHeight = bodyW, legH
	s.Body.Box.MarkNeedsLayout()

	if s.HUD != nil {
		s.HUD.Width, s.HUD.Height = winW, hudH
		s.HUD.Box.FixedWidth = winW
		s.Root.Place(s.HUD.Box, 0, winH-hudH)
		s.HUD.Box.MarkNeedsLayout()
	}
}

// UpdateHUD fills LiveHUD from a metrics snapshot + phase (no-op if HUD off).
func (s *ShellChrome) UpdateHUD(ability, phase string, app *embedder.PipelineApp, gateOK bool, core, extra string) {
	if s == nil || s.HUD == nil || app == nil {
		return
	}
	snap := app.Metrics().Snapshot()
	fps := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fps = 1000.0 / snap.AvgFrameIntervalMs
	}
	pol := snap.PresentPolicy
	if pol == "" {
		pol = scheduler.PresentPolicyFullPaint
	}
	s.HUD.Update(Snap{
		AbilityID:   ability,
		Phase:       phase,
		FPS:         fps,
		P95Ms:       snap.P95FrameIntervalMs,
		Policy:      pol,
		PresentMode: snap.PresentMode,
		PaintCount:  snap.PaintCount,
		Presents:    app.PresentCount(),
		Core:        core,
		GateOK:      gateOK,
		Extra:       extra,
	})
}

// NoteHUDTick advances HUD refresh budget.
func (s *ShellChrome) NoteHUDTick(dt float64) {
	if s != nil && s.HUD != nil {
		s.HUD.NoteTick(dt)
	}
}

// MergeBoundaryCache lifts cache lifetime counters into snap when metrics lag.
func MergeBoundaryCache(app *embedder.PipelineApp, snap *scheduler.FrameMetrics) {
	if app == nil || snap == nil {
		return
	}
	if cache := app.BoundaryCache(); cache != nil {
		if snap.BoundarySkip < cache.Skip {
			snap.BoundarySkip = cache.Skip
			snap.BoundaryRerecord = cache.Rerecord
		}
	}
}

// FmtSkip core string helper.
func FmtSkip(skip, rr int64) string {
	return fmt.Sprintf("skip=%d rr=%d", skip, rr)
}
