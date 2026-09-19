package splitter

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// Root uses AbsoluteBox (offsets preserved) with RenderBox bars carrying
// OnPaint. No custom RenderObject subclass: layout positions hosts edge to
// edge (sizes sum to the main length) and bars overlay centered on seams
// with the trigger box, so hit == layout == paint.

func (s *Splitter) ensureNodes() {
	if s == nil || s.root == nil {
		return
	}
	wantHosts := len(s.panels)
	for len(s.hosts) < wantHosts {
		h := rendering.NewRenderBox()
		s.hosts = append(s.hosts, h)
	}
	if len(s.hosts) > wantHosts {
		s.hosts = s.hosts[:wantHosts]
	}
	wantBars := s.BarCount()
	for len(s.bars) < wantBars {
		idx := len(s.bars)
		b := rendering.NewRenderBox()
		capIdx := idx
		capS := s
		b.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
			capS.paintBar(pc, size, capIdx)
		}
		s.bars = append(s.bars, b)
	}
	if len(s.bars) > wantBars {
		s.bars = s.bars[:wantBars]
	}
	// Refresh bar paint closures after slice growth (indices stable).
	for i, b := range s.bars {
		idx := i
		capS := s
		b.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
			capS.paintBar(pc, size, idx)
		}
	}
	if s.preview == nil {
		s.preview = rendering.NewRenderColorBox(2, 2, 0, 0, 0, 0)
	}
}

func (s *Splitter) syncHostChildren() {
	if s == nil {
		return
	}
	for i, p := range s.panels {
		if i >= len(s.hosts) {
			continue
		}
		h := s.hosts[i]
		if h == nil || p == nil {
			continue
		}
		destroyed := false
		if i < len(s.lastSizes) && s.lastSizes[i] == 0 && p.effectiveDestroy(s.destroyOnHidden) {
			destroyed = true
		}
		cur := h.Children()
		has := false
		for _, c := range cur {
			if c == p.child {
				has = true
				break
			}
		}
		if destroyed {
			if has {
				h.RemoveChild(p.child)
			}
			continue
		}
		if p.child == nil {
			if len(cur) > 0 {
				for _, c := range cur {
					h.RemoveChild(c)
				}
			}
			continue
		}
		if !has {
			for _, c := range cur {
				h.RemoveChild(c)
			}
			h.AddChild(p.child)
		}
	}
}

func (s *Splitter) layoutRoot(c rendering.Constraints) (float64, float64, []float64) {
	vertical := s.IsVertical()
	var W, H float64
	if s.width > 0 {
		W = s.width
	} else if c.MaxWidth < rendering.Unbounded/2 && c.MaxWidth > 0 {
		W = c.MaxWidth
	} else if vertical {
		W = DefaultSplitContainerCross
	} else {
		W = DefaultSplitContainerMain
	}
	if s.height > 0 {
		H = s.height
	} else if c.MaxHeight < rendering.Unbounded/2 && c.MaxHeight > 0 {
		H = c.MaxHeight
	} else if vertical {
		H = DefaultSplitContainerMain
	} else {
		H = DefaultSplitContainerCross
	}
	var L float64
	if vertical {
		L = H
	} else {
		L = W
	}
	sizes := s.resolveSizes(L)
	s.lastSizes = append([]float64(nil), sizes...)
	s.lastW, s.lastH = W, H
	// Stretch color-box panel content so it fills the host (test panels and
	// simple color placeholders). Generic nodes keep their own measure.
	for i, p := range s.panels {
		if p == nil || p.child == nil || i >= len(sizes) {
			continue
		}
		if cb, ok := p.child.(*rendering.RenderColorBox); ok {
			if !vertical {
				cb.Width, cb.Height = sizes[i], H
			} else {
				cb.Width, cb.Height = W, sizes[i]
			}
		}
	}
	s.syncHostChildren()
	trigger := s.SplitTriggerSize()
	s.root.FixedWidth, s.root.FixedHeight = W, H
	tok := s.themeTokens()
	bg := themeToRGBA(tok.ColorBgContainer)
	s.root.Background = &rendering.Color{R: bg.R, G: bg.G, B: bg.B, A: bg.A}
	// Hosts edge to edge.
	if !vertical {
		x := 0.0
		for i := range s.panels {
			w := 0.0
			if i < len(sizes) {
				w = sizes[i]
			}
			h := s.hosts[i]
			if h == nil {
				x += w
				continue
			}
			h.FixedWidth, h.FixedHeight = w, H
			s.root.Place(h, x, 0)
			x += w
		}
		acc := 0.0
		for i, b := range s.bars {
			if i < len(sizes) {
				acc += sizes[i]
			}
			b.FixedWidth, b.FixedHeight = trigger, H
			s.root.Place(b, acc-trigger/2, 0)
		}
		if drg := s.dragging.Load(); drg >= 0 && s.lazy && s.previewDelta != 0 {
			acc := 0.0
			for i := 0; i <= int(drg) && i < len(sizes); i++ {
				acc += sizes[i]
			}
			x := acc + s.previewDelta
			pc := themeToRGBA(tok.ColorPrimary)
			s.preview.R, s.preview.G, s.preview.B, s.preview.A = pc.R, pc.G, pc.B, 0.55
			s.preview.Width, s.preview.Height = 2, H
			s.root.Place(s.preview, x-1, 0)
		} else {
			s.unplace(s.preview)
		}
	} else {
		y := 0.0
		for i := range s.panels {
			hh := 0.0
			if i < len(sizes) {
				hh = sizes[i]
			}
			h := s.hosts[i]
			if h == nil {
				y += hh
				continue
			}
			h.FixedWidth, h.FixedHeight = W, hh
			s.root.Place(h, 0, y)
			y += hh
		}
		acc := 0.0
		for i, b := range s.bars {
			if i < len(sizes) {
				acc += sizes[i]
			}
			b.FixedWidth, b.FixedHeight = W, trigger
			s.root.Place(b, 0, acc-trigger/2)
		}
		if drg := s.dragging.Load(); drg >= 0 && s.lazy && s.previewDelta != 0 {
			acc := 0.0
			for i := 0; i <= int(drg) && i < len(sizes); i++ {
				acc += sizes[i]
			}
			y := acc + s.previewDelta
			pc := themeToRGBA(tok.ColorPrimary)
			s.preview.R, s.preview.G, s.preview.B, s.preview.A = pc.R, pc.G, pc.B, 0.55
			s.preview.Width, s.preview.Height = W, 2
			s.root.Place(s.preview, 0, y-1)
		} else {
			s.unplace(s.preview)
		}
	}
	return W, H, sizes
}

func (s *Splitter) unplace(n rendering.RenderObject) {
	if s == nil || s.root == nil || n == nil {
		return
	}
	s.root.RemoveChild(n)
}

func (s *Splitter) paintBar(pc *rendering.PaintContext, size rendering.Size, index int) {
	if s == nil || pc == nil {
		return
	}
	w, h := size.Width, size.Height
	if w <= 0 || h <= 0 {
		return
	}
	S := s.loadSnapshot()
	if index < 0 || index >= len(S.Resizable) {
		return
	}
	vertical := S.Vertical
	barW := S.BarSize
	hov := s.loadHovered()
	hovered := index < len(hov) && hov[index]
	active := s.barActive.Load() == int64(index) || s.dragging.Load() == int64(index)
	focused := s.focusedBar.Load() == int64(index)
	resizable := S.Resizable[index]

	base := toBarRGBA(S.BaseColor)
	hoverC := toBarRGBA(S.HoverColor)
	activeC := toBarRGBA(S.ActiveColor)
	handleC := toBarRGBA(S.HandleColor)

	bg := base
	if active {
		bg = activeC
	} else if hovered {
		bg = hoverC
	}
	if !vertical {
		cx := w / 2
		rendering.FillRect(pc, cx-barW/2, 0, barW, h, bg.r, bg.g, bg.b, bg.a)
		if resizable {
			hw := 2.0
			if S.HasDragger {
				hw = 4
			}
			hh := S.DraggableSize
			if hh > h-16 {
				hh = h - 16
			}
			if hh < 8 {
				hh = 8
			}
			cy := h / 2
			rendering.FillRoundRect(pc, cx-hw/2, cy-hh/2, hw, hh, 1, handleC.r, handleC.g, handleC.b, handleC.a)
		}
		s.paintCollapseMarks(pc, index, false, w, h, S)
		if focused {
			fc := toBarRGBA(S.FocusColor)
			rendering.StrokeRect(pc, 0.75, 0.75, w-1.5, h-1.5, 1.5, fc.r, fc.g, fc.b, fc.a)
		}
	} else {
		cy := h / 2
		rendering.FillRect(pc, 0, cy-barW/2, w, barW, bg.r, bg.g, bg.b, bg.a)
		if resizable {
			ww := S.DraggableSize
			if ww > w-16 {
				ww = w - 16
			}
			if ww < 8 {
				ww = 8
			}
			hh := 2.0
			if S.HasDragger {
				hh = 4
			}
			cx := w / 2
			rendering.FillRoundRect(pc, cx-ww/2, cy-hh/2, ww, hh, 1, handleC.r, handleC.g, handleC.b, handleC.a)
		}
		s.paintCollapseMarks(pc, index, true, w, h, S)
		if focused {
			fc := toBarRGBA(S.FocusColor)
			rendering.StrokeRect(pc, 0.75, 0.75, w-1.5, h-1.5, 1.5, fc.r, fc.g, fc.b, fc.a)
		}
	}
}

func (s *Splitter) paintCollapseMarks(pc *rendering.PaintContext, index int, horizontalBar bool, w, h float64, S SplitterSnap) {
	if index < 0 || index >= len(S.CollapsibleLeft) || index >= len(S.CollapsibleRight) {
		return
	}
	leftShow := S.CollapsibleLeft[index]
	rightShow := S.CollapsibleRight[index]
	handleC := toBarRGBA(S.HandleColor)
	if S.HasCollapseStart || S.HasCollapseEnd {
		cx, cy := w/2, h/2
		if leftShow && S.HasCollapseStart {
			rendering.FillRect(pc, cx-3, cy-3, 6, 6, handleC.r, handleC.g, handleC.b, handleC.a)
		} else if leftShow {
			rendering.FillRect(pc, cx-1, cy-5, 2, 10, handleC.r, handleC.g, handleC.b, handleC.a)
		}
		if rightShow && S.HasCollapseEnd {
			rendering.FillRect(pc, cx-3, cy-3, 6, 6, handleC.r, handleC.g, handleC.b, handleC.a)
		} else if rightShow {
			rendering.FillRect(pc, cx-1, cy-5, 2, 10, handleC.r, handleC.g, handleC.b, handleC.a)
		}
		return
	}
	if !horizontalBar {
		cy := h / 2
		cx := w / 2
		if leftShow {
			p := rendering.NewPath()
			p.MoveTo(cx+3, cy-5)
			p.LineTo(cx-2, cy)
			p.LineTo(cx+3, cy+5)
			p.Close()
			rendering.FillPath(pc, p, handleC.r, handleC.g, handleC.b, handleC.a)
		}
		if rightShow {
			p := rendering.NewPath()
			p.MoveTo(cx-3, cy-5)
			p.LineTo(cx+2, cy)
			p.LineTo(cx-3, cy+5)
			p.Close()
			rendering.FillPath(pc, p, handleC.r, handleC.g, handleC.b, handleC.a)
		}
	} else {
		cx := w / 2
		cy := h / 2
		if leftShow {
			p := rendering.NewPath()
			p.MoveTo(cx-5, cy+3)
			p.LineTo(cx, cy-2)
			p.LineTo(cx+5, cy+3)
			p.Close()
			rendering.FillPath(pc, p, handleC.r, handleC.g, handleC.b, handleC.a)
		}
		if rightShow {
			p := rendering.NewPath()
			p.MoveTo(cx-5, cy-3)
			p.LineTo(cx, cy+2)
			p.LineTo(cx+5, cy-3)
			p.Close()
			rendering.FillPath(pc, p, handleC.r, handleC.g, handleC.b, handleC.a)
		}
	}
}

type barRGBA struct{ r, g, b, a float64 }

func toBarRGBA(c render.RGBA) barRGBA { return barRGBA{c.R, c.G, c.B, c.A} }

func (s *Splitter) syncBarsLocked() {
	if s == nil {
		return
	}
	want := s.BarCount()
	if hov := s.loadHovered(); len(hov) != want {
		nh := make([]bool, want)
		copy(nh, hov)
		s.hovered.Store(nh)
	}
	if len(s.collapsed) != len(s.panels) {
		nc := make([]bool, len(s.panels))
		copy(nc, s.collapsed)
		s.collapsed = nc
	}
	if s.focusedBar.Load() >= int64(want) {
		s.focusedBar.Store(-1)
	}
	if s.dragging.Load() >= int64(want) {
		s.dragging.Store(-1)
	}
}
