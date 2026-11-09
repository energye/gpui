//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"errors"
	"math"
)

// MeshGradientColorModel selects how colors are interpolated across a
// MeshGradient grid.
type MeshGradientColorModel uint8

const (
	// MeshGradientBilinear bilerps each cell's four corner colors (the P3.9
	// MeshPatch rule).
	MeshGradientBilinear MeshGradientColorModel = iota
	// MeshGradientCatmullRom interpolates the whole vertex-color grid with
	// Catmull-Rom (C1; Compose's hasBicubicColor behavior).
	MeshGradientCatmullRom
)

// MeshGradientSpace selects the coordinate space of MeshGradientVertex.Pos
// and the tangent offsets.
type MeshGradientSpace uint8

const (
	// MeshGradientAbsolute means positions/tangents are user-space points.
	MeshGradientAbsolute MeshGradientSpace = iota
	// MeshGradientNormalized means positions are in [0,1] and map to Bounds;
	// tangent offsets are normalized too and scale with Bounds.
	MeshGradientNormalized
)

// MeshGradientVertex is one vertex of a rows x columns patch grid.
//
// Left/Top/Right/Bottom are Bezier control-point offsets relative to Pos
// (Compose MeshGradientScope semantics). nil means "infer from neighboring
// vertices"; a non-nil zero offset is an explicit zero/sharp tangent.
type MeshGradientVertex struct {
	Pos    Point
	Color  RGBA
	Left   *Point
	Top    *Point
	Right  *Point
	Bottom *Point
}

// MeshGradient is a rows x columns grid of Coons patches. Vertices are
// row-major: (rows+1)*(columns+1) entries. A 1x1 grid has four vertices and
// one patch, matching Compose's MeshGradient dimensions.
type MeshGradient struct {
	Rows, Cols int
	Vertices   []MeshGradientVertex
	ColorModel MeshGradientColorModel
	Space      MeshGradientSpace
	Bounds     Rect // used when Space == MeshGradientNormalized
}

// ErrMeshGradientShape reports an invalid grid shape, color model, space or
// normalized bounds.
var ErrMeshGradientShape = errors.New("render: mesh gradient shape is invalid")

// NewMeshGradient validates and copies the grid. ColorModel defaults to
// MeshGradientBilinear and Space to MeshGradientAbsolute; set the fields after
// construction when another model is wanted.
func NewMeshGradient(rows, cols int, vertices []MeshGradientVertex) (*MeshGradient, error) {
	vs := make([]MeshGradientVertex, len(vertices))
	for i, v := range vertices {
		vs[i] = cloneMeshGradientVertex(v)
	}
	g := &MeshGradient{
		Rows: rows, Cols: cols, Vertices: vs,
		ColorModel: MeshGradientBilinear, Space: MeshGradientAbsolute,
	}
	if err := validateMeshGradient(g); err != nil {
		return nil, err
	}
	return g, nil
}

func cloneMeshGradientVertex(v MeshGradientVertex) MeshGradientVertex {
	v.Left = clonePointPtr(v.Left)
	v.Top = clonePointPtr(v.Top)
	v.Right = clonePointPtr(v.Right)
	v.Bottom = clonePointPtr(v.Bottom)
	return v
}

func clonePointPtr(p *Point) *Point {
	if p == nil {
		return nil
	}
	q := *p
	return &q
}

// CloneMeshGradient returns a deep copy (vertices and tangent pointers).
func (g *MeshGradient) Clone() *MeshGradient {
	if g == nil {
		return nil
	}
	out := *g
	out.Vertices = make([]MeshGradientVertex, len(g.Vertices))
	for i, v := range g.Vertices {
		out.Vertices[i] = cloneMeshGradientVertex(v)
	}
	return &out
}

func validateMeshGradient(g *MeshGradient) error {
	if g == nil || g.Rows < 1 || g.Cols < 1 {
		return ErrMeshGradientShape
	}
	want := (g.Rows + 1) * (g.Cols + 1)
	if len(g.Vertices) != want {
		return ErrMeshGradientShape
	}
	if g.ColorModel != MeshGradientBilinear && g.ColorModel != MeshGradientCatmullRom {
		return ErrMeshGradientShape
	}
	if g.Space != MeshGradientAbsolute && g.Space != MeshGradientNormalized {
		return ErrMeshGradientShape
	}
	if g.Space == MeshGradientNormalized {
		if !isFinitePt(g.Bounds.Min) || !isFinitePt(g.Bounds.Max) ||
			g.Bounds.Max.X <= g.Bounds.Min.X || g.Bounds.Max.Y <= g.Bounds.Min.Y {
			return ErrMeshGradientShape
		}
	}
	for _, v := range g.Vertices {
		if !isFinitePt(v.Pos) || !isFiniteRGBA(v.Color) {
			return ErrVertsNonFinite
		}
		for _, p := range []*Point{v.Left, v.Top, v.Right, v.Bottom} {
			if p != nil && !isFinitePt(*p) {
				return ErrVertsNonFinite
			}
		}
	}
	return nil
}

// meshGradientResolvedVertex is one vertex after normalized mapping and
// tangent inference. All fields are user space.
type meshGradientResolvedVertex struct {
	pos    Point
	color  RGBA
	left   Point
	top    Point
	right  Point
	bottom Point
}

// resolveMeshGradient maps normalized positions/tangents to user space and
// infers missing tangents with the Compose rule:
//
//	horizontal = normalize(rightNeighbor - leftNeighbor) * 0.33
//	left  = -horizontal * distance(current, leftNeighbor)
//	right =  horizontal * distance(current, rightNeighbor)
//	vertical = normalize(bottomNeighbor - topNeighbor) * 0.33
//	top    = -vertical * distance(current, topNeighbor)
//	bottom =  vertical * distance(current, bottomNeighbor)
//
// Boundary neighbors are the vertex itself, so their distance (and offset) is
// zero. Explicit tangents are used as-is.
func resolveMeshGradient(g *MeshGradient) ([]meshGradientResolvedVertex, bool) {
	if g == nil || len(g.Vertices) == 0 {
		return nil, false
	}
	colsPlusOne := g.Cols + 1
	raw := make([]Point, len(g.Vertices))
	for i, v := range g.Vertices {
		raw[i] = v.Pos
	}
	out := make([]meshGradientResolvedVertex, len(g.Vertices))

	scaleX, scaleY := 1.0, 1.0
	if g.Space == MeshGradientNormalized {
		scaleX = g.Bounds.Max.X - g.Bounds.Min.X
		scaleY = g.Bounds.Max.Y - g.Bounds.Min.Y
	}
	mapPos := func(p Point) Point {
		if g.Space != MeshGradientNormalized {
			return p
		}
		return Pt(g.Bounds.Min.X+p.X*scaleX, g.Bounds.Min.Y+p.Y*scaleY)
	}
	mapVec := func(p Point) Point {
		if g.Space != MeshGradientNormalized {
			return p
		}
		return Pt(p.X*scaleX, p.Y*scaleY)
	}

	for r := 0; r <= g.Rows; r++ {
		for c := 0; c <= g.Cols; c++ {
			i := r*colsPlusOne + c
			cur := raw[i]
			li, ri, ti, bi := i, i, i, i
			if c > 0 {
				li = i - 1
			}
			if c < g.Cols {
				ri = i + 1
			}
			if r > 0 {
				ti = i - colsPlusOne
			}
			if r < g.Rows {
				bi = i + colsPlusOne
			}
			leftN, rightN, topN, bottomN := raw[li], raw[ri], raw[ti], raw[bi]
			horizontal := normalizeVector(rightN.Sub(leftN)).Mul(0.33)
			vertical := normalizeVector(bottomN.Sub(topN)).Mul(0.33)

			left := horizontal.Mul(-1).Mul(distPoint(cur, leftN))
			right := horizontal.Mul(distPoint(cur, rightN))
			top := vertical.Mul(-1).Mul(distPoint(cur, topN))
			bottom := vertical.Mul(distPoint(cur, bottomN))
			if v := g.Vertices[i].Left; v != nil {
				left = *v
			}
			if v := g.Vertices[i].Right; v != nil {
				right = *v
			}
			if v := g.Vertices[i].Top; v != nil {
				top = *v
			}
			if v := g.Vertices[i].Bottom; v != nil {
				bottom = *v
			}
			out[i] = meshGradientResolvedVertex{
				pos:    mapPos(cur),
				color:  g.Vertices[i].Color,
				left:   mapVec(left),
				top:    mapVec(top),
				right:  mapVec(right),
				bottom: mapVec(bottom),
			}
			if !isFinitePt(out[i].pos) || !isFinitePt(out[i].left) || !isFinitePt(out[i].top) ||
				!isFinitePt(out[i].right) || !isFinitePt(out[i].bottom) {
				return nil, false
			}
		}
	}
	return out, true
}

// Validate reports whether the grid can be drawn (shape, model, space and
// finite vertices/tangents).
func (g *MeshGradient) Validate() error { return validateMeshGradient(g) }

// ControlBounds returns the user-space AABB of the grid's vertices and their
// resolved tangent endpoints. It returns the zero Rect for an invalid grid.
func (g *MeshGradient) ControlBounds() Rect {
	resolved, ok := resolveMeshGradient(g)
	if !ok || len(resolved) == 0 {
		return Rect{}
	}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	add := func(p Point) {
		minX = math.Min(minX, p.X)
		minY = math.Min(minY, p.Y)
		maxX = math.Max(maxX, p.X)
		maxY = math.Max(maxY, p.Y)
	}
	for _, v := range resolved {
		add(v.pos)
		add(v.pos.Add(v.left))
		add(v.pos.Add(v.right))
		add(v.pos.Add(v.top))
		add(v.pos.Add(v.bottom))
	}
	if math.IsInf(minX, 1) {
		return Rect{}
	}
	return Rect{Min: Pt(minX, minY), Max: Pt(maxX, maxY)}
}

func distPoint(a, b Point) float64 { return a.Sub(b).Length() }

func normalizeVector(p Point) Point {
	l := p.Length()
	if l <= 1e-12 {
		return Point{}
	}
	return p.Mul(1 / l)
}

// meshGradientCellPatch builds the Coons patch for cell (r,c). Points follow
// the same Skia order as MeshPatchCoons.
func meshGradientCellPatch(g *MeshGradient, resolved []meshGradientResolvedVertex, r, c int) (MeshPatch, bool) {
	if g == nil || r < 0 || r >= g.Rows || c < 0 || c >= g.Cols {
		return MeshPatch{}, false
	}
	stride := g.Cols + 1
	itl := r*stride + c
	itr := itl + 1
	ibl := itl + stride
	ibr := ibl + 1
	if ibr >= len(resolved) {
		return MeshPatch{}, false
	}
	tl, tr, br, bl := resolved[itl], resolved[itr], resolved[ibr], resolved[ibl]
	pts := []Point{
		tl.pos,
		tl.pos.Add(tl.right), tr.pos.Add(tr.left), tr.pos,
		tr.pos.Add(tr.bottom), br.pos.Add(br.top), br.pos,
		br.pos.Add(br.left), bl.pos.Add(bl.right), bl.pos,
		bl.pos.Add(bl.top), tl.pos.Add(tl.bottom),
	}
	return MeshPatch{
		Style:  MeshPatchCoons,
		Points: pts,
		Colors: [4]RGBA{tl.color, tr.color, br.color, bl.color},
	}, true
}

// catmullWeights are the standard Catmull-Rom (B=0, C=0.5) weights for t in
// [0,1].
func catmullWeights(t float64) [4]float64 {
	t2 := t * t
	t3 := t2 * t
	return [4]float64{
		-0.5*t3 + t2 - 0.5*t,
		1.5*t3 - 2.5*t2 + 1,
		-1.5*t3 + 2*t2 + 0.5*t,
		0.5*t3 - 0.5*t2,
	}
}

// meshGradientCatmullRomColor evaluates the grid-level Catmull-Rom color at
// cell (r,c) local (u,v). The 4x4 neighborhood is clamped at the grid edges;
// colors are interpolated in premultiplied space and returned straight.
func meshGradientCatmullRomColor(g *MeshGradient, resolved []meshGradientResolvedVertex, r, c int, u, v float64) RGBA {
	if g == nil || len(resolved) == 0 {
		return RGBA{}
	}
	u = clamp01(u)
	v = clamp01(v)
	wu := catmullWeights(u)
	wv := catmullWeights(v)
	stride := g.Cols + 1

	pm := func(c RGBA) [4]float64 {
		a := clamp01(c.A)
		return [4]float64{clamp01(c.R) * a, clamp01(c.G) * a, clamp01(c.B) * a, a}
	}
	var rows [4][4]float64
	for i := 0; i < 4; i++ {
		rr := r - 1 + i
		if rr < 0 {
			rr = 0
		} else if rr > g.Rows {
			rr = g.Rows
		}
		var acc [4]float64
		for j := 0; j < 4; j++ {
			cc := c - 1 + j
			if cc < 0 {
				cc = 0
			} else if cc > g.Cols {
				cc = g.Cols
			}
			col := pm(resolved[rr*stride+cc].color)
			w := wu[j]
			for k := 0; k < 4; k++ {
				acc[k] += col[k] * w
			}
		}
		rows[i] = acc
	}
	var out [4]float64
	for i := 0; i < 4; i++ {
		for k := 0; k < 4; k++ {
			out[k] += rows[i][k] * wv[i]
		}
	}
	if out[3] <= 0 {
		return RGBA{}
	}
	return RGBA{
		R: clamp01(out[0] / out[3]),
		G: clamp01(out[1] / out[3]),
		B: clamp01(out[2] / out[3]),
		A: clamp01(out[3]),
	}
}

// meshGradientCellMeshes tessellates every cell with its own LOD and returns
// the per-cell colored meshes (positions in user space). The caller
// concatenates them into one Mesh (or chunks when >65536 vertices).
func meshGradientCellMeshes(g *MeshGradient, m Matrix) ([]Mesh, bool) {
	resolved, ok := resolveMeshGradient(g)
	if !ok {
		return nil, false
	}
	out := make([]Mesh, 0, g.Rows*g.Cols)
	for r := 0; r < g.Rows; r++ {
		for c := 0; c < g.Cols; c++ {
			patch, ok := meshGradientCellPatch(g, resolved, r, c)
			if !ok {
				return nil, false
			}
			lodX, lodY, ok := meshPatchLOD(patch, m)
			if !ok {
				return nil, false
			}
			var colorAt func(u, v float64) RGBA
			if g.ColorModel == MeshGradientCatmullRom {
				rr, cc := r, c
				colorAt = func(u, v float64) RGBA {
					return meshGradientCatmullRomColor(g, resolved, rr, cc, u, v)
				}
			} else {
				colorAt = func(u, v float64) RGBA {
					return meshPatchColorAt(patch, u, v)
				}
			}
			mesh, ok := tessellateMeshPatchColors(patch, lodX, lodY, colorAt)
			if !ok {
				return nil, false
			}
			out = append(out, mesh)
		}
	}
	return out, true
}

// tessellateMeshGradient concatenates all cell meshes into one Mesh. It
// returns false when the combined vertex count exceeds uint16 indices; the
// draw path chunks instead.
func tessellateMeshGradient(g *MeshGradient, m Matrix) (Mesh, bool) {
	if err := validateMeshGradient(g); err != nil {
		return Mesh{}, false
	}
	cells, ok := meshGradientCellMeshes(g, m)
	if !ok {
		return Mesh{}, false
	}
	total := 0
	for _, c := range cells {
		total += len(c.Positions)
	}
	if total > 65536 {
		return Mesh{}, false
	}
	pos := make([]Point, 0, total)
	colors := make([]RGBA, 0, total)
	idx := make([]uint16, 0, total*2)
	for _, c := range cells {
		base := uint16(len(pos))
		pos = append(pos, c.Positions...)
		colors = append(colors, c.Colors...)
		for _, i := range c.Indices {
			idx = append(idx, base+i)
		}
	}
	return Mesh{Positions: pos, Colors: colors, Indices: idx}, true
}

// DrawMeshGradient draws a high-level grid. Invalid or non-finite grids are
// skipped; use DrawMeshGradientEx to get the reason.
func (c *Context) DrawMeshGradient(g *MeshGradient) {
	_, _ = c.DrawMeshGradientEx(g, VertDrawOptions{})
}

// DrawMeshGradientEx validates then draws a high-level grid as one indexed
// Mesh (chunked on the uint16 limit). The result follows DrawMeshEx.
func (c *Context) DrawMeshGradientEx(g *MeshGradient, _ VertDrawOptions) (VertDrawResult, error) {
	if c == nil {
		return VertDrawResult{Skipped: true}, nil
	}
	if err := validateMeshGradient(g); err != nil {
		return VertDrawResult{}, err
	}
	cells, ok := meshGradientCellMeshes(g, c.totalMatrix())
	if !ok || len(cells) == 0 {
		return VertDrawResult{Skipped: true}, nil
	}
	useGPU := !CPUOnlyMode() && c.gpuCtxOps() != nil
	totalTris := 0
	var pos []Point
	var colors []RGBA
	var idx []uint16
	flush := func() {
		if len(pos) < 3 || len(idx) < 3 {
			pos, colors, idx = nil, nil, nil
			return
		}
		c.DrawMesh(Mesh{Positions: pos, Colors: colors, Indices: idx})
		totalTris += len(idx) / 3
		pos, colors, idx = nil, nil, nil
	}
	for _, cell := range cells {
		if len(pos)+len(cell.Positions) > 65536 {
			flush()
		}
		base := uint16(len(pos))
		pos = append(pos, cell.Positions...)
		colors = append(colors, cell.Colors...)
		for _, i := range cell.Indices {
			idx = append(idx, base+i)
		}
	}
	flush()
	if totalTris == 0 {
		return VertDrawResult{Skipped: true}, nil
	}
	return VertDrawResult{
		Degraded:  !useGPU,
		Reason:    mapDegradedReason(!useGPU),
		Triangles: totalTris,
	}, nil
}
