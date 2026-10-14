//go:build !nogpu

package gpu

import (
	"encoding/binary"
	"math"
)

// rigidQuadInfo describes a device-pixel quad that is a rotated rectangle:
// uniform scale + rotation + translation of an axis-aligned box (no shear).
// All values are device pixels (post-CTM), angle in radians of the top edge.
type rigidQuadInfo struct {
	cx, cy float32
	w, h   float32
	angle  float32
	ok     bool
}

// detectRigidQuad checks whether TL/TR/BR/BL form a rigid rectangle
// (opposite edges equal, adjacent edges perpendicular, diagonals equal).
// Tolerance is tight (sub-pixel) so shear/skew fall back to baked vertices.
// Pure math, no GPU I/O: safe to unit-test without a device.
func detectRigidQuad(tlx, tly, trx, try_, brx, bry, blx, bly float32) rigidQuadInfo {
	// Edge vectors in float64 for stable dot/length math.
	topX := float64(trx - tlx)
	topY := float64(try_ - tly)
	rightX := float64(brx - trx)
	rightY := float64(bry - try_)
	bottomX := float64(brx - blx)
	bottomY := float64(bry - bly)
	leftX := float64(blx - tlx)
	leftY := float64(bly - tly)

	topLen := math.Hypot(topX, topY)
	bottomLen := math.Hypot(bottomX, bottomY)
	rightLen := math.Hypot(rightX, rightY)
	leftLen := math.Hypot(leftX, leftY)
	if topLen <= 1e-9 || bottomLen <= 1e-9 || rightLen <= 1e-9 || leftLen <= 1e-9 {
		return rigidQuadInfo{}
	}
	maxDim := math.Max(math.Max(topLen, bottomLen), math.Max(rightLen, leftLen))
	// Absolute tolerance: 1e-3px covers float32 CTM rounding (ulp ~1e-5 at
	// a few hundred px) while rejecting real shear (px-scale deviation).
	const tol = 1e-3
	if math.Abs(topLen-bottomLen) > tol || math.Abs(rightLen-leftLen) > tol {
		return rigidQuadInfo{}
	}
	// Adjacent edges perpendicular: |dot| <= tol * |a| * |b| / maxDim-ish.
	// Use absolute form: dot/(lenA*lenB) is cos(angle); for 90deg cos=0.
	dot := topX*rightX + topY*rightY
	if math.Abs(dot) > tol*topLen*rightLen/math.Max(maxDim, 1) && math.Abs(dot/(topLen*rightLen)) > 1e-4 {
		return rigidQuadInfo{}
	}
	// Parallelogram check: midpoints of diagonals coincide.
	mid1X := (float64(tlx) + float64(brx)) * 0.5
	mid1Y := (float64(tly) + float64(bry)) * 0.5
	mid2X := (float64(trx) + float64(blx)) * 0.5
	mid2Y := (float64(try_) + float64(bly)) * 0.5
	if math.Abs(mid1X-mid2X) > tol || math.Abs(mid1Y-mid2Y) > tol {
		return rigidQuadInfo{}
	}
	// Rectangle check: diagonals equal.
	diag1 := math.Hypot(float64(brx-tlx), float64(bry-tly))
	diag2 := math.Hypot(float64(blx-trx), float64(bly-try_))
	if math.Abs(diag1-diag2) > tol {
		return rigidQuadInfo{}
	}
	cx := snap256(float32((mid1X + mid2X) * 0.5))
	cy := snap256(float32((mid1Y + mid2Y) * 0.5))
	w := snap256(float32((topLen + bottomLen) * 0.5))
	h := snap256(float32((rightLen + leftLen) * 0.5))
	angle := float32(math.Atan2(topY, topX))
	return rigidQuadInfo{cx: cx, cy: cy, w: w, h: h, angle: angle, ok: true}
}

// snap256 quantizes a device-pixel value to a 1/256px grid. Derived
// center/size from baked corners jitters ±1ulp across frames (float32 CTM
// rounding at a few hundred px is ~1e-5px); snapping absorbs the jitter so
// the axis-aligned base stays bit-identical and the vertex-hash skip can
// engage. Max error 1/512px (~8e-6 NDC at 480px) stays inside the 2e-5
// pixel-equivalence tolerance.
func snap256(v float32) float32 {
	return float32(math.Round(float64(v)*256) / 256)
}

// axisAlignedBase returns the unrotated box (x0,y0,x1,y1) for a rigid quad.
// Center/size arrive pre-snapped from detectRigidQuad, so the base stays
// bit-identical across frames despite ±1ulp baked-corner jitter.
func (r rigidQuadInfo) axisAlignedBase() (x0, y0, x1, y1 float32) {
	cx, cy := r.cx, r.cy
	w, h := r.w, r.h
	return cx - w/2, cy - h/2, cx + w/2, cy + h/2
}

// putImageRotationUniform writes one image uniform block combining the ortho
// projection with T(center)*R(angle)*T(-center), same 80-byte layout as
// putImageUniform. Angle 0 composes to bit-identical bytes as the ortho-only
// path (cos=1/sin=0 exact, translation term 0). Pure math, no GPU I/O.
func putImageRotationUniform(dst []byte, viewportW, viewportH uint32, opacity, cx, cy, angle float32) {
	if len(dst) < int(imageUniformSize) {
		panic("putImageRotationUniform: dst too small")
	}
	for i := 0; i < int(imageUniformSize); i++ {
		dst[i] = 0
	}
	cos := math.Cos(float64(angle))
	sin := math.Sin(float64(angle))
	w := float64(viewportW)
	h := float64(viewportH)
	if w <= 0 || h <= 0 {
		return
	}
	sx := 2.0 / w
	sy := -2.0 / h
	cxf, cyf := float64(cx), float64(cy)
	txAff := cxf - cos*cxf + sin*cyf
	tyAff := cyf - sin*cxf - cos*cyf
	m00 := sx * cos
	m01 := -sx * sin
	m10 := sy * sin
	m11 := sy * cos
	m03 := sx*txAff - 1.0
	m13 := sy*tyAff + 1.0
	// Flush -0.0 to +0.0 so angle 0 stays bit-identical to the ortho-only
	// path (cleared slots hold +0.0; -0.0 would break byte equality and
	// the uniform-skip hash).
	if m00 == 0 {
		m00 = 0
	}
	if m01 == 0 {
		m01 = 0
	}
	if m10 == 0 {
		m10 = 0
	}
	if m11 == 0 {
		m11 = 0
	}
	if m03 == 0 {
		m03 = 0
	}
	if m13 == 0 {
		m13 = 0
	}
	binary.LittleEndian.PutUint32(dst[0:], math.Float32bits(float32(m00)))
	binary.LittleEndian.PutUint32(dst[4:], math.Float32bits(float32(m10)))
	binary.LittleEndian.PutUint32(dst[16:], math.Float32bits(float32(m01)))
	binary.LittleEndian.PutUint32(dst[20:], math.Float32bits(float32(m11)))
	binary.LittleEndian.PutUint32(dst[40:], math.Float32bits(1.0))
	binary.LittleEndian.PutUint32(dst[48:], math.Float32bits(float32(m03)))
	binary.LittleEndian.PutUint32(dst[52:], math.Float32bits(float32(m13)))
	binary.LittleEndian.PutUint32(dst[60:], math.Float32bits(1.0))
	binary.LittleEndian.PutUint32(dst[64:], math.Float32bits(opacity))
}

// applyRotationUniform transforms one device-pixel point through a uniform
// block written by putImageRotationUniform (CPU mirror of vs_main).
func applyRotationUniform(u []byte, x, y float32) (ndcX, ndcY float32) {
	m00 := math.Float32frombits(binary.LittleEndian.Uint32(u[0:]))
	m10 := math.Float32frombits(binary.LittleEndian.Uint32(u[4:]))
	m01 := math.Float32frombits(binary.LittleEndian.Uint32(u[16:]))
	m11 := math.Float32frombits(binary.LittleEndian.Uint32(u[20:]))
	m03 := math.Float32frombits(binary.LittleEndian.Uint32(u[48:]))
	m13 := math.Float32frombits(binary.LittleEndian.Uint32(u[52:]))
	return x*m00 + y*m01 + m03, x*m10 + y*m11 + m13
}

// applyOrthoUniform transforms one point through putImageUniform output.
func applyOrthoUniform(u []byte, x, y float32) (ndcX, ndcY float32) {
	return applyRotationUniform(u, x, y)
}

// gpuTexHasRotation reports whether a GPU texture overlay command should
// ride the rotation-uniform path. Nil-safe; YUV and axis-aligned quads
// stay on the baked-vertex path with the shared ortho-only uniform.
func gpuTexHasRotation(cmd *GPUTextureDrawCommand) (rigidQuadInfo, bool) {
	if cmd == nil || cmd.IsYUV {
		return rigidQuadInfo{}, false
	}
	return quadRotationOf(cmd.TLX, cmd.TLY, cmd.TRX, cmd.TRY, cmd.BRX, cmd.BRY, cmd.BLX, cmd.BLY)
}

// gpuTexBaseCorners returns the vertex corners for a GPU texture overlay:
// the static axis-aligned base for rigid rotations, else the baked corners
// bit-identically. Nil-safe (zero corners → vertex fallback to Dst rect).
func gpuTexBaseCorners(cmd *GPUTextureDrawCommand) (tlx, tly, trx, try_, brx, bry, blx, bly float32) {
	if cmd == nil {
		return 0, 0, 0, 0, 0, 0, 0, 0
	}
	if rot, ok := gpuTexHasRotation(cmd); ok {
		x0, y0, x1, y1 := rot.axisAlignedBase()
		return x0, y0, x1, y0, x1, y1, x0, y1
	}
	return cmd.TLX, cmd.TLY, cmd.TRX, cmd.TRY, cmd.BRX, cmd.BRY, cmd.BLX, cmd.BLY
}

// writeGpuTexUniform writes one gpu-tex uniform slot: rotation-composed for
// rigid rotations, ortho-only otherwise (bit-identical to putImageUniform).
func writeGpuTexUniform(dst []byte, viewportW, viewportH uint32, opacity float32, rot rigidQuadInfo, hasRot bool) {
	if hasRot {
		putImageRotationUniform(dst, viewportW, viewportH, opacity, rot.cx, rot.cy, rot.angle)
		return
	}
	putImageUniform(dst, viewportW, viewportH, opacity)
}

// imageUniformKeyMatches reports whether a last-upload key covers a slot,
// including the rotation term. Zero rotation preserves the historic
// ortho-only skip semantics bit-identically.
func imageUniformKeyMatches(last *imageUniformLastKey, w, h uint32, opacity float32, rot rigidQuadInfo, hasRot bool) bool {
	if last == nil || !last.valid || last.w != w || last.h != h || last.opacity != opacity {
		return false
	}
	if last.hasRot != hasRot {
		return false
	}
	if hasRot && (last.rotCX != rot.cx || last.rotCY != rot.cy || last.rotW != rot.w || last.rotH != rot.h || last.rotAngle != rot.angle) {
		return false
	}
	return true
}

// storeImageUniformKey records a slot upload into a last-upload key.
func storeImageUniformKey(last *imageUniformLastKey, w, h uint32, opacity float32, rot rigidQuadInfo, hasRot bool) {
	if last == nil {
		return
	}
	last.w, last.h = w, h
	last.opacity = opacity
	last.hasRot = hasRot
	last.rotCX, last.rotCY = rot.cx, rot.cy
	last.rotW, last.rotH = rot.w, rot.h
	last.rotAngle = rot.angle
	last.valid = true
}

// quadRotationOf reports whether explicit TL/TR/BR/BL corners carry a
// non-trivial rigid rotation for the uniform path.
//
// Zero corners (axis fallback), axis-aligned quads, shear/skew, mirrored
// quads (negative orientation), and degenerate quads all report false and
// keep the baked-vertex path with the shared ortho-only uniform. Only true
// rigid rotations report true with the decomposed center/size/angle.
func quadRotationOf(tlx, tly, trx, try_, brx, bry, blx, bly float32) (rigidQuadInfo, bool) {
	if tlx == 0 && tly == 0 && trx == 0 && try_ == 0 &&
		brx == 0 && bry == 0 && blx == 0 && bly == 0 {
		return rigidQuadInfo{}, false
	}
	info := detectRigidQuad(tlx, tly, trx, try_, brx, bry, blx, bly)
	if !info.ok {
		return rigidQuadInfo{}, false
	}
	// Orientation: proper rotations preserve TL->TR->BR winding
	// (cross > 0); mirrored quads (negative determinant) must stay baked.
	cross := float64(trx-tlx)*float64(bry-try_) - float64(try_-tly)*float64(brx-trx)
	if !(cross > 0) {
		return rigidQuadInfo{}, false
	}
	// Axis-aligned quads keep the bit-identical baked path (protects
	// existing goldens); only real rotation rides the uniform.
	if info.angle == 0 {
		return rigidQuadInfo{}, false
	}
	return info, true
}
