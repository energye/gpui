package icon

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// Vector glyphs in a size×size box (Y-down). Each stays inside
// hit == layout == paint; lineWidth follows §6.2.1.

func lineWidth(size float64) float64 {
	w := size * 0.125
	if w < 1.6 {
		return 1.6
	}
	if w > 2.5 {
		return 2.5
	}
	return w
}

func stroke(c render.RGBA) (r, g, b, a float64) { return c.R, c.G, c.B, c.A }

func drawPlaceholder(pc *rendering.PaintContext, size float64, c render.RGBA) {
	r, g, b, a := stroke(c)
	p := rendering.NewPath()
	p.MoveTo(size/2, size*0.12)
	p.LineTo(size*0.88, size/2)
	p.LineTo(size/2, size*0.88)
	p.LineTo(size*0.12, size/2)
	p.Close()
	rendering.StrokePath(pc, p, lineWidth(size), r, g, b, a)
}

func drawGlyph(pc *rendering.PaintContext, name string, size float64, base, primary, secondary render.RGBA, twoTone bool) {
	main := base
	if twoTone {
		main = primary
	}
	r, g, b, a := stroke(main)
	w := lineWidth(size)
	switch name {
	case "check":
		p := rendering.NewPath()
		p.MoveTo(size*0.22, size*0.54)
		p.LineTo(size*0.45, size*0.72)
		p.LineTo(size*0.78, size*0.30)
		rendering.StrokePath(pc, p, w, r, g, b, a)
	case "close":
		rendering.StrokeLine(pc, size*0.28, size*0.28, size*0.72, size*0.72, w, r, g, b, a)
		rendering.StrokeLine(pc, size*0.72, size*0.28, size*0.28, size*0.72, w, r, g, b, a)
	case "plus":
		rendering.StrokeLine(pc, size*0.5, size*0.24, size*0.5, size*0.76, w, r, g, b, a)
		rendering.StrokeLine(pc, size*0.24, size*0.5, size*0.76, size*0.5, w, r, g, b, a)
	case "minus":
		rendering.StrokeLine(pc, size*0.24, size*0.5, size*0.76, size*0.5, w, r, g, b, a)
	case "left":
		p := rendering.NewPath()
		p.MoveTo(size*0.62, size*0.24)
		p.LineTo(size*0.36, size*0.5)
		p.LineTo(size*0.62, size*0.76)
		rendering.StrokePath(pc, p, w, r, g, b, a)
	case "right":
		p := rendering.NewPath()
		p.MoveTo(size*0.38, size*0.24)
		p.LineTo(size*0.64, size*0.5)
		p.LineTo(size*0.38, size*0.76)
		rendering.StrokePath(pc, p, w, r, g, b, a)
	case "up":
		p := rendering.NewPath()
		p.MoveTo(size*0.24, size*0.62)
		p.LineTo(size*0.5, size*0.36)
		p.LineTo(size*0.76, size*0.62)
		rendering.StrokePath(pc, p, w, r, g, b, a)
	case "down":
		p := rendering.NewPath()
		p.MoveTo(size*0.24, size*0.38)
		p.LineTo(size*0.5, size*0.64)
		p.LineTo(size*0.76, size*0.38)
		rendering.StrokePath(pc, p, w, r, g, b, a)
	case "search":
		rendering.StrokeCircle(pc, size*0.44, size*0.44, size*0.24, w, r, g, b, a)
		rendering.StrokeLine(pc, size*0.62, size*0.62, size*0.80, size*0.80, w, r, g, b, a)
	case "star":
		p := rendering.NewPath()
		p.MoveTo(size*0.5, size*0.16)
		p.LineTo(size*0.60, size*0.42)
		p.LineTo(size*0.86, size*0.44)
		p.LineTo(size*0.66, size*0.62)
		p.LineTo(size*0.72, size*0.86)
		p.LineTo(size*0.5, size*0.72)
		p.LineTo(size*0.28, size*0.86)
		p.LineTo(size*0.34, size*0.62)
		p.LineTo(size*0.14, size*0.44)
		p.LineTo(size*0.40, size*0.42)
		p.Close()
		rendering.StrokePath(pc, p, w, r, g, b, a)
	case "home":
		p := rendering.NewPath()
		p.MoveTo(size*0.18, size*0.52)
		p.LineTo(size*0.5, size*0.22)
		p.LineTo(size*0.82, size*0.52)
		p.LineTo(size*0.74, size*0.52)
		p.LineTo(size*0.74, size*0.80)
		p.LineTo(size*0.26, size*0.80)
		p.LineTo(size*0.26, size*0.52)
		p.Close()
		rendering.StrokePath(pc, p, w, r, g, b, a)
	case "setting":
		rendering.StrokeCircle(pc, size*0.5, size*0.5, size*0.16, w, r, g, b, a)
		for i := 0; i < 6; i++ {
			var x1, y1, x2, y2 float64
			switch i {
			case 0:
				x1, y1, x2, y2 = size*0.5, size*0.16, size*0.5, size*0.28
			case 1:
				x1, y1, x2, y2 = size*0.5, size*0.72, size*0.5, size*0.84
			case 2:
				x1, y1, x2, y2 = size*0.16, size*0.5, size*0.28, size*0.5
			case 3:
				x1, y1, x2, y2 = size*0.72, size*0.5, size*0.84, size*0.5
			case 4:
				x1, y1, x2, y2 = size*0.26, size*0.26, size*0.35, size*0.35
			default:
				x1, y1, x2, y2 = size*0.65, size*0.65, size*0.74, size*0.74
			}
			rendering.StrokeLine(pc, x1, y1, x2, y2, w, r, g, b, a)
		}
	case "sync":
		rendering.StrokeArc(pc, size*0.5, size*0.52, size*0.28, -0.4, 4.2, w, r, g, b, a)
		p := rendering.NewPath()
		p.MoveTo(size*0.72, size*0.22)
		p.LineTo(size*0.78, size*0.40)
		p.LineTo(size*0.60, size*0.36)
		p.Close()
		rendering.FillPath(pc, p, r, g, b, a)
	case "loading":
		// LoadingOutlined: open ring spinner, no dot (rotates via Tick).
		rendering.StrokeArc(pc, size*0.5, size*0.5, size*0.30, 0.6, 5.4, w, r, g, b, a)
	case "heart":
		if twoTone {
			rendering.FillCircle(pc, size*0.36, size*0.40, size*0.20, secondary.R, secondary.G, secondary.B, secondary.A)
			rendering.FillCircle(pc, size*0.64, size*0.40, size*0.20, secondary.R, secondary.G, secondary.B, secondary.A)
			p := rendering.NewPath()
			p.MoveTo(size*0.20, size*0.48)
			p.LineTo(size*0.5, size*0.82)
			p.LineTo(size*0.80, size*0.48)
			p.Close()
			rendering.FillPath(pc, p, secondary.R, secondary.G, secondary.B, secondary.A)
		}
		p := rendering.NewPath()
		p.MoveTo(size*0.5, size*0.82)
		p.LineTo(size*0.20, size*0.48)
		p.LineTo(size*0.22, size*0.34)
		p.LineTo(size*0.36, size*0.28)
		p.LineTo(size*0.5, size*0.38)
		p.LineTo(size*0.64, size*0.28)
		p.LineTo(size*0.78, size*0.34)
		p.LineTo(size*0.80, size*0.48)
		p.Close()
		rendering.StrokePath(pc, p, w, r, g, b, a)
	case "smile":
		if twoTone {
			rendering.FillCircle(pc, size*0.5, size*0.5, size*0.32, secondary.R, secondary.G, secondary.B, secondary.A)
		}
		rendering.StrokeCircle(pc, size*0.5, size*0.5, size*0.32, w, r, g, b, a)
		rendering.FillCircle(pc, size*0.40, size*0.42, w*0.45, r, g, b, a)
		rendering.FillCircle(pc, size*0.60, size*0.42, w*0.45, r, g, b, a)
		rendering.StrokeArc(pc, size*0.5, size*0.52, size*0.17, 0.5, 2.6, w, r, g, b, a)
	case "edit":
		p := rendering.NewPath()
		p.MoveTo(size*0.30, size*0.64)
		p.LineTo(size*0.58, size*0.30)
		p.LineTo(size*0.70, size*0.42)
		p.LineTo(size*0.42, size*0.76)
		p.Close()
		rendering.StrokePath(pc, p, w, r, g, b, a)
		rendering.StrokeLine(pc, size*0.24, size*0.76, size*0.36, size*0.76, w, r, g, b, a)
	case "delete":
		rendering.StrokeRect(pc, size*0.32, size*0.36, size*0.36, size*0.44, w, r, g, b, a)
		rendering.StrokeLine(pc, size*0.26, size*0.36, size*0.74, size*0.36, w, r, g, b, a)
		rendering.StrokeLine(pc, size*0.42, size*0.26, size*0.58, size*0.26, w, r, g, b, a)
	case "file-text":
		p := rendering.NewPath()
		p.MoveTo(size*0.30, size*0.16)
		p.LineTo(size*0.62, size*0.16)
		p.LineTo(size*0.72, size*0.28)
		p.LineTo(size*0.72, size*0.84)
		p.LineTo(size*0.30, size*0.84)
		p.Close()
		rendering.StrokePath(pc, p, w, r, g, b, a)
		rendering.StrokeLine(pc, size*0.38, size*0.44, size*0.64, size*0.44, w*0.8, r, g, b, a)
		rendering.StrokeLine(pc, size*0.38, size*0.56, size*0.64, size*0.56, w*0.8, r, g, b, a)
	case "info-circle":
		rendering.StrokeCircle(pc, size*0.5, size*0.5, size*0.32, w, r, g, b, a)
		rendering.StrokeLine(pc, size*0.5, size*0.44, size*0.5, size*0.66, w, r, g, b, a)
		rendering.FillCircle(pc, size*0.5, size*0.32, w*0.5, r, g, b, a)
	case "warning":
		p := rendering.NewPath()
		p.MoveTo(size*0.5, size*0.18)
		p.LineTo(size*0.82, size*0.74)
		p.LineTo(size*0.18, size*0.74)
		p.Close()
		rendering.StrokePath(pc, p, w, r, g, b, a)
		rendering.StrokeLine(pc, size*0.5, size*0.40, size*0.5, size*0.58, w, r, g, b, a)
		rendering.FillCircle(pc, size*0.5, size*0.66, w*0.45, r, g, b, a)
	case "exclamation-circle":
		rendering.StrokeCircle(pc, size*0.5, size*0.5, size*0.32, w, r, g, b, a)
		rendering.StrokeLine(pc, size*0.5, size*0.32, size*0.5, size*0.56, w, r, g, b, a)
		rendering.FillCircle(pc, size*0.5, size*0.66, w*0.5, r, g, b, a)
	case "check-circle":
		if twoTone {
			rendering.FillCircle(pc, size*0.5, size*0.5, size*0.32, secondary.R, secondary.G, secondary.B, secondary.A)
		} else {
			rendering.StrokeCircle(pc, size*0.5, size*0.5, size*0.32, w, r, g, b, a)
		}
		p := rendering.NewPath()
		p.MoveTo(size*0.36, size*0.52)
		p.LineTo(size*0.47, size*0.63)
		p.LineTo(size*0.66, size*0.40)
		rendering.StrokePath(pc, p, w, r, g, b, a)
	case "close-circle":
		rendering.StrokeCircle(pc, size*0.5, size*0.5, size*0.32, w, r, g, b, a)
		rendering.StrokeLine(pc, size*0.40, size*0.40, size*0.60, size*0.60, w*0.9, r, g, b, a)
		rendering.StrokeLine(pc, size*0.60, size*0.40, size*0.40, size*0.60, w*0.9, r, g, b, a)
	case "poweroff":
		// Power symbol: broken ring + vertical bar (Ant PoweroffOutlined).
		rendering.StrokeArc(pc, size*0.5, size*0.54, size*0.28, 0.7, 5.6, w, r, g, b, a)
		rendering.StrokeLine(pc, size*0.5, size*0.18, size*0.5, size*0.52, w, r, g, b, a)
	case "download":
		// DownloadOutlined: arrow down + base line (Ant DownloadOutlined).
		rendering.StrokeLine(pc, size*0.5, size*0.18, size*0.5, size*0.62, w, r, g, b, a)
		p := rendering.NewPath()
		p.MoveTo(size*0.32, size*0.46)
		p.LineTo(size*0.5, size*0.64)
		p.LineTo(size*0.68, size*0.46)
		rendering.StrokePath(pc, p, w, r, g, b, a)
		rendering.StrokeLine(pc, size*0.24, size*0.78, size*0.76, size*0.78, w, r, g, b, a)
	case "ellipsis":
		// EllipsisOutlined: three dots (Ant EllipsisOutlined).
		rendering.FillCircle(pc, size*0.24, size*0.5, w*0.55, r, g, b, a)
		rendering.FillCircle(pc, size*0.5, size*0.5, w*0.55, r, g, b, a)
		rendering.FillCircle(pc, size*0.76, size*0.5, w*0.55, r, g, b, a)
	case "ant-design":
		// AntDesignOutlined approximation: rounded square + "A" stroke.
		rendering.StrokeRect(pc, size*0.22, size*0.22, size*0.56, size*0.56, w, r, g, b, a)
		p2 := rendering.NewPath()
		p2.MoveTo(size*0.5, size*0.32)
		p2.LineTo(size*0.36, size*0.68)
		p2.MoveTo(size*0.5, size*0.32)
		p2.LineTo(size*0.64, size*0.68)
		p2.MoveTo(size*0.41, size*0.56)
		p2.LineTo(size*0.59, size*0.56)
		rendering.StrokePath(pc, p2, w*0.9, r, g, b, a)
	default:
		drawPlaceholder(pc, size, base)
	}
}
