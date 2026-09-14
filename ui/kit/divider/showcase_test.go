package divider_test

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/kit/divider"
	"github.com/energye/gpui/ui/rendering"
)

type dividerShowcaseSpec struct {
	CanvasW int     `json:"canvasW"`
	Margin  float64 `json:"margin"`
	Gap     float64 `json:"gap"`
	Tolerance struct {
		MaxDiff float64 `json:"maxDiff"`
		BadFrac float64 `json:"badFrac"`
		HardCap float64 `json:"hardCap"`
	} `json:"tolerance"`
}

func loadDividerShowcaseSpec(t *testing.T) dividerShowcaseSpec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "showcase_spec.json"))
	if err != nil {
		t.Fatalf("read showcase_spec.json: %v", err)
	}
	var s dividerShowcaseSpec
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse showcase spec: %v", err)
	}
	if s.CanvasW <= 0 || s.Gap < 0 {
		t.Fatalf("bad showcase spec %+v", s)
	}
	return s
}

func loadDividerShowcaseFaces(t *testing.T) (face14, face16 text.Face) {
	t.Helper()
	mk := func(pts float64) text.Face {
		f, _, err := text.LoadMultiFace(pts)
		if err != nil || f == nil {
			f2, _, err2 := rendering.TryLoadDefaultFace(pts)
			if err2 != nil || f2 == nil {
				t.Skipf("showcase needs a system face for real glyphs: %v / %v", err, err2)
			}
			return f2
		}
		return f
	}
	return mk(14), mk(16)
}

// TestDivider_Showcase_MainPaths lays §6.8 P0 main paths on one canvas:
// horizontal solid/dashed/dotted, with-text center/start/end, plain,
// size small/medium/large, variant+title combos, vertical group.
// Three evidences: logic probe (variant/rails/sizes), pixel assertions
// (rail line ink + title text ink + vertical ink), golden compare
// (tolerance from testdata/showcase_spec.json). Regenerate with UPDATE_GOLDEN=1.
func TestDivider_Showcase_MainPaths(t *testing.T) {
	spec := loadDividerShowcaseSpec(t)
	face14, face16 := loadDividerShowcaseFaces(t)

	W := float64(spec.CanvasW)
	margin, gap := spec.Margin, spec.Gap
	rowW := W - 2*margin

	withFace := func(d *divider.Divider) *divider.Divider {
		if d.Plain() {
			d.SetFace(face14)
		} else if d.HasTitle() {
			d.SetFace(face16)
		}
		return d
	}

	// Horizontal rows (§6.8 P0).
	solid := divider.NewDivider()
	dashed := divider.NewDivider()
	dashed.SetDashed(true)
	dotted := divider.NewDivider()
	dotted.SetVariant(divider.Dotted)
	center := withFace(divider.NewDividerWithTitle("Center"))
	start := withFace(divider.NewDividerWithTitle("Start"))
	start.SetTitlePlacement(divider.Start)
	end := withFace(divider.NewDividerWithTitle("End"))
	end.SetTitlePlacement(divider.End)
	plain := withFace(divider.NewDividerWithTitle("Plain Body"))
	plain.SetPlain(true)
	// Plain must use the 14pt face even though withFace ran before SetPlain.
	plain.SetFace(face14)
	small := divider.NewDivider()
	small.SetSize(divider.Small)
	medium := divider.NewDivider()
	medium.SetSize(divider.Medium)
	large := divider.NewDivider()
	large.SetSize(divider.Large)
	dashTitle := withFace(divider.NewDividerWithTitle("Dashed Title"))
	dashTitle.SetVariant(divider.Dashed)
	dotTitle := withFace(divider.NewDividerWithTitle("Dotted Title"))
	dotTitle.SetVariant(divider.Dotted)

	// Vertical group (§6.8 P0 vertical + variant).
	vSolid := divider.NewDivider()
	vSolid.SetVertical(true)
	vDashed := divider.NewDivider()
	vDashed.SetOrientation(divider.Vertical)
	vDashed.SetVariant(divider.Dashed)
	vDotted := divider.NewDivider()
	vDotted.SetVertical(true)
	vDotted.SetVariant(divider.Dotted)

	horiz := []*divider.Divider{solid, dashed, dotted, center, start, end, plain, small, medium, large, dashTitle, dotTitle}
	verts := []*divider.Divider{vSolid, vDashed, vDotted}

	// Logic probe before paint.
	if solid.EffectiveVariant() != divider.Solid || dashed.EffectiveVariant() != divider.Dashed || dotted.EffectiveVariant() != divider.Dotted {
		t.Fatal("variant probe")
	}
	if gs, ge := center.RailGrows(); gs != 1 || ge != 1 {
		t.Fatalf("center grows=%v/%v", gs, ge)
	}
	if gs, ge := start.RailGrows(); gs >= ge {
		t.Fatalf("start rail %v should be shorter than %v", gs, ge)
	}
	if gs, ge := end.RailGrows(); ge >= gs {
		t.Fatalf("end rail %v should be shorter than %v", gs, ge)
	}
	if plain.TitleFontSize() >= center.TitleFontSize() {
		t.Fatalf("plain %v should be smaller than default %v", plain.TitleFontSize(), center.TitleFontSize())
	}
	if !(small.MarginBlock() < medium.MarginBlock() && medium.MarginBlock() <= large.MarginBlock()) {
		t.Fatalf("size margins %v/%v/%v must ascend", small.MarginBlock(), medium.MarginBlock(), large.MarginBlock())
	}
	for _, v := range verts {
		if !v.IsVertical() || v.HasTitle() {
			t.Fatal("vertical probe")
		}
	}
	if vDashed.EffectiveOrientation() != divider.Vertical {
		t.Fatal("orientation probe")
	}

	// Layout every row; Layout/Node must stay non-zero (true-size chain).
	type placed struct {
		d *divider.Divider
		x float64
		y float64
		w float64
		h float64
	}
	var items []placed
	y := margin
	for i, d := range horiz {
		sz := d.Layout(rendering.Constraints{MinWidth: rowW, MaxWidth: rowW, MaxHeight: rendering.Unbounded})
		if sz.Width != rowW || sz.Height <= 0 {
			t.Fatalf("row %d layout=%v want width %v", i, sz, rowW)
		}
		if d.Node() == nil || d.ChromeNode() == nil {
			t.Fatalf("row %d node nil", i)
		}
		ns := d.Node().Size()
		if ns.Width <= 0 || ns.Height <= 0 {
			t.Fatalf("row %d node size=%v want >0", i, ns)
		}
		items = append(items, placed{d: d, x: margin, y: y, w: sz.Width, h: sz.Height})
		y += sz.Height + gap
	}
	// Vertical group shares one visual row, laid side by side.
	const vertGap = 48.0
	vx := margin
	vRowH := 0.0
	var vItems []placed
	for _, v := range verts {
		sz := v.Layout(rendering.Loose(rowW, 200))
		if sz.Width <= 0 || sz.Height <= 0 {
			t.Fatalf("vertical layout=%v", sz)
		}
		if v.Node() == nil {
			t.Fatal("vertical node nil")
		}
		if sz.Height > vRowH {
			vRowH = sz.Height
		}
		vItems = append(vItems, placed{d: v, x: vx, y: y, w: sz.Width, h: sz.Height})
		vx += sz.Width + vertGap
	}
	if vRowH <= 0 {
		t.Fatal("vertical row height")
	}
	y += vRowH + gap
	H := y - gap + margin

	dc := render.NewContext(int(W), int(H))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	for _, it := range items {
		it.d.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(it.x, it.y))
	}
	for _, it := range vItems {
		it.d.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(it.x, it.y))
	}
	got := dc.Image()

	// Pixel assertion 1: solid rail carries line ink (not pure white).
	s0 := items[0]
	railMidY := int(s0.y + s0.d.MarginBlock())
	lineInk := 0
	for xx := int(s0.x + 10); xx < int(s0.x+s0.w-10); xx++ {
		r, g, b, _ := got.At(xx, railMidY).RGBA()
		r8, g8, b8 := r/257, g/257, b/257
		if r8 < 250 || g8 < 250 || b8 < 250 {
			lineInk++
		}
	}
	if lineInk < 50 {
		t.Fatalf("solid rail ink pixels=%d want >=50", lineInk)
	}
	// Dashed has gaps so ink is fewer than solid but still visible;
	// dotted draws dots so ink is sparse but non-zero.
	countRailInk := func(it placed) int {
		n := 0
		yy := int(it.y + it.d.MarginBlock())
		for xx := int(it.x + 10); xx < int(it.x+it.w-10); xx++ {
			r, g, b, _ := got.At(xx, yy).RGBA()
			if r/257 < 250 || g/257 < 250 || b/257 < 250 {
				n++
			}
		}
		return n
	}
	dashInk, dotInk := countRailInk(items[1]), countRailInk(items[2])
	if dashInk <= 0 || dotInk <= 0 {
		t.Fatalf("variant ink dashed=%d dotted=%d want >0", dashInk, dotInk)
	}
	if dashInk >= lineInk {
		t.Fatalf("dashed ink=%d should be fewer than solid=%d (gaps visible)", dashInk, lineInk)
	}

	// Pixel assertion 2: title zone carries dark glyph ink (real text).
	c0 := items[3]
	dark := 0
	tx0 := int(c0.x + c0.d.RailStartWidth())
	tx1 := int(c0.x + c0.d.RailStartWidth() + c0.d.TitleBlockWidth())
	ty0, ty1 := int(c0.y+2), int(c0.y+c0.h-2)
	for yy := ty0; yy < ty1; yy++ {
		for xx := tx0; xx < tx1; xx++ {
			r, g, b, _ := got.At(xx, yy).RGBA()
			r8, g8, b8 := r/257, g/257, b/257
			if r8 < 110 && g8 < 110 && b8 < 110 {
				dark++
			}
		}
	}
	if dark < 30 {
		t.Fatalf("center title dark pixels=%d want >=30 (real glyphs missing?)", dark)
	}

	// Pixel assertion 3: vertical rails carry ink.
	vInk := 0
	v0 := vItems[0]
	vMidX := int(v0.x + v0.d.MarginInline())
	for yy := int(v0.y); yy < int(v0.y+v0.h); yy++ {
		r, g, b, _ := got.At(vMidX, yy).RGBA()
		if r/257 < 250 || g/257 < 250 || b/257 < 250 {
			vInk++
		}
	}
	if vInk <= 0 {
		t.Fatalf("vertical rail ink=%d want >0", vInk)
	}

	path := filepath.Join("testdata", "showcase_divider.png")
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
			m := max4div(diffDiv(r1, r2), diffDiv(g1, g2), diffDiv(b1, b2), diffDiv(a1, a2))
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

func diffDiv(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func max4div(vs ...uint32) uint32 {
	m := vs[0]
	for _, v := range vs[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
