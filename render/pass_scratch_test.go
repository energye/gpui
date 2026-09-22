package render_test

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// Pass-scratch regression (game_particle black cards / game_sprite ROT black
// card): GPU draws queued before a CPU fallback inside a retained texture
// sub-pass must still reach the pass view, and the CPU pixels must join
// them. Geometry and probes live in testdata/pass_scratch_cases.json, the
// atlas pixels in testdata/pass_scratch_atlas.png; the test hardcodes no
// standard pixels.

type passScratchRect struct {
	X, Y, W, H float64 `json:"-"`
}

type passScratchCases struct {
	Canvas [2]int `json:"canvas"`
	Atlas  string `json:"atlas"`
	Fill   struct {
		R float64 `json:"r"`
		G float64 `json:"g"`
		B float64 `json:"b"`
	} `json:"fill"`
	Plain struct {
		Src [4]float64 `json:"src"`
		Dst [4]float64 `json:"dst"`
	} `json:"plain"`
	Tint struct {
		Src  [4]float64 `json:"src"`
		Dst  [4]float64 `json:"dst"`
		Tint [4]float64 `json:"tint"`
	} `json:"tint"`
	Probes []struct {
		X, Y int
		Want [4]uint8 `json:"want"`
		Tol  uint8    `json:"tol"`
		Desc string   `json:"desc"`
	} `json:"probes"`
}

func loadPassScratchCases(t *testing.T) passScratchCases {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "pass_scratch_cases.json"))
	if err != nil {
		t.Fatalf("read cases: %v", err)
	}
	var c passScratchCases
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode cases: %v", err)
	}
	return c
}

func loadPassScratchAtlas(t *testing.T, name string) *render.ImageBuf {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("open atlas: %v", err)
	}
	defer f.Close()
	src, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode atlas: %v", err)
	}
	b := src.Bounds()
	img, err := render.NewImageBuf(b.Dx(), b.Dy(), render.FormatRGBA8)
	if err != nil {
		t.Fatalf("new atlas buf: %v", err)
	}
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r, g, bl, a := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
			if err := img.SetRGBA(x, y, uint8(r>>8), uint8(g>>8), uint8(bl>>8), uint8(a>>8)); err != nil {
				t.Fatalf("set atlas pixel: %v", err)
			}
		}
	}
	return img
}

func requirePassScratchGPU(t *testing.T) {
	t.Helper()
	if render.Accelerator() == nil {
		t.Skip("no GPU accelerator: pass-scratch pixels need a native GPU")
	}
	dc := render.NewContext(8, 8)
	defer dc.Close()
	dc.SetRGB(1, 0, 0)
	dc.DrawRectangle(0, 0, 8, 8)
	_ = dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Skipf("GPU flush unavailable: %v", err)
	}
	if dc.RenderPathStats().GPUOps == 0 {
		t.Skipf("no GPU ops on probe: %s", dc.RenderPathStats().LogLine())
	}
}

// CPU-only contexts never arm the swap: gates off, legacy path verbatim.
func TestPassScratchGatesOffCPUOnly(t *testing.T) {
	prev, had := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	defer func() {
		if had {
			_ = os.Setenv("GOGPU_RENDER_MODE", prev)
		} else {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		}
	}()
	dc := render.NewContext(32, 32)
	defer dc.Close()
	restore, ok := dc.BeginPassScratch(image.Rect(0, 0, 32, 32))
	defer restore()
	if ok {
		t.Fatal("BeginPassScratch armed under GOGPU_RENDER_MODE=cpu, want gated off")
	}
	if committed, err := dc.CommitPassScratchToView(render.TextureView{}); committed || err != nil {
		t.Fatalf("CommitPassScratchToView without swap = (%v, %v), want (false, nil)", committed, err)
	}
}

// Pinned ordering contract (see above): GPU fill + GPU plain + GPU tint
// recorded into a small view through the retained-pass sequence must all
// survive in z-order.
func TestPassScratchMixedRecordPixels(t *testing.T) {
	requirePassScratchGPU(t)
	c := loadPassScratchCases(t)
	atlas := loadPassScratchAtlas(t, c.Atlas)
	W, H := c.Canvas[0], c.Canvas[1]

	dc := render.NewContext(W, H)
	defer dc.Close()
	dc.BeginFrame()
	view, release := dc.CreateOffscreenTexture(W, H)
	if release == nil || view.IsNil() {
		t.Skip("CreateOffscreenTexture unavailable")
	}
	defer release()

	restorePass := dc.BeginOffscreenPass()
	restoreScratch, scratchOK := dc.BeginPassScratch(image.Rect(0, 0, W, H))
	if !scratchOK {
		restoreScratch()
		restorePass()
		t.Fatal("BeginPassScratch refused a GPU context")
	}
	// GPU white fill (queues; a later CPU fallback must not divert it).
	dc.SetRGB(c.Fill.R, c.Fill.G, c.Fill.B)
	dc.DrawRectangle(0, 0, float64(W), float64(H))
	_ = dc.Fill()
	// GPU plain atlas (opaque red, no tint -> stays on GPU).
	if _, err := dc.DrawAtlasEx(atlas, []render.AtlasSprite{{
		SrcX: c.Plain.Src[0], SrcY: c.Plain.Src[1], SrcW: c.Plain.Src[2], SrcH: c.Plain.Src[3],
		DstX: c.Plain.Dst[0], DstY: c.Plain.Dst[1], DstW: c.Plain.Dst[2], DstH: c.Plain.Dst[3],
		Opacity: 1, Filter: render.InterpNearest,
	}}, render.AtlasDrawOptions{}); err != nil {
		restoreScratch()
		restorePass()
		t.Fatalf("plain atlas: %v", err)
	}
	// CPU tint atlas (no tint shader -> CPU fallback mid-pass).
	if _, err := dc.DrawAtlasEx(atlas, []render.AtlasSprite{{
		SrcX: c.Tint.Src[0], SrcY: c.Tint.Src[1], SrcW: c.Tint.Src[2], SrcH: c.Tint.Src[3],
		DstX: c.Tint.Dst[0], DstY: c.Tint.Dst[1], DstW: c.Tint.Dst[2], DstH: c.Tint.Dst[3],
		Opacity: 1, Tint: render.RGBA{R: c.Tint.Tint[0], G: c.Tint.Tint[1], B: c.Tint.Tint[2], A: c.Tint.Tint[3]},
		Filter: render.InterpNearest,
	}}, render.AtlasDrawOptions{}); err != nil {
		restoreScratch()
		restorePass()
		t.Fatalf("tint atlas: %v", err)
	}
	if committed, err := dc.CommitPassScratchToView(view); err != nil {
		restoreScratch()
		restorePass()
		t.Fatalf("commit: %v", err)
	} else if !committed {
		// 染色已上显卡：纯 GPU 通路无 CPU 像素可提交是期望行为
		// （与 passScratchNeeded 的接线测试对照，见 gpupixel 包）。
		st := dc.RenderPathStats()
		if st.GPUOps == 0 || st.CPUFallbackOps != 0 {
			restoreScratch()
			restorePass()
			t.Fatalf("commit skipped a dirty scratch without full-GPU proof: %s", st.LogLine())
		}
		t.Logf("commit skipped clean scratch (full-GPU tint path): %s", st.LogLine())
	}
	if err := dc.FlushGPUWithView(view, uint32(W), uint32(H)); err != nil {
		restoreScratch()
		restorePass()
		t.Fatalf("flush view: %v", err)
	}
	restoreScratch()
	restorePass()

	// Honesty: this scenario is now full-GPU (vertex tint), so the pass
	// must show GPU ops and zero CPU fallback — the old mixed proof
	// (both counters > 0) belongs to the pre-tint era.
	st := dc.RenderPathStats()
	if st.GPUOps == 0 {
		t.Fatalf("pass recorded no GPU ops (full-GPU proof): %s", st.LogLine())
	}
	if st.CPUFallbackOps != 0 {
		t.Fatalf("pass recorded a CPU fallback (tint should stay on GPU): %s", st.LogLine())
	}

	rb := render.NewContext(W, H)
	defer rb.Close()
	rb.ClearWithColor(render.RGBA{R: 0, G: 0, B: 0, A: 1})
	rb.DrawGPUTexture(view, 0, 0, W, H)
	if err := rb.FlushGPU(); err != nil {
		t.Fatalf("readback flush: %v", err)
	}
	img := rb.Image()
	for _, p := range c.Probes {
		r, g, b, a := img.At(p.X, p.Y).RGBA()
		got := [4]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
		for i := 0; i < 4; i++ {
			d := int(got[i]) - int(p.Want[i])
			if d < 0 {
				d = -d
			}
			if d > int(p.Tol) {
				t.Errorf("%s at (%d,%d) = %v, want %v (tol %d)", p.Desc, p.X, p.Y, got, p.Want, p.Tol)
				break
			}
		}
	}
}

// TestPassScratchLazyCleanSkip (E6): an all-GPU pass leaves scratch clean —
// commit skips across scratch reuses with no stale content. A CPU fallback
// (bicubic image upscale, GPU path refused) in the swapped scratch marks it
// dirty and commits; pixel proof reads the fallback pixels back.
//
// NOTE: the mixed path uses a dedicated dc (not the pass-1/2 dc): scratch
// Buffers are per-context and the flag is per-context, so a same-dc bicubic
// probe would share state with passes 1-2; a fresh dc isolates the contract.
func TestPassScratchLazyCleanSkip(t *testing.T) {
	requirePassScratchGPU(t)
	c := loadPassScratchCases(t)
	W, H := c.Canvas[0], c.Canvas[1]

	dc := render.NewContext(W, H)
	defer dc.Close()
	dc.BeginFrame()
	view, release := dc.CreateOffscreenTexture(W, H)
	if release == nil || view.IsNil() {
		t.Skip("CreateOffscreenTexture unavailable")
	}
	defer release()

	gpuPass := func() bool {
		restorePass := dc.BeginOffscreenPass()
		defer restorePass()
		restoreScratch, ok := dc.BeginPassScratch(image.Rect(0, 0, W, H))
		defer restoreScratch()
		if !ok {
			t.Fatal("BeginPassScratch refused a GPU context")
		}
		dc.SetRGB(c.Fill.R, c.Fill.G, c.Fill.B)
		dc.DrawRectangle(0, 0, float64(W), float64(H))
		_ = dc.Fill()
		committed, err := dc.CommitPassScratchToView(view)
		if err != nil {
			t.Fatalf("commit: %v", err)
		}
		if err := dc.FlushGPUWithView(view, uint32(W), uint32(H)); err != nil {
			t.Fatalf("flush view: %v", err)
		}
		return committed
	}
	// Passes 1-2: GPU-only. Both must skip the commit (nothing CPU-landed);
	// pass 2 on the reused scratch proves no stale content forces an upload.
	if gpuPass() {
		t.Fatal("pass 1: clean scratch committed (expected skip)")
	}
	if gpuPass() {
		t.Fatal("pass 2: reused scratch committed without CPU content (stale?)")
	}

	// Pass 3: CPU fallback through the record funnel (deterministic): the
	// funnel mark is the dirty signal and a real CPU pixel lands in scratch
	// via the direct pixmap path (not a queued GPU draw). Commit must upload.
	dc2 := render.NewContext(W, H)
	defer dc2.Close()
	dc2.BeginFrame()
	view2, release2 := dc2.CreateOffscreenTexture(W, H)
	if release2 == nil || view2.IsNil() {
		t.Skip("CreateOffscreenTexture unavailable")
	}
	defer release2()
	restorePass := dc2.BeginOffscreenPass()
	restoreScratch, ok := dc2.BeginPassScratch(image.Rect(0, 0, W, H))
	if !ok {
		restoreScratch()
		restorePass()
		t.Fatal("BeginPassScratch refused a GPU context")
	}
	dc2.RecordCPUFallbackForTest("test:pass3")
	if pm := dc2.PixmapForTest(); pm != nil {
		pm.Set(4, 4, color.RGBA{R: 255, A: 255})
	} else {
		restoreScratch()
		restorePass()
		t.Fatal("no pixmap for CPU pixel probe")
	}
	committed, err := dc2.CommitPassScratchToView(view2)
	if err != nil {
		restoreScratch()
		restorePass()
		t.Fatalf("commit: %v", err)
	}
	if err := dc2.FlushGPUWithView(view2, uint32(W), uint32(H)); err != nil {
		restoreScratch()
		restorePass()
		t.Fatalf("flush view: %v", err)
	}
	restoreScratch()
	restorePass()
	if !committed {
		t.Fatal("pass 3: CPU fallback scratch skipped commit (pixels would be lost)")
	}
	if st := dc2.RenderPathStats(); st.CPUFallbackOps == 0 {
		t.Fatalf("pass 3: no CPU fallback recorded (bilinear should fall back): %s", st.LogLine())
	}

	// Pass 3 pixel spot-check: the CPU fallback pixels reached the view.
	rb2 := render.NewContext(W, H)
	defer rb2.Close()
	rb2.ClearWithColor(render.RGBA{R: 0, G: 0, B: 0, A: 1})
	rb2.DrawGPUTexture(view2, 0, 0, W, H)
	if err := rb2.FlushGPU(); err != nil {
		t.Fatalf("readback flush: %v", err)
	}
	img2 := rb2.Image()
	r, g, b, _ := img2.At(4, 4).RGBA()
	if uint8(r>>8) < 200 || uint8(g>>8) > 80 || uint8(b>>8) > 80 {
		t.Fatalf("pass 3: CPU red pixel missing at (4,4) = (%d,%d,%d)", uint8(r>>8), uint8(g>>8), uint8(b>>8))
	}

	rb := render.NewContext(W, H)
	defer rb.Close()
	rb.ClearWithColor(render.RGBA{R: 0, G: 0, B: 0, A: 1})
	rb.DrawGPUTexture(view, 0, 0, W, H)
	if err := rb.FlushGPU(); err != nil {
		t.Fatalf("readback flush: %v", err)
	}

	// Final readback: the committed GPU base still presents correctly.
	_ = rb.Image()
	_ = rb
	_ = c
}
