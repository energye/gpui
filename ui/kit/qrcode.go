package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// QRCode is a deterministic module grid from text (real QR codec later).
// https://ant.design/components/qrcode
type QRCode struct {
	Root *primitive.Decorated
	Text string
	Size float64
	// Modules is the last painted grid size (for tests).
	Modules int
	Face    text.Face
	// Fg / Bg when A>0 override black/white modules.
	Fg, Bg render.RGBA
}

// NewQRCode creates a QR stand-in labeled with text.
func NewQRCode(text string) *QRCode {
	q := &QRCode{Text: text, Size: 128, Modules: 21}
	q.rebuild()
	return q
}

// Node returns root.
func (q *QRCode) Node() core.Node {
	if q.Root == nil {
		q.rebuild()
	}
	return q.Root
}

// SetText updates payload and rebuilds modules (R2).
func (q *QRCode) SetText(text string) {
	if q == nil {
		return
	}
	q.Text = text
	q.rebuild()
}

// SetSize sets logical square size.
func (q *QRCode) SetSize(px float64) {
	if q == nil {
		return
	}
	q.Size = px
	q.rebuild()
}

func (q *QRCode) rebuild() {
	sz := q.Size
	if sz <= 0 {
		sz = 128
	}
	n := q.Modules
	if n < 21 {
		n = 21
		q.Modules = n
	}
	// Deterministic pseudo-QR modules from text hash (visual stand-in until codec).
	hash := 0
	for _, r := range q.Text {
		hash = hash*31 + int(r)
	}
	fg := q.Fg
	if fg.A <= 0 {
		fg = render.RGBA{R: 0, G: 0, B: 0, A: 1}
	}
	bg := q.Bg
	if bg.A <= 0 {
		bg = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	}
	pn := primitive.NewPainterNode(func(pc *core.PaintContext, sz core.Size) {
		if pc == nil {
			return
		}
		pc.FillLocalRect(0, 0, sz.Width, sz.Height, bg)
		cell := sz.Width / float64(n)
		for y := 0; y < n; y++ {
			for x := 0; x < n; x++ {
				v := (hash + x*17 + y*31) & 1
				// finder patterns (ISO-ish corners)
				if (x < 7 && y < 7) || (x >= n-7 && y < 7) || (x < 7 && y >= n-7) {
					v = 1
					if x > 1 && x < 5 && y > 1 && y < 5 {
						v = 0
					}
					if x > 2 && x < 4 && y > 2 && y < 4 {
						v = 1
					}
				}
				// timing patterns
				if y == 6 || x == 6 {
					if (x+y)%2 == 0 {
						v = 1
					} else {
						v = 0
					}
				}
				if v == 1 {
					pc.FillLocalRect(float64(x)*cell, float64(y)*cell, cell, cell, fg)
				}
			}
		}
	})
	pn.Width, pn.Height = sz, sz
	q.Root = primitive.NewDecorated(pn)
	q.Root.Width, q.Root.Height = sz, sz
	q.Root.StretchChild = true
	q.Root.Background = bg
	q.Root.BorderWidth = 1
	q.Root.BorderColor = render.RGBA{R: 0, G: 0, B: 0, A: 0.15}
}
