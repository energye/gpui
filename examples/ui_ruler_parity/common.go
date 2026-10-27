package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

const (
	winListW, winListH = 1200, 800
	winSpinW, winSpinH = 480, 320
)

type rgbColor struct{ R, G, B float64 }

func rgb(hex uint32) rgbColor {
	return rgbColor{
		R: float64((hex>>16)&0xff) / 255,
		G: float64((hex>>8)&0xff) / 255,
		B: float64(hex&0xff) / 255,
	}
}

var (
	colBg       = rgb(0x1A1E29)
	colHeader   = rgb(0x151922)
	colPanel    = rgb(0x202634)
	colPanel2   = rgb(0x2A3142)
	colInk      = rgb(0xE2E7F2)
	colMuted    = rgb(0x9AA6BD)
	colHudMuted = rgb(0x6E7A91)
	colLabel    = rgb(0x8CBFF2)
	colHud      = rgb(0x11151D)
)

func testdataPath(name string) string {
	if dir := os.Getenv("GPUI_RULER_TESTDATA"); dir != "" {
		return filepath.Join(dir, name)
	}
	return filepath.Join("examples", "ui_ruler_parity", "testdata", name)
}

func loadText(name string) string {
	b, err := os.ReadFile(testdataPath(name))
	if err != nil || len(b) == 0 {
		return "fallback " + name
	}
	return string(b)
}

func loadImage(path string) *render.ImageBuf {
	img, err := render.LoadImage(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ui_ruler_parity: load image %s: %v\n", path, err)
		return nil
	}
	return img
}

func newSolid(w, h float64, c rgbColor) *rendering.AbsoluteBox {
	b := rendering.NewAbsoluteBox(w, h)
	b.Background = &rendering.Color{R: c.R, G: c.G, B: c.B, A: 1}
	return b
}

func newRoot(w, h float64, c rgbColor) *rendering.AbsoluteBox {
	return newSolid(w, h, c)
}

func labelAt(parent *rendering.AbsoluteBox, s string, size, x, y float64, c rgbColor) *rendering.RenderText {
	t := wrkit.Label(s, size, c.R, c.G, c.B)
	parent.Place(t, x, y)
	return t
}

func labelWrapped(parent *rendering.AbsoluteBox, s string, size, maxW, x, y float64, c rgbColor) *rendering.RenderText {
	t := wrkit.Label(s, size, c.R, c.G, c.B)
	t.MaxWidth = maxW
	parent.Place(t, x, y)
	return t
}

func textHeight(t *rendering.RenderText) float64 {
	if t == nil {
		return 0
	}
	lay := t.TextLayout()
	if lay == nil || lay.LineCount() == 0 {
		if t.FontSize > 0 {
			return t.FontSize * 1.25
		}
		return 0
	}
	n := lay.LineCount()
	return lay.LineTop(n-1) + lay.LineHeight(n-1)
}

// approxTextW is the same 0.55 Latin / 1.0 CJK estimate wrkit.Label uses.
// It is only for right-edge placement, never for wrap decisions.
func approxTextW(s string, size float64) float64 {
	w := 0.0
	for _, r := range s {
		if r > 127 {
			w += size
		} else {
			w += size * 0.55
		}
	}
	return w
}

func rightLabelAt(parent *rendering.AbsoluteBox, s string, size, right, y float64, c rgbColor) *rendering.RenderText {
	return labelAt(parent, s, size, right-approxTextW(s, size), y, c)
}

// addHeader replicates the Flutter ruler's 48px top bar.
func addHeader(root *rendering.AbsoluteBox, title, right string) *rendering.AbsoluteBox {
	h := newSolid(winListW, 48, colHeader)
	root.Place(h, 0, 0)
	labelAt(h, title, 14, 14, 14, colInk)
	if right != "" {
		rightLabelAt(h, right, 11, winListW-14, 17, colMuted)
	}
	return h
}

// hud is the Flutter ruler's 72px status band. It is its own repaint boundary
// so a text update never dirties the scene.
type hud struct {
	panel     *rendering.AbsoluteBox
	left, mid *rendering.RenderText
	right     *rendering.RenderText
	lastMid   string
}

func addHUD(root *rendering.AbsoluteBox, left, mid, right string) *hud {
	p := newSolid(winListW, 72, colHud)
	p.SetRepaintBoundary(true)
	root.Place(p, 0, 728)
	l := labelAt(p, left, 13, 14, 26, colLabel)
	m := labelAt(p, mid, 12, 110, 27, colMuted)
	var r *rendering.RenderText
	if right != "" {
		r = rightLabelAt(p, right, 11, winListW-14, 28, colHudMuted)
	}
	return &hud{panel: p, left: l, mid: m, right: r, lastMid: mid}
}

func (h *hud) set(mid string) {
	if h == nil || mid == "" || mid == h.lastMid {
		return
	}
	h.lastMid = mid
	h.mid.SetText(mid)
	h.panel.MarkNeedsPaint()
}

func emojiFace(points float64) text.Face {
	for _, p := range []string{
		"/usr/share/fonts/truetype/noto/NotoColorEmoji.ttf",
		"/usr/share/fonts/TTF/NotoColorEmoji.ttf",
	} {
		src, err := text.NewFontSourceFromFile(p)
		if err != nil {
			continue
		}
		return src.Face(points)
	}
	return nil
}

// loadSnapshot decodes a PNG and returns its RGBA pixel accessor. Used by the
// spin glyph-ink probe and by per-scene extra checks.
func loadSnapshot(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

// inkCount counts pixels inside a logical rect that are visibly different
// from the declared background. Coordinates are top-left logical space.
func inkCount(img image.Image, x, y, w, h float64, bg rgbColor, tol uint8) int {
	b := img.Bounds()
	dprX := float64(b.Dx()) / winListW
	dprY := float64(b.Dy()) / winListH
	if b.Dx() == winSpinW && b.Dy() == winSpinH {
		dprX = float64(b.Dx()) / winSpinW
		dprY = float64(b.Dy()) / winSpinH
	}
	x0 := b.Min.X + int(x*dprX+0.5)
	y0 := b.Min.Y + int(y*dprY+0.5)
	x1 := b.Min.X + int((x+w)*dprX+0.5)
	y1 := b.Min.Y + int((y+h)*dprY+0.5)
	if x0 < b.Min.X {
		x0 = b.Min.X
	}
	if y0 < b.Min.Y {
		y0 = b.Min.Y
	}
	if x1 > b.Max.X {
		x1 = b.Max.X
	}
	if y1 > b.Max.Y {
		y1 = b.Max.Y
	}
	if tol == 0 {
		tol = 4
	}
	wantR, wantG, wantB := uint8(bg.R*255+0.5), uint8(bg.G*255+0.5), uint8(bg.B*255+0.5)
	n := 0
	for py := y0; py < y1; py++ {
		for px := x0; px < x1; px++ {
			cr, cg, cb, _ := img.At(px, py).RGBA()
			r, g, bl := uint8(cr>>8), uint8(cg>>8), uint8(cb>>8)
			if abs8(r, wantR) > int(tol) || abs8(g, wantG) > int(tol) || abs8(bl, wantB) > int(tol) {
				n++
			}
		}
	}
	return n
}

func abs8(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

// ensureImage is a tiny guard used by scenes that must fail closed when a
// committed testdata asset is missing.
func ensureImage(path string, wantW, wantH int) *render.ImageBuf {
	img := loadImage(path)
	if img == nil {
		return nil
	}
	if img.Width() != wantW || img.Height() != wantH {
		fmt.Fprintf(os.Stderr, "ui_ruler_parity: %s is %dx%d, want %dx%d\n", path, img.Width(), img.Height(), wantW, wantH)
	}
	return img
}
