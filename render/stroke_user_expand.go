package render

import (
	"math"

	"github.com/energye/gpui/render/internal/stroke"
)

// expandStrokeToPathSpace expands the current stroke in pure user space
// (undoing the CTM baked into path points by MoveTo/LineTo), then maps the
// outline back through c.matrix so it lives in the same coordinate space as
// normal path storage (pre-deviceMatrix). doFill then applies HiDPI deviceMatrix.
//
// This fixes T.03: non-uniform scale produces anisotropic stroke thickness
// (Skia/Cairo: stroke in user space, then transform the outline).
func (c *Context) expandStrokeToPathSpace() *Path {
	if c.path == nil || c.path.NumVerbs() == 0 {
		return nil
	}

	width := c.paint.EffectiveLineWidth()
	hairline := width <= 0
	if hairline {
		// 1 device pixel in path space (path space is pre-deviceMatrix).
		ds := c.deviceScale
		if ds <= 0 {
			ds = 1
		}
		width = 1.0 / ds
	}

	// Recover pure user-space geometry (before c.matrix was baked into points).
	userPath := c.path
	if !c.matrix.IsIdentity() {
		det := c.matrix.A*c.matrix.E - c.matrix.B*c.matrix.D
		if math.Abs(det) > 1e-12 {
			userPath = c.path.Transform(c.matrix.Invert())
		}
	}

	pathToStroke := userPath
	if c.paint.IsDashed() {
		dash := c.paint.EffectiveDash()
		if dash != nil && dash.IsDashed() {
			// Dash periods are specified in user space.
			pathToStroke = ApplyDash(userPath, dash)
			if pathToStroke == nil || pathToStroke.NumVerbs() == 0 {
				return nil
			}
		}
	}

	outline := expandStrokePath(pathToStroke, width, c.paint)
	if outline == nil || outline.NumVerbs() == 0 {
		return nil
	}

	// Map outline into path storage space (user CTM applied, device not yet).
	if c.matrix.IsIdentity() {
		return outline
	}
	return outline.Transform(c.matrix)
}

func expandStrokePath(p *Path, width float64, paint *Paint) *Path {
	if p == nil || p.NumVerbs() == 0 {
		return nil
	}
	if width < 1e-6 {
		width = 1e-6
	}
	style := stroke.Stroke{
		Width:      width,
		Cap:        convertLineCap(paint.EffectiveLineCap()),
		Join:       convertLineJoin(paint.EffectiveLineJoin()),
		MiterLimit: paint.EffectiveMiterLimit(),
	}
	if style.MiterLimit <= 0 {
		style.MiterLimit = 4.0
	}
	expander := stroke.NewStrokeExpander(style)
	outVerbs, outCoords := expander.Expand(convertVerbsToStroke(p.Verbs()), p.Coords())
	if len(outVerbs) == 0 {
		return nil
	}
	dst := NewPath()
	strokeResultToPath(dst, outVerbs, outCoords)
	return dst
}
