package main

import (
	"github.com/energye/gpui/ui/overlay"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// Layout constants (logical px). Temp nav: plain list, replaced by Tabs later.
const (
	galleryW   = 1200.0
	galleryH   = 800.0
	navW       = 240.0
	navItemH   = 36.0
	contentPad = 16.0
)

// Scene is the gallery graph (example-only; not under ui/).
type Scene struct {
	Width, Height float64
	Root          *rendering.AbsoluteBox
	Theme         *theme.Provider
	Overlay       *overlay.State
	Selected      string
	navBoxes      []*rendering.RenderColorBox
	navNames      []string
}

// NewScene builds the shell; selected picks the initial tab.
func NewScene(w, h float64, selected string) *Scene {
	if w <= 0 {
		w = galleryW
	}
	if h <= 0 {
		h = galleryH
	}
	s := &Scene{
		Width:   w,
		Height:  h,
		Theme:   theme.NewProvider(theme.DefaultTokens()),
		Overlay: overlay.New(),
	}
	if _, ok := Lookup(selected); !ok {
		selected = "overview"
	}
	s.Selected = selected
	s.Root = rendering.NewAbsoluteBox(w, h)
	s.rebuild()
	return s
}

// SelectedPage returns the active tab (standalone open uses this).
func (s *Scene) SelectedPage() Page {
	if p, ok := Lookup(s.Selected); ok {
		return p
	}
	p, _ := Lookup("overview")
	return p
}

// Select switches tab; returns false for unknown names.
func (s *Scene) Select(name string) bool {
	if _, ok := Lookup(name); !ok {
		return false
	}
	s.Selected = name
	s.rebuild()
	return true
}

func (s *Scene) rebuild() {
	tok := s.Theme.Current()
	s.Root = rendering.NewAbsoluteBox(s.Width, s.Height)
	s.Root.Background = &rendering.Color{R: tok.ColorBgLayout.R, G: tok.ColorBgLayout.G, B: tok.ColorBgLayout.B, A: 1}
	s.navBoxes = nil
	s.navNames = nil

	// Nav background.
	nav := rendering.NewRenderColorBox(navW, s.Height, tok.ColorBgContainer.R, tok.ColorBgContainer.G, tok.ColorBgContainer.B, 1)
	s.Root.Place(nav, 0, 0)

	pages := Pages()
	for i, p := range pages {
		y := float64(i)*navItemH + 8
		active := p.Name == s.Selected
		var box *rendering.RenderColorBox
		if active {
			box = rendering.NewRenderColorBox(navW-16, navItemH-4, tok.ColorPrimary.R, tok.ColorPrimary.G, tok.ColorPrimary.B, 1)
		} else {
			box = rendering.NewRenderColorBox(navW-16, navItemH-4, tok.ColorFillSecondary.R, tok.ColorFillSecondary.G, tok.ColorFillSecondary.B, 1)
		}
		s.Root.Place(box, 8, y)
		s.navBoxes = append(s.navBoxes, box)
		s.navNames = append(s.navNames, p.Name)
	}

	// Content card.
	page := s.SelectedPage()
	card := rendering.NewRenderColorBox(s.Width-navW-2*contentPad, 120, tok.ColorBgContainer.R, tok.ColorBgContainer.G, tok.ColorBgContainer.B, 1)
	card.SetRepaintBoundary(true)
	s.Root.Place(card, navW+contentPad, contentPad)

	// Section strips (one per section; visuals land with each control).
	y := 160.0
	for i := range page.Sections {
		alt := i%2 == 0
		var strip *rendering.RenderColorBox
		if alt {
			strip = rendering.NewRenderColorBox(s.Width-navW-2*contentPad, 44, tok.ColorFillTertiary.R, tok.ColorFillTertiary.G, tok.ColorFillTertiary.B, 1)
		} else {
			strip = rendering.NewRenderColorBox(s.Width-navW-2*contentPad, 44, tok.ColorFillSecondary.R, tok.ColorFillSecondary.G, tok.ColorFillSecondary.B, 1)
		}
		s.Root.Place(strip, navW+contentPad, y)
		y += 52
		if y > s.Height-60 {
			break
		}
	}
	s.Layout()
}

// Layout sizes the root and overlay band.
func (s *Scene) Layout() {
	if s == nil || s.Root == nil {
		return
	}
	s.Root.FixedWidth, s.Root.FixedHeight = s.Width, s.Height
	s.Root.Layout(rendering.Constraints{
		MinWidth: s.Width, MaxWidth: s.Width,
		MinHeight: s.Height, MaxHeight: s.Height,
	})
	if s.Overlay != nil {
		s.Overlay.Layout(s.Width, s.Height)
	}
}

// HitNav returns the page name under p, if any.
func (s *Scene) HitNav(x, y float64) (string, bool) {
	for i, b := range s.navBoxes {
		o := b.Offset()
		sz := b.Size()
		w, h := sz.Width, sz.Height
		if w <= 0 {
			w = b.Width
		}
		if h <= 0 {
			h = b.Height
		}
		if x >= o.X && y >= o.Y && x < o.X+w && y < o.Y+h {
			return s.navNames[i], true
		}
	}
	return "", false
}
