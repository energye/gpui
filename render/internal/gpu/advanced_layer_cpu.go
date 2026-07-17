package gpu

import (
	"image"

	"github.com/energye/gpui/render"
	intImage "github.com/energye/gpui/render/internal/image"
)

// cpuCompositeAdvancedLayer blends a premul RGBA layer (from GPU readback)
// onto parent pixmap Data using the same intImage path as Context.compositeLayer.
// Used for View-nil FlushGPU advanced Pop resolve (D05) so dual-tex Multiply
// does not wipe transparent outside regions.
//
// parentData is mutated in place via FromRaw zero-copy wrap.
func cpuCompositeAdvancedLayer(
	parentData []byte, parentW, parentH int,
	srcRGBA []byte, srcW, srcH int,
	mode render.BlendMode, opacity float64,
	damage image.Rectangle,
) error {
	if parentData == nil || srcRGBA == nil || parentW <= 0 || parentH <= 0 || srcW <= 0 || srcH <= 0 {
		return nil
	}
	needParent := parentW * parentH * 4
	needSrc := srcW * srcH * 4
	if len(parentData) < needParent || len(srcRGBA) < needSrc {
		return nil
	}

	// Zero-copy wrap: DrawImage writes into parentData through dstImg.
	dstImg, err := intImage.FromRaw(parentData, parentW, parentH, intImage.FormatRGBA8, parentW*4)
	if err != nil || dstImg == nil {
		return err
	}
	srcImg, err := intImage.FromRaw(srcRGBA, srcW, srcH, intImage.FormatRGBA8, srcW*4)
	if err != nil || srcImg == nil {
		return err
	}

	r := image.Rect(0, 0, srcW, srcH)
	if !damage.Empty() {
		const pad = 2
		d := damage.Inset(-pad).Intersect(r)
		if d.Empty() {
			return nil
		}
		r = d
	}
	r = r.Intersect(image.Rect(0, 0, parentW, parentH))
	if r.Empty() {
		return nil
	}

	if opacity < 0 {
		opacity = 0
	}
	if opacity > 1 {
		opacity = 1
	}

	srcRect := intImage.Rect{X: r.Min.X, Y: r.Min.Y, Width: r.Dx(), Height: r.Dy()}
	params := intImage.DrawParams{
		SrcRect: &srcRect,
		DstRect: intImage.Rect{
			X: r.Min.X, Y: r.Min.Y, Width: r.Dx(), Height: r.Dy(),
		},
		Interp:    intImage.InterpNearest,
		Opacity:   opacity,
		BlendMode: intImage.BlendMode(mode),
	}
	intImage.DrawImage(dstImg, srcImg, params)
	return nil
}
