package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Scroll is Ant Design–style scrollable region (overflow + optional bar chrome).
// Composition: ScrollViewport (clip + offset + thumb paint).
//
//	https://ant.design/components/ — use as overflow container for Tabs / List / Table.
type Scroll struct {
	Root  *primitive.ScrollViewport
	Theme *core.Theme
}

// NewScroll wraps content in a vertical ScrollViewport with scrollbar chrome.
func NewScroll(content core.Node) *Scroll {
	sv := primitive.NewScrollViewport(content)
	sv.ShowScrollbar = true
	sv.SetAxis(true, false)
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
}

// SetBarColors overrides track/thumb colors (A=0 keeps defaults).
func (s *Scroll) SetBarColors(track, thumb render.RGBA) {
	if s == nil || s.Root == nil {
		return
	}
	s.Root.TrackColor = track
	s.Root.ThumbColor = thumb
}

// Viewport returns the underlying primitive for advanced control.
func (s *Scroll) Viewport() *primitive.ScrollViewport {
	if s == nil {
		return nil
	}
	return s.Root
}
