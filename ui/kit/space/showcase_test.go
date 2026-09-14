package space_test

import (
	"encoding/json"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/space"
	"github.com/energye/gpui/ui/rendering"
)

type spaceShowcaseSpec struct {
	CanvasW int     `json:"canvasW"`
	Margin  float64 `json:"margin"`
	Gap     float64 `json:"gap"`
	Tolerance struct {
		MaxDiff float64 `json:"maxDiff"`
		BadFrac float64 `json:"badFrac"`
		HardCap float64 `json:"hardCap"`
	} `json:"tolerance"`
}

func loadSpaceShowcaseSpec(t *testing.T) spaceShowcaseSpec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "showcase_spec.json"))
	if err != nil {
		t.Fatalf("read showcase_spec.json: %v", err)
	}
	var s spaceShowcaseSpec
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse showcase spec: %v", err)
	}
	if s.CanvasW <= 0 || s.Gap < 0 {
		t.Fatalf("bad showcase spec %+v", s)
	}
	if s.Tolerance.MaxDiff != 12 || s.Tolerance.BadFrac != 0.005 || s.Tolerance.HardCap != 64 {
		t.Fatalf("showcase tolerance %+v want 12/0.005/64", s.Tolerance)
	}
	return s
}

func showBox(w, h, r, g, b float64) *rendering.RenderColorBox {
	return rendering.NewRenderColorBox(w, h, r, g, b, 1)
}

func diffShow(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func max4Show(vs ...uint32) uint32 {
	m := vs[0]
	for _, v := range vs[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

// TestSpace_Showcase_MainPaths lays §6.8 P0+P1 rows on one canvas: base /
// size-middle / size-large / vertical / align-center / wrap-narrow /
// separator / compact-double / compact-triple-addon / vertical-compact(P1) /
// sizeXY-numeric. Three evidences: logic probe (gaps/offsets/overlap),
// pixel assertions (red/blue shells, white gaps, gray separator, compact no
// white seam), golden file compare (tolerance from
// testdata/showcase_spec.json). Regenerate with UPDATE_GOLDEN=1.
func TestSpace_Showcase_MainPaths(t *testing.T) {
	spec := loadSpaceShowcaseSpec(t)
	fx := loadSpace(t)

	W := float64(spec.CanvasW)
	margin, rowGap := spec.Margin, spec.Gap
	rowW := W - 2*margin

	// R1 base.tsx: three children, default small gap.
	base := space.NewSpace(
		showBox(60, 32, 0.9, 0.2, 0.2),
		showBox(60, 32, 0.2, 0.4, 0.9),
		showBox(60, 32, 0.2, 0.7, 0.3),
	)
	// R2 size middle, R3 size large (size.tsx three rows).
	mid := space.NewSpace(
		showBox(50, 20, 0.95, 0.6, 0.15),
		showBox(50, 20, 0.6, 0.3, 0.8),
	)
	mid.SetSize(space.SpaceSizeMiddle)
	large := space.NewSpace(
		showBox(50, 20, 0.2, 0.6, 0.6),
		showBox(50, 20, 0.9, 0.2, 0.2),
	)
	large.SetSize(space.SpaceSizeLarge)
	// R4 vertical.tsx.
	vert := space.NewSpace(
		showBox(60, 32, 0.2, 0.4, 0.9),
		showBox(60, 32, 0.95, 0.6, 0.15),
	)
	vert.SetVertical(true)
	// R5 align.tsx: unequal heights, center.
	alignC := space.NewSpace(
		showBox(60, fx.ShortH, 0.9, 0.2, 0.2),
		showBox(60, fx.TallH, 0.2, 0.4, 0.9),
	)
	alignC.SetAlign(space.SpaceAlignCenter)
	// R6 wrap.tsx: narrow container forces a second row.
	wrap := space.NewSpace(
		showBox(fx.NarrowChildW, 32, 0.9, 0.2, 0.2),
		showBox(fx.NarrowChildW, 32, 0.2, 0.4, 0.9),
		showBox(fx.NarrowChildW, 32, 0.2, 0.7, 0.3),
	)
	wrap.SetWrap(true)
	// R7 separator.tsx.
	sepSpace := space.NewSpace(
		showBox(40, 20, 0.9, 0.2, 0.2),
		showBox(40, 20, 0.2, 0.4, 0.9),
	)
	sepSpace.SetSeparator(func() rendering.RenderObject {
		return rendering.NewRenderColorBox(8, 16, 0.6, 0.6, 0.6, 1)
	})
	// R8 compact.tsx: two raw cells share one border.
	compact2 := space.NewSpaceCompact(
		showBox(60, 32, 0.9, 0.2, 0.2),
		showBox(60, 32, 0.2, 0.4, 0.9),
	)
	// R9 compact-buttons.tsx: three addon cells, middle radius cleared.
	ad0 := space.NewSpaceAddon(showBox(48, 20, 0.9, 0.2, 0.2))
	ad1 := space.NewSpaceAddon(showBox(48, 20, 0.2, 0.7, 0.3))
	ad2 := space.NewSpaceAddon(showBox(48, 20, 0.2, 0.4, 0.9))
	compact3 := space.NewSpaceCompact()
	compact3.AddAddon(ad0)
	compact3.AddAddon(ad1)
	compact3.AddAddon(ad2)
	// R10 vertical compact (P1 vertical-compact page; overlap already proven).
	vcompact := space.NewSpaceCompact(
		showBox(60, 32, 0.6, 0.3, 0.8),
		showBox(60, 32, 0.2, 0.6, 0.6),
	)
	vcompact.SetVertical(true)
	// R11 numeric [col,row]: col 12 stays visibly distinct from 8/16/24.
	xy := space.NewSpace(
		showBox(50, 20, 0.2, 0.7, 0.3),
		showBox(50, 20, 0.6, 0.3, 0.8),
	)
	xy.SetSizeXY(12, 20)

	// Logic probe before paint: gaps, orientation, align, wrap, separator.
	if math.Abs(base.ResolvedGap()-fx.GapSmall) > fx.Tolerance {
		t.Fatalf("base gap=%v want %v", base.ResolvedGap(), fx.GapSmall)
	}
	if math.Abs(mid.ResolvedGap()-fx.GapMiddle) > fx.Tolerance {
		t.Fatalf("mid gap=%v want %v", mid.ResolvedGap(), fx.GapMiddle)
	}
	if math.Abs(large.ResolvedGap()-fx.GapLarge) > fx.Tolerance {
		t.Fatalf("large gap=%v want %v", large.ResolvedGap(), fx.GapLarge)
	}
	if !vert.IsVertical() || vert.EffectiveOrientation() != space.SpaceVertical {
		t.Fatal("vertical probe")
	}
	if alignC.EffectiveAlign() != space.SpaceAlignCenter {
		t.Fatalf("align=%q want center", alignC.EffectiveAlign())
	}
	if !wrap.Wrap() {
		t.Fatal("wrap flag must stick")
	}
	if sepSpace.SeparatorCount() != 1 || !sepSpace.SeparatorsAriaHidden() || sepSpace.SeparatorFocusable() {
		t.Fatal("separator probe")
	}
	if math.Abs(compact2.Overlap()-1) > fx.Tolerance {
		t.Fatalf("compact overlap=%v want 1", compact2.Overlap())
	}
	if compact3.Size() != space.SpaceSizeMiddle {
		t.Fatalf("compact size=%d want middle", compact3.Size())
	}
	if r := ad1.EffectiveRadius(); r != 0 {
		t.Fatalf("middle addon radius=%v want 0", r)
	}
	if !vcompact.IsVertical() {
		t.Fatal("vertical compact probe")
	}
	if math.Abs(xy.ResolvedColGap()-12) > fx.Tolerance || math.Abs(xy.ResolvedRowGap()-20) > fx.Tolerance {
		t.Fatalf("xy=%v/%v want 12/20", xy.ResolvedColGap(), xy.ResolvedRowGap())
	}

	// Layout every row; Layout/Node must stay non-zero (true-size chain).
	type placed struct {
		node rendering.RenderObject
		x, y float64
		w, h float64
	}
	var items []placed
	y := margin
	placeSpace := func(s *space.Space, maxW float64) placed {
		sz := s.Layout(rendering.Loose(maxW, 800))
		if sz.Width <= 0 || sz.Height <= 0 {
			t.Fatalf("space layout=%v want >0", sz)
		}
		if s.Node() == nil {
			t.Fatal("space node nil")
		}
		if ns := s.Node().Size(); ns.Width <= 0 || ns.Height <= 0 {
			t.Fatalf("space node size=%v want >0", ns)
		}
		it := placed{node: s.Node(), x: margin, y: y, w: sz.Width, h: sz.Height}
		items = append(items, it)
		y += sz.Height + rowGap
		return it
	}
	placeCompact := func(c *space.SpaceCompact, maxW float64) placed {
		sz := c.Layout(rendering.Loose(maxW, 800))
		if sz.Width <= 0 || sz.Height <= 0 {
			t.Fatalf("compact layout=%v want >0", sz)
		}
		if c.Node() == nil {
			t.Fatal("compact node nil")
		}
		if ns := c.Node().Size(); ns.Width <= 0 || ns.Height <= 0 {
			t.Fatalf("compact node size=%v want >0", ns)
		}
		it := placed{node: c.Node(), x: margin, y: y, w: sz.Width, h: sz.Height}
		items = append(items, it)
		y += sz.Height + rowGap
		return it
	}
	basePl := placeSpace(base, rowW)
	midPl := placeSpace(mid, rowW)
	largePl := placeSpace(large, rowW)
	vertPl := placeSpace(vert, rowW)
	alignPl := placeSpace(alignC, rowW)
	wrapPl := placeSpace(wrap, fx.NarrowW)
	sepPl := placeSpace(sepSpace, rowW)
	compact2Pl := placeCompact(compact2, rowW)
	compact3Pl := placeCompact(compact3, rowW)
	vcompactPl := placeCompact(vcompact, rowW)
	xyPl := placeSpace(xy, rowW)
	_ = midPl
	_ = largePl
	_ = xyPl
	H := y - rowGap + margin

	// Geometry probe after layout: gaps visibly distinct, no zero boxes.
	off, sz := offsets(base), sizes(base)
	for i := 0; i < 2; i++ {
		if math.Abs(spacingX(off, sz, i)-fx.GapSmall) > fx.Tolerance {
			t.Fatalf("base spacing %d=%v want %v", i, spacingX(off, sz, i), fx.GapSmall)
		}
	}
	voff, vsz := offsets(vert), sizes(vert)
	if math.Abs(spacingY(voff, vsz, 0)-fx.GapSmall) > fx.Tolerance {
		t.Fatalf("vertical spacing=%v want %v", spacingY(voff, vsz, 0), fx.GapSmall)
	}
	wrapOff := offsets(wrap)
	if !(wrapOff[1].Y > wrapOff[0].Y || wrapOff[2].Y > wrapOff[0].Y) {
		t.Fatalf("wrap must break: %+v", wrapOff)
	}
	ckids := compact2.Children()
	if len(ckids) != 2 {
		t.Fatalf("compact children=%d want 2", len(ckids))
	}
	if shared := (ckids[0].Offset().X + 60) - ckids[1].Offset().X; math.Abs(shared-1) > fx.Tolerance {
		t.Fatalf("compact shared=%v want 1", shared)
	}
	if w := sepSpace.SeparatorAt(0).Size().Width; w <= 0 {
		t.Fatalf("separator width=%v want >0", w)
	}

	dc := render.NewContext(int(W), int(H))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	for _, it := range items {
		it.node.Paint(rendering.NewPaintContext(dc, 1).WithOrigin(it.x, it.y))
	}
	got := dc.Image()

	rgb := func(x, y int) (uint32, uint32, uint32) {
		r, g, b, _ := got.At(x, y).RGBA()
		return r / 257, g / 257, b / 257
	}
	isWhite := func(r, g, b uint32) bool { return r > 240 && g > 240 && b > 240 }

	// Pixel 1: base row red box, white gap, blue box.
	br, bg, bb := rgb(int(basePl.x+30), int(basePl.y+16))
	if !(br > 180 && bg < 120 && bb < 120) {
		t.Fatalf("base first #%02x%02x%02x want red", br, bg, bb)
	}
	gr, gg, gb := rgb(int(basePl.x+60+4), int(basePl.y+16))
	if !isWhite(gr, gg, gb) {
		t.Fatalf("base gap #%02x%02x%02x want white", gr, gg, gb)
	}
	sr, sg, sb := rgb(int(basePl.x+68+30), int(basePl.y+16))
	if !(sb > 180 && sr < 120) {
		t.Fatalf("base second #%02x%02x%02x want blue", sr, sg, sb)
	}

	// Pixel 2: vertical row stacks with a white row gap.
	vr, vg, vb := rgb(int(vertPl.x+30), int(vertPl.y+16))
	if !(vb > 180 && vr < 120) {
		t.Fatalf("vertical first #%02x%02x%02x want blue", vr, vg, vb)
	}
	wr, wg, wb := rgb(int(vertPl.x+30), int(vertPl.y+32+4))
	if !isWhite(wr, wg, wb) {
		t.Fatalf("vertical gap #%02x%02x%02x want white", wr, wg, wb)
	}
	or, og, ob := rgb(int(vertPl.x+30), int(vertPl.y+40+16))
	if !(or > 180 && og > 120) {
		t.Fatalf("vertical second #%02x%02x%02x want orange", or, og, ob)
	}

	// Pixel 3: separator cell is gray, children stay red/blue.
	sep := sepSpace.SeparatorAt(0)
	sepOff := sep.Offset()
	pr, pg, pb := rgb(int(sepPl.x+sepOff.X+4), int(sepPl.y+sepOff.Y+8))
	if !(pr > 110 && pr < 200 && pg > 110 && pg < 200 && pb > 110 && pb < 200) {
		t.Fatalf("separator #%02x%02x%02x want gray", pr, pg, pb)
	}

	// Pixel 4: compact double has no white seam; shared edge stays blue.
	cr, cg, cb := rgb(int(compact2Pl.x+59+4), int(compact2Pl.y+16))
	if !(cb > 180 && cr < 120) {
		t.Fatalf("compact second #%02x%02x%02x want blue", cr, cg, cb)
	}
	jr, jg, jb := rgb(int(compact2Pl.x+59), int(compact2Pl.y+16))
	if isWhite(jr, jg, jb) {
		t.Fatalf("compact seam #%02x%02x%02x looks white, want overlap", jr, jg, jb)
	}

	// Pixel 5: align-center short box is vertically centered (tall 40).
	aOff, aSz := offsets(alignC), sizes(alignC)
	shortMid := alignPl.y + aOff[0].Y + aSz[0].Height/2
	tallMid := alignPl.y + aOff[1].Y + aSz[1].Height/2
	if math.Abs(shortMid-tallMid) > fx.Tolerance {
		t.Fatalf("align mids %v vs %v", shortMid, tallMid)
	}
	_ = compact3Pl
	_ = vcompactPl
	_ = wrapPl
	_ = alignPl
	_ = sepPl

	path := filepath.Join("testdata", "showcase_space.png")
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
		t.Fatalf("showcase bounds %v want %v (regen with UPDATE_GOLDEN=1)", got.Bounds(), want.Bounds())
	}
	maxDiff := uint32(spec.Tolerance.MaxDiff * 257)
	hardCap := uint32(spec.Tolerance.HardCap * 257)
	bad := 0
	total := got.Bounds().Dx() * got.Bounds().Dy()
	for yy := 0; yy < got.Bounds().Dy(); yy++ {
		for xx := 0; xx < got.Bounds().Dx(); xx++ {
			r1, g1, b1, a1 := got.At(xx, yy).RGBA()
			r2, g2, b2, a2 := want.At(xx, yy).RGBA()
			m := max4Show(diffShow(r1, r2), diffShow(g1, g2), diffShow(b1, b2), diffShow(a1, a2))
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
