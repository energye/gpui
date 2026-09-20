package kit

// ModalHostPanelRect is the pure panel geometry for one entry.
// Painting stays in prim when F0-2 lands; this file only computes rects.
type ModalHostPanelRect struct {
	X, Y, W, H float64
	ZIndex     int
	Centered   bool
}

// ResolveModalHostWidth returns explicit width else 416 for confirm family.
func ResolveModalHostWidth(cfg ModalHostConfirmConfig) float64 {
	if cfg.WidthSet && cfg.Width > 0 {
		return cfg.Width
	}
	if cfg.Width > 0 && cfg.WidthSet {
		return cfg.Width
	}
	return 416
}

// ComputeModalHostPanel centers horizontally; Y is top 100 unless centered.
func ComputeModalHostPanel(viewportW, viewportH float64, cfg ModalHostConfirmConfig, zIndex int) ModalHostPanelRect {
	w := ResolveModalHostWidth(cfg)
	if w <= 0 {
		w = 416
	}
	if w > viewportW-48 {
		w = viewportW - 48
	}
	h := 220.0
	if viewportH-200 < h {
		h = viewportH - 200
		if h < 120 {
			h = 120
		}
	}
	x := (viewportW - w) / 2
	y := 100.0
	centered := cfg.Centered
	if centered {
		y = (viewportH - h) / 2
		if y < 24 {
			y = 24
		}
	} else if y+h > viewportH-24 {
		y = viewportH - h - 24
		if y < 24 {
			y = 24
		}
	}
	return ModalHostPanelRect{X: x, Y: y, W: w, H: h, ZIndex: zIndex, Centered: centered}
}

// ZIndexForDepth returns base + depth*10 stacking.
func ZIndexForDepth(base, depth int) int {
	if base == 0 {
		base = 1000
	}
	if depth < 0 {
		depth = 0
	}
	return base + depth*10
}
