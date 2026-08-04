//go:build !nogpu

package gpu

import (
	"os"
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// TestProbeTTC_CJKGlyphMaskGPURender renders DejaVu + TTC-CJK glyph-mask
// text through the REAL GPU pipeline and reads back pixels — mirrors the
// R18 real-window layout where multiple faces share the glyph mask engine.
func TestProbeTTC_CJKGlyphMaskGPURender(t *testing.T) {
	device, queue, cleanup := reproRealDevice(t)
	defer cleanup()

	const W, H = 512, 96
	engine := NewGlyphMaskEngine()

	// Latin face first (occupies page 0), then CJK TTC face (page 1+).
	latin := reproFont(t)
	cjk := ttcCJKFace(t)

	type line struct {
		face text.Face
		s    string
	}
	lines := []line{
		{latin, "GROUP-A allowed"},
		{cjk, "合成"},
		{latin, "BUDGET REJECT"},
	}

	engine.SetLCDLayout(text.LCDLayoutRGB)

	sampleCount := uint32(1)
	if os.Getenv("TTCPROBE_MSAA") == "4" {
		sampleCount = 4
	}
	session := NewGPURenderSession(device, queue, sampleCount)
	t.Logf("sampleCount=%d", sampleCount)

	// Fill page 0 with 2000 unique CJK glyphs so the probe glyphs spill to page 1.
	var sb strings.Builder
	for i := 0; i < 2000; i++ {
		sb.WriteRune(rune(0x4E00 + i))
	}
	if b, err := engine.LayoutText(cjk, sb.String(), 8, 8, render.RGBA{A: 1}, render.Identity(), 1.0); err == nil {
		t.Logf("filler quads=%d page=%d", len(b.Quads), b.AtlasPageIndex)
	}

	var batches []GlyphMaskBatch
	y := 24.0
	for _, ln := range lines {
		b, err := engine.LayoutText(ln.face, ln.s, 8, y, render.RGBA{A: 1}, render.Identity(), 1.0)
		if err != nil {
			t.Fatalf("LayoutText %q: %v", ln.s, err)
		}
		if len(b.Quads) > 0 {
			batches = append(batches, b)
			t.Logf("line %q quads=%d page=%d", ln.s, len(b.Quads), b.AtlasPageIndex)
		}
		y += 30
	}
	if len(batches) < 2 {
		t.Fatal("need >=2 batches (latin + CJK)")
	}
	cjkBatches := 0
	for _, b := range batches {
		if b.AtlasPageIndex > 0 {
			cjkBatches++
		}
	}
	t.Logf("batches on page>0: %d", cjkBatches)

	if err := engine.SyncAtlasTextures(device, queue); err != nil {
		t.Fatalf("SyncAtlasTextures: %v", err)
	}
	if err := session.ensureClipBindLayout(); err != nil {
		t.Fatalf("ensureClipBindLayout: %v", err)
	}
	hasLCD := false
	for i := range batches {
		if batches[i].IsLCD {
			hasLCD = true
			break
		}
	}
	if err := session.ensureGlyphMaskPipeline(hasLCD); err != nil {
		t.Fatalf("ensureGlyphMaskPipeline: %v", err)
	}
	for i, b := range batches {
		view := engine.PageTextureView(b.AtlasPageIndex)
		if view == nil {
			t.Fatalf("nil atlas view for batch %d", i)
		}
		session.SetGlyphMaskAtlasView(i, view, b.IsLCD)
	}

	data := make([]uint8, W*H*4)
	for i := range data {
		data[i] = 255
	}
	target := render.GPURenderTarget{Data: data, Width: W, Height: H, Stride: W * 4}

	// Grouped path == the live app's path (sets s.frameW/H).
	group := ScissorGroup{GlyphMaskBatches: batches}
	if err := session.RenderFrameGrouped(target, []ScissorGroup{group}, nil, nil); err != nil {
		t.Fatalf("RenderFrameGrouped: %v", err)
	}

	nz := 0
	var rows []int
	for i := 0; i < len(data); i += 4 {
		if data[i] < 200 || data[i+1] < 200 || data[i+2] < 200 {
			nz++
			r := i / 4 / W
			if len(rows) == 0 || rows[len(rows)-1] != r {
				rows = append(rows, r)
			}
		}
	}
	t.Logf("GPU readback nonzero px=%d rows=%v", nz, rows)
	if nz == 0 {
		t.Fatal("GPU rendered NOTHING")
	}
}
