package render

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// P3.9a: high-level mesh gradient (grid + tangents + color model).
// Fixtures live in testdata/mesh_gradient_cases.json; this file only decodes
// them and asserts the public/internal grid contract.

type mgVertexJSON struct {
	Pos    [2]float64  `json:"pos"`
	Color  [4]int      `json:"color"`
	Left   *[2]float64 `json:"left"`
	Top    *[2]float64 `json:"top"`
	Right  *[2]float64 `json:"right"`
	Bottom *[2]float64 `json:"bottom"`
}

type mgTangentJSON struct {
	Left   [2]float64 `json:"left"`
	Top    [2]float64 `json:"top"`
	Right  [2]float64 `json:"right"`
	Bottom [2]float64 `json:"bottom"`
}

type mgSampleJSON struct {
	R     int     `json:"r"`
	C     int     `json:"c"`
	U     float64 `json:"u"`
	V     float64 `json:"v"`
	Color [4]int  `json:"color"`
}

type mgGridCase struct {
	Rows             int             `json:"rows"`
	Cols             int             `json:"cols"`
	Vertices         []mgVertexJSON  `json:"vertices"`
	ExpectedTangents []mgTangentJSON `json:"expected_tangents"`
	Samples          []mgSampleJSON  `json:"samples"`
}

type mgNormCase struct {
	Rows              int             `json:"rows"`
	Cols              int             `json:"cols"`
	Bounds            [4]float64      `json:"bounds"`
	Vertices          []mgVertexJSON  `json:"vertices"`
	ExpectedPositions [][2]float64    `json:"expected_positions"`
	ExpectedTangents  []mgTangentJSON `json:"expected_tangents"`
}

type mgRawGrid struct {
	Positions [][2]float64 `json:"positions"`
	Colors    [][4]int     `json:"colors"`
}

type mgCases struct {
	TangentGrid mgGridCase `json:"tangent_grid"`
	CatmullGrid mgGridCase `json:"catmull_grid"`
	Normalized  mgNormCase `json:"normalized_grid"`
	Grid5x5     mgRawGrid  `json:"grid_5x5"`
	Sizes       []struct {
		Rows int `json:"rows"`
		Cols int `json:"cols"`
	} `json:"sizes"`
}

func loadMeshGradientCases(t *testing.T) mgCases {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "mesh_gradient_cases.json"))
	if err != nil {
		t.Fatalf("read testdata/mesh_gradient_cases.json: %v", err)
	}
	var c mgCases
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode mesh_gradient_cases.json: %v", err)
	}
	return c
}

func mgColor(c [4]int) RGBA {
	return RGBA{R: float64(c[0]) / 255, G: float64(c[1]) / 255, B: float64(c[2]) / 255, A: float64(c[3]) / 255}
}

func mgVertex(v mgVertexJSON) MeshGradientVertex {
	out := MeshGradientVertex{Pos: Pt(v.Pos[0], v.Pos[1]), Color: mgColor(v.Color)}
	if v.Left != nil {
		p := Pt(v.Left[0], v.Left[1])
		out.Left = &p
	}
	if v.Top != nil {
		p := Pt(v.Top[0], v.Top[1])
		out.Top = &p
	}
	if v.Right != nil {
		p := Pt(v.Right[0], v.Right[1])
		out.Right = &p
	}
	if v.Bottom != nil {
		p := Pt(v.Bottom[0], v.Bottom[1])
		out.Bottom = &p
	}
	return out
}

func mgBuild(t *testing.T, gc mgGridCase) *MeshGradient {
	t.Helper()
	vs := make([]MeshGradientVertex, len(gc.Vertices))
	for i, v := range gc.Vertices {
		vs[i] = mgVertex(v)
	}
	g, err := NewMeshGradient(gc.Rows, gc.Cols, vs)
	if err != nil {
		t.Fatalf("NewMeshGradient: %v", err)
	}
	return g
}

func mgPointClose(got Point, want [2]float64) bool {
	return math.Abs(got.X-want[0]) <= 1e-9 && math.Abs(got.Y-want[1]) <= 1e-9
}

func mgColorClose(got RGBA, want [4]int, tol int) bool {
	ch := [4]float64{got.R, got.G, got.B, got.A}
	for i, v := range ch {
		n := int(math.Round(v * 255))
		d := n - want[i]
		if d < 0 {
			d = -d
		}
		if d > tol {
			return false
		}
	}
	return true
}

func TestMeshGradientTangentInference(t *testing.T) {
	c := loadMeshGradientCases(t)
	g := mgBuild(t, c.TangentGrid)
	resolved, ok := resolveMeshGradient(g)
	if !ok {
		t.Fatal("resolveMeshGradient failed")
	}
	if len(resolved) != len(c.TangentGrid.ExpectedTangents) {
		t.Fatalf("resolved %d vertices, want %d", len(resolved), len(c.TangentGrid.ExpectedTangents))
	}
	for i, want := range c.TangentGrid.ExpectedTangents {
		if !mgPointClose(resolved[i].left, want.Left) {
			t.Errorf("vertex %d left = %v want %v", i, resolved[i].left, want.Left)
		}
		if !mgPointClose(resolved[i].right, want.Right) {
			t.Errorf("vertex %d right = %v want %v", i, resolved[i].right, want.Right)
		}
		if !mgPointClose(resolved[i].top, want.Top) {
			t.Errorf("vertex %d top = %v want %v", i, resolved[i].top, want.Top)
		}
		if !mgPointClose(resolved[i].bottom, want.Bottom) {
			t.Errorf("vertex %d bottom = %v want %v", i, resolved[i].bottom, want.Bottom)
		}
	}
}

func TestMeshGradientC1SharedEdges(t *testing.T) {
	c := loadMeshGradientCases(t)
	g := mgBuild(t, c.TangentGrid)
	resolved, ok := resolveMeshGradient(g)
	if !ok {
		t.Fatal("resolve failed")
	}
	for r := 0; r < g.Rows; r++ {
		for col := 0; col < g.Cols; col++ {
			p, ok := meshGradientCellPatch(g, resolved, r, col)
			if !ok {
				t.Fatalf("cell %d,%d patch failed", r, col)
			}
			if col+1 < g.Cols {
				q, ok := meshGradientCellPatch(g, resolved, r, col+1)
				if !ok {
					t.Fatal("right patch failed")
				}
				// right edge of p = [4],[5]; left edge of q = [11],[10].
				if !mgPointClose(p.Points[4], [2]float64{q.Points[11].X, q.Points[11].Y}) ||
					!mgPointClose(p.Points[5], [2]float64{q.Points[10].X, q.Points[10].Y}) {
					t.Errorf("vertical shared edge mismatch at cell %d,%d: p4=%v q11=%v p5=%v q10=%v",
						r, col, p.Points[4], q.Points[11], p.Points[5], q.Points[10])
				}
			}
			if r+1 < g.Rows {
				q, ok := meshGradientCellPatch(g, resolved, r+1, col)
				if !ok {
					t.Fatal("bottom patch failed")
				}
				// bottom edge of p = [8],[7]; top edge of q = [1],[2].
				if !mgPointClose(p.Points[8], [2]float64{q.Points[1].X, q.Points[1].Y}) ||
					!mgPointClose(p.Points[7], [2]float64{q.Points[2].X, q.Points[2].Y}) {
					t.Errorf("horizontal shared edge mismatch at cell %d,%d: p8=%v q1=%v p7=%v q2=%v",
						r, col, p.Points[8], q.Points[1], p.Points[7], q.Points[2])
				}
			}
		}
	}
}

func TestMeshGradientBilinearMatchesPatches(t *testing.T) {
	c := loadMeshGradientCases(t)
	g := mgBuild(t, c.TangentGrid)
	got, ok := tessellateMeshGradient(g, Identity())
	if !ok {
		t.Fatal("tessellateMeshGradient failed")
	}
	resolved, ok := resolveMeshGradient(g)
	if !ok {
		t.Fatal("resolve failed")
	}
	var wantPos []Point
	var wantCol []RGBA
	var wantIdx []uint16
	for r := 0; r < g.Rows; r++ {
		for col := 0; col < g.Cols; col++ {
			p, ok := meshGradientCellPatch(g, resolved, r, col)
			if !ok {
				t.Fatal("patch failed")
			}
			lodX, lodY, ok := meshPatchLOD(p, Identity())
			if !ok {
				t.Fatal("LOD failed")
			}
			m, ok := tessellateMeshPatch(p, lodX, lodY)
			if !ok {
				t.Fatal("tessellate patch failed")
			}
			base := uint16(len(wantPos))
			wantPos = append(wantPos, m.Positions...)
			wantCol = append(wantCol, m.Colors...)
			for _, idx := range m.Indices {
				wantIdx = append(wantIdx, base+idx)
			}
		}
	}
	if len(got.Positions) != len(wantPos) || len(got.Indices) != len(wantIdx) {
		t.Fatalf("mesh size = %d/%d want %d/%d", len(got.Positions), len(got.Indices), len(wantPos), len(wantIdx))
	}
	for i := range wantPos {
		if !mgPointClose(got.Positions[i], [2]float64{wantPos[i].X, wantPos[i].Y}) {
			t.Fatalf("position %d = %v want %v", i, got.Positions[i], wantPos[i])
		}
		if !mgColorClose(got.Colors[i], [4]int{int(math.Round(wantCol[i].R * 255)), int(math.Round(wantCol[i].G * 255)), int(math.Round(wantCol[i].B * 255)), int(math.Round(wantCol[i].A * 255))}, 1) {
			t.Fatalf("color %d = %v want %v", i, got.Colors[i], wantCol[i])
		}
	}
	for i := range wantIdx {
		if got.Indices[i] != wantIdx[i] {
			t.Fatalf("index %d = %d want %d", i, got.Indices[i], wantIdx[i])
		}
	}
}

func TestMeshGradientCatmullRomColor(t *testing.T) {
	c := loadMeshGradientCases(t)
	g := mgBuild(t, c.CatmullGrid)
	g.ColorModel = MeshGradientCatmullRom
	resolved, ok := resolveMeshGradient(g)
	if !ok {
		t.Fatal("resolve failed")
	}
	for _, s := range c.CatmullGrid.Samples {
		got := meshGradientCatmullRomColor(g, resolved, s.R, s.C, s.U, s.V)
		if !mgColorClose(got, s.Color, 1) {
			t.Errorf("catmull cell %d,%d (%v,%v) = %v want %v", s.R, s.C, s.U, s.V, got, s.Color)
		}
	}
}

func TestMeshGradientBoundarySizes(t *testing.T) {
	c := loadMeshGradientCases(t)
	for _, sz := range c.Sizes {
		rows, cols := sz.Rows, sz.Cols
		vs := make([]MeshGradientVertex, 0, (rows+1)*(cols+1))
		for r := 0; r <= rows; r++ {
			for col := 0; col <= cols; col++ {
				i := r*5 + col // grid_5x5 is 5x5 row-major
				vs = append(vs, MeshGradientVertex{
					Pos:   Pt(c.Grid5x5.Positions[i][0], c.Grid5x5.Positions[i][1]),
					Color: mgColor(c.Grid5x5.Colors[i]),
				})
			}
		}
		g := &MeshGradient{Rows: rows, Cols: cols, Vertices: vs, ColorModel: MeshGradientBilinear, Space: MeshGradientAbsolute}
		if err := validateMeshGradient(g); err != nil {
			t.Fatalf("%dx%d validate: %v", rows, cols, err)
		}
		m, ok := tessellateMeshGradient(g, Identity())
		if !ok || len(m.Positions) == 0 || len(m.Indices) == 0 {
			t.Fatalf("%dx%d tessellate failed/empty", rows, cols)
		}
		for _, idx := range m.Indices {
			if int(idx) >= len(m.Positions) {
				t.Fatalf("%dx%d index %d out of range %d", rows, cols, idx, len(m.Positions))
			}
		}
		if len(m.Indices)%3 != 0 {
			t.Fatalf("%dx%d index count %d not multiple of 3", rows, cols, len(m.Indices))
		}
	}
}

func TestMeshGradientNormalizedSpace(t *testing.T) {
	c := loadMeshGradientCases(t)
	vs := make([]MeshGradientVertex, len(c.Normalized.Vertices))
	for i, v := range c.Normalized.Vertices {
		vs[i] = mgVertex(v)
	}
	g := &MeshGradient{
		Rows: c.Normalized.Rows, Cols: c.Normalized.Cols, Vertices: vs,
		ColorModel: MeshGradientBilinear, Space: MeshGradientNormalized,
		Bounds: Rect{Min: Pt(c.Normalized.Bounds[0], c.Normalized.Bounds[1]), Max: Pt(c.Normalized.Bounds[2], c.Normalized.Bounds[3])},
	}
	resolved, ok := resolveMeshGradient(g)
	if !ok {
		t.Fatal("resolve failed")
	}
	for i, want := range c.Normalized.ExpectedPositions {
		if !mgPointClose(resolved[i].pos, want) {
			t.Errorf("pos %d = %v want %v", i, resolved[i].pos, want)
		}
	}
	for i, want := range c.Normalized.ExpectedTangents {
		if !mgPointClose(resolved[i].left, want.Left) || !mgPointClose(resolved[i].right, want.Right) ||
			!mgPointClose(resolved[i].top, want.Top) || !mgPointClose(resolved[i].bottom, want.Bottom) {
			t.Errorf("tangent %d = L%v R%v T%v B%v want L%v R%v T%v B%v", i,
				resolved[i].left, resolved[i].right, resolved[i].top, resolved[i].bottom,
				want.Left, want.Right, want.Top, want.Bottom)
		}
	}
}

func TestMeshGradientInvalidShape(t *testing.T) {
	c := loadMeshGradientCases(t)
	base := mgBuild(t, c.TangentGrid)
	dc := NewContext(32, 32)
	defer dc.Close()

	cases := []struct {
		name string
		g    *MeshGradient
	}{
		{"zero rows", &MeshGradient{Rows: 0, Cols: 1, Vertices: base.Vertices[:2]}},
		{"short vertices", &MeshGradient{Rows: 1, Cols: 1, Vertices: base.Vertices[:3]}},
		{"bad color model", &MeshGradient{Rows: 3, Cols: 3, Vertices: base.Vertices, ColorModel: MeshGradientColorModel(99)}},
		{"bad space", &MeshGradient{Rows: 3, Cols: 3, Vertices: base.Vertices, Space: MeshGradientSpace(99)}},
		{"normalized without bounds", &MeshGradient{Rows: 3, Cols: 3, Vertices: base.Vertices, Space: MeshGradientNormalized}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateMeshGradient(tc.g); !errors.Is(err, ErrMeshGradientShape) {
				t.Fatalf("validate = %v want ErrMeshGradientShape", err)
			}
			if _, err := dc.DrawMeshGradientEx(tc.g, VertDrawOptions{}); !errors.Is(err, ErrMeshGradientShape) {
				t.Fatalf("DrawMeshGradientEx = %v want ErrMeshGradientShape", err)
			}
		})
	}
}

func TestMeshGradientCTM(t *testing.T) {
	c := loadMeshGradientCases(t)
	g := mgBuild(t, c.TangentGrid)
	a, ok := tessellateMeshGradient(g, Identity())
	if !ok {
		t.Fatal("identity tessellate failed")
	}
	b, ok := tessellateMeshGradient(g, Scale(2, 2))
	if !ok {
		t.Fatal("scale tessellate failed")
	}
	if len(b.Indices) <= len(a.Indices) {
		t.Fatalf("Scale(2) tris = %d, identity = %d; want more", len(b.Indices)/3, len(a.Indices)/3)
	}
}

func TestMeshGradientDrawCPUProbes(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	c := loadMeshGradientCases(t)

	// Bilinear 1x1: four distinct corners, center = bilerp.
	// Row-major: row 0 = TL, TR; row 1 = BL, BR.
	vs := []MeshGradientVertex{
		{Pos: Pt(0, 0), Color: RGBA{R: 1, A: 1}},         // TL
		{Pos: Pt(100, 0), Color: RGBA{G: 1, A: 1}},       // TR
		{Pos: Pt(0, 100), Color: RGBA{R: 1, G: 1, A: 1}}, // BL
		{Pos: Pt(100, 100), Color: RGBA{B: 1, A: 1}},     // BR
	}
	g := &MeshGradient{Rows: 1, Cols: 1, Vertices: vs, ColorModel: MeshGradientBilinear, Space: MeshGradientAbsolute}
	dc := NewContext(128, 128)
	defer dc.Close()
	dc.ClearWithColor(White)
	res, err := dc.DrawMeshGradientEx(g, VertDrawOptions{})
	if err != nil {
		t.Fatalf("DrawMeshGradientEx bilinear: %v", err)
	}
	if res.Skipped || res.Triangles == 0 || !res.Degraded {
		t.Fatalf("bilinear result = %+v, want CPU triangles", res)
	}
	probes := []struct {
		x, y int
		want [4]uint8
		tol  uint8
	}{
		{3, 3, [4]uint8{255, 0, 0, 255}, 16},
		{97, 3, [4]uint8{0, 255, 0, 255}, 16},
		{97, 97, [4]uint8{0, 0, 255, 255}, 16},
		{3, 97, [4]uint8{255, 255, 0, 255}, 16},
		{50, 50, [4]uint8{128, 128, 64, 255}, 20},
	}
	for _, p := range probes {
		got := mgSample(t, dc, p.x, p.y)
		if !mgClose8(got, p.want, p.tol) {
			t.Errorf("bilinear (%d,%d) = %v want %v tol %d", p.x, p.y, got, p.want, p.tol)
		}
	}

	// CatmullRom 2x2: sample the first cell center against the JSON fixture.
	cg := mgBuild(t, c.CatmullGrid)
	cg.ColorModel = MeshGradientCatmullRom
	dc2 := NewContext(300, 300)
	defer dc2.Close()
	dc2.ClearWithColor(White)
	if _, err := dc2.DrawMeshGradientEx(cg, VertDrawOptions{}); err != nil {
		t.Fatalf("DrawMeshGradientEx catmull: %v", err)
	}
	var want [4]int
	for _, s := range c.CatmullGrid.Samples {
		if s.R == 0 && s.C == 0 && s.U == 0.5 && s.V == 0.5 {
			want = s.Color
			break
		}
	}
	got := mgSample(t, dc2, 50, 50)
	if !mgClose8(got, [4]uint8{uint8(want[0]), uint8(want[1]), uint8(want[2]), uint8(want[3])}, 16) {
		t.Errorf("catmull cell center = %v want %v", got, want)
	}
}

func mgSample(t *testing.T, dc *Context, x, y int) [4]uint8 {
	t.Helper()
	img := dc.Image()
	r, g, b, a := img.At(x, y).RGBA()
	return [4]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}

func mgClose8(got, want [4]uint8, tol uint8) bool {
	for i := range got {
		d := int(got[i]) - int(want[i])
		if d < 0 {
			d = -d
		}
		if d > int(tol) {
			return false
		}
	}
	return true
}
