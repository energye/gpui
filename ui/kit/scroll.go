package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Scroll is Ant Design–style scrollable region (overflow + scrollbar chrome).
// Composition: ScrollViewport (clip + offset) + independent Scrollbar policy.
//
// Default: Auto visibility + non-overlay gutters — content layout width/height
// subtracts bar Thickness so content and styles never overlap the bar strip.
//
//	sc := kit.NewScroll(content)
//	sb := sc.Scrollbar()           // mutable policy for custom config
//	sb.Vertical = primitive.ScrollbarHover
//	sb.Thickness = 8
//
// https://ant.design/components/ — overflow container for Tabs / List / Table.
type Scroll struct {
	Root  *primitive.ScrollViewport
	Theme *core.Theme
}

// NewScroll wraps content in a vertical ScrollViewport with default Auto bars.
func NewScroll(content core.Node) *Scroll {
	sv := primitive.NewScrollViewport(content)
	sv.SetAxis(true, false)
	sv.SetScrollbar(primitive.DefaultScrollbar())
	// Vertical-only: disable horizontal bar policy so no bottom gutter.
	sb := sv.Scrollbar()
	sb.Horizontal = primitive.ScrollbarNever
	return &Scroll{Root: sv}
}

// Node returns the scroll viewport.
func (s *Scroll) Node() core.Node {
	if s == nil || s.Root == nil {
		return nil
	}
	return s.Root
}

// SetSize fixes viewport size (0 = take parent max).
func (s *Scroll) SetSize(w, h float64) {
	if s == nil || s.Root == nil {
		return
	}
	s.Root.Width, s.Root.Height = w, h
}

// SetAxis configures vertical and/or horizontal scrolling.
func (s *Scroll) SetAxis(vertical, horizontal bool) {
	if s == nil || s.Root == nil {
		return
	}
	s.Root.SetAxis(vertical, horizontal)
	sb := s.Root.Scrollbar()
	if vertical && !horizontal {
		sb.Horizontal = primitive.ScrollbarNever
	}
	if horizontal && !vertical {
		sb.Vertical = primitive.ScrollbarNever
	}
}

// SetScrollbar installs a full scrollbar policy (nil → DefaultScrollbar).
func (s *Scroll) SetScrollbar(bar *primitive.Scrollbar) {
	if s == nil || s.Root == nil {
		return
	}
	s.Root.SetScrollbar(bar)
}

// Scrollbar returns the mutable scrollbar policy for custom configuration.
// Callers may change Visibility / Overlay / Thickness / colors, then layout/paint.
func (s *Scroll) Scrollbar() *primitive.Scrollbar {
	if s == nil || s.Root == nil {
		return primitive.DefaultScrollbar()
	}
	return s.Root.Scrollbar()
}

// SetShowScrollbar master enable (false = Never; true keeps visibility policy).
func (s *Scroll) SetShowScrollbar(v bool) {
	if s == nil || s.Root == nil {
		return
	}
	s.Root.SetShowScrollbar(v)
}

// SetScrollbarVisibility sets both enabled axes to the same visibility mode.
//
//	Never | Auto | Always | Hover
func (s *Scroll) SetScrollbarVisibility(v primitive.ScrollbarVisibility) {
	if s == nil || s.Root == nil {
		return
	}
	s.Root.SetScrollbarVisibility(v)
}

// SetBarColors overrides track/thumb colors (A=0 keeps defaults).
func (s *Scroll) SetBarColors(track, thumb render.RGBA) {
	if s == nil || s.Root == nil {
		return
	}
	s.Scrollbar().SetColors(track, thumb, render.RGBA{})
	s.Root.MarkNeedsPaint()
}

// SetBarThickness sets idle bar thickness (vertical=width, horizontal=height).
// Content gutters update on next layout via GutterThickness().
func (s *Scroll) SetBarThickness(px float64) {
	if s == nil || s.Root == nil {
		return
	}
	s.Scrollbar().SetThickness(px)
	s.Root.BarSize = px
	s.Root.MarkNeedsLayout()
	s.Root.MarkNeedsPaint()
}

// SetBarHoverThickness sets expanded size when pointer is on the bar.
func (s *Scroll) SetBarHoverThickness(px float64) {
	if s == nil || s.Root == nil {
		return
	}
	s.Scrollbar().SetHoverThickness(px)
	s.Root.MarkNeedsLayout()
	s.Root.MarkNeedsPaint()
}

// SetShowTrack toggles painting the rail under the thumb.
func (s *Scroll) SetShowTrack(v bool) {
	if s == nil || s.Root == nil {
		return
	}
	s.Scrollbar().SetShowTrack(v)
	s.Root.MarkNeedsPaint()
}

// SetExpandOnHover enables bar grow on pointer-over-strip.
func (s *Scroll) SetExpandOnHover(v bool) {
	if s == nil || s.Root == nil {
		return
	}
	s.Scrollbar().SetExpandOnHover(v)
	s.Root.MarkNeedsLayout()
	s.Root.MarkNeedsPaint()
}

// SetOverlay when false (default) content never overlaps bars.
func (s *Scroll) SetOverlay(overlay bool) {
	if s == nil || s.Root == nil {
		return
	}
	s.Scrollbar().SetOverlay(overlay)
	s.Root.MarkNeedsLayout()
	s.Root.MarkNeedsPaint()
}

// ConfigureScrollbar applies a mutator to the live Scrollbar then dirties layout/paint.
func (s *Scroll) ConfigureScrollbar(fn func(*primitive.Scrollbar)) {
	if s == nil || s.Root == nil || fn == nil {
		return
	}
	fn(s.Scrollbar())
	s.Root.MarkNeedsLayout()
	s.Root.MarkNeedsPaint()
}

// AttachTicker enables Hover auto-hide countdown after wheel.
func (s *Scroll) AttachTicker(t *core.Tree) {
	if s != nil && s.Root != nil {
		s.Root.AttachTicker(t)
	}
}

// Viewport returns the underlying primitive for advanced control.
func (s *Scroll) Viewport() *primitive.ScrollViewport {
	if s == nil {
		return nil
	}
	return s.Root
}
