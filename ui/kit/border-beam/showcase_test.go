package border_beam_test

import (
	"encoding/json"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	borderbeam "github.com/energye/gpui/ui/kit/border-beam"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type borderBeamShowcaseSpec struct {
	CanvasW      int       `json:"canvasW"`
	Margin       float64   `json:"margin"`
	Gap          float64   `json:"gap"`
	ColGap       float64   `json:"colGap"`
	ChildW       float64   `json:"childW"`
	ChildH       float64   `json:"childH"`
	BasicTick    float64   `json:"basicTick"`
	HoverTick    float64   `json:"hoverTick"`
	CustomTick   float64   `json:"customTick"`
	ColorTick    float64   `json:"colorTick"`
	TrioTick     float64   `json:"trioTick"`
	CustomRadius float64   `json:"customRadius"`
	Durations    []float64 `json:"durations"`
	Sizes        []float64 `json:"sizes"`
	ThickWidth   float64   `json:"thickWidth"`
	OutsetWide   float64   `json:"outsetWide"`
	Tolerance    struct {
		MaxDiff float64 `json:"maxDiff"`
		BadFrac float64 `json:"badFrac"`
		HardCap float64 `json:"hardCap"`
	} `json:"tolerance"`
	P0Examples []string `json:"p0Examples"`
}

func loadBorderBeamShowcaseSpec(t *testing.T) borderBeamShowcaseSpec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "showcase_spec.json"))
	if err != nil {
		t.Fatalf("read showcase_spec.json: %v", err)
	}
	var s borderBeamShowcaseSpec
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse showcase spec: %v", err)
	}
	if s.CanvasW <= 0 || s.Gap < 0 || s.ChildW <= 0 || s.ChildH <= 0 {
		t.Fatalf("bad showcase spec %+v", s)
	}
	if s.Tolerance.MaxDiff <= 0 || s.Tolerance.HardCap <= 0 {
		t.Fatalf("bad showcase tolerance %+v", s.Tolerance)
	}
	if len(s.P0Examples) != 7 || len(s.Durations) != 3 || len(s.Sizes) != 3 {
		t.Fatalf("bad showcase matrix %+v", s)
	}
	return s
}

// TestBorderBeam_Showcase_MainPaths lays §6.8 P0 main paths on one canvas:
// basic / hover(hovered) / custom-container(radius 8) / customized-color
// gradient + single-red / duration trio(3/6/12) / size trio(56/100/160) /
// line-width trio(default/thick/outset). The beam has no own text; children
// are plain white containers, so no placeholder bars are drawn.
// Three evidences: logic probe (defaults/visibility/phases/layout/boundary/
// theme), pixel assertions (red beam pixel + blue edge ink + white interior),
// golden file compare (tolerance from testdata/showcase_spec.json).
// Regenerate with UPDATE_GOLDEN=1.
func TestBorderBeam_Showcase_MainPaths(t *testing.T) {
	spec := loadBorderBeamShowcaseSpec(t)

	W := float64(spec.CanvasW)
	margin, gap, colGap := spec.Margin, spec.Gap, spec.ColGap
	cw, ch := spec.ChildW, spec.ChildH

	mk := func() *rendering.RenderColorBox {
		// Light-gray container so the decorated box reads on the white
		// canvas; the beam remains the only saturated ink.
		return rendering.NewRenderColorBox(cw, ch, 0.97, 0.97, 0.97, 1)
	}

	// R1 basic.tsx: default beam.
	basic := borderbeam.NewBorderBeam(mk())
	basic.Tick(spec.BasicTick)

	// R2 hover.tsx: hidden until hover; showcase paints the hovered state.
	hover := borderbeam.NewBorderBeam(mk())
	hover.SetShowOnHover(true)
	if hover.IsBeamVisible() {
		t.Fatal("hover-only beam must hide before hover")
	}
	hover.SetHovered(true)
	if !hover.IsBeamVisible() {
		t.Fatal("hovered beam should show")
	}
	hover.Tick(spec.HoverTick)
	hover.SetHovered(false)
	if hover.IsBeamVisible() {
		t.Fatal("unhovered beam should hide again")
	}
	hover.SetHovered(true)
	hover.Tick(0) // re-dirty without moving phase

	// R3 custom-container.tsx: explicit radius (Card-style 8).
	custom := borderbeam.NewBorderBeam(mk())
	custom.SetBorderRadius(spec.CustomRadius)
	custom.Tick(spec.CustomTick)

	// R4 customized-color.tsx: gradient stops + single color.
	grad := borderbeam.NewBorderBeam(mk())
	grad.SetColorStops(
		borderbeam.BorderBeamColorStop{Color: render.Hex("#ff4d4f"), Percent: 0},
		borderbeam.BorderBeamColorStop{Color: render.Hex("#faad14"), Percent: 50},
		borderbeam.BorderBeamColorStop{Color: render.Hex("#1677ff"), Percent: 100},
	)
	grad.Tick(spec.ColorTick)
	single := borderbeam.NewBorderBeam(mk())
	single.SetColor(render.RGBA{R: 1, G: 0, B: 0, A: 1})
	single.SetLineWidth(4)
	single.SetSize(80)
	single.Tick(spec.ColorTick)

	// R5 duration.tsx: 3/6/12 trio, same dt so phases spread inversely.
	var durs []*borderbeam.BorderBeam
	for _, d := range spec.Durations {
		b := borderbeam.NewBorderBeam(mk())
		b.SetDuration(d)
		b.Tick(spec.TrioTick)
		durs = append(durs, b)
	}

	// R6 size.tsx: segment-length trio.
	var sizes []*borderbeam.BorderBeam
	for _, s := range spec.Sizes {
		b := borderbeam.NewBorderBeam(mk())
		b.SetSize(s)
		b.Tick(spec.TrioTick)
		sizes = append(sizes, b)
	}

	// R7 line-width.tsx + outset(P0): default / thick / outset-wide.
	thin := borderbeam.NewBorderBeam(mk())
	thin.Tick(spec.TrioTick)
	thick := borderbeam.NewBorderBeam(mk())
	thick.SetLineWidth(spec.ThickWidth)
	thick.Tick(spec.TrioTick)
	wide := borderbeam.NewBorderBeam(mk())
	wide.SetOutset(spec.OutsetWide)
	wide.Tick(spec.TrioTick)

	// Logic probe before paint.
	if basic.ResolvedDuration() != 6 || basic.ResolvedSize() != 100 || basic.ResolvedLineWidth() != 1 {
		t.Fatalf("basic defaults d=%v s=%v w=%v", basic.ResolvedDuration(), basic.ResolvedSize(), basic.ResolvedLineWidth())
	}
	if basic.ResolvedOutset() != 0 {
		t.Fatalf("basic outset=%v want 0", basic.ResolvedOutset())
	}
	if math.Abs(basic.Phase()-spec.BasicTick/6) > 1e-9 {
		t.Fatalf("basic phase=%v want %v", basic.Phase(), spec.BasicTick/6)
	}
	if math.Abs(custom.ResolvedBorderRadius()-spec.CustomRadius) > 1e-9 {
		t.Fatalf("custom radius=%v want %v", custom.ResolvedBorderRadius(), spec.CustomRadius)
	}
	if len(grad.ResolvedColorStops()) != 3 {
		t.Fatalf("gradient stops=%d want 3", len(grad.ResolvedColorStops()))
	}
	if st := single.ResolvedColorStops(); len(st) != 1 || st[0].Color != (render.RGBA{R: 1, G: 0, B: 0, A: 1}) {
		t.Fatalf("single stops=%+v want 1 red", st)
	}
	for i, d := range spec.Durations {
		if durs[i].ResolvedDuration() != d {
			t.Fatalf("duration[%d]=%v want %v", i, durs[i].ResolvedDuration(), d)
		}
	}
	if !(durs[0].Phase() > durs[1].Phase() && durs[1].Phase() > durs[2].Phase()) {
		t.Fatalf("duration phases %v/%v/%v must descend (faster first)", durs[0].Phase(), durs[1].Phase(), durs[2].Phase())
	}
	for i, s := range spec.Sizes {
		if sizes[i].ResolvedSize() != s {
			t.Fatalf("size[%d]=%v want %v", i, sizes[i].ResolvedSize(), s)
		}
	}
	if thick.ResolvedLineWidth() != spec.ThickWidth {
		t.Fatalf("thick width=%v want %v", thick.ResolvedLineWidth(), spec.ThickWidth)
	}
	if wide.ResolvedOutset() != spec.OutsetWide {
		t.Fatalf("wide outset=%v want %v", wide.ResolvedOutset(), spec.OutsetWide)
	}
	// Default beam color follows the Theme primary (not a fixed color).
	toks := theme.Default.Current()
	head := basic.ResolvedColorStops()[0].Color
	wantHead := render.RGBA{R: toks.ColorPrimary.R, G: toks.ColorPrimary.G, B: toks.ColorPrimary.B, A: toks.ColorPrimary.A}
	if head != wantHead {
		t.Fatalf("default head=%+v want theme primary %+v", head, wantHead)
	}
	// Hotspot rule: host + beam layer are repaint boundaries (no long window).
	if !basic.Node().IsRepaintBoundary() {
		t.Fatal("host must be a repaint boundary (anim-over-static)")
	}
	kids := basic.Node().Children()
	if len(kids) == 0 || !kids[len(kids)-1].IsRepaintBoundary() {
		t.Fatal("beam layer must be a repaint boundary")
	}

	// Layout rows; Layout/Node must stay non-zero (true-size chain).
	type placed struct {
		b *borderbeam.BorderBeam
		x float64
		y float64
		w float64
		h float64
	}
	var items []placed
	y := margin
	layoutSingle := func(b *borderbeam.BorderBeam) placed {
		t.Helper()
		sz := b.Layout(rendering.Loose(1000, 1000))
		if math.Abs(sz.Width-cw) > 0.5 || math.Abs(sz.Height-ch) > 0.5 {
			t.Fatalf("layout=%vx%v want %vx%v", sz.Width, sz.Height, cw, ch)
		}
		ns := b.Node().Size()
		if ns.Width <= 0 || ns.Height <= 0 {
			t.Fatalf("node size=%v want >0", ns)
		}
		if !b.IsBeamVisible() {
			t.Fatal("showcase beam must be visible")
		}
		p := placed{b: b, x: margin, y: y, w: sz.Width, h: sz.Height}
		items = append(items, p)
		y += sz.Height + gap
		return p
	}
	layoutRow := func(row []*borderbeam.BorderBeam) {
		t.Helper()
		x := margin
		maxH := 0.0
		type sized struct{ w, h float64 }
		ss := make([]sized, len(row))
		for i, b := range row {
			sz := b.Layout(rendering.Loose(1000, 1000))
			if math.Abs(sz.Width-cw) > 0.5 || math.Abs(sz.Height-ch) > 0.5 {
				t.Fatalf("row layout=%vx%v want %vx%v", sz.Width, sz.Height, cw, ch)
			}
			ns := b.Node().Size()
			if ns.Width <= 0 || ns.Height <= 0 {
				t.Fatalf("row node size=%v want >0", ns)
			}
			if !b.IsBeamVisible() {
				t.Fatal("showcase beam must be visible")
			}
			ss[i] = sized{w: sz.Width, h: sz.Height}
			if sz.Height > maxH {
				maxH = sz.Height
			}
		}
		for i, b := range row {
			items = append(items, placed{b: b, x: x, y: y, w: ss[i].w, h: ss[i].h})
			x += ss[i].w + colGap
		}
		y += maxH + gap
	}

	basicP := layoutSingle(basic)
	layoutSingle(hover)
	layoutSingle(custom)
	layoutRow([]*borderbeam.BorderBeam{grad, single})
	layoutRow(durs)
	layoutRow(sizes)
	layoutRow([]*borderbeam.BorderBeam{thin, thick, wide})
	H := y - gap + margin

	var singleP placed
	for _, it := range items {
		if it.b == single {
			singleP = it
		}
	}
	if singleP.b == nil {
		t.Fatal("single item missing")
	}

	dc := render.NewContext(int(W), int(H))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	for _, it := range items {
		it.b.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(it.x, it.y))
	}
	got := dc.Image()

	// Pixel assertion 1: single-red top edge is solid red. Phase 0.15 puts
	// the 80px segment on the top edge (local x in [~60,~140], y=0).
	if r, g, bl, _ := got.At(int(singleP.x+110), int(singleP.y)).RGBA(); r < 0x8000 || g > 0x8000 || bl > 0x8000 {
		r2, g2, b2, _ := got.At(int(singleP.x+110), int(singleP.y)).RGBA()
		t.Fatalf("red beam pixel #%04x%04x%04x want red", r2, g2, b2)
	}
	// Pixel assertion 2: basic top edge carries blue beam ink (thin AA line
	// counts as ink when R drops and B stays high; white shell never does).
	blueInk := 0
	ey := int(basicP.y)
	for xx := int(basicP.x + 10); xx < int(basicP.x+cw-10); xx++ {
		r, g, bl, _ := got.At(xx, ey).RGBA()
		r8, b8 := r/257, bl/257
		_ = g
		if r8 < 200 && b8 > 200 {
			blueInk++
		}
	}
	if blueInk < 20 {
		t.Fatalf("basic edge blue ink=%d want >=20 (beam invisible?)", blueInk)
	}
	// Pixel assertion 3: interior stays container gray (beam is border-only).
	if r, g, bl, _ := got.At(int(basicP.x+cw/2), int(basicP.y+ch/2)).RGBA(); r/257 < 240 || g/257 < 240 || bl/257 < 240 {
		t.Fatalf("interior #%02x%02x%02x want white", r/257, g/257, bl/257)
	}

	path := filepath.Join("testdata", "showcase_border_beam.png")
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
			m := max4BeamShow(diffBeamShow(r1, r2), diffBeamShow(g1, g2), diffBeamShow(b1, b2), diffBeamShow(a1, a2))
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

func diffBeamShow(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func max4BeamShow(vs ...uint32) uint32 {
	m := vs[0]
	for _, v := range vs[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
