package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Image is a placeholder image box with alt label.
// https://ant.design/components/image
type Image struct {
	Root *primitive.Decorated
	Alt  string
	// Src is an optional path/url label (shown when Alt empty).
	Src   string
	W, H  float64
	Face  text.Face
	Theme *core.Theme
	// Pixels is optional packed RGBA (len >= PixelW*PixelH*4). When set,
	// paint samples a low-res grid (real GPU texture upload remains later).
	Pixels         []byte
	PixelW, PixelH int
	// Fallback is used when no pixels (A>0).
	Fallback render.RGBA
	// Preview enables preview mode flag (state only for R3).
	Preview bool
}

// NewImage creates an image frame (placeholder or pixel-backed).
func NewImage(alt string, w, h float64) *Image {
	im := &Image{Alt: alt, W: w, H: h}
	im.rebuild()
	return im
}

// Node returns root.
func (im *Image) Node() core.Node {
	if im.Root == nil {
		im.rebuild()
	}
	return im.Root
}

// SetSrc sets source path/url label and rebuilds caption.
func (im *Image) SetSrc(src string) {
	if im == nil {
		return
	}
	im.Src = src
	im.rebuild()
}

// SetPixels installs an RGBA buffer for sampled paint (R2 bitmap path).
func (im *Image) SetPixels(w, h int, rgba []byte) {
	if im == nil {
		return
	}
	im.PixelW, im.PixelH = w, h
	im.Pixels = rgba
	im.rebuild()
}

// SetPreview toggles preview mode flag.
func (im *Image) SetPreview(on bool) {
	if im == nil {
		return
	}
	im.Preview = on
}

func (im *Image) rebuild() {
	th := DefaultTheme()
	if im.Theme != nil {
		th = im.Theme
	}
	w, h := im.W, im.H
	if w <= 0 {
		w = 120
	}
	if h <= 0 {
		h = 80
	}
	caption := im.Alt
	if caption == "" {
		caption = im.Src
	}
	lab := primitive.NewText(caption)
	lab.FontSize = 12
	lab.Face = im.Face
	lab.Color = th.Color(core.TokenColorTextSecondary)
	pw, ph := im.PixelW, im.PixelH
	pix := im.Pixels
	fb := im.Fallback
	pn := primitive.NewPainterNode(func(pc *core.PaintContext, sz core.Size) {
		if pc == nil {
			return
		}
		if len(pix) >= pw*ph*4 && pw > 0 && ph > 0 {
			// Sample pixel buffer into a coarse grid (max 32×32 cells).
			gx, gy := pw, ph
			if gx > 32 {
				gx = 32
			}
			if gy > 32 {
				gy = 32
			}
			cw, ch := sz.Width/float64(gx), sz.Height/float64(gy)
			for y := 0; y < gy; y++ {
				for x := 0; x < gx; x++ {
					sx := x * pw / gx
					sy := y * ph / gy
					i := (sy*pw + sx) * 4
					if i+3 >= len(pix) {
						continue
					}
					c := render.RGBA{
						R: float64(pix[i]) / 255,
						G: float64(pix[i+1]) / 255,
						B: float64(pix[i+2]) / 255,
						A: float64(pix[i+3]) / 255,
					}
					pc.FillLocalRect(float64(x)*cw, float64(y)*ch, cw+0.5, ch+0.5, c)
				}
			}
			return
		}
		// Soft gradient-like bands + optional fallback tint.
		base := render.RGBA{R: 0.93, G: 0.94, B: 0.96, A: 1}
		top := render.RGBA{R: 0.82, G: 0.88, B: 0.95, A: 1}
		if fb.A > 0 {
			base, top = fb, fb
		}
		pc.FillLocalRect(0, 0, sz.Width, sz.Height, base)
		pc.FillLocalRect(0, 0, sz.Width, sz.Height*0.45, top)
	})
	pn.Width, pn.Height = w, h
	stack := primitive.NewStack(pn, primitive.Positioned(core.AlignCenter, lab))
	im.Root = primitive.NewDecorated(stack)
	im.Root.Width, im.Root.Height = w, h
	im.Root.StretchChild = true
	im.Root.BorderWidth = 1
	im.Root.BorderColor = th.Color(core.TokenColorBorder)
	im.Root.Radius = 4
	im.Root.Background = th.Color(core.TokenColorBgLayout)
}
