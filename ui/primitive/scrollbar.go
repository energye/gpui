package primitive

import (
	"github.com/energye/gpui/render"
)

// ScrollbarVisibility controls when a bar axis is painted.
//
//	Never  — bar off (overflow:hidden chrome)
//	Auto   — only when content overflows
//	Always — paint track whenever the axis is enabled
//	Hover  — overflow + (pointer over viewport | thumb drag | recent wheel)
type ScrollbarVisibility int

const (
	ScrollbarNever ScrollbarVisibility = iota
	ScrollbarAuto
	ScrollbarAlways
	ScrollbarHover
)

// Scrollbar is a standalone chrome/policy object used by scroll containers
// (ScrollViewport, kit.Scroll, Tabs overflow, List, …). Not a layout child;
// hosts enable it and paint/hit-test via these settings.
//
// Common config (defaults in DefaultScrollbar):
//
//	sb := viewport.Scrollbar()
//	sb.Thickness = 8            // 轨道/条厚度（垂直=宽，水平=高）
//	sb.HoverThickness = 12      // 鼠标在条上时加宽/加高
//	sb.ShowTrack = true         // 是否画滑轨
//	sb.TrackColor = ...         // 滑轨颜色
//	sb.ThumbColor = ...         // 滑块颜色
//	sb.ExpandOnHover = true     // 移入条时用 HoverThickness
//
// Content never overlaps the bar when Overlay=false (default): layout insets
// use GutterThickness() = max(Thickness, HoverThickness when ExpandOnHover).
type Scrollbar struct {
	// Enabled master switch (false ⇒ Never on both axes).
	Enabled bool

	// Vertical / Horizontal visibility (default Auto).
	Vertical   ScrollbarVisibility
	Horizontal ScrollbarVisibility

	// Overlay when true allows content under the bar (not recommended).
	// Default false: content max size subtracts GutterThickness().
	Overlay bool

	// --- size (垂直条 Thickness=宽；水平条 Thickness=高) ---

	// Thickness idle bar size in logical px. Default 6. Min 1.
	Thickness float64
	// HoverThickness when pointer is over the bar strip (0 = same as Thickness).
	// Used when ExpandOnHover is true.
	HoverThickness float64
	// ExpandOnHover grows bar to HoverThickness while pointer is on the strip
	// (default true). Gutter reserves max(Thickness, HoverThickness) so expand
	// does not cover content.
	ExpandOnHover bool
	// MinThumb minimum thumb length along the scroll axis. Default 20.
	MinThumb float64
	// MaxThumbFraction caps thumb as fraction of track (0 = no cap; e.g. 0.9).
	MaxThumbFraction float64
	// Radius corner radius of thumb (0 → half of current thickness).
	Radius float64
	// TrackRadius corner radius of track (0 → same as Radius / half thickness).
	TrackRadius float64
	// Margin inset of the bar from the viewport edge (logical px). Default 0.
	// Reduces painted bar but gutter still uses full GutterThickness unless
	// MarginInsideGutter is true.
	Margin float64
	// Padding inside track around thumb (logical px). Default 1.
	Padding float64

	// --- track (滑轨) ---

	// ShowTrack paints the track under the thumb. Default true.
	// When false, only the thumb is drawn (macOS-like minimal chrome).
	ShowTrack bool
	// TrackColor when A>0 overrides default track fill.
	TrackColor render.RGBA
	// TrackHoverColor when A>0 used while pointer is on the bar strip.
	TrackHoverColor render.RGBA

	// --- thumb (滑块) ---

	// ThumbColor when A>0 overrides default thumb fill.
	ThumbColor render.RGBA
	// ThumbHoverColor when A>0 used while pointer on bar or dragging.
	ThumbHoverColor render.RGBA
	// ThumbActiveColor when A>0 used while dragging thumb.
	ThumbActiveColor render.RGBA

	// --- interaction ---

	// AutoHideDelay seconds after last wheel before Hover bars hide
	// (pointer outside). 0 → 1.0s.
	AutoHideDelay float64
	// DragThumb allows pointer-drag on thumb (default true).
	DragThumb bool
	// TrackClick jumps by ~PageFraction of viewport when clicking track (default true).
	TrackClick bool
	// WheelStep multiplies wheel DY/DX (0 → 1).
	WheelStep float64
	// PageFraction of content box used for track-click jumps (0 → 0.9).
	PageFraction float64

	// FadeTrack is deprecated alias for ShowTrack (kept for older call sites).
	// Prefer ShowTrack. When ShowTrack is left default-true and FadeTrack is
	// explicitly false via zero-value clone edge cases, ShowTrack wins if set
	// through SetShowTrack.
	FadeTrack bool
}

// DefaultScrollbar returns Auto + non-overlapping gutter with common chrome defaults.
func DefaultScrollbar() *Scrollbar {
	return &Scrollbar{
		Enabled:          true,
		Vertical:         ScrollbarAuto,
		Horizontal:       ScrollbarAuto,
		Overlay:          false,
		Thickness:        6,
		HoverThickness:   10,
		ExpandOnHover:    true,
		MinThumb:         20,
		MaxThumbFraction: 1,
		Radius:           0,
		TrackRadius:      0,
		Margin:           1,
		Padding:          1,
		ShowTrack:        true,
		FadeTrack:        true,
		AutoHideDelay:    1.0,
		DragThumb:        true,
		TrackClick:       true,
		WheelStep:        1,
		PageFraction:     0.9,
		// Default colors: light Ant-like
		TrackColor:       render.RGBA{R: 0, G: 0, B: 0, A: 0.06},
		ThumbColor:       render.RGBA{R: 0, G: 0, B: 0, A: 0.25},
		ThumbHoverColor:  render.RGBA{R: 0, G: 0, B: 0, A: 0.40},
		ThumbActiveColor: render.RGBA{R: 0, G: 0, B: 0, A: 0.55},
	}
}

// HoverScrollbar is overflow + hover reveal, non-overlapping gutter.
func HoverScrollbar() *Scrollbar {
	b := DefaultScrollbar()
	b.Vertical = ScrollbarHover
	b.Horizontal = ScrollbarHover
	return b
}

// AlwaysScrollbar paints track whenever the axis is enabled.
func AlwaysScrollbar() *Scrollbar {
	b := DefaultScrollbar()
	b.Vertical = ScrollbarAlways
	b.Horizontal = ScrollbarAlways
	return b
}

// NeverScrollbar disables chrome (wheel may still scroll).
func NeverScrollbar() *Scrollbar {
	b := DefaultScrollbar()
	b.Enabled = false
	b.Vertical = ScrollbarNever
	b.Horizontal = ScrollbarNever
	return b
}

// Clone returns a shallow copy.
func (b *Scrollbar) Clone() *Scrollbar {
	if b == nil {
		return DefaultScrollbar()
	}
	c := *b
	return &c
}

// --- fluent setters (chainable) ---

func (b *Scrollbar) SetEnabled(v bool) *Scrollbar {
	if b != nil {
		b.Enabled = v
	}
	return b
}

func (b *Scrollbar) SetVisibility(v, h ScrollbarVisibility) *Scrollbar {
	if b != nil {
		b.Vertical, b.Horizontal = v, h
		b.Enabled = v != ScrollbarNever || h != ScrollbarNever
	}
	return b
}

func (b *Scrollbar) SetOverlay(v bool) *Scrollbar {
	if b != nil {
		b.Overlay = v
	}
	return b
}

// SetThickness sets idle bar thickness (vertical width / horizontal height).
func (b *Scrollbar) SetThickness(px float64) *Scrollbar {
	if b != nil {
		if px < 1 {
			px = 1
		}
		b.Thickness = px
		if b.HoverThickness > 0 && b.HoverThickness < px {
			b.HoverThickness = px
		}
	}
	return b
}

// SetHoverThickness sets expanded size when pointer is on the bar (0 = no expand).
func (b *Scrollbar) SetHoverThickness(px float64) *Scrollbar {
	if b != nil {
		b.HoverThickness = px
		if px > 0 {
			b.ExpandOnHover = true
		}
	}
	return b
}

// SetExpandOnHover enables growing to HoverThickness when pointer is on the bar.
func (b *Scrollbar) SetExpandOnHover(v bool) *Scrollbar {
	if b != nil {
		b.ExpandOnHover = v
	}
	return b
}

func (b *Scrollbar) SetMinThumb(px float64) *Scrollbar {
	if b != nil {
		b.MinThumb = px
	}
	return b
}

func (b *Scrollbar) SetRadius(px float64) *Scrollbar {
	if b != nil {
		b.Radius = px
	}
	return b
}

func (b *Scrollbar) SetTrackRadius(px float64) *Scrollbar {
	if b != nil {
		b.TrackRadius = px
	}
	return b
}

func (b *Scrollbar) SetMargin(px float64) *Scrollbar {
	if b != nil {
		b.Margin = px
	}
	return b
}

func (b *Scrollbar) SetPadding(px float64) *Scrollbar {
	if b != nil {
		b.Padding = px
	}
	return b
}

// SetShowTrack toggles the rail/track under the thumb.
func (b *Scrollbar) SetShowTrack(v bool) *Scrollbar {
	if b != nil {
		b.ShowTrack = v
		b.FadeTrack = v
	}
	return b
}

func (b *Scrollbar) SetTrackColor(c render.RGBA) *Scrollbar {
	if b != nil {
		b.TrackColor = c
	}
	return b
}

func (b *Scrollbar) SetThumbColor(c render.RGBA) *Scrollbar {
	if b != nil {
		b.ThumbColor = c
	}
	return b
}

func (b *Scrollbar) SetThumbHoverColor(c render.RGBA) *Scrollbar {
	if b != nil {
		b.ThumbHoverColor = c
	}
	return b
}

func (b *Scrollbar) SetThumbActiveColor(c render.RGBA) *Scrollbar {
	if b != nil {
		b.ThumbActiveColor = c
	}
	return b
}

// SetColors sets track + thumb (+ optional hover). Pass A=0 to skip a slot.
func (b *Scrollbar) SetColors(track, thumb, thumbHover render.RGBA) *Scrollbar {
	if b == nil {
		return b
	}
	if track.A > 0 {
		b.TrackColor = track
	}
	if thumb.A > 0 {
		b.ThumbColor = thumb
	}
	if thumbHover.A > 0 {
		b.ThumbHoverColor = thumbHover
	}
	return b
}

func (b *Scrollbar) SetAutoHideDelay(sec float64) *Scrollbar {
	if b != nil {
		b.AutoHideDelay = sec
	}
	return b
}

func (b *Scrollbar) SetDragThumb(v bool) *Scrollbar {
	if b != nil {
		b.DragThumb = v
	}
	return b
}

func (b *Scrollbar) SetTrackClick(v bool) *Scrollbar {
	if b != nil {
		b.TrackClick = v
	}
	return b
}

func (b *Scrollbar) SetWheelStep(m float64) *Scrollbar {
	if b != nil {
		b.WheelStep = m
	}
	return b
}

// GutterThickness is the layout inset reserved for the bar (content never under it).
// Uses max(Thickness, HoverThickness) when ExpandOnHover so expand stays in gutter.
func (b *Scrollbar) GutterThickness() float64 {
	t := b.idleThickness()
	if b != nil && b.ExpandOnHover {
		h := b.HoverThickness
		if h > t {
			return h
		}
	}
	return t
}

func (b *Scrollbar) idleThickness() float64 {
	if b == nil {
		return 6
	}
	t := b.Thickness
	if t <= 0 {
		t = 6
	}
	return t
}

// thickness resolves paint size (expand when onBar && ExpandOnHover).
func (b *Scrollbar) thickness(onBar bool) float64 {
	t := b.idleThickness()
	if b != nil && onBar && b.ExpandOnHover {
		h := b.HoverThickness
		if h > t {
			return h
		}
	}
	return t
}

func (b *Scrollbar) minThumb() float64 {
	if b == nil || b.MinThumb <= 0 {
		return 20
	}
	return b.MinThumb
}

func (b *Scrollbar) maxThumbFraction() float64 {
	if b == nil || b.MaxThumbFraction <= 0 {
		return 1
	}
	if b.MaxThumbFraction > 1 {
		return 1
	}
	return b.MaxThumbFraction
}

func (b *Scrollbar) radius(bar float64) float64 {
	if b != nil && b.Radius > 0 {
		return b.Radius
	}
	return bar / 2
}

func (b *Scrollbar) trackRadius(bar float64) float64 {
	if b != nil && b.TrackRadius > 0 {
		return b.TrackRadius
	}
	return b.radius(bar)
}

func (b *Scrollbar) margin() float64 {
	if b == nil || b.Margin < 0 {
		return 0
	}
	return b.Margin
}

func (b *Scrollbar) padding() float64 {
	if b == nil || b.Padding < 0 {
		return 0
	}
	return b.Padding
}

func (b *Scrollbar) showTrack() bool {
	if b == nil {
		return true
	}
	// Prefer ShowTrack; FadeTrack kept as synonym for older code.
	if !b.ShowTrack && !b.FadeTrack {
		return false
	}
	if b.ShowTrack {
		return true
	}
	return b.FadeTrack
}

func (b *Scrollbar) autoHideDelay() float64 {
	if b == nil || b.AutoHideDelay <= 0 {
		return 1.0
	}
	return b.AutoHideDelay
}

func (b *Scrollbar) wheelStep() float64 {
	if b == nil || b.WheelStep <= 0 {
		return 1
	}
	return b.WheelStep
}

func (b *Scrollbar) pageFraction() float64 {
	if b == nil || b.PageFraction <= 0 {
		return 0.9
	}
	return b.PageFraction
}

func (b *Scrollbar) dragEnabled() bool {
	return b == nil || b.DragThumb
}

func (b *Scrollbar) trackClickEnabled() bool {
	return b == nil || b.TrackClick
}

func (b *Scrollbar) shouldShow(v ScrollbarVisibility, overflow, hovered, dragging, reveal bool) bool {
	if b != nil && !b.Enabled {
		return false
	}
	switch v {
	case ScrollbarNever:
		return false
	case ScrollbarAlways:
		return true
	case ScrollbarAuto:
		return overflow
	case ScrollbarHover:
		if !overflow {
			return false
		}
		return hovered || dragging || reveal
	default:
		return overflow
	}
}

func (b *Scrollbar) trackCol(hover bool) render.RGBA {
	if hover && b != nil && b.TrackHoverColor.A > 0 {
		return b.TrackHoverColor
	}
	if b != nil && b.TrackColor.A > 0 {
		return b.TrackColor
	}
	return render.RGBA{R: 0, G: 0, B: 0, A: 0.06}
}

func (b *Scrollbar) thumbCol(hover, active bool) render.RGBA {
	if active && b != nil && b.ThumbActiveColor.A > 0 {
		return b.ThumbActiveColor
	}
	if hover && b != nil && b.ThumbHoverColor.A > 0 {
		return b.ThumbHoverColor
	}
	if b != nil && b.ThumbColor.A > 0 {
		return b.ThumbColor
	}
	if active {
		return render.RGBA{R: 0, G: 0, B: 0, A: 0.55}
	}
	if hover {
		return render.RGBA{R: 0, G: 0, B: 0, A: 0.4}
	}
	return render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
}
