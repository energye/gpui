//go:build !nogpu

package gpu

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// TestProbeTTC_4FaceMixedGPURender reproduces the LIVE R18 window text path in
// the engine: 4 faces (DejaVu + Noto CJK TTC + Garuda + Lohit), per-run
// LayoutText batches exactly like drawStringMultiFace, one RenderFrameGrouped,
// then readback. Regression check: CJK run pixels must be nonzero.
func TestProbeTTC_4FaceMixedGPURender(t *testing.T) {
	device, queue, cleanup := reproRealDevice(t)
	defer cleanup()

	fonts := []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/tlwg/Garuda.ttf",
		"/usr/share/fonts/truetype/lohit-devanagari/Lohit-Devanagari.ttf",
	}
	var faces []text.Face
	for _, p := range fonts {
		if _, err := os.Stat(p); err != nil {
			t.Skipf("missing font %s", p)
		}
		src, err := text.NewFontSourceFromFile(p)
		if err != nil {
			t.Skipf("load %s: %v", p, err)
		}
		faces = append(faces, src.Face(12))
	}
	mf, err := text.NewMultiFace(faces...)
	if err != nil {
		t.Fatal(err)
	}

	const W, H = 640, 64
	samples := testSampleCount(t, device)
	if ms := os.Getenv("FFACE_MSAA"); ms != "" {
		var n uint32
		_, err := fmt.Sscanf(ms, "%d", &n)
		if err == nil {
			samples = n
		}
	}
	engine := NewGlyphMaskEngine()
	session := NewGPURenderSession(device, queue, samples)
	defer t.Logf("sample_count=%d", samples)

	var batches []GlyphMaskBatch
	var cjkQuads []GlyphMaskQuad
	s := "GROUP-A allowed (合成) BUDGET REJECT 预算"
	for _, run := range mf.Runs(s) {
		b, err := engine.LayoutText(run.Face, run.Text, float64(run.X), 32, render.RGBA{A: 1}, render.Identity(), 1.0)
		if err != nil {
			t.Fatalf("LayoutText run %q: %v", run.Text, err)
		}
		if len(b.Quads) > 0 {
			for i := range b.Quads {
				if run.Face == faces[1] {
					cjkQuads = append(cjkQuads, b.Quads[i])
				}
			}
			for _, q := range b.Quads {
				t.Logf("DBG engine quad run=%q x0=%.1f y0=%.1f x1=%.1f y1=%.1f u0=%.4f u1=%.4f pg=%d", run.Text, q.X0, q.Y0, q.X1, q.Y1, q.U0, q.U1, q.Page)
			}
			batches = append(batches, b)
		}
	}
	if len(batches) == 0 {
		t.Fatal("no batches")
	}
	if len(cjkQuads) == 0 {
		t.Fatal("no CJK quads in mixed run")
	}
	t.Logf("batches=%d cjkQuads=%d", len(batches), len(cjkQuads))

	if err := engine.SyncAtlasTextures(device, queue); err != nil {
		t.Fatalf("SyncAtlasTextures: %v", err)
	}
	if err := session.ensureClipBindLayout(); err != nil {
		t.Fatal(err)
	}
	if err := session.ensureGlyphMaskPipeline(false); err != nil {
		t.Fatal(err)
	}
	for i, b := range batches {
		view := engine.PageTextureView(b.AtlasPageIndex)
		if view == nil {
			t.Fatalf("nil atlas view batch %d page %d", i, b.AtlasPageIndex)
		}
		session.SetGlyphMaskAtlasView(i, view, b.IsLCD)
	}

	data := make([]uint8, W*H*4)
	for i := range data {
		data[i] = 255
	}
	target := render.GPURenderTarget{Data: data, Width: W, Height: H, Stride: W * 4}
	group := ScissorGroup{GlyphMaskBatches: batches}
	if err := session.RenderFrameGrouped(target, []ScissorGroup{group}, nil, nil); err != nil {
		t.Fatalf("RenderFrameGrouped: %v", err)
	}

	x0, x1 := int(cjkQuads[0].X0), int(cjkQuads[0].X0)
	y0, y1 := int(cjkQuads[0].Y0), int(cjkQuads[0].Y0)
	for _, q := range cjkQuads {
		x0 = min(x0, int(q.X0))
		x1 = max(x1, int(q.X1))
		y0 = min(y0, int(q.Y0))
		y1 = max(y1, int(q.Y1))
	}
	t.Logf("cjk region x=[%d,%d) y=[%d,%d)", x0, x1, y0, y1)

	// Per-row dark-pixel counts across the FULL frame (all faces mixed).
	rows := map[int]int{}
	cjkPix := 0
	for y := y0; y < y1 && y < H; y++ {
		for px := x0; px < x1 && px < W; px++ {
			i := (y*W + px) * 4
			if data[i] < 120 && data[i+1] < 120 && data[i+2] < 120 {
				cjkPix++
				rows[y]++
			}
		}
	}
	totalDark := 0
	for y := 0; y < H; y++ {
		for px := 0; px < W; px++ {
			i := (y*W + px) * 4
			if data[i] < 120 && data[i+1] < 120 && data[i+2] < 120 {
				totalDark++
			}
		}
	}
	t.Logf("cjk region dark px=%d  full frame dark px=%d", cjkPix, totalDark)

	if out := os.Getenv("FOURFACE_OUT"); out != "" {
		f, err := os.Create(out)
		if err == nil {
			img := &image.RGBA{Pix: data, Stride: W * 4, Rect: image.Rect(0, 0, W, H)}
			_ = png.Encode(f, img)
			f.Close()
			t.Logf("wrote %s", out)
		}
	}

	if cjkPix < 20 {
		t.Errorf("CJK mixed-face glyph-mask region rendered %d dark px (expect >20) — same hole as live window", cjkPix)
	}
}
