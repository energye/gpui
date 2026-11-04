package render_test

import (
	"encoding/json"
	"errors"
	"image"
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

// 本文件只读 render/testdata/depth_r5_cases.json，不硬编码标准色。
//
// P3.7 批 2 步 2c：render 层不再排序（SortDepthSprites/IsDepthSorted/SetDepth/
// DepthTest 全删，排序判据只在 engine/camera 一份）。这里剩下的职责是「按提交序画」
// 这条渲染契约：提交序 = 盖序，坏值 fail-closed，降级标记照旧。

type depthSpriteDef struct {
	Name    string     `json:"name"`
	Src     [4]float64 `json:"src"`
	Dst     [4]float64 `json:"dst"`
	Opacity float64    `json:"opacity"`
	Depth   float64    `json:"depth"`
}

type depthProbeDef struct {
	X    int      `json:"x"`
	Y    int      `json:"y"`
	Want [4]uint8 `json:"want"`
	Tol  uint8    `json:"tol"`
	Desc string   `json:"desc"`
}

type depthCaseDef struct {
	Note    string           `json:"note"`
	Sprites []depthSpriteDef `json:"sprites"`
	Probes  []depthProbeDef  `json:"probes"`
}

type depthR5Cases struct {
	Canvas [2]int `json:"canvas"`
	Atlas  struct {
		W     int `json:"w"`
		H     int `json:"h"`
		Quads []struct {
			Name string `json:"name"`
			X    int    `json:"x"`
			Y    int    `json:"y"`
			W    int    `json:"w"`
			H    int    `json:"h"`
			RGBA [4]int `json:"rgba"`
		} `json:"quads"`
	} `json:"atlas"`
	DepthNote string `json:"depth_note"`
	Parity    struct {
		MaxChangedPct float64 `json:"max_changed_pct"`
		MaxMeanAbs    float64 `json:"max_mean_abs"`
		Note          string  `json:"note"`
	} `json:"parity"`
	WindowIntent string                  `json:"window_intent"`
	Cases        map[string]depthCaseDef `json:"cases"`
}

func loadDepthR5Cases(t *testing.T) depthR5Cases {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "depth_r5_cases.json"))
	if err != nil {
		t.Fatalf("read testdata/depth_r5_cases.json: %v", err)
	}
	var c depthR5Cases
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode depth_r5_cases.json: %v", err)
	}
	if len(c.Cases) == 0 || len(c.Atlas.Quads) == 0 {
		t.Fatal("depth_r5_cases.json missing cases or quads")
	}
	if c.DepthNote == "" {
		t.Fatal("depth_r5_cases.json missing depth_note freeze")
	}
	return c
}

func toDepthSprites(defs []depthSpriteDef) []render.DepthSprite {
	out := make([]render.DepthSprite, len(defs))
	for i, d := range defs {
		out[i] = render.DepthSprite{
			Sprite: render.AtlasSprite{
				SrcX: d.Src[0], SrcY: d.Src[1], SrcW: d.Src[2], SrcH: d.Src[3],
				DstX: d.Dst[0], DstY: d.Dst[1], DstW: d.Dst[2], DstH: d.Dst[3],
				Opacity: d.Opacity,
			},
			Depth: d.Depth,
			Name:  d.Name,
		}
	}
	return out
}

func buildDepthR5Image(t *testing.T, c depthR5Cases) *render.ImageBuf {
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

func depthSample(t *testing.T, dc *render.Context, x, y int) [4]uint8 {
	t.Helper()
	img := dc.Image()
	b := img.Bounds()
	if x < b.Min.X || x >= b.Max.X || y < b.Min.Y || y >= b.Max.Y {
		t.Fatalf("probe %d,%d outside bounds %v", x, y, b)
	}
	r, g, b2, a := img.At(x, y).RGBA()
	return [4]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b2 >> 8), uint8(a >> 8)}
}

func depthClose(got, want [4]uint8, tol uint8) bool {
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

func depthCheckProbes(t *testing.T, dc *render.Context, probes []depthProbeDef) {
	t.Helper()
	for i, p := range probes {
		got := depthSample(t, dc, p.X, p.Y)
		if !depthClose(got, p.Want, p.Tol) {
			t.Errorf("probe[%d] %s (%d,%d) = %v, want %v tol %d", i, p.Desc, p.X, p.Y, got, p.Want, p.Tol)
		}
	}
}

func withDepthCPU(t *testing.T) {
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

func withDepthGPU(t *testing.T) {
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

func newDepthWhite(w, h int) *render.Context {
	dc := render.NewContext(w, h)
	dc.ClearWithColor(render.White)
	return dc
}

func drawDepthCPU(t *testing.T, dc *render.Context, img *render.ImageBuf, def depthCaseDef) render.DepthDrawResult {
	t.Helper()
	res, err := dc.DrawDepthSprites(img, toDepthSprites(def.Sprites))
	if err != nil {
		t.Fatalf("DrawDepthSprites %q: %v", def.Note, err)
	}
	return res
}

func paintDepthCase(c depthR5Cases, img *render.ImageBuf, name string) image.Image {
	def := c.Cases[name]
	dc := newDepthWhite(c.Canvas[0], c.Canvas[1])
	defer dc.Close()
	if _, err := dc.DrawDepthSprites(img, toDepthSprites(def.Sprites)); err != nil {
		panic(name + ": " + err.Error())
	}
	return dc.Image()
}

// A: 提交序 = 盖序。调用方排好远先近后 ⇒ 近盖远；反过来提交 ⇒ 远盖近。
func TestDepthR5FarFirstFromCases(t *testing.T) {
	withDepthCPU(t)
	c := loadDepthR5Cases(t)
	img := buildDepthR5Image(t, c)
	w, h := c.Canvas[0], c.Canvas[1]
	for _, name := range []string{"far_first", "submission_order", "same_depth_stable", "three_layers"} {
		def, ok := c.Cases[name]
		if !ok {
			t.Fatalf("cases has no %q", name)
		}
		dc := newDepthWhite(w, h)
		res := drawDepthCPU(t, dc, img, def)
		if res.Skipped || res.Drawn != len(def.Sprites) {
			t.Errorf("%s result = %+v, want Drawn=%d", name, res, len(def.Sprites))
		}
		depthCheckProbes(t, dc, def.Probes)
		dc.Close()
	}
	// 同一批精灵只换提交序，画面必须不同（render 若还偷偷自己排序，这两张会逐位相同）。
	st, err := imagediff.Images(paintDepthCase(c, img, "far_first"), paintDepthCase(c, img, "submission_order"))
	if err != nil {
		t.Fatalf("far_first vs submission_order diff: %v", err)
	}
	if st.ChangedPixels == 0 {
		t.Fatal("换了提交序画面却一模一样：render 还在自己排序？")
	}
}

// B: 坏值不崩，哨兵错分得清（排序哨兵随能力一起迁回 engine）。
func TestDepthR5EdgesNoCrash(t *testing.T) {
	withDepthCPU(t)
	c := loadDepthR5Cases(t)
	img := buildDepthR5Image(t, c)
	dc := newDepthWhite(32, 32)
	defer dc.Close()
	if res, err := dc.DrawDepthSprites(nil, toDepthSprites(c.Cases["far_first"].Sprites)); err != nil || !res.Skipped {
		t.Errorf("nil img = %+v err=%v, want Skipped nil err", res, err)
	}
	if res, err := dc.DrawDepthSprites(img, nil); err != nil || !res.Skipped {
		t.Errorf("nil sprites = %+v err=%v, want Skipped nil err", res, err)
	}
	bad := []render.DepthSprite{
		{Sprite: render.AtlasSprite{SrcX: 0, SrcY: 0, SrcW: 0, SrcH: 8, DstX: 4, DstY: 4, DstW: 8, DstH: 8, Opacity: 1}, Depth: 1, Name: "zero-src"},
		{Sprite: render.AtlasSprite{SrcX: 0, SrcY: 0, SrcW: 8, SrcH: 8, DstX: 4, DstY: 4, DstW: 0, DstH: 8, Opacity: 1}, Depth: 0, Name: "zero-dst"},
	}
	if res, err := dc.DrawDepthSprites(img, bad); err != nil || !res.Skipped {
		t.Errorf("zero rects = %+v err=%v, want Skipped nil err", res, err)
	}
	if got := depthSample(t, dc, 16, 16); got != [4]uint8{255, 255, 255, 255} {
		t.Errorf("zero rects polluted canvas: %v", got)
	}
	nanSp := []render.DepthSprite{
		{Sprite: render.AtlasSprite{SrcX: 0, SrcY: 0, SrcW: 8, SrcH: 8, DstX: 4, DstY: 4, DstW: 8, DstH: 8, Opacity: 1}, Depth: math.NaN()},
	}
	if _, err := dc.DrawDepthSprites(img, nanSp); !errors.Is(err, render.ErrDepthNonFinite) {
		t.Errorf("nan depth err = %v, want ErrDepthNonFinite", err)
	}
	infSp := []render.DepthSprite{
		{Sprite: render.AtlasSprite{SrcX: 0, SrcY: 0, SrcW: 8, SrcH: 8, DstX: 4, DstY: 4, DstW: 8, DstH: 8, Opacity: 1}, Depth: math.Inf(1)},
	}
	if _, err := dc.DrawDepthSprites(img, infSp); !errors.Is(err, render.ErrDepthNonFinite) {
		t.Errorf("inf depth err = %v, want ErrDepthNonFinite", err)
	}
	nanPx := []render.DepthSprite{
		{Sprite: render.AtlasSprite{SrcX: math.NaN(), SrcY: 0, SrcW: 8, SrcH: 8, DstX: 4, DstY: 4, DstW: 8, DstH: 8, Opacity: 1}, Depth: 1},
	}
	if _, err := dc.DrawDepthSprites(img, nanPx); !errors.Is(err, render.ErrDepthNonFinite) {
		t.Errorf("nan sprite err = %v, want ErrDepthNonFinite", err)
	}
	badFilt := []render.DepthSprite{
		{Sprite: render.AtlasSprite{SrcX: 0, SrcY: 0, SrcW: 8, SrcH: 8, DstX: 4, DstY: 4, DstW: 8, DstH: 8, Opacity: 1, Filter: render.InterpolationMode(99)}, Depth: 1},
	}
	if _, err := dc.DrawDepthSprites(img, badFilt); !errors.Is(err, render.ErrAtlasUnsupportedFilter) {
		t.Errorf("bad filter err = %v, want ErrAtlasUnsupportedFilter", err)
	}
	// 构造哨兵仍在（DepthSprite 数据格式随迁保留）。
	if _, err := render.NewDepthSprite("nan", render.AtlasSprite{
		SrcX: 0, SrcY: 0, SrcW: 8, SrcH: 8, DstX: 4, DstY: 4, DstW: 8, DstH: 8, Opacity: 1,
	}, math.NaN()); !errors.Is(err, render.ErrDepthNonFinite) {
		t.Errorf("NewDepthSprite nan err = %v, want ErrDepthNonFinite", err)
	}
}

// D: 百个遮挡帧率有数（这里模拟已排好序的调用方）。
func TestDepthR5PerfOcclusion(t *testing.T) {
	withDepthCPU(t)
	c := loadDepthR5Cases(t)
	img := buildDepthR5Image(t, c)
	base := toDepthSprites(c.Cases["far_first"].Sprites)
	batch := make([]render.DepthSprite, 0, 100)
	for i := 0; i < 100; i++ {
		sp := base[i%len(base)]
		cp := sp.Sprite
		cp.DstX = float64(4 + (i%10)*2)
		cp.DstY = float64(4 + (i/10)*2)
		cp.DstW, cp.DstH = 4, 4
		batch = append(batch, render.DepthSprite{Sprite: cp, Depth: float64(100 - i), Name: sp.Name})
	}
	if !sort.SliceIsSorted(batch, func(i, j int) bool { return batch[i].Depth > batch[j].Depth }) {
		t.Fatal("压测批必须先排成远先近后")
	}
	dc := newDepthWhite(64, 64)
	defer dc.Close()
	const n = 50
	start := time.Now()
	drawn := 0
	for i := 0; i < n; i++ {
		dc.ClearWithColor(render.White)
		res, err := dc.DrawDepthSprites(img, batch)
		if err != nil {
			t.Fatalf("batch: %v", err)
		}
		drawn = res.Drawn
	}
	el := time.Since(start)
	t.Logf("depth-r5-perf: %d batches x100 sprites in %v (%.2f ms/batch drawn=%d)", n, el, float64(el.Milliseconds())/float64(n), drawn)
	if drawn != 100 {
		t.Errorf("batch drawn = %d, want 100", drawn)
	}
}

// E: 长跑交替提交，最终盖序跟着最后一次提交序。
func TestDepthR5LongRunStable(t *testing.T) {
	withDepthCPU(t)
	c := loadDepthR5Cases(t)
	img := buildDepthR5Image(t, c)
	w, h := c.Canvas[0], c.Canvas[1]
	a := c.Cases["far_first"]
	b := c.Cases["submission_order"]
	dc := newDepthWhite(w, h)
	defer dc.Close()
	const n = 500
	last := a
	for i := 0; i < n; i++ {
		dc.ClearWithColor(render.White)
		last = a
		if i%2 == 1 {
			last = b
		}
		if _, err := dc.DrawDepthSprites(img, toDepthSprites(last.Sprites)); err != nil {
			t.Fatalf("rep %d: %v", i, err)
		}
	}
	depthCheckProbes(t, dc, last.Probes)
	t.Logf("depth-r5-longrun: %d alternating draws ok, last case probes hold", n)
}

// C: 显卡与 CPU 只差采样舍入（需真卡，无卡跳过不假绿）。
func TestDepthR5CPUGPUParity(t *testing.T) {
	if render.Accelerator() == nil {
		t.Skipf("no GPU accelerator, parity needs native GPU")
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
		t.Skipf("no GPU ops on probe, parity needs native GPU")
	}
	c := loadDepthR5Cases(t)
	img := buildDepthR5Image(t, c)
	w, h := c.Canvas[0], c.Canvas[1]
	for _, name := range []string{"far_first", "three_layers"} {
		def := c.Cases[name]
		withDepthCPU(t)
		dcCPU := newDepthWhite(w, h)
		if _, err := dcCPU.DrawDepthSprites(img, toDepthSprites(def.Sprites)); err != nil {
			dcCPU.Close()
			t.Fatalf("%s CPU: %v", name, err)
		}
		cpuImg := dcCPU.Image()
		dcCPU.Close()
		withDepthGPU(t)
		dcGPU := newDepthWhite(w, h)
		if _, err := dcGPU.DrawDepthSprites(img, toDepthSprites(def.Sprites)); err != nil {
			dcGPU.Close()
			t.Fatalf("%s GPU: %v", name, err)
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
		t.Logf("%s parity changed=%d/%d (%.3f%%) mean=%.3f rmse=%.3f max=%d gpu=%s", name, st.ChangedPixels, st.TotalPixels, changedPct, st.MeanAbs, st.RMSE, st.MaxDelta, "see env log")
		if changedPct > c.Parity.MaxChangedPct {
			t.Fatalf("%s parity changed %.3f%% > %.1f%%", name, changedPct, c.Parity.MaxChangedPct)
		}
		if st.MeanAbs > c.Parity.MaxMeanAbs {
			t.Fatalf("%s parity mean %.3f > %.1f", name, st.MeanAbs, c.Parity.MaxMeanAbs)
		}
	}
}

func TestDepthR5OffscreenGolden(t *testing.T) {
	withDepthCPU(t)
	c := loadDepthR5Cases(t)
	t.Logf("window-intent: %s", c.WindowIntent)
	img := buildDepthR5Image(t, c)
	w, h := c.Canvas[0], c.Canvas[1]
	names := make([]string, 0, len(c.Cases))
	for name := range c.Cases {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		def := c.Cases[name]
		dc := newDepthWhite(w, h)
		res, err := dc.DrawDepthSprites(img, toDepthSprites(def.Sprites))
		if err != nil {
			dc.Close()
			t.Fatalf("%s: %v", name, err)
		}
		if res.Skipped || res.Drawn != len(def.Sprites) {
			t.Errorf("%s result = %+v, want Drawn=%d", name, res, len(def.Sprites))
		}
		depthCheckProbes(t, dc, def.Probes)
		dc.Close()
	}
}
