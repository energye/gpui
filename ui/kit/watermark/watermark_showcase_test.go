package watermark_test

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/kit/watermark"
	"github.com/energye/gpui/ui/rendering"
)

type wmShowcaseSpec struct {
	CanvasW    int       `json:"canvasW"`
	Margin     float64   `json:"margin"`
	Gap        float64   `json:"gap"`
	RowHeights []float64 `json:"rowHeights"`
	FontRow    struct {
		Content  string    `json:"content"`
		FontSize float64   `json:"fontSize"`
		Color    []float64 `json:"color"`
		Weight   string    `json:"weight"`
		Style    string    `json:"style"`
		Family   string    `json:"family"`
		Align    string    `json:"align"`
		Rotate   float64   `json:"rotate"`
		Gap      []float64 `json:"gap"`
	} `json:"fontRow"`
	SizeRow struct {
		Content  string    `json:"content"`
		Width    float64   `json:"width"`
		Height   float64   `json:"height"`
		Gap      []float64 `json:"gap"`
		Offset   []float64 `json:"offset"`
		Rotate   float64   `json:"rotate"`
		FontSize float64   `json:"fontSize"`
	} `json:"sizeRow"`
	Tolerance struct {
		MaxDiff float64 `json:"maxDiff"`
		BadFrac float64 `json:"badFrac"`
		HardCap float64 `json:"hardCap"`
	} `json:"tolerance"`
}

func loadWmShowcaseSpec(t *testing.T) wmShowcaseSpec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "showcase_spec.json"))
	if err != nil {
		t.Fatalf("read showcase_spec.json: %v", err)
	}
	var s wmShowcaseSpec
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse showcase spec: %v", err)
	}
	if s.CanvasW <= 0 || len(s.RowHeights) != 7 {
		t.Fatalf("bad showcase spec %+v", s)
	}
	return s
}

func loadWmShowcaseFaces(t *testing.T) map[int]text.Face {
	t.Helper()
	text.ClearSystemFontPaths()
	out := map[int]text.Face{}
	for _, pts := range []int{14, 16, 18, 20} {
		f, desc, err := rendering.TryLoadDefaultFace(float64(pts))
		if err != nil || f == nil {
			t.Skipf("showcase needs a system face for real glyphs: %v", err)
		}
		t.Logf("showcase face %d: %s", pts, desc)
		out[pts] = f
	}
	return out
}

// TestWatermark_Showcase_MultiStyle lays §6.8 P0 main paths on one canvas:
// basic / multi-line+per-line font / image+fallback / custom rotate-gap-offset-
// fontsize-zIndex / full font (color-weight-style-family-align) / explicit
// width-height-spacing / portal Wrap (Modal/Drawer inherit). Three evidences:
// logic probe (HasMark/Layout/Node/TiledCount per row), pixel assertions
// (dark glyph ink in custom row, blue image pixels, faint ink in basic row),
// golden file compare (tolerance from testdata/showcase_spec.json).
// Regenerate with UPDATE_GOLDEN=1.
func TestWatermark_Showcase_MultiStyle(t *testing.T) {
	spec := loadWmShowcaseSpec(t)
	faces := loadWmShowcaseFaces(t)
	cases := loadWatermark(t)

	W := float64(spec.CanvasW)
	margin, gap := spec.Margin, spec.Gap
	rowW := W - 2*margin
	mkBg := func(h float64, shade float64) *rendering.RenderColorBox {
		return rendering.NewRenderColorBox(rowW, h, shade, shade, shade, 1)
	}

	// R1 basic (basic.tsx): default rotate/gap, faint default color stays.
	r1 := watermark.NewWatermark(mkBg(spec.RowHeights[0], 1))
	r1.SetContentStrings(cases.Cases[0].Content...)
	r1.SetFace(faces[16])

	// R2 multi-line (multi-line.tsx): two rows with per-line fontSize.
	multi := cases.Cases[1]
	r2 := watermark.NewWatermark(mkBg(spec.RowHeights[1], 1))
	lines := make([]watermark.WatermarkContentLine, 0, len(multi.Content))
	for i, s := range multi.Content {
		ln := watermark.WatermarkContentLine{Text: s}
		if i < len(multi.Fonts) && multi.Fonts[i] > 0 {
			ln.Font = watermark.WatermarkFont{FontSize: multi.Fonts[i]}
			ln.HasFont = true
		}
		lines = append(lines, ln)
	}
	r2.SetContentLines(lines...)
	r2.SetFace(faces[16])

	// R3 image (image.tsx): pixels win, content stays as FAQ fallback.
	img := cases.Cases[2]
	r3 := watermark.NewWatermark(mkBg(spec.RowHeights[2], 1))
	r3.SetContent("Ant Design")
	r3.SetImage(img.Image)
	r3.SetWidth(img.Width)
	r3.SetHeight(img.Height)
	r3.SetImagePixels(12, 8, solidPixels(12, 8, 30, 120, 220, 255))
	r3.SetFace(faces[16])

	// R4 custom (custom.tsx): rotate/gap/offset/fontSize/zIndex, dark for ink.
	custom := cases.Cases[3]
	r4 := watermark.NewWatermark(mkBg(spec.RowHeights[3], 1))
	r4.SetContentStrings(custom.Content...)
	if custom.Rotate != nil {
		r4.SetRotate(*custom.Rotate)
	}
	if len(custom.Gap) == 2 {
		r4.SetGap(custom.Gap[0], custom.Gap[1])
	}
	if len(custom.Offset) == 2 {
		r4.SetOffset(custom.Offset[0], custom.Offset[1])
	}
	if custom.FontSize != nil {
		r4.SetFontSize(*custom.FontSize)
	}
	if custom.ZIndex != nil {
		r4.SetZIndex(*custom.ZIndex)
	}
	r4.SetFontColor(render.RGBA{R: 0, G: 0, B: 0, A: 1})
	r4.SetFace(faces[20])

	// R5 full font: color/weight/style/family/align/rotate/spacing (§6.8 font).
	r5 := watermark.NewWatermark(mkBg(spec.RowHeights[4], 1))
	r5.SetContent(spec.FontRow.Content)
	r5.SetFontSize(spec.FontRow.FontSize)
	if len(spec.FontRow.Color) == 4 {
		c := spec.FontRow.Color
		r5.SetFontColor(render.RGBA{R: c[0], G: c[1], B: c[2], A: c[3]})
	}
	r5.SetFontWeight(spec.FontRow.Weight)
	r5.SetFontStyle(spec.FontRow.Style)
	r5.SetFontFamily(spec.FontRow.Family)
	r5.SetTextAlign(spec.FontRow.Align)
	r5.SetRotate(spec.FontRow.Rotate)
	if len(spec.FontRow.Gap) == 2 {
		r5.SetGap(spec.FontRow.Gap[0], spec.FontRow.Gap[1])
	}
	r5.SetFace(faces[18])

	// R6 explicit size/spacing: width/height + tight gap + offset + tilt.
	r6 := watermark.NewWatermark(mkBg(spec.RowHeights[5], 1))
	r6.SetContent(spec.SizeRow.Content)
	r6.SetWidth(spec.SizeRow.Width)
	r6.SetHeight(spec.SizeRow.Height)
	if len(spec.SizeRow.Gap) == 2 {
		r6.SetGap(spec.SizeRow.Gap[0], spec.SizeRow.Gap[1])
	}
	if len(spec.SizeRow.Offset) == 2 {
		r6.SetOffset(spec.SizeRow.Offset[0], spec.SizeRow.Offset[1])
	}
	r6.SetRotate(spec.SizeRow.Rotate)
	r6.SetFontSize(spec.SizeRow.FontSize)
	r6.SetFontColor(render.RGBA{R: 0, G: 0, B: 0, A: 1})
	r6.SetZIndex(10)
	r6.SetFace(faces[14])

	// R7 portal (portal.tsx): inherit Wrap to Modal/Drawer content.
	portal := cases.Cases[4]
	src := watermark.NewWatermark(mkBg(spec.RowHeights[6], 1))
	src.SetContentStrings(portal.Content...)
	src.SetFace(faces[16])
	if !src.Inherit() {
		t.Fatal("inherit default true")
	}
	r7 := src.Wrap(mkBg(spec.RowHeights[6], 0.96))
	// Wrap copies face+config; keep face explicit for determinism.
	r7.SetFace(faces[16])

	rows := []*watermark.Watermark{r1, r2, r3, r4, r5, r6, r7}

	// Logic probe before paint.
	for i, wm := range rows {
		if !wm.HasMark() {
			t.Fatalf("row %d should mark", i)
		}
		if wm.Node() == nil {
			t.Fatalf("row %d Node nil", i)
		}
		if wm.TiledCount(rowW, spec.RowHeights[i]) <= 0 {
			t.Fatalf("row %d should tile", i)
		}
	}
	if len(r2.ContentLines()) != 2 {
		t.Fatalf("multi lines=%d want 2", len(r2.ContentLines()))
	}
	if !r3.IsImageMode() {
		t.Fatal("image row must be image mode")
	}
	if r4.ResolvedRotate() != *custom.Rotate {
		t.Fatalf("custom rotate=%v", r4.ResolvedRotate())
	}
	if r5.EffectiveFontWeight() != spec.FontRow.Weight || r5.EffectiveTextAlign() != spec.FontRow.Align {
		t.Fatal("font row weight/align probe")
	}
	mw6, mh6 := r6.ResolvedMarkSize()
	if mw6 != spec.SizeRow.Width || mh6 != spec.SizeRow.Height {
		t.Fatalf("size row=%vx%v", mw6, mh6)
	}
	if len(r7.ContentLines()) != len(src.ContentLines()) || !r7.Inherit() {
		t.Fatal("portal Wrap probe")
	}

	type placed struct {
		wm *watermark.Watermark
		x  float64
		y  float64
		w  float64
		h  float64
	}
	var items []placed
	y := margin
	for i, wm := range rows {
		h := spec.RowHeights[i]
		sz := wm.Layout(rendering.Tight(rowW, h))
		if sz.Width != rowW || sz.Height != h {
			t.Fatalf("row %d layout=%v want %vx%v", i, sz, rowW, h)
		}
		if sz.Width <= 0 || sz.Height <= 0 {
			t.Fatalf("row %d layout non-zero", i)
		}
		items = append(items, placed{wm: wm, x: margin, y: y, w: sz.Width, h: sz.Height})
		y += sz.Height + gap
	}
	H := y - gap + margin

	dc := render.NewContext(int(W), int(H))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	for _, it := range items {
		it.wm.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(it.x, it.y))
	}
	got := dc.Image()

	// Pixel 1: custom dark row carries glyph ink (real text, no bars).
	customIt := items[3]
	dark := 0
	for yy := int(customIt.y); yy < int(customIt.y+customIt.h); yy++ {
		for xx := int(customIt.x); xx < int(customIt.x+customIt.w); xx++ {
			r, g, b, _ := got.At(xx, yy).RGBA()
			if r/257 < 110 && g/257 < 110 && b/257 < 110 {
				dark++
			}
		}
	}
	if dark < 30 {
		t.Fatalf("custom row dark pixels=%d want >=30 (real glyphs missing?)", dark)
	}

	// Pixel 2: image row carries blue image pixels (host pixels, not text).
	imgIt := items[2]
	blue := 0
	for yy := int(imgIt.y); yy < int(imgIt.y+imgIt.h); yy++ {
		for xx := int(imgIt.x); xx < int(imgIt.x+imgIt.w); xx++ {
			r, g, b, _ := got.At(xx, yy).RGBA()
			r8, g8, b8 := r/257, g/257, b/257
			if b8 > 150 && r8 < 150 && g8 < 200 {
				blue++
			}
		}
	}
	if blue < 200 {
		t.Fatalf("image row blue pixels=%d want >=200", blue)
	}

	// Pixel 3: basic faint row still carries non-white ink (default 0.15).
	basicIt := items[0]
	faint := 0
	for yy := int(basicIt.y); yy < int(basicIt.y+basicIt.h); yy++ {
		for xx := int(basicIt.x); xx < int(basicIt.x+basicIt.w); xx++ {
			r, g, b, _ := got.At(xx, yy).RGBA()
			r8, g8, b8 := r/257, g/257, b/257
			if r8 < 250 || g8 < 250 || b8 < 250 {
				faint++
			}
		}
	}
	if faint < 20 {
		t.Fatalf("basic row faint pixels=%d want >=20", faint)
	}

	path := filepath.Join("testdata", "showcase_watermark.png")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("write showcase: %v", err)
		}
		if err := png.Encode(f, got); err != nil {
			f.Close()
			t.Fatalf("encode showcase: %v", err)
		}
		f.Close()
		t.Logf("showcase rewritten: %s (%dx%d)", path, int(W), int(H))
		return
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open showcase %s (run once with UPDATE_GOLDEN=1): %v", path, err)
	}
	want, err := png.Decode(f)
	f.Close()
	if err != nil {
		t.Fatalf("decode showcase: %v", err)
	}
	if !want.Bounds().Eq(got.Bounds()) {
		t.Fatalf("showcase bounds %v want %v (spec drift? regen with UPDATE_GOLDEN=1)", got.Bounds(), want.Bounds())
	}
	maxDiff := uint32(spec.Tolerance.MaxDiff * 257)
	hardCap := uint32(spec.Tolerance.HardCap * 257)
	bad := 0
	total := got.Bounds().Dx() * got.Bounds().Dy()
	for yy := 0; yy < got.Bounds().Dy(); yy++ {
		for xx := 0; xx < got.Bounds().Dx(); xx++ {
			r1, g1, b1, a1 := got.At(xx, yy).RGBA()
			r2, g2, b2, a2 := want.At(xx, yy).RGBA()
			m := wmMax(diffWm(r1, r2), diffWm(g1, g2), diffWm(b1, b2), diffWm(a1, a2))
			if m > hardCap {
				t.Fatalf("showcase pixel (%d,%d) diff %d exceeds hard cap", xx, yy, m/257)
			}
			if m > maxDiff {
				bad++
			}
		}
	}
	if float64(bad)/float64(total) > spec.Tolerance.BadFrac {
		t.Fatalf("showcase bad pixels %d/%d exceed %.1f%%", bad, total, spec.Tolerance.BadFrac*100)
	}
}

func diffWm(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func wmMax(vs ...uint32) uint32 {
	m := vs[0]
	for _, v := range vs[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
