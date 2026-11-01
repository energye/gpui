//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package render

// ColorFilter maps one straight-alpha color to another (P3.4, the
// SkColorFilters family slot): Filter receives straight 0-1 components and
// returns the mapped straight color. Composition is function composition
// (Compose). Context.ApplyColorFilter runs one over the content, handling
// the premul storage round trip.
type ColorFilter interface {
	Filter(r, g, b, a float32) (float32, float32, float32, float32)
}

// ApplyColorFilter runs a ColorFilter over the content (P3.4). CPU-side
// only: color filters are cheap, and the GPU graph's straight-domain color
// passes (matrix class) cover the accelerator-worthy cases already.
func (c *Context) ApplyColorFilter(cf ColorFilter) {
	if cf == nil || c == nil || c.pixmap == nil {
		return
	}
	_ = c.FlushGPU()
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		w := colorFilterPixmapWrap{cf}
		w.Apply(src, dst)
	})
	c.markLayerFullComposite()
}

// colorFilterPixmapWrap runs a ColorFilter over a premul pixmap
// (unpremul → filter → premul, the established color-filter domain).
type colorFilterPixmapWrap struct{ cf ColorFilter }

// Apply executes the filter over the full pixmaps.
func (w colorFilterPixmapWrap) Apply(src, dst *Pixmap) {
	data := src.Data()
	d := dst.Data()
	w2 := src.Width()
	for y := 0; y < src.Height(); y++ {
		for x := 0; x < w2; x++ {
			i := (y*w2 + x) * 4
			a := float32(data[i+3]) / 255.0
			if a == 0 {
				d[i], d[i+1], d[i+2], d[i+3] = 0, 0, 0, 0
				continue
			}
			r := float32(data[i]) / 255.0 / a
			g := float32(data[i+1]) / 255.0 / a
			b := float32(data[i+2]) / 255.0 / a
			fr, fg, fb, fa := w.cf.Filter(r, g, b, a)
			fa *= a
			d[i] = uint8(clamp01c(fr * fa))
			d[i+1] = uint8(clamp01c(fg * fa))
			d[i+2] = uint8(clamp01c(fb * fa))
			d[i+3] = uint8(clamp01c(fa))
		}
	}
}

func clamp01c(v float32) float32 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return float32(v * 255)
}
