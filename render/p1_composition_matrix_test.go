//go:build !nogpu

package render_test

// Phase A — arbitrary composition dimension probes (not widget/antd catalog).
//
// Architecture:
//   render.Context → accelerator → gpu/webgpu → gpu/rwgpu → libwgpu_native
//
// IDs D01+ match docs/P1_COMPOSITION_MATRIX.md.
// Hard rules: GPUOps>0, structural pixels, real native path.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/stdgate/compare"
	"github.com/energye/gpui/stdgate/scene"
)

func compMakeImage(t *testing.T, w, h int, r, g, b uint8) *render.ImageBuf {
	t.Helper()
	img, err := render.NewImageBuf(w, h, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			_ = img.SetRGBA(x, y, r, g, b, 255)
		}
	}
	return img
}

// compRepoTmpCompDir resolves <repo>/tmp/comp for PNG dumps (cwd may be render/).
func compRepoTmpCompDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			out := filepath.Join(dir, "tmp", "comp")
			if err := os.MkdirAll(out, 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", out, err)
			}
			return out
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	out := filepath.Join(wd, "tmp", "comp")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", out, err)
	}
	return out
}

// compSavePNG writes GPU-flushed context pixels to gpui/tmp/comp/<name>.png
// and compares against frozen refs in testdata/refs/standard.json + standard/ when present.
//
// Env:
//
//	GPUI_UPDATE_COMP_REFS=1  — copy actual into refs (refresh baseline; review diffs)
//	GPUI_SKIP_COMP_REFS=1     — skip pixel compare (debug only)
//	GPUI_REQUIRE_COMP_REFS=1  — fail if ref PNG missing
func compSavePNG(t *testing.T, dc *render.Context, name string) {
	t.Helper()
	path := filepath.Join(compRepoTmpCompDir(t), name+".png")
	if err := dc.SavePNG(path); err != nil {
		t.Fatalf("SavePNG %s: %v", path, err)
	}
	t.Logf("wrote %s", path)
	compCompareRef(t, name, path)
}

// compRepoRoot finds the module root (directory containing go.mod).
func compRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return wd
}

// compRefsCatalogPath is testdata/refs/standard.json (all case meta).
func compRefsCatalogPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(compRepoRoot(t), "testdata", "refs", "standard.json")
}

// compRefsImageDir is testdata/refs/standard/ (PNG only).
func compRefsImageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(compRepoRoot(t), "testdata", "refs", "standard")
}

// compCompareRef diffs actual PNG against testdata/refs/standard.json + standard/<id>.png.
func compCompareRef(t *testing.T, name, actualPath string) {
	t.Helper()
	if os.Getenv("GPUI_SKIP_COMP_REFS") == "1" {
		return
	}
	catPath := compRefsCatalogPath(t)
	imgDir := compRefsImageDir(t)
	refPNG := filepath.Join(imgDir, name+".png")

	if os.Getenv("GPUI_UPDATE_COMP_REFS") == "1" {
		if err := os.MkdirAll(imgDir, 0o755); err != nil {
			t.Fatalf("mkdir refs images: %v", err)
		}
		b, err := os.ReadFile(actualPath)
		if err != nil {
			t.Fatalf("read actual: %v", err)
		}
		if err := os.WriteFile(refPNG, b, 0o644); err != nil {
			t.Fatalf("update ref png: %v", err)
		}
		var cat *compare.Catalog
		if _, err := os.Stat(catPath); err == nil {
			cat, err = compare.LoadCatalog(catPath)
			if err != nil {
				t.Fatalf("load catalog: %v", err)
			}
		} else {
			cat = &compare.Catalog{
				Name:        "standard",
				Oracle:      "gpui-capture",
				ImagesDir:   "standard",
				DefaultDiff: compare.DefaultPolicy(),
				Cases:       map[string]compare.CaseMeta{},
			}
		}
		meta := cat.Case(name)
		meta.ID = name
		meta.File = name + ".png"
		meta.Oracle = cat.Oracle
		if meta.Diff.MaxMeanAbs <= 0 {
			meta.Diff = cat.DefaultDiff
		}
		cat.UpsertCase(meta)
		cat.ImagesDir = "standard"
		if err := compare.WriteCatalog(catPath, cat); err != nil {
			t.Fatalf("write catalog: %v", err)
		}
		t.Logf("updated standard ref %s + catalog", refPNG)
		return
	}

	if _, err := os.Stat(catPath); err != nil {
		require := os.Getenv("GPUI_REQUIRE_COMP_REFS") == "1" && len(name) >= 2 && name[0] == 'D' && name[1] >= '0' && name[1] <= '9'
		if require {
			t.Fatalf("missing catalog %s", catPath)
		}
		t.Logf("no catalog — skip compare for %s", name)
		return
	}
	cat, err := compare.LoadCatalog(catPath)
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	if _, err := os.Stat(refPNG); err != nil {
		// also try path from catalog entry
		if p, err2 := cat.RefPNG(name); err2 == nil {
			refPNG = p
		}
	}
	if _, err := os.Stat(refPNG); err != nil {
		require := os.Getenv("GPUI_REQUIRE_COMP_REFS") == "1" && len(name) >= 2 && name[0] == 'D' && name[1] >= '0' && name[1] <= '9'
		if require {
			t.Fatalf("missing ref %s (set GPUI_UPDATE_COMP_REFS=1 to capture)", refPNG)
		}
		t.Logf("no ref for %s — skip compare", name)
		return
	}
	res, meta, err := cat.CompareNamed(name, actualPath)
	if err != nil {
		t.Fatalf("compare %s: %v", name, err)
	}
	if res.Pass {
		t.Logf("ref OK %s mean_abs=%.3f rmse=%.3f changed=%.4f oracle=%s",
			name, res.Stats.MeanAbs, res.Stats.RMSE, res.Stats.ChangedRatio, meta.Oracle)
		return
	}
	diffDir := filepath.Join(compRepoRoot(t), "tmp", "comp_diff")
	_ = os.MkdirAll(diffDir, 0o755)
	expImg, err1 := compare.DecodePNG(refPNG)
	actImg, err2 := compare.DecodePNG(actualPath)
	if err1 == nil && err2 == nil {
		diffPath := filepath.Join(diffDir, name+"_diff.png")
		if err := compare.WriteDiffPNG(expImg, actImg, diffPath); err == nil {
			t.Logf("wrote diff %s", diffPath)
		}
	}
	t.Fatalf("ref mismatch %s: %s (mean_abs=%.3f rmse=%.3f max_delta=%d changed=%d/%d) ref=%s actual=%s",
		name, res.Reason, res.Stats.MeanAbs, res.Stats.RMSE, res.Stats.MaxDelta,
		res.Stats.ChangedPixels, res.Stats.TotalPixels, refPNG, actualPath)
}

// compAutoSavePNG dumps PNG named from the test function (Dxx_...).
func compAutoSavePNG(t *testing.T, dc *render.Context) {
	t.Helper()
	name := t.Name()
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimPrefix(name, "TestP1_Comp_")
	name = strings.ReplaceAll(name, "/", "_")
	if name == "" {
		name = "unnamed"
	}
	compSavePNG(t, dc, name)
}

// compRunScene loads testdata/scenes/<id>.json and renders with gpui.
// Caller must Close the context.
func compRunScene(t *testing.T, id string) *render.Context {
	t.Helper()
	root := compRepoRoot(t)
	path := filepath.Join(root, "testdata", "scenes", id+".json")
	s, err := scene.Load(path)
	if err != nil {
		t.Fatalf("load scene %s: %v", id, err)
	}
	dc, err := scene.RunGPUI(s, root)
	if err != nil {
		t.Fatalf("run scene %s: %v", id, err)
	}
	return dc
}

// D01: nested ClipRect × semi-transparent PushLayer × text.
func TestP1_Comp_D01_ClipLayerText(t *testing.T) {
	p1RequireGPU(t)
	dc := compRunScene(t, "D01_ClipLayerText")
	defer dc.Close()
	compSavePNG(t, dc, "D01_ClipLayerText")

	r, g, b, _ := p1Sample(dc, 160, 100)
	p1NotNearWhite(t, "D01 layer body", r, g, b)
	if g < 40 {
		t.Fatalf("D01 expected warm/yellow contribution rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 8, 8)
	if b2 > 200 && r2 < 80 {
		t.Fatalf("D01 outer chrome leaked blue rgba=%d,%d,%d", r2, g2, b2)
	}
	r3, g3, b3, _ := p1Sample(dc, 36, 36)
	if b3 < 100 {
		t.Fatalf("D01 outer clip band not blue-ish rgba=%d,%d,%d", r3, g3, b3)
	}
}

// D02: ClipRoundRect × DrawImage × BlendPlus.
func TestP1_Comp_D02_ClipImageBlend(t *testing.T) {
	p1RequireGPU(t)
	dc := compRunScene(t, "D02_ClipImageBlend")
	defer dc.Close()
	compSavePNG(t, dc, "D02_ClipImageBlend")

	r, g, b, _ := p1Sample(dc, 100, 80)
	p1NotNearWhite(t, "D02 clipped content", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 24, 24)
	if r2 < 150 || g2 > 100 {
		t.Fatalf("D02 base red missing outside clip rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D03: path Clip × PushLayer × solid fill.
func TestP1_Comp_D03_ClipPathLayerFill(t *testing.T) {
	p1RequireGPU(t)
	dc := compRunScene(t, "D03_ClipPathLayerFill")
	defer dc.Close()
	compSavePNG(t, dc, "D03_ClipPathLayerFill")

	rRing, gRing, bRing, _ := p1Sample(dc, 130, 60)
	if gRing < 80 {
		t.Fatalf("D03 diamond fill not green-ish rgba=%d,%d,%d", rRing, gRing, bRing)
	}
	r2, g2, b2, _ := p1Sample(dc, 20, 20)
	if g2 > 200 && r2 < 50 {
		t.Fatalf("D03 clip leaked green to corner rgba=%d,%d,%d", r2, g2, b2)
	}
	_ = b2
}

// D04: HiDPI × hairline × text.
func TestP1_Comp_D04_HiDPIHairlineText(t *testing.T) {
	p1RequireGPU(t)
	dc := compRunScene(t, "D04_HiDPIHairlineText")
	defer dc.Close()
	compSavePNG(t, dc, "D04_HiDPIHairlineText")

	ink := 0
	for x := 20; x < 300; x++ {
		r, g, b, _ := p1Sample(dc, x, 40)
		if r < 200 || g < 200 || b < 200 {
			ink++
		}
	}
	if ink < 8 {
		t.Fatalf("D04 hairline ink too low: %d", ink)
	}
	textInk := 0
	for y := 100; y < 140; y++ {
		for x := 12; x < 300; x++ {
			r, g, b, _ := p1Sample(dc, x, y)
			if r < 220 || g < 220 || b < 220 {
				textInk++
			}
		}
	}
	if textInk < 10 {
		t.Fatalf("D04 text ink too low: %d", textInk)
	}
	t.Logf("D04 hairlineInk=%d textInk=%d", ink, textInk)
}

// D05: outer clip × Multiply layer × nested Normal layer.
func TestP1_Comp_D05_LayerBlendClip(t *testing.T) {
	p1RequireGPU(t)
	dc := compRunScene(t, "D05_LayerBlendClip")
	defer dc.Close()
	compSavePNG(t, dc, "D05_LayerBlendClip")

	r, g, b, _ := p1Sample(dc, 130, 95)
	p1NotNearWhite(t, "D05 card", r, g, b)
	if b < 40 {
		t.Fatalf("D05 expected blue-ish card rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 10, 10)
	if r2 < 120 || g2 < 150 {
		t.Fatalf("D05 outside clip should stay light cyan rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D06: image × text × clip × backdrop.
func TestP1_Comp_D06_ImageTextClipBackdrop(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 360, 240
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 13)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// Content board
	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	img := compMakeImage(t, 48, 48, 40, 140, 220)
	for i := 0; i < 4; i++ {
		x := 24 + float64(i)*80
		dc.ClipRect(x, 40, 64, 64)
		dc.DrawImage(img, x+8, 48)
		dc.ResetClip()
		dc.SetRGB(0.15, 0.16, 0.2)
		dc.DrawString(fmt.Sprintf("tile-%d", i), x+8, 128)
	}

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("pre-backdrop flush: %v", err)
	}
	base := dc.RenderPathStats().GPUOps

	dc.PushBackdropLayer(render.BlendNormal, 1)
	dc.SetRGBA(0, 0, 0, 0.40)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(80, 50, 200, 140, 12)
	_ = dc.Fill()
	dc.SetRGB(0.12, 0.12, 0.15)
	dc.DrawString("backdrop×image×text", 100, 120)
	dc.PopLayer()

	p1Flush(t, dc)
	compSavePNG(t, dc, "D06_ImageTextClipBackdrop")
	stats := dc.RenderPathStats()
	if stats.GPUOps <= base {
		t.Fatalf("D06 backdrop should add GPUOps base=%d now=%s", base, stats.LogLine())
	}
	// Card center white
	r, g, b, _ := p1Sample(dc, 180, 120)
	if r < 200 || g < 200 || b < 200 {
		t.Fatalf("D06 modal card missing rgba=%d,%d,%d", r, g, b)
	}
	// Dimmed board corner
	r2, g2, b2, _ := p1Sample(dc, 12, 12)
	if r2 > 180 && g2 > 180 && b2 > 180 {
		t.Fatalf("D06 expected dimmed backdrop corner rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D07: Translate/Scale × ClipRect × fill.
func TestP1_Comp_D07_TransformClipFill(t *testing.T) {
	p1RequireGPU(t)
	dc := compRunScene(t, "D07_TransformClipFill")
	defer dc.Close()
	compSavePNG(t, dc, "D07_TransformClipFill")

	r, g, b, _ := p1Sample(dc, 80, 50)
	if r < 120 {
		t.Fatalf("D07 transformed red missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 280, 20)
	if r2 < 200 {
		t.Fatalf("D07 expected clean corner rgba=%d,%d,%d", r2, g2, b2)
	}
	r3, g3, b3, _ := p1Sample(dc, 60, 150)
	if g3 < 80 {
		t.Fatalf("D07 green strip missing rgba=%d,%d,%d", r3, g3, b3)
	}
}

// D08: multi-region redraw — full base then two dirty regions re-inked correctly.
func TestP1_Comp_D08_MultiRegionRedraw(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 320, 200
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// Stable chrome
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawRectangle(0, 0, w, 36)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("multi-region redraw", 12, 24)

	// Two content panels
	dc.SetRGB(0.88, 0.90, 0.93)
	dc.DrawRoundedRectangle(16, 52, 130, 120, 8)
	_ = dc.Fill()
	dc.DrawRoundedRectangle(174, 52, 130, 120, 8)
	_ = dc.Fill()
	dc.SetRGB(0.2, 0.22, 0.26)
	dc.DrawString("A", 70, 120)
	dc.DrawString("B", 230, 120)

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("base flush: %v", err)
	}
	baseOps := dc.RenderPathStats().GPUOps

	// Dirty region A: recolor left panel
	dc.ClipRect(16, 52, 130, 120)
	dc.SetRGB(0.20, 0.45, 0.90)
	dc.DrawRectangle(16, 52, 130, 120)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("A*", 66, 120)
	dc.ResetClip()

	// Dirty region B: recolor right panel
	dc.ClipRect(174, 52, 130, 120)
	dc.SetRGB(0.15, 0.65, 0.40)
	dc.DrawRectangle(174, 52, 130, 120)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("B*", 226, 120)
	dc.ResetClip()

	p1Flush(t, dc)
	compAutoSavePNG(t, dc)
	stats := dc.RenderPathStats()
	if stats.GPUOps <= baseOps {
		t.Fatalf("D08 dirty redraw should increase GPUOps base=%d now=%s", baseOps, stats.LogLine())
	}

	// Left blue
	r, g, b, _ := p1Sample(dc, 80, 110)
	if b < 100 || r > b {
		t.Fatalf("D08 left dirty region not blue rgba=%d,%d,%d", r, g, b)
	}
	// Right green
	r2, g2, b2, _ := p1Sample(dc, 240, 110)
	if g2 < 100 || g2 < r2 {
		t.Fatalf("D08 right dirty region not green rgba=%d,%d,%d", r2, g2, b2)
	}
	// Header unchanged dark
	r3, g3, b3, _ := p1Sample(dc, 20, 18)
	if r3 > 80 || g3 > 80 {
		t.Fatalf("D08 header should stay dark rgba=%d,%d,%d", r3, g3, b3)
	}
	// Between panels: still white-ish page
	r4, g4, b4, _ := p1Sample(dc, 160, 110)
	if r4 < 200 {
		t.Fatalf("D08 gap between regions polluted rgba=%d,%d,%d", r4, g4, b4)
	}
}

// TestP1_Comp_DimensionAxes_DumpPNG renders a one-shot board of the Phase A
// dimension axes (docs/P1_COMPOSITION_MATRIX.md) and writes:
//
//	tmp/comp/dimension_axes.png
func TestP1_Comp_DimensionAxes_DumpPNG(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 720, 420
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 14)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRectangle(0, 0, w, 44)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("Phase A dimension axes — composition matrix", 16, 28)

	axes := []struct {
		title string
		draw  func()
	}{
		{"clip", func() {
			dc.ClipRoundRect(0, 0, 140, 90, 12)
			dc.SetRGB(0.25, 0.55, 0.9)
			dc.DrawRectangle(0, 0, 200, 120)
			_ = dc.Fill()
			dc.ResetClip()
		}},
		{"layer", func() {
			dc.SetRGB(0.3, 0.35, 0.45)
			dc.DrawRoundedRectangle(10, 10, 100, 60, 8)
			_ = dc.Fill()
			dc.PushLayer(render.BlendNormal, 0.55)
			dc.SetRGB(0.95, 0.75, 0.2)
			dc.DrawRoundedRectangle(30, 25, 100, 55, 8)
			_ = dc.Fill()
			dc.PopLayer()
		}},
		{"blend", func() {
			dc.SetRGB(0.2, 0.6, 0.9)
			dc.DrawCircle(50, 45, 30)
			_ = dc.Fill()
			dc.SetBlendMode(render.BlendMultiply)
			dc.SetRGB(0.95, 0.4, 0.3)
			dc.DrawCircle(75, 45, 30)
			_ = dc.Fill()
			dc.SetBlendMode(render.BlendNormal)
		}},
		{"text", func() {
			_ = dc.LoadFontFace(font, 16)
			dc.SetRGB(0.15, 0.16, 0.2)
			dc.DrawString("Aa 文字", 16, 50)
			_ = dc.LoadFontFace(font, 12)
		}},
		{"image", func() {
			img := compMakeImage(t, 48, 48, 40, 160, 220)
			dc.DrawImage(img, 20, 20)
			dc.DrawImage(img, 55, 35)
		}},
		{"transform", func() {
			dc.Push()
			dc.Translate(70, 50)
			dc.Rotate(0.35)
			dc.SetRGB(0.9, 0.35, 0.4)
			dc.DrawRoundedRectangle(-40, -20, 80, 40, 6)
			_ = dc.Fill()
			dc.Pop()
		}},
		{"HiDPI", func() {
			dc.SetRGB(0.2, 0.25, 0.35)
			dc.SetLineWidth(0)
			dc.DrawLine(10, 20, 130, 20)
			_ = dc.Stroke()
			dc.DrawLine(10, 40, 130, 40)
			_ = dc.Stroke()
			dc.SetRGB(0.9, 0.92, 0.95)
			dc.DrawString("scale/hairline", 16, 70)
		}},
		{"damage", func() {
			dc.SetRGB(0.85, 0.87, 0.9)
			dc.DrawRoundedRectangle(8, 8, 124, 74, 8)
			_ = dc.Fill()
			dc.ClipRect(20, 20, 50, 40)
			dc.SetRGB(0.25, 0.55, 0.95)
			dc.DrawRectangle(0, 0, 200, 120)
			_ = dc.Fill()
			dc.ResetClip()
			dc.ClipRect(70, 35, 50, 40)
			dc.SetRGB(0.3, 0.75, 0.45)
			dc.DrawRectangle(0, 0, 200, 120)
			_ = dc.Fill()
			dc.ResetClip()
		}},
	}

	for i, ax := range axes {
		col, row := i%4, i/4
		x, y := 20+float64(col)*175, 60+float64(row)*170
		// card
		dc.SetRGB(0.98, 0.98, 1)
		dc.DrawRoundedRectangle(x, y, 160, 150, 12)
		_ = dc.Fill()
		dc.SetRGB(0.2, 0.22, 0.28)
		dc.DrawString(ax.title, x+12, y+22)
		// content region
		dc.Push()
		dc.Translate(x+10, y+36)
		ax.draw()
		dc.Pop()
	}

	p1Flush(t, dc)
	compSavePNG(t, dc, "dimension_axes")
	r, g, b, _ := p1Sample(dc, 40, 20)
	if r > 80 && g > 80 && b > 80 {
		// header dark expected
		r, g, b, _ = p1Sample(dc, 8, 8)
	}
	if r > 100 && g > 100 && b > 100 {
		t.Fatalf("dimension axes header missing rgba=%d,%d,%d", r, g, b)
	}
}
