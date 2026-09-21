package kit

// Fixed QR geometry aligned to qr-code.md §6.2.1.
// Painting stays in prim when F0-2 lands; this file only computes rects.
const (
	// QRCodeDefaultSize is the outer edge (160).
	QRCodeDefaultSize = 160.0
	// QRCodePad is the bordered inner padding (paddingSM = 12).
	QRCodePad = 12.0
	// QRCodeRadius is the outer corner radius (borderRadiusLG = 8,
	// not the generic radius 6).
	QRCodeRadius = 8.0
	// QRCodeLineWidth is the border width (1).
	QRCodeLineWidth = 1.0
	// QRCodeDefaultIcon is the default center icon edge (40).
	QRCodeDefaultIcon = 40.0
)

// QRCodeRect is one QR part rect in logical pixels.
type QRCodeRect struct {
	X, Y, W, H float64
}

// QRCodeLayout carries the outer/content/icon rects.
type QRCodeLayout struct {
	Outer   QRCodeRect
	Content QRCodeRect
	Icon    QRCodeRect
	HasIcon bool
	Pad     float64
	Radius  float64
}

// ResolveQRCodePadding returns 12 bordered else 0 (borderless).
func ResolveQRCodePadding(bordered bool) float64 {
	if bordered {
		return QRCodePad
	}
	return 0
}

// ComputeQRCodeLayout returns outer/content/icon rects for an edge.
// Icon is centered in content; empty icon source hides it.
func ComputeQRCodeLayout(edge float64, bordered bool, iconW, iconH float64, hasIcon bool) QRCodeLayout {
	if edge <= 0 {
		edge = QRCodeDefaultSize
	}
	pad := ResolveQRCodePadding(bordered)
	radius := QRCodeRadius
	if !bordered {
		radius = 0
	}
	content := QRCodeRect{X: pad, Y: pad, W: edge - pad*2, H: edge - pad*2}
	var icon QRCodeRect
	if hasIcon && iconW > 0 && iconH > 0 {
		if iconW > content.W {
			iconW = content.W
		}
		if iconH > content.H {
			iconH = content.H
		}
		icon = QRCodeRect{
			X: pad + (content.W-iconW)/2,
			Y: pad + (content.H-iconH)/2,
			W: iconW,
			H: iconH,
		}
	}
	return QRCodeLayout{
		Outer:   QRCodeRect{X: 0, Y: 0, W: edge, H: edge},
		Content: content,
		Icon:    icon,
		HasIcon: hasIcon && iconW > 0 && iconH > 0,
		Pad:     pad,
		Radius:  radius,
	}
}

// QRCodeModuleSize returns the module edge for a matrix with margin.
func QRCodeModuleSize(contentEdge float64, modules, margin int) float64 {
	total := modules + margin*2
	if total <= 0 || contentEdge <= 0 {
		return 0
	}
	return contentEdge / float64(total)
}

// QRCodeModuleXY returns the top-left of module (r, c) with margin.
func QRCodeModuleXY(contentX, contentY, module float64, r, c, margin int) (x, y float64) {
	return contentX + (float64(c+margin))*module, contentY + (float64(r+margin))*module
}

// QRCodePaddedSize returns the padded matrix edge (modules + 2*margin).
func QRCodePaddedSize(modules, margin int) int {
	if margin < 0 {
		margin = 0
	}
	return modules + margin*2
}

// ExcavateQRCodeIcon clears the modules overlapped by the center icon.
// iconX/Y/W/H are content-relative logical pixels; module is the module
// edge from QRCodeModuleSize. Returns a copy; the input matrix is kept.
// Draw layers call this before rasterizing so the icon sits on white.
func ExcavateQRCodeIcon(matrix [][]bool, iconX, iconY, iconW, iconH, module float64) [][]bool {
	out := make([][]bool, len(matrix))
	for i, row := range matrix {
		out[i] = append([]bool(nil), row...)
	}
	if module <= 0 || iconW <= 0 || iconH <= 0 || len(matrix) == 0 {
		return out
	}
	n := len(matrix)
	c0 := int(iconX / module)
	r0 := int(iconY / module)
	c1 := int((iconX + iconW - 1) / module)
	r1 := int((iconY + iconH - 1) / module)
	if c0 < 0 {
		c0 = 0
	}
	if r0 < 0 {
		r0 = 0
	}
	if c1 >= n {
		c1 = n - 1
	}
	if r1 >= n {
		r1 = n - 1
	}
	for r := r0; r <= r1; r++ {
		for c := c0; c <= c1; c++ {
			out[r][c] = false
		}
	}
	return out
}
