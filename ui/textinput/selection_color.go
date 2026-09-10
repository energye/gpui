package textinput

import "github.com/energye/gpui/ui/rendering"

// 全局选中色，未设置时用引擎默认 0.22,0.45,0.85,0.35（Flutter 选区蓝）
var (
	defaultSelR, defaultSelG, defaultSelB, defaultSelA = 0.22, 0.45, 0.85, 0.35
	hasDefaultSel                                      bool
)

// SetDefaultSelectionColor 设置全局选中色（全局未设时回退到引擎默认）
func SetDefaultSelectionColor(r, g, b, a float64) {
	defaultSelR, defaultSelG, defaultSelB, defaultSelA = r, g, b, a
	hasDefaultSel = true
}

// ClearDefaultSelectionColor 清掉全局，回到引擎默认
func ClearDefaultSelectionColor() { hasDefaultSel = false }

// DefaultSelectionColor 返回当前全局色（未设时即引擎默认）
func DefaultSelectionColor() (r, g, b, a float64) {
	if hasDefaultSel {
		return defaultSelR, defaultSelG, defaultSelB, defaultSelA
	}
	return 0.22, 0.45, 0.85, 0.35
}

func resolveSelColor(hasCustom bool, cr, cg, cb, ca float64) (r, g, b, a float64) {
	if hasCustom {
		return cr, cg, cb, ca
	}
	return DefaultSelectionColor()
}

// clipHighlightRect intersects a highlight rect (owner coords) with the
// visible window. A fully scrolled-out range reports ok=false so the caller
// builds nothing: an invisible box must not enter the tree — the retained
// texture path records the origin-anchored slice, so a giant box covering
// scrolled text never shows its background (B/C double-click report).
// Flutter paints selection per visible line for the same reason. Every
// scroll path re-runs sync (directly or via selection change), so the clip
// tracks the window.
func clipHighlightRect(hl, vis rendering.Rect) (clipped rendering.Rect, ok bool) {
	// NOTE: plain if-clamps, not min/max — this package already defines
	// min/max helpers for other types (see input_box.go).
	x0, y0 := hl.Min.X, hl.Min.Y
	x1, y1 := hl.Max.X, hl.Max.Y
	if x0 < vis.Min.X {
		x0 = vis.Min.X
	}
	if y0 < vis.Min.Y {
		y0 = vis.Min.Y
	}
	if x1 > vis.Max.X {
		x1 = vis.Max.X
	}
	if y1 > vis.Max.Y {
		y1 = vis.Max.Y
	}
	if x1 <= x0 || y1 <= y0 {
		return rendering.Rect{}, false
	}
	return rendering.Rect{
		Min: rendering.Point{X: x0, Y: y0},
		Max: rendering.Point{X: x1, Y: y1},
	}, true
}

// newClippedHighlight builds the visible slice of one selection highlight:
// clip to the window, size the box to the slice, pin its owner position and
// flag it so flow layout paints it without resize/reposition. Nil when the
// range is fully scrolled out — the caller then appends nothing.
func newClippedHighlight(hl, vis rendering.Rect, r, g, b, a float64) *rendering.RenderColorBox {
	c, ok := clipHighlightRect(hl, vis)
	if !ok {
		return nil
	}
	s := c.Size()
	hb := rendering.NewRenderColorBox(s.Width, s.Height, r, g, b, a)
	hb.MoveTo(c.Min.X, c.Min.Y)
	hb.SetManualLayout(true)
	return hb
}
