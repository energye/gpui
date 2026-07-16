//go:build linux && !nogpu

package main

import (
	"math"

	"github.com/energye/gpui/render"
)

// mesh3dPool reuses triangle buffers so the 3D module does not churn heap each frame
// (Skia-class soak: low alloc → stable RSS). Objects write directly into one batch
// and flush with a single DrawMesh (GPU colored triangles).
type mesh3dPool struct {
	pos []render.Point
	col []render.RGBA
	idx []uint16
	nV  int
	nI  int
}

var gMesh3D mesh3dPool

type vec3 struct{ X, Y, Z float64 }

// mat3 is a row-major 3×3 rotation (object + camera fused once per object).
type mat3 struct {
	M00, M01, M02 float64
	M10, M11, M12 float64
	M20, M21, M22 float64
}

func (p *mesh3dPool) beginFrame() {
	p.nV, p.nI = 0, 0
}

func (p *mesh3dPool) growBatch(addV, addI int) {
	needV := p.nV + addV
	needI := p.nI + addI
	if cap(p.pos) < needV {
		npos := make([]render.Point, needV, needV*2+16)
		ncol := make([]render.RGBA, needV, needV*2+16)
		copy(npos, p.pos[:p.nV])
		copy(ncol, p.col[:p.nV])
		p.pos, p.col = npos, ncol
	} else {
		p.pos = p.pos[:needV]
		p.col = p.col[:needV]
	}
	if cap(p.idx) < needI {
		nidx := make([]uint16, needI, needI*2+16)
		copy(nidx, p.idx[:p.nI])
		p.idx = nidx
	} else {
		p.idx = p.idx[:needI]
	}
}

// alloc reserves nVert/nIdx in the batch and returns writable views (no scratch copy).
func (p *mesh3dPool) alloc(nVert, nIdx int) (pos []render.Point, col []render.RGBA, idx []uint16, base uint16) {
	base = uint16(p.nV)
	p.growBatch(nVert, nIdx)
	pos = p.pos[p.nV : p.nV+nVert]
	col = p.col[p.nV : p.nV+nVert]
	idx = p.idx[p.nI : p.nI+nIdx]
	p.nV += nVert
	p.nI += nIdx
	return
}

func (p *mesh3dPool) flush(dc *render.Context) {
	if p.nV < 3 || p.nI < 3 {
		return
	}
	dc.DrawMesh(render.Mesh{
		Positions: p.pos[:p.nV],
		Colors:    p.col[:p.nV],
		Indices:   p.idx[:p.nI],
	})
}

// sinLUT: 256-entry table for hot vertex color paths (Skia-class: less libm).
var sinLUT [256]float64

func init() {
	for i := 0; i < 256; i++ {
		sinLUT[i] = math.Sin(float64(i) * (2 * math.Pi / 256))
	}
}

func fastSin(x float64) float64 {
	// x radians → [0,256)
	const inv = 256 / (2 * math.Pi)
	// math.Mod is slower; scale + floor
	s := x * inv
	// bring into positive range without Mod
	if s < 0 {
		s = -s
	}
	i := int(s) & 255
	return sinLUT[i]
}

func rotMat(yaw, pitch, roll float64) mat3 {
	// Z(roll) * Y(pitch) * X(yaw) — same order as legacy rotXYZ.
	cz, sz := math.Cos(roll), math.Sin(roll)
	cy, sy := math.Cos(pitch), math.Sin(pitch)
	cx, sx := math.Cos(yaw), math.Sin(yaw)
	// Rz
	r00, r01, r02 := cz, -sz, 0.0
	r10, r11, r12 := sz, cz, 0.0
	r20, r21, r22 := 0.0, 0.0, 1.0
	// * Ry
	a00 := r00*cy + r02*(-sy)
	a01 := r01
	a02 := r00*sy + r02*cy
	a10 := r10*cy + r12*(-sy)
	a11 := r11
	a12 := r10*sy + r12*cy
	a20 := r20*cy + r22*(-sy)
	a21 := r21
	a22 := r20*sy + r22*cy
	// * Rx
	return mat3{
		M00: a00, M01: a01*cx + a02*sx, M02: a01*(-sx) + a02*cx,
		M10: a10, M11: a11*cx + a12*sx, M12: a11*(-sx) + a12*cx,
		M20: a20, M21: a21*cx + a22*sx, M22: a21*(-sx) + a22*cx,
	}
}

func mulMat(a, b mat3) mat3 {
	return mat3{
		M00: a.M00*b.M00 + a.M01*b.M10 + a.M02*b.M20,
		M01: a.M00*b.M01 + a.M01*b.M11 + a.M02*b.M21,
		M02: a.M00*b.M02 + a.M01*b.M12 + a.M02*b.M22,
		M10: a.M10*b.M00 + a.M11*b.M10 + a.M12*b.M20,
		M11: a.M10*b.M01 + a.M11*b.M11 + a.M12*b.M21,
		M12: a.M10*b.M02 + a.M11*b.M12 + a.M12*b.M22,
		M20: a.M20*b.M00 + a.M21*b.M10 + a.M22*b.M20,
		M21: a.M20*b.M01 + a.M21*b.M11 + a.M22*b.M21,
		M22: a.M20*b.M02 + a.M21*b.M12 + a.M22*b.M22,
	}
}

func (m mat3) apply(p vec3) vec3 {
	return vec3{
		X: m.M00*p.X + m.M01*p.Y + m.M02*p.Z,
		Y: m.M10*p.X + m.M11*p.Y + m.M12*p.Z,
		Z: m.M20*p.X + m.M21*p.Y + m.M22*p.Z,
	}
}

// camMat matches legacy project3D(camPitch, camYaw): rotXYZ(p, camPitch, camYaw, 0).
func camMat(camYaw, camPitch float64) mat3 {
	return rotMat(camPitch, camYaw, 0)
}

func projectXF(p vec3, xf mat3, ox, oy, scale, persp float64) render.Point {
	q := xf.apply(p)
	w := 1.0 / (persp + q.Z)
	return render.Point{
		X: ox + q.X*scale*w,
		Y: oy - q.Y*scale*w,
	}
}

func gradColor(u, v, t, phase float64) render.RGBA {
	// LUT sin for Gouraud vertex colors (GPU interpolates).
	r := 0.5 + 0.5*fastSin(u*math.Pi*2+t*1.3+phase)
	g := 0.5 + 0.5*fastSin(v*math.Pi*2+t*1.7+phase*1.3)
	b := 0.5 + 0.5*fastSin((u+v)*math.Pi+t*0.9+phase*0.7)
	return render.RGBA{R: r, G: g, B: b, A: 0.95}
}

// mesh3DIsPrimary reports whether Mesh3D is the hero content of this process.
// S22-style (mesh3d + optional bg/text/hud) fills the whole window; composite
// scenes keep a large centered stage so other modules remain visible.
func mesh3DIsPrimary(f FeatureFlags) bool {
	if !f.Mesh3D {
		return false
	}
	return !(f.GlowOrbs || f.Cards || f.Paths || f.DashStroke || f.Clip || f.Layer ||
		f.Backdrop || f.Mask || f.Image || f.Filter || f.Transform || f.Blend ||
		f.Vertices || f.Pixels || f.Polygon || f.Gradient || f.Pattern || f.AdvBlend ||
		f.RRectClip || f.ScrollUI || f.TextLCD || f.Damage)
}

// drawMesh3DScene renders pseudo-3D objects: random-morphing gradient meshes with
// yaw/pitch/roll rotation. Uses GPU DrawMesh (colored triangles) — no CPU path.
//
// When Mesh3D is the primary scenario module, the stage fills the entire window
// (user requirement: 单独 3D 沾满窗口). In full-composite stress, a large centered
// stage is used so other modules remain partially visible.
func drawMesh3DScene(dc *render.Context, fw, fh, t float64, lite bool) {
	if dc == nil || fw < 8 || fh < 8 {
		return
	}
	full := mesh3DIsPrimary(Features)

	var px, py, pw, ph float64
	if full {
		px, py, pw, ph = 0, 0, fw, fh
		dc.SetRGBA(0.035, 0.04, 0.075, 1.0)
		dc.DrawRectangle(0, 0, fw, fh)
		_ = dc.Fill()
		dc.SetRGBA(0.25, 0.65, 1.0, 0.22)
		dc.SetLineWidth(2.0)
		inset := math.Min(fw, fh) * 0.015
		dc.DrawRectangle(inset, inset, fw-2*inset, fh-2*inset)
		_ = dc.Stroke()
	} else {
		pw, ph = fw*0.82, fh*0.78
		if lite {
			pw, ph = fw*0.70, fh*0.66
		}
		px, py = (fw-pw)*0.5, (fh-ph)*0.12
		dc.SetRGBA(0.05, 0.06, 0.10, 0.72)
		dc.DrawRoundedRectangle(px, py, pw, ph, 12)
		_ = dc.Fill()
		dc.SetRGBA(0.35, 0.75, 1.0, 0.45)
		dc.SetLineWidth(1.2)
		dc.DrawRoundedRectangle(px, py, pw, ph, 12)
		_ = dc.Stroke()
	}

	gMesh3D.beginFrame()

	cx := px + pw*0.5
	cy := py + ph*0.46
	base := math.Min(pw, ph)
	scale := base * 0.30
	if full {
		scale = base * 0.32
	}
	if lite {
		scale *= 0.88
	}

	camYaw := t * 0.35
	camPitch := 0.32 + 0.12*math.Sin(t*0.5)
	perspective := 2.55
	cam := camMat(camYaw, camPitch)

	// --- Object A: hero cube (center) ---
	draw3DCube(cx, cy-scale*0.05, scale*1.15,
		t*1.1, t*0.7, t*0.4, cam, perspective, t)

	// --- Object B: faceted ball (left) ---
	sub := 1
	if !lite && full {
		sub = 2
	}
	draw3DBall(cx-scale*1.55, cy-scale*0.08, scale*0.95,
		t*0.9, t*1.2, t*0.55, cam, perspective, t, sub)

	// --- Object C: morph poly (right) ---
	draw3DMorphStar(cx+scale*1.55, cy+scale*0.05, scale*0.90,
		t*1.3, t*0.85, t*0.95, cam, perspective, t)

	// --- Object D: wide terrain along bottom ---
	if !lite || full {
		tw := scale * 3.4
		if full {
			tw = scale * 3.8
		}
		// Terrain uses a mild extra yaw baked into object matrix.
		draw3DTerrain(cx, cy+scale*1.20, tw, scale*0.95,
			t*0.4, cam, perspective, t)
	}

	// --- Object E: secondary orbiting cube (full-window only) ---
	if full && !lite {
		orbit := scale * 1.85
		ox := cx + math.Cos(t*0.85)*orbit
		oy := cy - scale*0.55 + math.Sin(t*0.85)*orbit*0.35
		draw3DCube(ox, oy, scale*0.42,
			t*1.6, t*1.1, t*0.8, cam, perspective, t+1.7)
	}

	// One GPU mesh submit for all solid objects.
	gMesh3D.flush(dc)
	_ = dc
}

func draw3DCube(ox, oy, scale, yaw, pitch, roll float64, cam mat3, persp, t float64) {
	corners := [8]vec3{
		{-1, -1, -1}, {1, -1, -1}, {1, 1, -1}, {-1, 1, -1},
		{-1, -1, 1}, {1, -1, 1}, {1, 1, 1}, {-1, 1, 1},
	}
	type face struct {
		i0, i1, i2, i3 int
		u0, v0         float64
	}
	faces := [6]face{
		{0, 1, 2, 3, 0, 0},
		{5, 4, 7, 6, 1, 0},
		{4, 0, 3, 7, 0, 1},
		{1, 5, 6, 2, 1, 1},
		{3, 2, 6, 7, 0.5, 0},
		{4, 5, 1, 0, 0.5, 1},
	}
	const nVert = 24
	const nIdx = 36
	pos, col, idx, base := gMesh3D.alloc(nVert, nIdx)
	xf := mulMat(cam, rotMat(yaw, pitch, roll))

	// Pre-rotate corners once (shared by faces).
	var rc [8]vec3
	for i := 0; i < 8; i++ {
		rc[i] = xf.apply(corners[i])
	}
	// Project helper using already-transformed coords (identity xf).
	proj := func(p vec3) render.Point {
		w := 1.0 / (persp + p.Z)
		return render.Point{X: ox + p.X*scale*w, Y: oy - p.Y*scale*w}
	}

	vi, ii := 0, 0
	for fi := 0; fi < 6; fi++ {
		f := faces[fi]
		ids := [4]int{f.i0, f.i1, f.i2, f.i3}
		b := vi
		for k := 0; k < 4; k++ {
			pos[vi] = proj(rc[ids[k]])
			u := f.u0 + float64(k%2)*0.5 + float64(fi)*0.07
			v := f.v0 + float64(k/2)*0.5
			// Dynamic face color phase (gradient + time morph).
			col[vi] = gradColor(u, v, t, float64(fi)*0.9+0.15*fastSin(t*2.3+float64(fi)))
			vi++
		}
		idx[ii+0] = base + uint16(b+0)
		idx[ii+1] = base + uint16(b+1)
		idx[ii+2] = base + uint16(b+2)
		idx[ii+3] = base + uint16(b+0)
		idx[ii+4] = base + uint16(b+2)
		idx[ii+5] = base + uint16(b+3)
		ii += 6
	}
}

func draw3DBall(ox, oy, scale, yaw, pitch, roll float64, cam mat3, persp, t float64, subdiv int) {
	latN := 6 + subdiv*3
	lonN := 10 + subdiv*4
	nVert := (latN + 1) * (lonN + 1)
	nIdx := latN * lonN * 6
	pos, col, idx, base := gMesh3D.alloc(nVert, nIdx)
	xf := mulMat(cam, rotMat(yaw, pitch, roll))

	vi := 0
	for j := 0; j <= latN; j++ {
		v := float64(j) / float64(latN)
		phi := v * math.Pi
		sy, cy := math.Sin(phi), math.Cos(phi)
		for i := 0; i <= lonN; i++ {
			u := float64(i) / float64(lonN)
			theta := u * 2 * math.Pi
			// Animated random-ish radius morph (stable in u,v).
			wobble := 1.0 + 0.10*fastSin(u*8+t*2.1) + 0.07*fastSin(v*6-t*1.4+1.7)
			wobble += 0.04 * fastSin(u*13+v*9+t*3.1)
			p := vec3{
				X: sy * math.Cos(theta) * wobble,
				Y: cy * wobble,
				Z: sy * math.Sin(theta) * wobble,
			}
			pos[vi] = projectXF(p, xf, ox, oy, scale, persp)
			col[vi] = gradColor(u+t*0.05, v, t, 1.7)
			vi++
		}
	}
	ii := 0
	stride := lonN + 1
	for j := 0; j < latN; j++ {
		for i := 0; i < lonN; i++ {
			i0 := base + uint16(j*stride+i)
			i1 := i0 + 1
			i2 := i0 + uint16(stride)
			i3 := i2 + 1
			idx[ii+0] = i0
			idx[ii+1] = i1
			idx[ii+2] = i2
			idx[ii+3] = i1
			idx[ii+4] = i3
			idx[ii+5] = i2
			ii += 6
		}
	}
}

func draw3DMorphStar(ox, oy, scale, yaw, pitch, roll float64, cam mat3, persp, t float64) {
	const n = 10
	// apex + outer ring + mid ring
	nVert := 1 + n*2
	nIdx := n*3 + n*6
	pos, col, idx, base := gMesh3D.alloc(nVert, nIdx)
	xf := mulMat(cam, rotMat(yaw, pitch, roll))

	// Apex
	apex := vec3{0, 1.15 + 0.12*fastSin(t*2.4), 0}
	pos[0] = projectXF(apex, xf, ox, oy, scale, persp)
	col[0] = gradColor(0.5, 0, t, 2.2)

	for i := 0; i < n; i++ {
		a := float64(i)/float64(n)*2*math.Pi + t*0.55
		// Random-morph spike length (deterministic sin hash).
		spike := 0.55 + 0.45*fastSin(t*1.7+float64(i)*1.3)
		spike += 0.12 * fastSin(t*3.1+float64(i)*2.1)
		p := vec3{math.Cos(a) * spike, 0.05 * fastSin(t+float64(i)), math.Sin(a) * spike}
		pos[1+i] = projectXF(p, xf, ox, oy, scale, persp)
		col[1+i] = gradColor(float64(i)/float64(n), 0.8, t, 3.1)
	}
	for i := 0; i < n; i++ {
		a := (float64(i)+0.5)/float64(n)*2*math.Pi - t*0.25
		r := 0.35 + 0.22*fastSin(t*1.5+float64(i)*0.9)
		p := vec3{math.Cos(a) * r, 0.25 + 0.12*fastSin(t*1.8+float64(i)), math.Sin(a) * r}
		pos[1+n+i] = projectXF(p, xf, ox, oy, scale, persp)
		col[1+n+i] = gradColor(float64(i)/float64(n), 0.4, t, 0.6)
	}

	ii := 0
	for i := 0; i < n; i++ {
		i1 := base + uint16(1+i)
		i2 := base + uint16(1+(i+1)%n)
		idx[ii+0] = base
		idx[ii+1] = i1
		idx[ii+2] = i2
		ii += 3
	}
	for i := 0; i < n; i++ {
		b0 := base + uint16(1+i)
		b1 := base + uint16(1+(i+1)%n)
		m0 := base + uint16(1+n+i)
		m1 := base + uint16(1+n+(i+1)%n)
		idx[ii+0] = m0
		idx[ii+1] = b0
		idx[ii+2] = b1
		idx[ii+3] = m0
		idx[ii+4] = b1
		idx[ii+5] = m1
		ii += 6
	}
}

func draw3DTerrain(ox, oy, wScale, hScale, yaw float64, cam mat3, persp, t float64) {
	const cols, rows = 10, 6 // slightly denser morphing terrain (still one batch)
	nVert := (cols + 1) * (rows + 1)
	nIdx := cols * rows * 6
	pos, col, idx, base := gMesh3D.alloc(nVert, nIdx)
	// Terrain base tilt + animated yaw.
	obj := mulMat(rotMat(0.4, yaw, 0), rotMat(0, 0, 0.08*math.Sin(t*0.7)))
	xf := mulMat(cam, obj)
	s := (wScale + hScale) * 0.35
	xStretch := (wScale / (hScale + 1e-6)) * 0.55

	vi := 0
	for j := 0; j <= rows; j++ {
		v := float64(j) / float64(rows)
		for i := 0; i <= cols; i++ {
			u := float64(i) / float64(cols)
			h := 0.35 * math.Sin(u*5+t*1.6) * math.Cos(v*4-t*1.1)
			h += 0.14 * math.Sin((u+v)*7+t*2.2)
			h += 0.08 * math.Sin(u*11-t*1.9) * math.Cos(v*9+t)
			p := vec3{
				X: (u - 0.5) * 2.4,
				Y: h - 0.3,
				Z: (v - 0.5) * 1.6,
			}
			q := projectXF(p, xf, ox, oy, s, persp)
			// Non-uniform X stretch for wide stage.
			q.X = ox + (q.X-ox)*xStretch
			pos[vi] = q
			c := gradColor(u, v+t*0.03, t, 4.0)
			c.R = clamp01(c.R*0.6 + (h+0.35)*0.7)
			c.G = clamp01(c.G*0.7 + 0.25)
			col[vi] = c
			vi++
		}
	}
	ii := 0
	stride := cols + 1
	for j := 0; j < rows; j++ {
		for i := 0; i < cols; i++ {
			i0 := base + uint16(j*stride+i)
			i1 := i0 + 1
			i2 := i0 + uint16(stride)
			i3 := i2 + 1
			idx[ii+0] = i0
			idx[ii+1] = i1
			idx[ii+2] = i2
			idx[ii+3] = i1
			idx[ii+4] = i3
			idx[ii+5] = i2
			ii += 6
		}
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
