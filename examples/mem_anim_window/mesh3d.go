//go:build linux && !nogpu

package main

import (
	"math"

	"github.com/energye/gpui/render"
)

// mesh3dPool reuses triangle buffers so the 3D module does not churn heap each frame
// (Skia-class soak: low alloc → stable RSS). Objects append into one batch and
// flush with a single DrawMesh to cut CPU encoder overhead.
type mesh3dPool struct {
	pos []render.Point
	col []render.RGBA
	idx []uint16
	// scratch for one object before append
	spos []render.Point
	scol []render.RGBA
	sidx []uint16
	nV   int
	nI   int
}

var gMesh3D mesh3dPool

type vec3 struct{ X, Y, Z float64 }

func (p *mesh3dPool) beginFrame() {
	p.nV, p.nI = 0, 0
}

func (p *mesh3dPool) growBatch(addV, addI int) {
	needV := p.nV + addV
	needI := p.nI + addI
	if cap(p.pos) < needV {
		npos := make([]render.Point, needV, needV*2)
		ncol := make([]render.RGBA, needV, needV*2)
		copy(npos, p.pos[:p.nV])
		copy(ncol, p.col[:p.nV])
		p.pos, p.col = npos, ncol
	} else {
		p.pos = p.pos[:needV]
		p.col = p.col[:needV]
	}
	if cap(p.idx) < needI {
		nidx := make([]uint16, needI, needI*2)
		copy(nidx, p.idx[:p.nI])
		p.idx = nidx
	} else {
		p.idx = p.idx[:needI]
	}
}

func (p *mesh3dPool) ensureScratch(nVert, nIdx int) {
	if cap(p.spos) < nVert {
		p.spos = make([]render.Point, nVert)
		p.scol = make([]render.RGBA, nVert)
	} else {
		p.spos = p.spos[:nVert]
		p.scol = p.scol[:nVert]
	}
	if cap(p.sidx) < nIdx {
		p.sidx = make([]uint16, nIdx)
	} else {
		p.sidx = p.sidx[:nIdx]
	}
}

func (p *mesh3dPool) appendScratch(nVert, nIdx int) {
	base := p.nV
	p.growBatch(nVert, nIdx)
	copy(p.pos[base:base+nVert], p.spos[:nVert])
	copy(p.col[base:base+nVert], p.scol[:nVert])
	for i := 0; i < nIdx; i++ {
		p.idx[p.nI+i] = p.sidx[i] + uint16(base)
	}
	p.nV += nVert
	p.nI += nIdx
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

// mesh3DIsPrimary reports whether Mesh3D is the hero content of this process.
// S22-style (mesh3d + optional bg/text/hud) fills the whole window; composite
// scenes keep a large centered stage so other modules stay visible.
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
//
// Objects:
//  1. Spinning gradient cube (hero, center)
//  2. Icosphere-like faceted ball (left)
//  3. Morphing star / polyhedron (right)
//  4. Wavy terrain plane (bottom, wide)
//  5. Secondary orbiting cube (full quality only)
func drawMesh3DScene(dc *render.Context, fw, fh, t float64, lite bool) {
	if dc == nil || fw < 8 || fh < 8 {
		return
	}
	full := mesh3DIsPrimary(Features)

	var px, py, pw, ph float64
	if full {
		// Full-window 3D stage (S22): dark base covers the whole surface.
		px, py, pw, ph = 0, 0, fw, fh
		dc.SetRGBA(0.035, 0.04, 0.075, 1.0)
		dc.DrawRectangle(0, 0, fw, fh)
		_ = dc.Fill()
		// Soft inner frame so edges read as a stage, not a corner widget.
		dc.SetRGBA(0.25, 0.65, 1.0, 0.22)
		dc.SetLineWidth(2.0)
		inset := math.Min(fw, fh) * 0.015
		dc.DrawRectangle(inset, inset, fw-2*inset, fh-2*inset)
		_ = dc.Stroke()
	} else {
		// Composite: large centered stage (~80% viewport), not the old corner plate.
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

	// Layout from full stage rect so objects scale with window size.
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

	// Shared camera: mild perspective + slow orbit of whole group.
	camYaw := t * 0.35
	camPitch := 0.32 + 0.12*math.Sin(t*0.5)
	perspective := 2.55

	// --- Object A: hero cube (center, largest) ---
	draw3DCube(dc, cx, cy-scale*0.05, scale*1.15,
		t*1.1, t*0.7, t*0.4, camYaw, camPitch, perspective, t, !lite)

	// --- Object B: faceted ball (left) ---
	sub := 1
	if !lite {
		sub = 2
	}
	draw3DBall(dc, cx-scale*1.55, cy-scale*0.08, scale*0.95,
		t*0.9, t*1.2, t*0.55, camYaw, camPitch, perspective, t, sub)

	// --- Object C: morph poly (right) ---
	draw3DMorphStar(dc, cx+scale*1.55, cy+scale*0.05, scale*0.90,
		t*1.3, t*0.85, t*0.95, camYaw, camPitch, perspective, t)

	// --- Object D: wide terrain along bottom ---
	if !lite || full {
		tw := scale * 3.4
		if full {
			tw = scale * 3.8
		}
		draw3DTerrain(dc, cx, cy+scale*1.20, tw, scale*0.95,
			t*0.4, camYaw*0.5, camPitch*0.6, perspective, t)
	}

	// --- Object E: secondary orbiting cube (full quality / full-window only) ---
	if full && !lite {
		orbit := scale * 1.85
		ox := cx + math.Cos(t*0.85)*orbit
		oy := cy - scale*0.55 + math.Sin(t*0.85)*orbit*0.35
		draw3DCube(dc, ox, oy, scale*0.42,
			t*1.6, t*1.1, t*0.8, camYaw*0.6, camPitch, perspective, t+1.7, false)
	}

	// One GPU mesh submit for all solid objects.
	gMesh3D.flush(dc)
}

func rotXYZ(p vec3, yaw, pitch, roll float64) vec3 {
	// roll Z
	cz, sz := math.Cos(roll), math.Sin(roll)
	x1 := p.X*cz - p.Y*sz
	y1 := p.X*sz + p.Y*cz
	z1 := p.Z
	// pitch Y
	cy, sy := math.Cos(pitch), math.Sin(pitch)
	x2 := x1*cy + z1*sy
	y2 := y1
	z2 := -x1*sy + z1*cy
	// yaw X
	cx, sx := math.Cos(yaw), math.Sin(yaw)
	return vec3{
		X: x2,
		Y: y2*cx - z2*sx,
		Z: y2*sx + z2*cx,
	}
}

func project3D(p vec3, ox, oy, scale, camYaw, camPitch, perspective float64) render.Point {
	// Camera orbit around Y then X.
	q := rotXYZ(p, camPitch, camYaw, 0)
	// Perspective divide: z forward.
	w := 1.0 / (perspective + q.Z)
	return render.Point{
		X: ox + q.X*scale*w,
		Y: oy - q.Y*scale*w,
	}
}

func gradColor(u, v, t, phase float64) render.RGBA {
	// Animated multi-stop-like gradient via sin mix (vertex Gouraud on GPU).
	r := 0.5 + 0.5*math.Sin(u*math.Pi*2+t*1.3+phase)
	g := 0.5 + 0.5*math.Sin(v*math.Pi*2+t*1.7+phase*1.3)
	b := 0.5 + 0.5*math.Sin((u+v)*math.Pi+t*0.9+phase*0.7)
	// Boost saturation-ish.
	return render.RGBA{R: r, G: g, B: b, A: 0.95}
}

func commitObject(nVert, nIdx int) {
	gMesh3D.appendScratch(nVert, nIdx)
}

func draw3DCube(dc *render.Context, ox, oy, scale, yaw, pitch, roll, camYaw, camPitch, persp, t float64, wire bool) {
	// 8 corners of unit cube [-1,1]^3
	corners := [8]vec3{
		{-1, -1, -1}, {1, -1, -1}, {1, 1, -1}, {-1, 1, -1},
		{-1, -1, 1}, {1, -1, 1}, {1, 1, 1}, {-1, 1, 1},
	}
	// 12 triangles (2 per face) — 24 verts with unique face colors for gradients
	// Use 24 verts (4 per face) for nicer face gradients.
	type face struct {
		i0, i1, i2, i3 int
		u0, v0         float64
	}
	faces := []face{
		{0, 1, 2, 3, 0, 0},   // -Z
		{5, 4, 7, 6, 1, 0},   // +Z
		{4, 0, 3, 7, 0, 1},   // -X
		{1, 5, 6, 2, 1, 1},   // +X
		{3, 2, 6, 7, 0.5, 0}, // +Y
		{4, 5, 1, 0, 0.5, 1}, // -Y
	}
	const nVert = 24
	const nIdx = 36
	gMesh3D.ensureScratch(nVert, nIdx)
	vi, ii := 0, 0
	for fi, f := range faces {
		ids := [4]int{f.i0, f.i1, f.i2, f.i3}
		base := vi
		for k := 0; k < 4; k++ {
			p := corners[ids[k]]
			// slight face puff so edges read better
			p.X *= 1.0
			rp := rotXYZ(p, yaw, pitch, roll)
			gMesh3D.spos[vi] = project3D(rp, ox, oy, scale, camYaw, camPitch, persp)
			u := f.u0 + float64(k%2)*0.5 + float64(fi)*0.07
			v := f.v0 + float64(k/2)*0.5
			gMesh3D.scol[vi] = gradColor(u, v, t, float64(fi)*0.9)
			vi++
		}
		// two tris
		gMesh3D.sidx[ii+0] = uint16(base + 0)
		gMesh3D.sidx[ii+1] = uint16(base + 1)
		gMesh3D.sidx[ii+2] = uint16(base + 2)
		gMesh3D.sidx[ii+3] = uint16(base + 0)
		gMesh3D.sidx[ii+4] = uint16(base + 2)
		gMesh3D.sidx[ii+5] = uint16(base + 3)
		ii += 6
	}
	commitObject(nVert, nIdx)

	if !wire {
		return
	}
	// Edge wire for 3D readability (optional; skipped in lite/composite pressure).
	dc.SetRGBA(1, 1, 1, 0.35)
	dc.SetLineWidth(1.0)
	var wp [8]render.Point
	for i := 0; i < 8; i++ {
		rp := rotXYZ(corners[i], yaw, pitch, roll)
		wp[i] = project3D(rp, ox, oy, scale, camYaw, camPitch, persp)
	}
	edges := [12][2]int{{0, 1}, {1, 2}, {2, 3}, {3, 0}, {4, 5}, {5, 6}, {6, 7}, {7, 4}, {0, 4}, {1, 5}, {2, 6}, {3, 7}}
	for _, e := range edges {
		dc.MoveTo(wp[e[0]].X, wp[e[0]].Y)
		dc.LineTo(wp[e[1]].X, wp[e[1]].Y)
	}
	_ = dc.Stroke()
}

func draw3DBall(dc *render.Context, ox, oy, scale, yaw, pitch, roll, camYaw, camPitch, persp, t float64, subdiv int) {
	// Latitude/longitude sphere mesh.
	latN := 6 + subdiv*3
	lonN := 10 + subdiv*4
	nVert := (latN + 1) * (lonN + 1)
	nIdx := latN * lonN * 6
	gMesh3D.ensureScratch(nVert, nIdx)
	vi := 0
	for j := 0; j <= latN; j++ {
		v := float64(j) / float64(latN)
		phi := v * math.Pi // 0..pi
		sy, cy := math.Sin(phi), math.Cos(phi)
		for i := 0; i <= lonN; i++ {
			u := float64(i) / float64(lonN)
			theta := u * 2 * math.Pi
			// Random-ish radius wobble (stable in u,v, animated).
			wobble := 1.0 + 0.08*math.Sin(u*8+t*2.1) + 0.06*math.Cos(v*6-t*1.4)
			p := vec3{
				X: sy * math.Cos(theta) * wobble,
				Y: cy * wobble,
				Z: sy * math.Sin(theta) * wobble,
			}
			rp := rotXYZ(p, yaw, pitch, roll)
			gMesh3D.spos[vi] = project3D(rp, ox, oy, scale, camYaw, camPitch, persp)
			gMesh3D.scol[vi] = gradColor(u+t*0.05, v, t, 1.7)
			vi++
		}
	}
	ii := 0
	stride := lonN + 1
	for j := 0; j < latN; j++ {
		for i := 0; i < lonN; i++ {
			i0 := uint16(j*stride + i)
			i1 := i0 + 1
			i2 := i0 + uint16(stride)
			i3 := i2 + 1
			gMesh3D.sidx[ii+0] = i0
			gMesh3D.sidx[ii+1] = i1
			gMesh3D.sidx[ii+2] = i2
			gMesh3D.sidx[ii+3] = i1
			gMesh3D.sidx[ii+4] = i3
			gMesh3D.sidx[ii+5] = i2
			ii += 6
		}
	}
	commitObject(nVert, nIdx)
}

func draw3DMorphStar(dc *render.Context, ox, oy, scale, yaw, pitch, roll, camYaw, camPitch, persp, t float64) {
	// Star pyramid: tip + N base verts + mid ring; morph radius over time.
	const n = 7
	nVert := 1 + n + n
	nIdx := n*3 + n*6 // tip fan + mid band
	gMesh3D.ensureScratch(nVert, nIdx)

	tip := vec3{0, 1.2, 0}
	rp := rotXYZ(tip, yaw, pitch, roll)
	gMesh3D.spos[0] = project3D(rp, ox, oy, scale, camYaw, camPitch, persp)
	gMesh3D.scol[0] = gradColor(0.5, 0, t, 2.2)

	for i := 0; i < n; i++ {
		a := float64(i)/float64(n)*2*math.Pi + t*0.4
		r := 0.55 + 0.45*math.Sin(a*3+t*1.8) + 0.15*math.Sin(t*2.7+float64(i))
		if i%2 == 0 {
			r *= 1.35
		}
		p := vec3{math.Cos(a) * r, -0.55, math.Sin(a) * r}
		rp = rotXYZ(p, yaw, pitch, roll)
		gMesh3D.spos[1+i] = project3D(rp, ox, oy, scale, camYaw, camPitch, persp)
		gMesh3D.scol[1+i] = gradColor(float64(i)/float64(n), 0.8, t, 3.1)
	}
	for i := 0; i < n; i++ {
		a := (float64(i)+0.5)/float64(n)*2*math.Pi - t*0.25
		r := 0.4 + 0.2*math.Cos(t*1.5+float64(i)*0.9)
		p := vec3{math.Cos(a) * r, 0.25 + 0.1*math.Sin(t+float64(i)), math.Sin(a) * r}
		rp = rotXYZ(p, yaw, pitch, roll)
		gMesh3D.spos[1+n+i] = project3D(rp, ox, oy, scale, camYaw, camPitch, persp)
		gMesh3D.scol[1+n+i] = gradColor(float64(i)/float64(n), 0.4, t, 0.6)
	}

	ii := 0
	for i := 0; i < n; i++ {
		i1 := uint16(1 + i)
		i2 := uint16(1 + (i+1)%n)
		gMesh3D.sidx[ii+0] = 0
		gMesh3D.sidx[ii+1] = i1
		gMesh3D.sidx[ii+2] = i2
		ii += 3
	}
	for i := 0; i < n; i++ {
		b0 := uint16(1 + i)
		b1 := uint16(1 + (i+1)%n)
		m0 := uint16(1 + n + i)
		m1 := uint16(1 + n + (i+1)%n)
		gMesh3D.sidx[ii+0] = m0
		gMesh3D.sidx[ii+1] = b0
		gMesh3D.sidx[ii+2] = b1
		gMesh3D.sidx[ii+3] = m0
		gMesh3D.sidx[ii+4] = b1
		gMesh3D.sidx[ii+5] = m1
		ii += 6
	}
	commitObject(nVert, nIdx)
}

func draw3DTerrain(dc *render.Context, ox, oy, wScale, hScale, yaw, camYaw, camPitch, persp, t float64) {
	const cols, rows = 8, 5
	nVert := (cols + 1) * (rows + 1)
	nIdx := cols * rows * 6
	gMesh3D.ensureScratch(nVert, nIdx)
	vi := 0
	for j := 0; j <= rows; j++ {
		v := float64(j) / float64(rows)
		for i := 0; i <= cols; i++ {
			u := float64(i) / float64(cols)
			// height field
			h := 0.35 * math.Sin(u*5+t*1.6) * math.Cos(v*4-t*1.1)
			h += 0.12 * math.Sin((u+v)*7+t*2.2)
			p := vec3{
				X: (u - 0.5) * 2.4,
				Y: h - 0.3,
				Z: (v - 0.5) * 1.6,
			}
			rp := rotXYZ(p, 0.4, yaw, 0)
			// non-uniform scale for plate
			q := project3D(rp, ox, oy, (wScale+hScale)*0.35, camYaw, camPitch, persp)
			// stretch x a bit
			q.X = ox + (q.X-ox)*(wScale/(hScale+1e-6))*0.55
			gMesh3D.spos[vi] = q
			gMesh3D.scol[vi] = gradColor(u, v+t*0.03, t, 4.0)
			// height-tint
			gMesh3D.scol[vi].R = clamp01(gMesh3D.scol[vi].R*0.6 + (h+0.35)*0.7)
			gMesh3D.scol[vi].G = clamp01(gMesh3D.scol[vi].G*0.7 + 0.25)
			vi++
		}
	}
	ii := 0
	stride := cols + 1
	for j := 0; j < rows; j++ {
		for i := 0; i < cols; i++ {
			i0 := uint16(j*stride + i)
			i1 := i0 + 1
			i2 := i0 + uint16(stride)
			i3 := i2 + 1
			gMesh3D.sidx[ii+0] = i0
			gMesh3D.sidx[ii+1] = i1
			gMesh3D.sidx[ii+2] = i2
			gMesh3D.sidx[ii+3] = i1
			gMesh3D.sidx[ii+4] = i3
			gMesh3D.sidx[ii+5] = i2
			ii += 6
		}
	}
	commitObject(nVert, nIdx)
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
