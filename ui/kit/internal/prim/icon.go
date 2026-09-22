// Package prim icon glyphs: the L1 vector drawing for F1 Icon.
//
// This file owns all GPU-adjacent work for icons. Product code in
// ui/kit (icon_*.go) only resolves names, sizes, colors and angles;
// it never touches rendering. The example window composes these
// painters through kit facades, so Hit == Layout == Paint stays true.
package prim

import (
	"math"

	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// IconGlyphNames lists the P0 built-in registry (icon.md 1.2).
func IconGlyphNames() []string {
	return []string{
		"check", "close", "info-circle", "warning",
		"home", "setting", "smile", "sync",
		"loading", "heart", "star", "search",
		"plus", "minus", "edit", "delete",
		"left", "right", "up", "down",
		"file-text", "close-circle", "check-circle", "exclamation-circle",
	}
}

// IsKnownIconGlyph reports registry membership (case-sensitive).
func IsKnownIconGlyph(name string) bool {
	for _, n := range IconGlyphNames() {
		if n == name {
			return true
		}
	}
	return false
}

// IconLineWidth follows icon.md 6.2.1: size*0.125 clamped to 1.6..2.5
// at 16px, scaling linearly for showcase sizes.
func IconLineWidth(size float64) float64 {
	w := size * 0.125
	if size <= 20 {
		if w < 1.6 {
			w = 1.6
		}
		if w > 2.5 {
			w = 2.5
		}
	}
	if w < 1 {
		w = 1
	}
	return w
}

// IconPaintSpec carries everything the L1 painter needs. Colors come
// from kit resolve (theme or explicit), never literals here.
type IconPaintSpec struct {
	Name      string
	Theme     string // outlined|filled|twotone, empty means outlined
	Main      theme.Color
	Secondary theme.Color
	HasSecond bool
	TwoTone   bool
	AngleDeg  float64
}

// IconPainterFor builds a Painter drawing the glyph centered in a
// size x size box, rotated by AngleDeg about the center. Unknown names
// draw a diamond placeholder and never panic.
func IconPainterFor(size float64, spec IconPaintSpec) Painter {
	s := size
	sp := spec
	return func(pc *rendering.PaintContext, box rendering.Size) {
		if pc == nil {
			return
		}
		PaintIconGlyph(pc, box, s, sp)
	}
}

// PaintIconGlyph draws one glyph in box (box is the layout square).
// Official 848-icon paths paint first (1:1); the hand-drawn P0 set
// remains as fallback for offline/custom names. Unknown names draw a
// diamond placeholder and never panic.
func PaintIconGlyph(pc *rendering.PaintContext, box rendering.Size, size float64, spec IconPaintSpec) {
	if pc == nil || size <= 0 {
		return
	}
	cx, cy := box.Width/2, box.Height/2
	pc.Save()
	if spec.AngleDeg != 0 {
		pc.RotateAbout(spec.AngleDeg*math.Pi/180, cx, cy)
	}
	// Official path: paint at box origin (local coords 0..size).
	th := spec.Theme
	if th == "" {
		th = "outlined"
	}
	second := spec.Secondary
	hasSecond := spec.HasSecond
	if spec.TwoTone && !hasSecond {
		// Two-tone without explicit secondary uses derived halo.
		hasSecond = false
	}
	if PaintAntdIcon(pc, size, spec.Name, th, spec.Main, second, hasSecond) {
		pc.RestoreCanvas()
		return
	}
	drawIconBody(pc, cx, cy, size, spec)
	pc.RestoreCanvas()
}

func mainColor(spec IconPaintSpec) (r, g, b, a float64) {
	return spec.Main.R, spec.Main.G, spec.Main.B, spec.Main.A
}

func secondColor(spec IconPaintSpec) (r, g, b, a float64) {
	if spec.HasSecond {
		return spec.Secondary.R, spec.Secondary.G, spec.Secondary.B, spec.Secondary.A
	}
	// Derived secondary: main at ~15% over transparent (two-tone halo).
	return spec.Main.R, spec.Main.G, spec.Main.B, 0.15
}

func drawIconBody(pc *rendering.PaintContext, cx, cy, size float64, spec IconPaintSpec) {
	lw := IconLineWidth(size)
	r, g, b, a := mainColor(spec)
	sr, sg, sb, sa := secondColor(spec)
	u := size / 16.0
	if u <= 0 {
		u = 1
	}
	// Two-tone halo: faint disc so the secondary color participates.
	if spec.TwoTone {
		rendering.FillCircle(pc, cx, cy, size*0.42, sr, sg, sb, sa)
	}
	line := func(x1, y1, x2, y2 float64) {
		rendering.StrokeLine(pc, cx+x1*u, cy+y1*u, cx+x2*u, cy+y2*u, lw, r, g, b, a)
	}
	circle := func(dx, dy, rad float64) {
		rendering.StrokeCircle(pc, cx+dx*u, cy+dy*u, rad*u, lw, r, g, b, a)
	}
	dot := func(dx, dy, rad float64) {
		rendering.FillCircle(pc, cx+dx*u, cy+dy*u, rad*u, r, g, b, a)
	}
	switch spec.Name {
	case "check":
		line(-4.5, 0.5, -1.5, 3.5)
		line(-1.5, 3.5, 5, -3.5)
	case "close":
		line(-3.5, -3.5, 3.5, 3.5)
		line(-3.5, 3.5, 3.5, -3.5)
	case "info-circle":
		circle(0, 0, 6)
		line(0, -2.5, 0, 0.5)
		dot(0, 2.8, 0.9)
	case "warning":
		line(-5.5, 4.5, 0, -5)
		line(0, -5, 5.5, 4.5)
		line(-5.5, 4.5, 5.5, 4.5)
		line(0, -1.5, 0, 1.5)
		dot(0, 3.2, 0.9)
	case "home":
		line(-5.5, -0.5, 0, -5.5)
		line(0, -5.5, 5.5, -0.5)
		rendering.StrokeRect(pc, cx-3.5*u, cy-0.5*u, 7*u, 5.5*u, lw, r, g, b, a)
		rendering.FillRect(pc, cx-1.2*u, cy+1.5*u, 2.4*u, 3.5*u, r, g, b, a)
	case "setting":
		circle(0, 0, 2.6)
		for i := 0; i < 8; i++ {
			ang := float64(i) * math.Pi / 4
			x1, y1 := math.Cos(ang)*3.6, math.Sin(ang)*3.6
			x2, y2 := math.Cos(ang)*5.6, math.Sin(ang)*5.6
			line(x1, y1, x2, y2)
		}
	case "smile":
		circle(0, 0, 6)
		dot(-2.2, -1.5, 0.8)
		dot(2.2, -1.5, 0.8)
		rendering.StrokeArc(pc, cx, cy+0.5*u, 3.2*u, 0.35*math.Pi, 0.65*math.Pi, lw, r, g, b, a)
	case "sync":
		rendering.StrokeArc(pc, cx, cy, 4.6*u, -0.4*math.Pi, 0.7*math.Pi, lw, r, g, b, a)
		rendering.StrokeArc(pc, cx, cy, 4.6*u, 0.6*math.Pi, 1.7*math.Pi, lw, r, g, b, a)
		line(4.6, -1.8, 4.6, 1.2)
		line(4.6, 1.2, 2.2, 0.6)
		line(-4.6, 1.8, -4.6, -1.2)
		line(-4.6, -1.2, -2.2, -0.6)
	case "loading":
		rendering.StrokeArc(pc, cx, cy, 5*u, 0, 1.5*math.Pi, lw+0.4, r, g, b, a)
		dot(5*math.Cos(1.5*math.Pi), 5*math.Sin(1.5*math.Pi), 0.9)
	case "heart":
		rendering.FillCircle(pc, cx-2.3*u, cy-1.2*u, 2.6*u, r, g, b, a)
		rendering.FillCircle(pc, cx+2.3*u, cy-1.2*u, 2.6*u, r, g, b, a)
		rendering.FillRect(pc, cx-4.4*u, cy-0.6*u, 8.8*u, 2.6*u, r, g, b, a)
		line(-4.4, 2, 0, 6)
		line(4.4, 2, 0, 6)
	case "star":
		pts := starPoints(5, 6, 2.7)
		for i := 0; i < len(pts); i++ {
			j := (i + 1) % len(pts)
			line(pts[i][0], pts[i][1], pts[j][0], pts[j][1])
		}
	case "search":
		circle(-1, -1, 3.6)
		line(1.6, 1.6, 5, 5)
	case "plus":
		line(-4, 0, 4, 0)
		line(0, -4, 0, 4)
	case "minus":
		line(-4, 0, 4, 0)
	case "edit":
		line(-4.5, 4.5, 3, -3)
		line(3, -3, 5, -5)
		line(5, -5, 3.4, -5.2)
		line(5, -5, 5.2, -3.4)
		line(-4.5, 4.5, -2.5, 4.8)
		line(-4.5, 4.5, -4.8, 2.5)
	case "delete":
		line(-4, -4, 4, -4)
		line(-2.5, -4, -2.5, -5.5)
		line(2.5, -4, 2.5, -5.5)
		rendering.StrokeRect(pc, cx-3.4*u, cy-4*u, 6.8*u, 8*u, lw, r, g, b, a)
		line(-1.2, -1.5, -1.2, 2.5)
		line(1.2, -1.5, 1.2, 2.5)
	case "left":
		line(2.5, -5, -2.5, 0)
		line(-2.5, 0, 2.5, 5)
	case "right":
		line(-2.5, -5, 2.5, 0)
		line(2.5, 0, -2.5, 5)
	case "up":
		line(-5, 2.5, 0, -2.5)
		line(0, -2.5, 5, 2.5)
	case "down":
		line(-5, -2.5, 0, 2.5)
		line(0, 2.5, 5, -2.5)
	case "file-text":
		rendering.StrokeRect(pc, cx-3.5*u, cy-5*u, 7*u, 10*u, lw, r, g, b, a)
		line(-1.8, -2, 1.8, -2)
		line(-1.8, 0, 1.8, 0)
		line(-1.8, 2, 1.2, 2)
	case "close-circle":
		circle(0, 0, 6)
		line(-2.2, -2.2, 2.2, 2.2)
		line(-2.2, 2.2, 2.2, -2.2)
	case "check-circle":
		circle(0, 0, 6)
		line(-3, 0.3, -0.8, 2.4)
		line(-0.8, 2.4, 3.2, -2.2)
	case "exclamation-circle":
		circle(0, 0, 6)
		line(0, -3, 0, 1)
		dot(0, 2.8, 0.9)
	default:
		// Diamond placeholder for unknown names: never blank, never panic.
		line(0, -5, 5, 0)
		line(5, 0, 0, 5)
		line(0, 5, -5, 0)
		line(-5, 0, 0, -5)
	}
}

func starPoints(n int, outer, inner float64) [][2]float64 {
	pts := make([][2]float64, 0, n*2)
	for i := 0; i < n*2; i++ {
		rad := outer
		if i%2 == 1 {
			rad = inner
		}
		ang := -math.Pi/2 + float64(i)*math.Pi/float64(n)
		pts = append(pts, [2]float64{rad * math.Cos(ang), rad * math.Sin(ang)})
	}
	return pts
}
