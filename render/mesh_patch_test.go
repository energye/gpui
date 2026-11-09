package render

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// P3.9 mesh gradient: capability tests written before the implementation
// (red first). Test fixtures live in testdata/mesh_patch_cases.json; this
// file only decodes them and asserts the public/internal patch contract.

type meshPatchCase struct {
	Style  string       `json:"style"`
	Points [][2]float64 `json:"points"`
	Colors [][4]float64 `json:"colors"`
	LOD    []int        `json:"lod"`

	Corners []struct {
		Role  string     `json:"role"`
		Grid  [2]int     `json:"grid"`
		Pos   [2]float64 `json:"pos"`
		Color [4]int     `json:"color"`
	} `json:"corners"`
	Center struct {
		Grid  [2]int `json:"grid"`
		Color [4]int `json:"color"`
	} `json:"center"`
	PlanarSamples []struct {
		U   float64    `json:"u"`
		V   float64    `json:"v"`
		Pos [2]float64 `json:"pos"`
	} `json:"planar_samples"`
	PlanarTol float64 `json:"planar_tol"`
	Samples   []struct {
		U   float64    `json:"u"`
		V   float64    `json:"v"`
		Pos [2]float64 `json:"pos"`
	} `json:"samples"`
	Tol         float64 `json:"tol"`
	ExpectedLOD []int   `json:"expected_lod"`
	MaxVertices int     `json:"max_vertices"`
	MaxIndices  int     `json:"max_indices"`

	Patches []struct {
		Points [][2]float64 `json:"points"`
		Colors [][4]float64 `json:"colors"`
	} `json:"patches"`
	Shared []struct {
		A struct {
			Patch  int    `json:"patch"`
			Corner string `json:"corner"`
		} `json:"a"`
		B struct {
			Patch  int    `json:"patch"`
			Corner string `json:"corner"`
		} `json:"b"`
		Pos   [2]float64 `json:"pos"`
		Color [4]int     `json:"color"`
	} `json:"shared"`
}

func loadMeshPatchCases(t *testing.T) map[string]meshPatchCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "mesh_patch_cases.json"))
	if err != nil {
		t.Fatalf("read testdata/mesh_patch_cases.json: %v", err)
	}
	var out map[string]meshPatchCase
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode mesh_patch_cases.json: %v", err)
	}
	return out
}

func meshPatchStyleFromString(t *testing.T, s string) MeshPatchStyle {
	t.Helper()
	switch s {
	case "coons":
		return MeshPatchCoons
	case "bicubic":
		return MeshPatchBicubic
	default:
		t.Fatalf("unknown fixture style %q", s)
		return 0
	}
}

func meshPatchFromCase(t *testing.T, c meshPatchCase) MeshPatch {
	t.Helper()
	pts := make([]Point, len(c.Points))
	for i, p := range c.Points {
		pts[i] = Pt(p[0], p[1])
	}
	var cols [4]RGBA
	for i := 0; i < 4 && i < len(c.Colors); i++ {
		cols[i] = RGBA{R: c.Colors[i][0], G: c.Colors[i][1], B: c.Colors[i][2], A: c.Colors[i][3]}
	}
	return MeshPatch{Style: meshPatchStyleFromString(t, c.Style), Points: pts, Colors: cols}
}

func meshPatchGridIndex(x, y, lodX, lodY int) int { return x*(lodY+1) + y }

func meshPatchCornerGrid(role string, lodX, lodY int) (int, int) {
	switch role {
	case "tl":
		return 0, 0
	case "tr":
		return lodX, 0
	case "br":
		return lodX, lodY
	case "bl":
		return 0, lodY
	default:
		panic("bad corner role " + role)
	}
}

func meshPatchPosClose(got Point, want [2]float64, tol float64) bool {
	return math.Abs(got.X-want[0]) <= tol && math.Abs(got.Y-want[1]) <= tol
}

func meshPatchColorClose(got RGBA, want [4]int, tol int) bool {
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

func meshPatchNonWhite(t *testing.T, dc *Context) int {
	t.Helper()
	img := dc.Image()
	b := img.Bounds()
	n := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bb, _ := img.At(x, y).RGBA()
			if r>>8 < 250 || g>>8 < 250 || bb>>8 < 250 {
				n++
			}
		}
	}
	return n
}

func TestMeshPatchCoonsControlOrder(t *testing.T) {
	d := loadMeshPatchCases(t)
	c := d["coons_square"]
	p := meshPatchFromCase(t, c)
	lodX, lodY, ok := meshPatchLOD(p, Identity())
	if !ok || lodX != c.LOD[0] || lodY != c.LOD[1] {
		t.Fatalf("LOD = %d,%d ok=%v want %v", lodX, lodY, ok, c.LOD)
	}
	mesh, ok := tessellateMeshPatch(p, lodX, lodY)
	if !ok {
		t.Fatal("tessellateMeshPatch returned false")
	}
	if want := (lodX + 1) * (lodY + 1); len(mesh.Positions) != want {
		t.Fatalf("vertices = %d want %d", len(mesh.Positions), want)
	}
	if want := lodX * lodY * 6; len(mesh.Indices) != want {
		t.Fatalf("indices = %d want %d", len(mesh.Indices), want)
	}
	for _, corner := range c.Corners {
		gx, gy := meshPatchCornerGrid(corner.Role, lodX, lodY)
		idx := meshPatchGridIndex(gx, gy, lodX, lodY)
		if !meshPatchPosClose(mesh.Positions[idx], corner.Pos, 1e-9) {
			t.Errorf("corner %s pos = %v want %v", corner.Role, mesh.Positions[idx], corner.Pos)
		}
		if !meshPatchColorClose(mesh.Colors[idx], corner.Color, 1) {
			t.Errorf("corner %s color = %v want %v", corner.Role, mesh.Colors[idx], corner.Color)
		}
	}
	idx := meshPatchGridIndex(c.Center.Grid[0], c.Center.Grid[1], lodX, lodY)
	if !meshPatchColorClose(mesh.Colors[idx], c.Center.Color, 1) {
		t.Errorf("center color = %v want %v", mesh.Colors[idx], c.Center.Color)
	}
}

func TestMeshPatchCoonsPlanarIdentity(t *testing.T) {
	d := loadMeshPatchCases(t)
	c := d["coons_square"]
	p := meshPatchFromCase(t, c)
	lodX, lodY, ok := meshPatchLOD(p, Identity())
	if !ok {
		t.Fatal("LOD not ok")
	}
	mesh, ok := tessellateMeshPatch(p, lodX, lodY)
	if !ok {
		t.Fatal("tessellate failed")
	}
	tl := c.Corners[0].Pos
	br := c.Corners[2].Pos
	tol := c.PlanarTol
	if tol == 0 {
		tol = 1e-9
	}
	for y := 0; y <= lodY; y++ {
		for x := 0; x <= lodX; x++ {
			u := float64(x) / float64(lodX)
			v := float64(y) / float64(lodY)
			want := [2]float64{tl[0] + (br[0]-tl[0])*u, tl[1] + (br[1]-tl[1])*v}
			got := mesh.Positions[meshPatchGridIndex(x, y, lodX, lodY)]
			if !meshPatchPosClose(got, want, tol) {
				t.Fatalf("grid %d,%d pos = %v want %v", x, y, got, want)
			}
		}
	}
}

func TestMeshPatchCoonsPremulCornerColors(t *testing.T) {
	d := loadMeshPatchCases(t)
	c := d["coons_premul"]
	p := meshPatchFromCase(t, c)
	lodX, lodY, ok := meshPatchLOD(p, Identity())
	if !ok {
		t.Fatal("LOD not ok")
	}
	mesh, ok := tessellateMeshPatch(p, lodX, lodY)
	if !ok {
		t.Fatal("tessellate failed")
	}
	for _, corner := range c.Corners {
		gx, gy := meshPatchCornerGrid(corner.Role, lodX, lodY)
		idx := meshPatchGridIndex(gx, gy, lodX, lodY)
		if !meshPatchColorClose(mesh.Colors[idx], corner.Color, 1) {
			t.Errorf("corner %s color = %v want %v", corner.Role, mesh.Colors[idx], corner.Color)
		}
	}
	idx := meshPatchGridIndex(c.Center.Grid[0], c.Center.Grid[1], lodX, lodY)
	if !meshPatchColorClose(mesh.Colors[idx], c.Center.Color, 1) {
		t.Errorf("premul center color = %v want %v", mesh.Colors[idx], c.Center.Color)
	}
}

func TestMeshPatchLOD(t *testing.T) {
	d := loadMeshPatchCases(t)
	for _, name := range []string{"lod_small", "lod_large", "lod_cap"} {
		c := d[name]
		p := meshPatchFromCase(t, c)
		lodX, lodY, ok := meshPatchLOD(p, Identity())
		if !ok || lodX != c.ExpectedLOD[0] || lodY != c.ExpectedLOD[1] {
			t.Errorf("%s LOD = %d,%d ok=%v want %v", name, lodX, lodY, ok, c.ExpectedLOD)
			continue
		}
		mesh, ok := tessellateMeshPatch(p, lodX, lodY)
		if !ok {
			t.Errorf("%s tessellate failed", name)
			continue
		}
		if c.MaxVertices > 0 && len(mesh.Positions) > c.MaxVertices {
			t.Errorf("%s vertices = %d > max %d", name, len(mesh.Positions), c.MaxVertices)
		}
		if c.MaxIndices > 0 && len(mesh.Indices) > c.MaxIndices {
			t.Errorf("%s indices = %d > max %d", name, len(mesh.Indices), c.MaxIndices)
		}
	}
	// Non-finite control points must not draw (Skia GetLevelOfDetail returns 0).
	c := d["coons_square"]
	p := meshPatchFromCase(t, c)
	p.Points[0] = Point{X: math.Inf(1), Y: 0}
	if _, _, ok := meshPatchLOD(p, Identity()); ok {
		t.Error("non-finite patch LOD ok, want false")
	}
	dc := NewContext(16, 16)
	defer dc.Close()
	if _, err := dc.DrawMeshPatchEx(p, VertDrawOptions{}); !errors.Is(err, ErrVertsNonFinite) {
		t.Fatalf("DrawMeshPatchEx non-finite err = %v want ErrVertsNonFinite", err)
	}
}

func TestMeshPatchTensorSurface(t *testing.T) {
	d := loadMeshPatchCases(t)
	for _, name := range []string{"tensor_planar", "tensor_curved"} {
		c := d[name]
		p := meshPatchFromCase(t, c)
		tol := c.Tol
		if tol == 0 {
			tol = 1e-9
		}
		for _, s := range c.Samples {
			got, ok := meshPatchSurfacePoint(p, s.U, s.V)
			if !ok {
				t.Fatalf("%s surface (%v,%v) not ok", name, s.U, s.V)
			}
			if !meshPatchPosClose(got, s.Pos, tol) {
				t.Errorf("%s surface (%v,%v) = %v want %v", name, s.U, s.V, got, s.Pos)
			}
		}
	}
}

func TestMeshPatchTensorPlanarIdentity(t *testing.T) {
	d := loadMeshPatchCases(t)
	c := d["tensor_planar"]
	p := meshPatchFromCase(t, c)
	tol := c.Tol
	if tol == 0 {
		tol = 1e-9
	}
	for _, s := range c.Samples {
		got, ok := meshPatchSurfacePoint(p, s.U, s.V)
		if !ok {
			t.Fatalf("surface (%v,%v) not ok", s.U, s.V)
		}
		if !meshPatchPosClose(got, s.Pos, tol) {
			t.Errorf("planar tensor (%v,%v) = %v want %v", s.U, s.V, got, s.Pos)
		}
	}
}

func TestMeshPatchGridSharedCorners(t *testing.T) {
	d := loadMeshPatchCases(t)
	for _, name := range []string{"grid_2x2", "grid_3x3"} {
		c := d[name]
		meshes := make([]Mesh, len(c.Patches))
		lods := make([][2]int, len(c.Patches))
		for i, pc := range c.Patches {
			p := MeshPatch{Style: MeshPatchCoons, Colors: [4]RGBA{}, Points: make([]Point, len(pc.Points))}
			for j, pt := range pc.Points {
				p.Points[j] = Pt(pt[0], pt[1])
			}
			for j := 0; j < 4 && j < len(pc.Colors); j++ {
				p.Colors[j] = RGBA{R: pc.Colors[j][0], G: pc.Colors[j][1], B: pc.Colors[j][2], A: pc.Colors[j][3]}
			}
			lodX, lodY, ok := meshPatchLOD(p, Identity())
			if !ok {
				t.Fatalf("%s patch %d LOD not ok", name, i)
			}
			mesh, ok := tessellateMeshPatch(p, lodX, lodY)
			if !ok {
				t.Fatalf("%s patch %d tessellate failed", name, i)
			}
			meshes[i] = mesh
			lods[i] = [2]int{lodX, lodY}
		}
		for _, sh := range c.Shared {
			a := meshes[sh.A.Patch]
			b := meshes[sh.B.Patch]
			ax, ay := meshPatchCornerGrid(sh.A.Corner, lods[sh.A.Patch][0], lods[sh.A.Patch][1])
			bx, by := meshPatchCornerGrid(sh.B.Corner, lods[sh.B.Patch][0], lods[sh.B.Patch][1])
			ai := meshPatchGridIndex(ax, ay, lods[sh.A.Patch][0], lods[sh.A.Patch][1])
			bi := meshPatchGridIndex(bx, by, lods[sh.B.Patch][0], lods[sh.B.Patch][1])
			if !meshPatchPosClose(a.Positions[ai], sh.Pos, 1e-9) || !meshPatchPosClose(b.Positions[bi], sh.Pos, 1e-9) {
				t.Errorf("%s shared pos a=%v b=%v want %v", name, a.Positions[ai], b.Positions[bi], sh.Pos)
			}
			if !meshPatchColorClose(a.Colors[ai], sh.Color, 1) || !meshPatchColorClose(b.Colors[bi], sh.Color, 1) {
				t.Errorf("%s shared color a=%v b=%v want %v", name, a.Colors[ai], b.Colors[bi], sh.Color)
			}
		}
	}
}

func TestMeshPatchInvalidShape(t *testing.T) {
	d := loadMeshPatchCases(t)
	base := d["coons_square"]
	good := meshPatchFromCase(t, base)

	newDC := func() *Context {
		dc := NewContext(32, 32)
		dc.ClearWithColor(White)
		return dc
	}
	cases := []struct {
		name  string
		patch MeshPatch
		want  error
	}{
		{"coons short", MeshPatch{Style: MeshPatchCoons, Points: good.Points[:11], Colors: good.Colors}, ErrMeshPatchShape},
		{"bicubic short", MeshPatch{Style: MeshPatchBicubic, Points: good.Points[:12], Colors: good.Colors}, ErrMeshPatchShape},
		{"unknown style", MeshPatch{Style: MeshPatchStyle(99), Points: good.Points, Colors: good.Colors}, ErrMeshPatchShape},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dc := newDC()
			defer dc.Close()
			if _, err := dc.DrawMeshPatchEx(tc.patch, VertDrawOptions{}); !errors.Is(err, tc.want) {
				t.Fatalf("DrawMeshPatchEx err = %v want %v", err, tc.want)
			}
			dc.DrawMeshPatch(tc.patch)
			if n := meshPatchNonWhite(t, dc); n != 0 {
				t.Fatalf("invalid patch drew %d non-white pixels", n)
			}
		})
	}

	nonFinitePoint := good
	nonFinitePoint.Points = append([]Point(nil), good.Points...)
	nonFinitePoint.Points[0] = Point{X: math.NaN(), Y: 0}
	dc := newDC()
	defer dc.Close()
	if _, err := dc.DrawMeshPatchEx(nonFinitePoint, VertDrawOptions{}); !errors.Is(err, ErrVertsNonFinite) {
		t.Fatalf("non-finite point err = %v want ErrVertsNonFinite", err)
	}

	nonFiniteColor := good
	nonFiniteColor.Colors[0] = RGBA{R: math.Inf(1), A: 1}
	dc2 := newDC()
	defer dc2.Close()
	if _, err := dc2.DrawMeshPatchEx(nonFiniteColor, VertDrawOptions{}); !errors.Is(err, ErrVertsNonFinite) {
		t.Fatalf("non-finite color err = %v want ErrVertsNonFinite", err)
	}
}

func TestMeshPatchCTMLOD(t *testing.T) {
	d := loadMeshPatchCases(t)
	c := d["coons_square"]
	p := meshPatchFromCase(t, c)
	lx, ly, ok := meshPatchLOD(p, Identity())
	if !ok || lx != c.LOD[0] || ly != c.LOD[1] {
		t.Fatalf("identity LOD = %d,%d ok=%v want %v", lx, ly, ok, c.LOD)
	}
	sx, sy, ok := meshPatchLOD(p, Scale(2, 2))
	if !ok || sx != c.LOD[0]*2 || sy != c.LOD[1]*2 {
		t.Fatalf("Scale(2) LOD = %d,%d ok=%v want %d,%d", sx, sy, ok, c.LOD[0]*2, c.LOD[1]*2)
	}
}

func TestMeshPatchDrawCPUProbes(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	d := loadMeshPatchCases(t)
	c := d["coons_square"]
	p := meshPatchFromCase(t, c)
	dc := NewContext(128, 128)
	defer dc.Close()
	dc.ClearWithColor(White)
	res, err := dc.DrawMeshPatchEx(p, VertDrawOptions{})
	if err != nil {
		t.Fatalf("DrawMeshPatchEx: %v", err)
	}
	if res.Skipped || res.Triangles != 200 || !res.Degraded {
		t.Fatalf("CPU result = %+v, want 200 triangles degraded", res)
	}
	sample := func(x, y int) [4]uint8 {
		img := dc.Image()
		r, g, b, a := img.At(x, y).RGBA()
		return [4]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
	}
	probes := []struct {
		name string
		x, y int
		want [4]uint8
		tol  uint8
	}{
		{"tl", 3, 3, [4]uint8{255, 0, 0, 255}, 16},
		{"tr", 97, 3, [4]uint8{0, 255, 0, 255}, 16},
		{"br", 97, 97, [4]uint8{0, 0, 255, 255}, 16},
		{"bl", 3, 97, [4]uint8{255, 255, 0, 255}, 16},
		{"center", 50, 50, [4]uint8{128, 128, 64, 255}, 20},
	}
	for _, pr := range probes {
		got := sample(pr.x, pr.y)
		for i := range got {
			d := int(got[i]) - int(pr.want[i])
			if d < 0 {
				d = -d
			}
			if d > int(pr.tol) {
				t.Errorf("%s (%d,%d) = %v want %v tol %d", pr.name, pr.x, pr.y, got, pr.want, pr.tol)
				break
			}
		}
	}
}
