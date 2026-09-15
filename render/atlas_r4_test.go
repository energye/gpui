package render_test

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/render/internal/testutil/imagediff"
)

// R4 S30: game_sprite--case=rot 窗随 P2 建，S30 只留离屏对比。
// 本文件只读 render/testdata/atlas_r4_cases.json，不硬编码标准色。

type atlasQuadDef struct {
	Name string `json:"name"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
	W    int    `json:"w"`
	H    int    `json:"h"`
	RGBA [4]int `json:"rgba"`
}

type atlasSpriteDef struct {
	Src     [4]float64  `json:"src"`
	Dst     [4]float64  `json:"dst"`
	Opacity float64     `json:"opacity"`
	Rot     float64     `json:"rot"`
	FlipX   bool        `json:"flipx"`
	FlipY   bool        `json:"flipy"`
	Pivot   *[2]float64 `json:"pivot"`
	Tint    *[4]float64 `json:"tint"`
	Filter  string      `json:"filter"`
}

type atlasProbeDef struct {
	X    int      `json:"x"`
	Y    int      `json:"y"`
	Want [4]uint8 `json:"want"`
	Tol  uint8    `json:"tol"`
	Desc string   `json:"desc"`
}

type atlasCaseDef struct {
	Note    string           `json:"note"`
	Sprites []atlasSpriteDef `json:"sprites"`
	Probes  []atlasProbeDef  `json:"probes"`
}

type atlasR4Cases struct {
	Canvas [2]int `json:"canvas"`
	Atlas  struct {
		W     int            `json:"w"`
		H     int            `json:"h"`
		Quads []atlasQuadDef `json:"quads"`
	} `json:"atlas"`
	DegradedReason string `json:"degraded_reason"`
	Parity         struct {
		MaxChangedPct float64 `json:"max_changed_pct"`
		MaxMeanAbs    float64 `json:"max_mean_abs"`
		Note          string  `json:"note"`
	} `json:"parity"`
	WindowIntent string                  `json:"window_intent"`
	Cases        map[string]atlasCaseDef `json:"cases"`
}

func loadAtlasR4Cases(t *testing.T) atlasR4Cases {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "atlas_r4_cases.json"))
	if err != nil {
		t.Fatalf("read testdata/atlas_r4_cases.json: %v", err)
	}
	var c atlasR4Cases
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode atlas_r4_cases.json: %v", err)
	}
	if len(c.Cases) == 0 || len(c.Atlas.Quads) == 0 {
		t.Fatal("atlas_r4_cases.json missing cases or quads")
	}
	if c.DegradedReason != "verts:DrawAtlas" {
		t.Fatalf("degraded_reason = %q, want frozen verts:DrawAtlas", c.DegradedReason)
	}
	return c
}

func atlasR4Filter(s string) render.InterpolationMode {
	switch s {
	case "nearest":
		return render.InterpNearest
	case "bicubic":
		return render.InterpBicubic
	case "bilinear":
		return render.InterpBilinear
	default:
		return render.InterpolationMode(0)
	}
}

func toAtlasSprites(defs []atlasSpriteDef) []render.AtlasSprite {
	out := make([]render.AtlasSprite, len(defs))
	for i, d := range defs {
		sp := render.AtlasSprite{
			SrcX: d.Src[0], SrcY: d.Src[1], SrcW: d.Src[2], SrcH: d.Src[3],
			DstX: d.Dst[0], DstY: d.Dst[1], DstW: d.Dst[2], DstH: d.Dst[3],
			Opacity: d.Opacity, Rot: d.Rot, FlipX: d.FlipX, FlipY: d.FlipY,
			Filter: atlasR4Filter(d.Filter),
		}
		if d.Pivot != nil {
			sp.PivotX, sp.PivotY = d.Pivot[0], d.Pivot[1]
		}
		if d.Tint != nil {
			sp.Tint = render.RGBA{R: d.Tint[0], G: d.Tint[1], B: d.Tint[2], A: d.Tint[3]}
		}
		out[i] = sp
	}
	return out
}

func buildAtlasR4Image(t *testing.T, c atlasR4Cases) *render.ImageBuf {
	t.Helper()
	img, err := render.NewImageBuf(c.Atlas.W, c.Atlas.H, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf atlas: %v", err)
	}
	for _, q := range c.Atlas.Quads {
		for y := q.Y; y < q.Y+q.H; y++ {
			for x := q.X; x < q.X+q.W; x++ {
				if err := img.SetRGBA(x, y, uint8(q.RGBA[0]), uint8(q.RGBA[1]), uint8(q.RGBA[2]), uint8(q.RGBA[3])); err != nil {
					t.Fatalf("SetRGBA atlas %d,%d: %v", x, y, err)
				}
			}
		}
	}
	return img
}

func atlasSample(t *testing.T, dc *render.Context, x, y int) [4]uint8 {
	t.Helper()
	img := dc.Image()
	b := img.Bounds()
	if x < b.Min.X || x >= b.Max.X || y < b.Min.Y || y >= b.Max.Y {
		t.Fatalf("probe %d,%d outside bounds %v", x, y, b)
	}
	r, g, b2, a := img.At(x, y).RGBA()
	return [4]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b2 >> 8), uint8(a >> 8)}
}

func atlasClose(got, want [4]uint8, tol uint8) bool {
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

func atlasCheckProbes(t *testing.T, dc *render.Context, probes []atlasProbeDef) {
	t.Helper()
	for i, p := range probes {
		got := atlasSample(t, dc, p.X, p.Y)
		if !atlasClose(got, p.Want, p.Tol) {
			t.Errorf("probe[%d] %s (%d,%d) = %v, want %v tol %d", i, p.Desc, p.X, p.Y, got, p.Want, p.Tol)
		}
	}
}

func withAtlasCPU(t *testing.T) {
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

func withAtlasGPU(t *testing.T) {
	t.Helper()
	orig, had := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Unsetenv("GOGPU_RENDER_MODE")
	t.Cleanup(func() {
		if had {
			_ = os.Setenv("GOGPU_RENDER_MODE", orig)
		} else {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		}
	})
}

func newAtlasWhite(w, h int) *render.Context {
	dc := render.NewContext(w, h)
	dc.ClearWithColor(render.White)
	return dc
}

func drawAtlasR4CPU(t *testing.T, dc *render.Context, img *render.ImageBuf, def atlasCaseDef) render.AtlasDrawResult {
	t.Helper()
	res, err := dc.DrawAtlasEx(img, toAtlasSprites(def.Sprites), render.AtlasDrawOptions{})
	if err != nil {
		t.Fatalf("DrawAtlasEx: %v", err)
	}
	return res
}

// A: 老路零值即老味，新 Ex 身份 CPU 真值必过；老与新 GPU 逐位一致需真卡，无卡跳过。
func TestAtlasR4IdentityFromCases(t *testing.T) {
	c := loadAtlasR4Cases(t)
	def := c.Cases["identity"]
	img := buildAtlasR4Image(t, c)
	w, h := c.Canvas[0], c.Canvas[1]

	// CPU 真值恒过（新 Ex 尊重 GOGPU_RENDER_MODE=cpu）。
	withAtlasCPU(t)
	dcCPU := newAtlasWhite(w, h)
	resCPU, err := dcCPU.DrawAtlasEx(img, toAtlasSprites(def.Sprites), render.AtlasDrawOptions{})
	if err != nil {
		dcCPU.Close()
		t.Fatalf("DrawAtlasEx identity CPU: %v", err)
	}
	if resCPU.Skipped || resCPU.Drawn != len(def.Sprites) {
		t.Errorf("identity CPU Ex = %+v, want Drawn=%d", resCPU, len(def.Sprites))
	}
	if !resCPU.Degraded || resCPU.Reason != c.DegradedReason {
		t.Errorf("identity CPU Ex = %+v, want Degraded with %q", resCPU, c.DegradedReason)
	}
	atlasCheckProbes(t, dcCPU, def.Probes)
	dcCPU.Close()

	// GPU 老与新逐位一致：需真卡冲刷成功，无卡记一笔回 PASS（C 由 parity 另行跳过）。
	if render.Accelerator() == nil {
		t.Logf("no GPU accelerator, old-vs-new GPU road deferred to native-GPU machine")
		return
	}
	probe := render.NewContext(8, 8)
	probe.ClearWithColor(render.White)
	probe.SetRGB(1, 0, 0)
	probe.DrawRectangle(0, 0, 8, 8)
	_ = probe.Fill()
	if err := probe.FlushGPU(); err != nil {
		probe.Close()
		t.Logf("GPU flush unavailable, old-vs-new GPU road deferred: %v", err)
		return
	}
	isGPU := probe.RenderPathStats().GPUOps > 0
	probe.Close()
	if !isGPU {
		t.Logf("no GPU ops on probe, old-vs-new GPU road deferred")
		return
	}
	withAtlasGPU(t)
	dcOld := newAtlasWhite(w, h)
	dcOld.DrawAtlas(img, toAtlasSprites(def.Sprites))
	if err := dcOld.FlushGPU(); err != nil {
		dcOld.Close()
		if errors.Is(err, render.ErrFallbackToCPU) {
			t.Logf("flush fell back to CPU (no native GPU in this env), old-vs-new GPU deferred")
			return
		}
		t.Fatalf("old FlushGPU: %v", err)
	}
	if dcOld.RenderPathStats().GPUOps == 0 {
		dcOld.Close()
		t.Logf("no GPUOps on old road (no native GPU in this env), old-vs-new GPU deferred")
		return
	}
	atlasCheckProbes(t, dcOld, def.Probes)
	oldImg := dcOld.Image()
	dcOld.Close()

	dcNew := newAtlasWhite(w, h)
	res, err := dcNew.DrawAtlasEx(img, toAtlasSprites(def.Sprites), render.AtlasDrawOptions{})
	if err != nil {
		dcNew.Close()
		t.Fatalf("DrawAtlasEx identity: %v", err)
	}
	if res.Skipped || res.Drawn != len(def.Sprites) {
		dcNew.Close()
		t.Errorf("identity Ex = %+v, want Drawn=%d", res, len(def.Sprites))
	}
	if err := dcNew.FlushGPU(); err != nil {
		dcNew.Close()
		if errors.Is(err, render.ErrFallbackToCPU) {
			t.Logf("flush fell back to CPU (no native GPU in this env), old-vs-new GPU deferred")
			return
		}
		t.Fatalf("new FlushGPU: %v", err)
	}
	atlasCheckProbes(t, dcNew, def.Probes)
	st, err := imagediff.Images(oldImg, dcNew.Image())
	dcNew.Close()
	if err != nil {
		t.Fatalf("diff old vs new identity: %v", err)
	}
	if st.ChangedPixels != 0 || st.MaxDelta != 0 {
		t.Fatalf("old vs new identity must be bitwise identical, changed=%d max=%d", st.ChangedPixels, st.MaxDelta)
	}
	t.Logf("atlas-r4-oldroad: old vs new identity bitwise identical, drawn=%d", res.Drawn)
}

// A: 旋转翻转轴心数对（CPU 真值）。
func TestAtlasR4RotFlipFromCases(t *testing.T) {
	withAtlasCPU(t)
	c := loadAtlasR4Cases(t)
	img := buildAtlasR4Image(t, c)
	w, h := c.Canvas[0], c.Canvas[1]
	for _, name := range []string{"rot90_center", "rot180_center", "rot180_origin", "flipx_center", "flipy_center"} {
		def, ok := c.Cases[name]
		if !ok {
			t.Fatalf("cases has no %q", name)
		}
		dc := newAtlasWhite(w, h)
		res := drawAtlasR4CPU(t, dc, img, def)
		if res.Skipped || res.Drawn != len(def.Sprites) {
			t.Errorf("%s Ex = %+v, want Drawn=%d", name, res, len(def.Sprites))
		}
		atlasCheckProbes(t, dc, def.Probes)
		dc.Close()
	}
}

// A: 染色与单图过滤数对；染色在默认环境也强制 CPU（Degraded）。
func TestAtlasR4TintFilterFromCases(t *testing.T) {
	withAtlasCPU(t)
	c := loadAtlasR4Cases(t)
	img := buildAtlasR4Image(t, c)
	w, h := c.Canvas[0], c.Canvas[1]
	for _, name := range []string{"tint_gray", "filter_nearest_scaled", "filter_bilinear_scaled", "filter_bicubic_identity"} {
		def, ok := c.Cases[name]
		if !ok {
			t.Fatalf("cases has no %q", name)
		}
		dc := newAtlasWhite(w, h)
		res := drawAtlasR4CPU(t, dc, img, def)
		if res.Skipped || res.Drawn != len(def.Sprites) {
			t.Errorf("%s Ex = %+v, want Drawn=%d", name, res, len(def.Sprites))
		}
		if !res.Degraded || res.Reason != c.DegradedReason {
			t.Errorf("%s CPU Ex = %+v, want Degraded with %q", name, res, c.DegradedReason)
		}
		atlasCheckProbes(t, dc, def.Probes)
		dc.Close()
	}
	// 染色无着色器：有卡也回 CPU，整批同真值。
	withAtlasGPU(t)
	dc := newAtlasWhite(w, h)
	defer dc.Close()
	res, err := dc.DrawAtlasEx(img, toAtlasSprites(c.Cases["tint_gray"].Sprites), render.AtlasDrawOptions{})
	if err != nil {
		t.Fatalf("tint GPU-env Ex: %v", err)
	}
	if !res.Degraded || res.Reason != c.DegradedReason {
		t.Errorf("tint GPU-env Ex = %+v, want Degraded with %q", res, c.DegradedReason)
	}
	if err := dc.FlushGPU(); err != nil && !errors.Is(err, render.ErrFallbackToCPU) {
		t.Fatalf("tint FlushGPU: %v", err)
	}
	atlasCheckProbes(t, dc, c.Cases["tint_gray"].Probes)
}

// B: 空零坏数不崩，哨兵错分得清。
func TestAtlasR4EdgesNoCrash(t *testing.T) {
	withAtlasCPU(t)
	c := loadAtlasR4Cases(t)
	img := buildAtlasR4Image(t, c)
	dc := newAtlasWhite(32, 32)
	defer dc.Close()
	if res, err := dc.DrawAtlasEx(nil, toAtlasSprites(c.Cases["identity"].Sprites), render.AtlasDrawOptions{}); err != nil || !res.Skipped {
		t.Errorf("nil img Ex = %+v err=%v, want Skipped nil err", res, err)
	}
	if res, err := dc.DrawAtlasEx(img, nil, render.AtlasDrawOptions{}); err != nil || !res.Skipped {
		t.Errorf("nil sprites Ex = %+v err=%v, want Skipped nil err", res, err)
	}
	// 坏块跳过不崩：零源零目标全跳过。
	bad := []render.AtlasSprite{
		{SrcX: 0, SrcY: 0, SrcW: 0, SrcH: 8, DstX: 4, DstY: 4, DstW: 8, DstH: 8, Opacity: 1},
		{SrcX: 0, SrcY: 0, SrcW: 8, SrcH: 8, DstX: 4, DstY: 4, DstW: 0, DstH: 8, Opacity: 1},
	}
	if res, err := dc.DrawAtlasEx(img, bad, render.AtlasDrawOptions{}); err != nil || !res.Skipped {
		t.Errorf("bad rects Ex = %+v err=%v, want Skipped nil err", res, err)
	}
	if got := atlasSample(t, dc, 16, 16); got != [4]uint8{255, 255, 255, 255} {
		t.Errorf("bad rects polluted canvas: %v", got)
	}
	// 非有限数显式错。
	nanSp := []render.AtlasSprite{{SrcX: math.NaN(), SrcY: 0, SrcW: 8, SrcH: 8, DstX: 4, DstY: 4, DstW: 8, DstH: 8, Opacity: 1}}
	if _, err := dc.DrawAtlasEx(img, nanSp, render.AtlasDrawOptions{}); !errors.Is(err, render.ErrAtlasNonFinite) {
		t.Errorf("nan err = %v, want ErrAtlasNonFinite", err)
	}
	infTint := []render.AtlasSprite{{SrcX: 0, SrcY: 0, SrcW: 8, SrcH: 8, DstX: 4, DstY: 4, DstW: 8, DstH: 8, Opacity: 1, Tint: render.RGBA{R: math.Inf(1)}}}
	if _, err := dc.DrawAtlasEx(img, infTint, render.AtlasDrawOptions{}); !errors.Is(err, render.ErrAtlasNonFinite) {
		t.Errorf("inf tint err = %v, want ErrAtlasNonFinite", err)
	}
	// 未知过滤显式错。
	badFilt := []render.AtlasSprite{{SrcX: 0, SrcY: 0, SrcW: 8, SrcH: 8, DstX: 4, DstY: 4, DstW: 8, DstH: 8, Opacity: 1, Filter: render.InterpolationMode(99)}}
	if _, err := dc.DrawAtlasEx(img, badFilt, render.AtlasDrawOptions{}); !errors.Is(err, render.ErrAtlasUnsupportedFilter) {
		t.Errorf("bad filter err = %v, want ErrAtlasUnsupportedFilter", err)
	}
	// 老签名坏路仍不崩。
	dc.DrawAtlas(nil, toAtlasSprites(c.Cases["identity"].Sprites))
	dc.DrawAtlas(img, nil)
	dc.DrawAtlas(img, bad)
}

// D: 百块批量耗时有数。
func TestAtlasR4PerfBatch(t *testing.T) {
	withAtlasCPU(t)
	c := loadAtlasR4Cases(t)
	img := buildAtlasR4Image(t, c)
	base := toAtlasSprites(c.Cases["identity"].Sprites)
	batch := make([]render.AtlasSprite, 0, 100)
	for i := 0; i < 100; i++ {
		sp := base[0]
		sp.DstX = float64(8 + (i%10)*2)
		sp.DstY = float64(8 + (i/10)*2)
		sp.DstW, sp.DstH = 2, 2
		batch = append(batch, sp)
	}
	dc := newAtlasWhite(64, 64)
	defer dc.Close()
	const n = 50
	start := time.Now()
	drawn := 0
	for i := 0; i < n; i++ {
		dc.ClearWithColor(render.White)
		res, err := dc.DrawAtlasEx(img, batch, render.AtlasDrawOptions{})
		if err != nil {
			t.Fatalf("batch Ex: %v", err)
		}
		drawn = res.Drawn
	}
	el := time.Since(start)
	t.Logf("atlas-r4-perf: %d batches x100 sprites in %v (%.2f ms/batch drawn=%d)", n, el, float64(el.Milliseconds())/float64(n), drawn)
	if drawn != 100 {
		t.Errorf("batch drawn = %d, want 100", drawn)
	}
}

// E: 交替长跑不漂不漏。
func TestAtlasR4LongRunStable(t *testing.T) {
	withAtlasCPU(t)
	c := loadAtlasR4Cases(t)
	img := buildAtlasR4Image(t, c)
	w, h := c.Canvas[0], c.Canvas[1]
	idSp := toAtlasSprites(c.Cases["identity"].Sprites)
	rotSp := toAtlasSprites(c.Cases["rot90_center"].Sprites)
	dc := newAtlasWhite(w, h)
	defer dc.Close()
	const n = 500
	for i := 0; i < n; i++ {
		dc.ClearWithColor(render.White)
		sp := idSp
		if i%2 == 1 {
			sp = rotSp
		}
		if _, err := dc.DrawAtlasEx(img, sp, render.AtlasDrawOptions{}); err != nil {
			t.Fatalf("rep %d: %v", i, err)
		}
	}
	// 末次奇数即 rot90：TR 该红。
	got := atlasSample(t, dc, 20, 12)
	if !atlasClose(got, [4]uint8{255, 0, 0, 255}, 1) {
		t.Fatalf("longrun final probe = %v, want red tol 1", got)
	}
	t.Logf("atlas-r4-longrun: %d alternating draws ok, final TR %v", n, got)
}

// C: 显卡与 CPU 只差采样舍入（需真卡，无卡跳过不假绿）。
func TestAtlasR4CPUGPUParity(t *testing.T) {
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
	c := loadAtlasR4Cases(t)
	img := buildAtlasR4Image(t, c)
	w, h := c.Canvas[0], c.Canvas[1]
	for _, name := range []string{"identity", "rot90_center"} {
		def := c.Cases[name]
		withAtlasCPU(t)
		dcCPU := newAtlasWhite(w, h)
		if _, err := dcCPU.DrawAtlasEx(img, toAtlasSprites(def.Sprites), render.AtlasDrawOptions{}); err != nil {
			dcCPU.Close()
			t.Fatalf("%s CPU Ex: %v", name, err)
		}
		cpuImg := dcCPU.Image()
		dcCPU.Close()
		withAtlasGPU(t)
		dcGPU := newAtlasWhite(w, h)
		if _, err := dcGPU.DrawAtlasEx(img, toAtlasSprites(def.Sprites), render.AtlasDrawOptions{}); err != nil {
			dcGPU.Close()
			t.Fatalf("%s GPU Ex: %v", name, err)
		}
		if err := dcGPU.FlushGPU(); err != nil {
			dcGPU.Close()
			if errors.Is(err, render.ErrFallbackToCPU) {
				t.Skipf("flush fell back to CPU (no native GPU in this env): %v", err)
			}
			t.Fatalf("%s flush: %v", name, err)
		}
		if dcGPU.RenderPathStats().GPUOps == 0 {
			dcGPU.Close()
			t.Skipf("no GPUOps for %s (no native GPU in this env): %s", name, dcGPU.RenderPathStats().LogLine())
		}
		gpuImg := dcGPU.Image()
		dcGPU.Close()
		st, err := imagediff.Images(cpuImg, gpuImg)
		if err != nil {
			t.Fatalf("%s diff: %v", name, err)
		}
		changedPct := 100 * float64(st.ChangedPixels) / float64(st.TotalPixels)
		t.Logf("%s parity changed=%d/%d (%.3f%%) mean=%.3f rmse=%.3f max=%d", name, st.ChangedPixels, st.TotalPixels, changedPct, st.MeanAbs, st.RMSE, st.MaxDelta)
		if changedPct > c.Parity.MaxChangedPct {
			t.Fatalf("%s parity changed %.3f%% > %.1f%%", name, changedPct, c.Parity.MaxChangedPct)
		}
		if st.MeanAbs > c.Parity.MaxMeanAbs {
			t.Fatalf("%s parity mean %.3f > %.1f", name, st.MeanAbs, c.Parity.MaxMeanAbs)
		}
	}
}

// F: 离屏金比对即未来 game_sprite--case=rot 窗的依据（窗随 P2 建）。
func TestAtlasR4OffscreenGolden(t *testing.T) {
	withAtlasCPU(t)
	c := loadAtlasR4Cases(t)
	t.Logf("window-intent: %s", c.WindowIntent)
	img := buildAtlasR4Image(t, c)
	w, h := c.Canvas[0], c.Canvas[1]
	names := make([]string, 0, len(c.Cases))
	for name := range c.Cases {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		def := c.Cases[name]
		dc := newAtlasWhite(w, h)
		res := drawAtlasR4CPU(t, dc, img, def)
		if res.Skipped || res.Drawn != len(def.Sprites) {
			t.Errorf("%s Ex = %+v, want Drawn=%d", name, res, len(def.Sprites))
		}
		atlasCheckProbes(t, dc, def.Probes)
		dc.Close()
	}
}
