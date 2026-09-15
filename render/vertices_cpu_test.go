package render_test

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/internal/testutil/imagediff"
	_ "github.com/energye/gpui/render/gpu"
)

type vertsProbe struct {
	X    int      `json:"x"`
	Y    int      `json:"y"`
	Want [4]uint8 `json:"want"`
	Tol  uint8    `json:"tol"`
	Desc string   `json:"desc"`
}

type vertsDraw struct {
	Positions [][2]float64 `json:"positions"`
	Colors    [][4]float64 `json:"colors"`
	Mode      string       `json:"mode"`
	Probes    []vertsProbe `json:"probes"`
}

type vertsSolid struct {
	Positions [][2]float64 `json:"positions"`
	Fill      [4]float64   `json:"fill"`
	Probes    []vertsProbe `json:"probes"`
}

type vertsMesh struct {
	Positions [][2]float64 `json:"positions"`
	Colors    [][4]float64 `json:"colors"`
	Indices   []uint16     `json:"indices"`
	Probes    []vertsProbe `json:"probes"`
}

type vertsCases struct {
	DrawTri        vertsDraw  `json:"draw_tri"`
	DrawFan        vertsDraw  `json:"draw_fan"`
	MeshIndexed    vertsMesh  `json:"mesh_indexed"`
	SolidTri       vertsSolid `json:"solid_tri"`
	UniformTri     vertsDraw  `json:"uniform_tri"`
	DegradedReason string     `json:"degraded_reason"`
	Golden         struct {
		Canvas        [2]int  `json:"canvas"`
		File          string  `json:"file"`
		MaxChangedPct float64 `json:"max_changed_pct"`
		MaxMeanAbs    float64 `json:"max_mean_abs"`
		Note          string  `json:"note"`
	} `json:"golden"`
}

func loadVertsCases(t *testing.T) vertsCases {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "verts_cases.json"))
	if err != nil {
		t.Fatalf("read testdata/verts_cases.json: %v", err)
	}
	var c vertsCases
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode verts_cases.json: %v", err)
	}
	return c
}

func vertsToPoints(p [][2]float64) []render.Point {
	out := make([]render.Point, len(p))
	for i := range p {
		out[i] = render.Pt(p[i][0], p[i][1])
	}
	return out
}

func vertsToColors(c [][4]float64) []render.RGBA {
	out := make([]render.RGBA, len(c))
	for i := range c {
		out[i] = render.RGBA{R: c[i][0], G: c[i][1], B: c[i][2], A: c[i][3]}
	}
	return out
}

func vertsMode(s string) render.VertexMode {
	if s == "fan" {
		return render.VertexModeTriangleFan
	}
	return render.VertexModeTriangles
}

func vertsSample(t *testing.T, dc *render.Context, x, y int) [4]uint8 {
	t.Helper()
	img := dc.Image()
	b := img.Bounds()
	if x < b.Min.X || x >= b.Max.X || y < b.Min.Y || y >= b.Max.Y {
		t.Fatalf("probe %d,%d outside bounds %v", x, y, b)
	}
	r, g, b2, a := img.At(x, y).RGBA()
	return [4]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b2 >> 8), uint8(a >> 8)}
}

func vertsClose(got, want [4]uint8, tol uint8) bool {
	for i := 0; i < 4; i++ {
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

func vertsCheckProbes(t *testing.T, dc *render.Context, probes []vertsProbe) {
	t.Helper()
	for i, p := range probes {
		got := vertsSample(t, dc, p.X, p.Y)
		if !vertsClose(got, p.Want, p.Tol) {
			t.Errorf("probe[%d] %s (%d,%d) = %v, want %v tol %d", i, p.Desc, p.X, p.Y, got, p.Want, p.Tol)
		}
	}
}

func withVertsCPU(t *testing.T) {
	t.Helper()
	orig, had := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	t.Cleanup(func() {
		if had {
			_ = os.Setenv("GOGPU_RENDER_MODE", orig)
		} else {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		}
	})
}

func newVertsWhite(w, h int) *render.Context {
	dc := render.NewContext(w, h)
	dc.ClearWithColor(render.White)
	return dc
}

// A: RGB triangle is true gradient, not average solid.
func TestVertsDrawTriFromCases(t *testing.T) {
	withVertsCPU(t)
	c := loadVertsCases(t)
	dc := newVertsWhite(64, 64)
	defer dc.Close()
	dc.DrawVertices(vertsToPoints(c.DrawTri.Positions), vertsToColors(c.DrawTri.Colors), vertsMode(c.DrawTri.Mode))
	vertsCheckProbes(t, dc, c.DrawTri.Probes)
}

// A: fan quad hub interpolates, corners keep dominant channels.
func TestVertsDrawFanFromCases(t *testing.T) {
	withVertsCPU(t)
	c := loadVertsCases(t)
	dc := newVertsWhite(64, 64)
	defer dc.Close()
	dc.DrawVertices(vertsToPoints(c.DrawFan.Positions), vertsToColors(c.DrawFan.Colors), vertsMode(c.DrawFan.Mode))
	vertsCheckProbes(t, dc, c.DrawFan.Probes)
}

// A+C: indexed mesh matches triangle list; Ex reports degraded CPU fallback.
func TestVertsMeshIndexedFromCases(t *testing.T) {
	withVertsCPU(t)
	c := loadVertsCases(t)
	dc := newVertsWhite(64, 64)
	defer dc.Close()
	res, err := dc.DrawMeshEx(render.Mesh{
		Positions: vertsToPoints(c.MeshIndexed.Positions),
		Colors:    vertsToColors(c.MeshIndexed.Colors),
		Indices:   c.MeshIndexed.Indices,
	}, render.VertDrawOptions{})
	if err != nil {
		t.Fatalf("DrawMeshEx: %v", err)
	}
	if !res.Degraded || res.Reason != c.DegradedReason {
		t.Errorf("mesh Ex = %+v, want Degraded with reason %q", res, c.DegradedReason)
	}
	if dc.LastCPUFallbackReason() != c.DegradedReason {
		t.Errorf("fallback reason = %q, want %q", dc.LastCPUFallbackReason(), c.DegradedReason)
	}
	vertsCheckProbes(t, dc, c.MeshIndexed.Probes)
}

// A: solid (no vertex colors) keeps original Fill path.
func TestVertsSolidTriFromCases(t *testing.T) {
	withVertsCPU(t)
	c := loadVertsCases(t)
	dc := newVertsWhite(64, 64)
	defer dc.Close()
	f := c.SolidTri.Fill
	dc.SetRGBA(f[0], f[1], f[2], f[3])
	dc.DrawVertices(vertsToPoints(c.SolidTri.Positions), nil, render.VertexModeTriangles)
	vertsCheckProbes(t, dc, c.SolidTri.Probes)
}

// A: identical vertex colors keep the AA Fill path (tooltip arrow regression lock).
func TestVertsUniformTriFromCases(t *testing.T) {
	withVertsCPU(t)
	c := loadVertsCases(t)
	dc := newVertsWhite(64, 64)
	defer dc.Close()
	dc.DrawVertices(vertsToPoints(c.UniformTri.Positions), vertsToColors(c.UniformTri.Colors), vertsMode(c.UniformTri.Mode))
	vertsCheckProbes(t, dc, c.UniformTri.Probes)
	// Bit-exact vs explicit solid Fill: same path, no hard-edge regression.
	uniImg := dc.Image()
	dc2 := newVertsWhite(64, 64)
	defer dc2.Close()
	dc2.SetRGBA(1, 0, 0, 1)
	dc2.DrawVertices(vertsToPoints(c.UniformTri.Positions), nil, render.VertexModeTriangles)
	st, err := imagediff.Images(uniImg, dc2.Image())
	if err != nil {
		t.Fatalf("diff uniform vs solid: %v", err)
	}
	if st.ChangedPixels != 0 || st.MaxDelta != 0 {
		t.Fatalf("uniform colors must equal solid Fill, changed=%d max=%d", st.ChangedPixels, st.MaxDelta)
	}
}

// B: empty mesh skips, bad values report sentinel errors, never crash.
func TestVertsEdgeNoCrash(t *testing.T) {
	withVertsCPU(t)
	dc := newVertsWhite(32, 32)
	defer dc.Close()
	res, err := dc.DrawVerticesEx(nil, nil, render.VertexModeTriangles, render.VertDrawOptions{})
	if err != nil || !res.Skipped {
		t.Errorf("empty Ex = %+v err=%v, want Skipped nil err", res, err)
	}
	if got := vertsSample(t, dc, 16, 16); got != [4]uint8{255, 255, 255, 255} {
		t.Errorf("empty polluted canvas: %v", got)
	}
	nanPos := []render.Point{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: math.NaN(), Y: 10}}
	if _, err := dc.DrawVerticesEx(nanPos, nil, render.VertexModeTriangles, render.VertDrawOptions{}); !errors.Is(err, render.ErrVertsNonFinite) {
		t.Errorf("nan err = %v, want ErrVertsNonFinite", err)
	}
	infCol := []render.RGBA{{R: 1, G: 0, B: 0, A: 1}, {R: 0, G: 1, B: 0, A: 1}, {R: 0, G: 0, B: math.Inf(1), A: 1}}
	okPos := []render.Point{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 0, Y: 10}}
	if _, err := dc.DrawVerticesEx(okPos, infCol, render.VertexModeTriangles, render.VertDrawOptions{}); !errors.Is(err, render.ErrVertsNonFinite) {
		t.Errorf("inf color err = %v, want ErrVertsNonFinite", err)
	}
	badMesh := render.Mesh{Positions: okPos, Indices: []uint16{0, 1, 9}}
	if _, err := dc.DrawMeshEx(badMesh, render.VertDrawOptions{}); !errors.Is(err, render.ErrVertsBadIndex) {
		t.Errorf("bad index err = %v, want ErrVertsBadIndex", err)
	}
	r2, err := dc.DrawMeshEx(render.Mesh{}, render.VertDrawOptions{})
	if err != nil || !r2.Skipped {
		t.Errorf("empty mesh Ex = %+v err=%v, want Skipped nil err", r2, err)
	}
	// Old signatures stay no-op on degenerate input, never panic.
	dc.DrawVertices(nil, nil, render.VertexModeTriangles)
	dc.DrawMesh(render.Mesh{})
	// Degenerate triangle draws nothing but no error on Ex validate path.
	dgn := []render.Point{{X: 5, Y: 5}, {X: 5, Y: 5}, {X: 5, Y: 5}}
	dgnCol := []render.RGBA{{R: 1, G: 0, B: 0, A: 1}, {R: 0, G: 1, B: 0, A: 1}, {R: 0, G: 0, B: 1, A: 1}}
	resD, err := dc.DrawVerticesEx(dgn, dgnCol, render.VertexModeTriangles, render.VertDrawOptions{})
	if err != nil || !resD.Degraded || resD.Triangles != 1 {
		t.Errorf("degenerate Ex = %+v err=%v, want Degraded Triangles=1", resD, err)
	}
	if got := vertsSample(t, dc, 16, 16); got != [4]uint8{255, 255, 255, 255} {
		t.Errorf("degenerate polluted canvas: %v", got)
	}
}

// D: single-mesh CPU cost with numbers.
func TestVertsPerfSingle(t *testing.T) {
	withVertsCPU(t)
	c := loadVertsCases(t)
	pos := vertsToPoints(c.DrawTri.Positions)
	cols := vertsToColors(c.DrawTri.Colors)
	dc := newVertsWhite(64, 64)
	defer dc.Close()
	const n = 200
	start := time.Now()
	for i := 0; i < n; i++ {
		dc.ClearWithColor(render.White)
		dc.DrawVertices(pos, cols, render.VertexModeTriangles)
	}
	el := time.Since(start)
	t.Logf("verts-perf: %d draws 64x64 RGB tri in %v (%.2f us/draw)", n, el, float64(el.Microseconds())/float64(n))
}

// E: repeated draws do not leak or drift.
func TestVertsLongRunNoLeak(t *testing.T) {
	withVertsCPU(t)
	c := loadVertsCases(t)
	triPos := vertsToPoints(c.DrawTri.Positions)
	triCol := vertsToColors(c.DrawTri.Colors)
	fanPos := vertsToPoints(c.DrawFan.Positions)
	fanCol := vertsToColors(c.DrawFan.Colors)
	dc := newVertsWhite(64, 64)
	defer dc.Close()
	const n = 500
	for i := 0; i < n; i++ {
		dc.ClearWithColor(render.White)
		if i%2 == 1 {
			dc.DrawVertices(fanPos, fanCol, render.VertexModeTriangleFan)
		} else {
			dc.DrawVertices(triPos, triCol, render.VertexModeTriangles)
		}
	}
	// Last iteration is fan (i=499 odd): hub neighborhood must be red-blue mix.
	got := vertsSample(t, dc, 30, 30)
	want := [4]uint8{124, 0, 131, 255}
	if !vertsClose(got, want, 4) {
		t.Fatalf("longrun final probe = %v, want %v tol 4", got, want)
	}
	t.Logf("verts-longrun: %d alternating draws ok, final hub %v", n, got)
}

// F: offscreen golden regression, tolerance frozen in JSON.
func TestVertsGoldenGouraud(t *testing.T) {
	withVertsCPU(t)
	c := loadVertsCases(t)
	dc := newVertsWhite(c.Golden.Canvas[0], c.Golden.Canvas[1])
	defer dc.Close()
	dc.DrawVertices(vertsToPoints(c.DrawTri.Positions), vertsToColors(c.DrawTri.Colors), vertsMode(c.DrawTri.Mode))
	out := dc.Image()
	want, err := imagediff.DecodePNG(filepath.Join("testdata", c.Golden.File))
	if err != nil {
		t.Fatalf("decode golden %s: %v", c.Golden.File, err)
	}
	st, err := imagediff.Images(out, want)
	if err != nil {
		t.Fatalf("diff golden: %v", err)
	}
	changedPct := 100 * float64(st.ChangedPixels) / float64(st.TotalPixels)
	t.Logf("golden diff changed=%d/%d (%.3f%%) mean=%.3f rmse=%.3f max=%d tol_pct=%.1f tol_mean=%.1f",
		st.ChangedPixels, st.TotalPixels, changedPct, st.MeanAbs, st.RMSE, st.MaxDelta, c.Golden.MaxChangedPct, c.Golden.MaxMeanAbs)
	if changedPct > c.Golden.MaxChangedPct {
		t.Fatalf("golden changed %.3f%% > %.1f%%", changedPct, c.Golden.MaxChangedPct)
	}
	if st.MeanAbs > c.Golden.MaxMeanAbs {
		t.Fatalf("golden mean %.3f > %.1f", st.MeanAbs, c.Golden.MaxMeanAbs)
	}
}

// C: CPU vs GPU parity — true gradient must not collapse to solid, diff only AA edge.
// Needs native GPU; no-GPU envs skip instead of fake green.
func TestVertsCPUGPUParity(t *testing.T) {
	if render.Accelerator() == nil {
		t.Skip("no GPU accelerator, parity needs native GPU")
	}
	probe := render.NewContext(8, 8)
	probe.ClearWithColor(render.White)
	probe.SetRGB(1, 0, 0)
	probe.DrawRectangle(0, 0, 8, 8)
	_ = probe.Fill()
	if err := probe.FlushGPU(); err != nil {
		probe.Close()
		t.Skipf("GPU flush unavailable: %v", err)
	}
	isGPU := probe.RenderPathStats().GPUOps > 0
	probe.Close()
	if !isGPU {
		t.Skip("no GPU ops on probe, parity needs native GPU")
	}
	c := loadVertsCases(t)
	pos := vertsToPoints(c.DrawTri.Positions)
	cols := vertsToColors(c.DrawTri.Colors)
	orig, had := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	dcCPU := newVertsWhite(64, 64)
	dcCPU.DrawVertices(pos, cols, render.VertexModeTriangles)
	cpuImg := dcCPU.Image()
	dcCPU.Close()
	_ = os.Unsetenv("GOGPU_RENDER_MODE")
	if had {
		_ = os.Setenv("GOGPU_RENDER_MODE", orig)
	}
	dcGPU := newVertsWhite(64, 64)
	dcGPU.DrawVertices(pos, cols, render.VertexModeTriangles)
	if err := dcGPU.FlushGPU(); err != nil {
		dcGPU.Close()
		if errors.Is(err, render.ErrFallbackToCPU) {
			t.Skipf("flush fell back to CPU (no native GPU in this env): %v", err)
		}
		t.Fatalf("flush: %v", err)
	}
	if dcGPU.RenderPathStats().GPUOps == 0 {
		dcGPU.Close()
		t.Skipf("no GPUOps (no native GPU in this env): %s", dcGPU.RenderPathStats().LogLine())
	}
	gpuImg := dcGPU.Image()
	dcGPU.Close()
	st, err := imagediff.Images(cpuImg, gpuImg)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	changedPct := 100 * float64(st.ChangedPixels) / float64(st.TotalPixels)
	t.Logf("parity changed=%d/%d (%.3f%%) mean=%.3f rmse=%.3f max=%d", st.ChangedPixels, st.TotalPixels, changedPct, st.MeanAbs, st.RMSE, st.MaxDelta)
	// True gradient must not collapse to average solid: an off-center
	// probe differs strongly from mean-fill. (32,40) is the triangle
	// centroid where barycentric weights are exactly 1/3 each, so even a
	// true gradient reads near-mean there; use near-red (14,50) instead.
	meanSolid := render.RGBA{R: 1.0 / 3, G: 1.0 / 3, B: 1.0 / 3, A: 1}
	r, g, b, _ := cpuImg.At(14, 50).RGBA()
	mr := uint8(float64(meanSolid.R) * 255)
	mg := uint8(float64(meanSolid.G) * 255)
	mb := uint8(float64(meanSolid.B) * 255)
	dr := int(r>>8) - int(mr)
	dg := int(g>>8) - int(mg)
	db := int(b>>8) - int(mb)
	if dr < 0 {
		dr = -dr
	}
	if dg < 0 {
		dg = -dg
	}
	if db < 0 {
		db = -db
	}
	if dr+dg+db < 30 {
		t.Fatalf("near-red %v too close to average solid [%d,%d,%d]: gradient collapsed?", [4]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}, mr, mg, mb)
	}
	if changedPct > 1.0 {
		t.Fatalf("parity changed %.3f%% > 1%% (gradient became solid?)", changedPct)
	}
}
