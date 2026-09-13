//go:build !nogpu

package gpu

import (
	"encoding/binary"
	"math"

	"github.com/energye/gpui/render"
)

// Standard layout (Skia Graphite / Vello style): tessellated geometry is
// cached in USER space (transform-independent), and the per-draw affine
// transform (user → device pixels) rides in the stencil uniform. Animated
// draws that only move (translate/rotate/uniform-scale) hit the geometry
// cache every frame; only a 64B uniform + tiny cover quad upload per draw.
//
// Fallback (baked device-space + identity matrix) applies whenever the
// transform is not a similarity (non-uniform scale / skew), the matrix is
// singular, AA is off (device-space snap), or the stroke is dashed/hairline.
// The fallback path preserves the baked behavior bit-identically.

// uniformSimilarityOK reports whether m is a similarity transform
// (translate / rotate / uniform-scale, no skew or non-uniform scale) with a
// usable scale, returning the uniform scale factor s (device px per user unit).
// Only similarities preserve stroke width and AA band shape under a single
// scalar, so only they may use the transform-uniform fast path.
func uniformSimilarityOK(m render.Matrix) (s float64, ok bool) {
	// Column norms must match (uniform scale), columns must be orthogonal
	// (no skew), and the scale must be finite and non-degenerate.
	c0 := m.A*m.A + m.D*m.D
	c1 := m.B*m.B + m.E*m.E
	if !(c0 > 1e-18) || !(c1 > 1e-18) {
		return 0, false
	}
	s0 := math.Sqrt(c0)
	// Relative tolerance: coordinates span ~1e3 px, float64 gives ~1e-13.
	rel := math.Abs(c0-c1) / (c0 + c1)
	if rel > 1e-9 {
		return 0, false
	}
	dot := m.A*m.B + m.D*m.E
	if math.Abs(dot)/(c0+c1) > 1e-9 {
		return 0, false
	}
	if math.IsNaN(s0) || math.IsInf(s0, 0) {
		return 0, false
	}
	return s0, true
}

// invertAffine inverts m, reporting false for singular matrices.
// (render.Matrix.Invert masks singularity as identity; F2 must distinguish.)
func invertAffine(m render.Matrix) (render.Matrix, bool) {
	det := m.A*m.E - m.B*m.D
	if math.Abs(det) < 1e-12 {
		return render.Matrix{}, false
	}
	invDet := 1.0 / det
	return render.Matrix{
		A: m.E * invDet,
		B: -m.B * invDet,
		C: (m.B*m.F - m.C*m.E) * invDet,
		D: -m.D * invDet,
		E: m.A * invDet,
		F: (m.C*m.D - m.A*m.F) * invDet,
	}, true
}

// quantCoord is the F2 hash quantum (user units) and quantScaleQ the
// relative quantum for scales/widths. Round-trip error of an unbaked
// similarity is ~1e-13 at stage magnitudes; the flatten tolerance is ~1e-1.
// Quantum 1e-6 sits 1e5× below visibility (merged shapes rasterize
// identically) while keeping boundary-flip misses negligible (~1e-7/coord,
// vs ~1e-4 at 1e-9 where true values straddling a grid line flip every
// frame). Scales quantize at 1e-9 relative: per-frame rotation preserves
// column norms only up to float64 rounding (~1e-16), which would otherwise
// flip cache keys every frame with zero visual difference.
const (
	quantCoord  = 1e6
	quantScaleQ = 1e9
)

func quantizeCoord(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}
	q := math.Round(x*quantCoord) / quantCoord
	if q == 0 {
		return 0 // normalize -0.0 → +0.0 (equal numerically, differ in bits → hash flip)
	}
	return q
}

// quantizeScale snaps a scale/width to 1e-9 relative: per-frame rotation
// preserves column norms only up to float64 rounding (~1e-16), which would
// otherwise flip cache keys every frame with zero visual difference.
func quantizeScale(s float64) float64 {
	if !(s > 0) || math.IsNaN(s) || math.IsInf(s, 0) {
		return s
	}
	return math.Round(s*quantScaleQ) / quantScaleQ
}

func unbakedHash(path *render.Path, inv render.Matrix) uint64 {
	h := fnv64aNew()
	var vb [1]byte
	for _, v := range path.Verbs() {
		vb[0] = byte(v)
		h.write(vb[:])
	}
	var buf [8]byte
	putF64 := func(f float64) {
		u := math.Float64bits(f)
		buf[0] = byte(u)
		buf[1] = byte(u >> 8)
		buf[2] = byte(u >> 16)
		buf[3] = byte(u >> 24)
		buf[4] = byte(u >> 32)
		buf[5] = byte(u >> 40)
		buf[6] = byte(u >> 48)
		buf[7] = byte(u >> 56)
		h.write(buf[:])
	}
	coords := path.Coords()
	verbs := path.Verbs()
	off := 0
	for _, v := range verbs {
		n := 0
		switch v {
		case render.MoveTo, render.LineTo:
			n = 2
		case render.QuadTo:
			n = 4
		case render.CubicTo:
			n = 6
		case render.Close:
			n = 0
		default:
			n = 0
		}
		for i := 0; i < n; i += 2 {
			if off+i+1 >= len(coords) {
				break
			}
			x := coords[off+i]
			y := coords[off+i+1]
			putF64(quantizeCoord(inv.A*x + inv.B*y + inv.C))
			putF64(quantizeCoord(inv.D*x + inv.E*y + inv.F))
		}
		off += n
	}
	// Trailing coords beyond verbs (should not happen) still affect identity.
	for ; off < len(coords); off++ {
		putF64(coords[off])
	}
	return h.sum()
}

// unbakePath materializes path transformed by inv (device → user space).
// Only called on cache miss; hits use unbakedHash and never allocate here.
// NOTE: the materialized path keeps full float64 precision — only the hash
// is quantized. A shape tessellated from it is bit-comparable to one from
// the original user-space path up to ~1e-13, far below the flatten tolerance.
func unbakePath(path *render.Path, inv render.Matrix) *render.Path {
	out := render.NewPath()
	path.Iterate(func(verb render.PathVerb, coords []float64) {
		switch verb {
		case render.MoveTo:
			p := inv.TransformPoint(render.Pt(coords[0], coords[1]))
			out.MoveTo(p.X, p.Y)
		case render.LineTo:
			p := inv.TransformPoint(render.Pt(coords[0], coords[1]))
			out.LineTo(p.X, p.Y)
		case render.QuadTo:
			c := inv.TransformPoint(render.Pt(coords[0], coords[1]))
			p := inv.TransformPoint(render.Pt(coords[2], coords[3]))
			out.QuadraticTo(c.X, c.Y, p.X, p.Y)
		case render.CubicTo:
			c1 := inv.TransformPoint(render.Pt(coords[0], coords[1]))
			c2 := inv.TransformPoint(render.Pt(coords[2], coords[3]))
			p := inv.TransformPoint(render.Pt(coords[4], coords[5]))
			out.CubicTo(c1.X, c1.Y, c2.X, c2.Y, p.X, p.Y)
		case render.Close:
			out.Close()
		}
	})
	return out
}

// deviceCoverQuad builds the stencil cover quad (device pixels) from the
// baked path bounds. The cover pass has no transform (quad is pre-placed),
// so this stays O(1) via the incrementally maintained bounds.
func deviceCoverQuadFromBounds(path *render.Path) [12]float32 {
	r := path.Bounds()
	return deviceCoverQuadRect(float64(r.Min.X), float64(r.Min.Y), float64(r.Max.X), float64(r.Max.Y), render.Identity())
}

// deviceCoverQuadTransformed builds the device-space cover quad from a
// user-space path and its total matrix (F2 stroke tail: only the raw outline
// exists, so the baked bounds are unavailable).
func deviceCoverQuadTransformed(rawPath *render.Path, m render.Matrix) [12]float32 {
	r := rawPath.Bounds()
	return deviceCoverQuadRect(float64(r.Min.X), float64(r.Min.Y), float64(r.Max.X), float64(r.Max.Y), m)
}

func deviceCoverQuadRect(minX, minY, maxX, maxY float64, m render.Matrix) [12]float32 {
	if !m.IsIdentity() {
		p1 := m.TransformPoint(render.Pt(minX, minY))
		p2 := m.TransformPoint(render.Pt(maxX, minY))
		p3 := m.TransformPoint(render.Pt(maxX, maxY))
		p4 := m.TransformPoint(render.Pt(minX, maxY))
		minX = math.Min(math.Min(p1.X, p2.X), math.Min(p3.X, p4.X))
		minY = math.Min(math.Min(p1.Y, p2.Y), math.Min(p3.Y, p4.Y))
		maxX = math.Max(math.Max(p1.X, p2.X), math.Max(p3.X, p4.X))
		maxY = math.Max(math.Max(p1.Y, p2.Y), math.Max(p3.Y, p4.Y))
	}
	fminX := float32(minX) - fanCoverPadding
	fminY := float32(minY) - fanCoverPadding
	fmaxX := float32(maxX) + fanCoverPadding
	fmaxY := float32(maxY) + fanCoverPadding
	return [12]float32{
		fminX, fminY, fmaxX, fminY, fmaxX, fmaxY,
		fminX, fminY, fmaxX, fmaxY, fminX, fmaxY,
	}
}

// makeStencilUniform builds the 64B stencil uniform: viewport + affine rows
// + color. Layout must match shaders/stencil_fill.wgsl Uniforms.
func makeStencilUniform(w, h uint32, m render.Matrix, color [4]float32) []byte {
	buf := make([]byte, stencilFillUniformSize)
	binary.LittleEndian.PutUint32(buf[0:4], math.Float32bits(float32(w)))
	binary.LittleEndian.PutUint32(buf[4:8], math.Float32bits(float32(h)))
	// bytes 8..15 pad zero (already zeroed by make).
	binary.LittleEndian.PutUint32(buf[16:20], math.Float32bits(float32(m.A)))
	binary.LittleEndian.PutUint32(buf[20:24], math.Float32bits(float32(m.B)))
	binary.LittleEndian.PutUint32(buf[24:28], math.Float32bits(float32(m.C)))
	// bytes 28..31 pad.
	binary.LittleEndian.PutUint32(buf[32:36], math.Float32bits(float32(m.D)))
	binary.LittleEndian.PutUint32(buf[36:40], math.Float32bits(float32(m.E)))
	binary.LittleEndian.PutUint32(buf[40:44], math.Float32bits(float32(m.F)))
	// bytes 44..47 pad.
	binary.LittleEndian.PutUint32(buf[48:52], math.Float32bits(color[0]))
	binary.LittleEndian.PutUint32(buf[52:56], math.Float32bits(color[1]))
	binary.LittleEndian.PutUint32(buf[56:60], math.Float32bits(color[2]))
	binary.LittleEndian.PutUint32(buf[60:64], math.Float32bits(color[3]))
	return buf
}

// fnv64aNew is a tiny FNV-1a writer (avoids hash/fnv allocs on the hot path).
type fnv64a struct{ h uint64 }

func fnv64aNew() *fnv64a { return &fnv64a{h: 14695981039346656037} }

func (f *fnv64a) write(b []byte) {
	for _, c := range b {
		f.h ^= uint64(c)
		f.h *= 1099511628211
	}
}

func (f *fnv64a) sum() uint64 { return f.h }
