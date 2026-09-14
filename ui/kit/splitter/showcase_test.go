package splitter_test

import (
	"encoding/json"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/kit/splitter"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type showcaseSpec struct {
	CanvasW      float64 `json:"canvasW"`
	Margin       float64 `json:"margin"`
	Gap          float64 `json:"gap"`
	RowH         float64 `json:"rowH"`
	TallH        float64 `json:"tallH"`
	CustomBar    string  `json:"customBar"`
	CustomHandle string  `json:"customHandle"`
	Tolerance    struct {
		MaxDiff float64 `json:"maxDiff"`
		BadFrac float64 `json:"badFrac"`
		HardCap float64 `json:"hardCap"`
	} `json:"tolerance"`
}

func loadShowcaseSpec(t *testing.T) showcaseSpec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "showcase_spec.json"))
	if err != nil {
		t.Fatalf("read showcase_spec.json: %v", err)
	}
	var s showcaseSpec
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse showcase spec: %v", err)
	}
	if s.CanvasW <= 0 || s.RowH <= 0 || s.TallH <= 0 {
		t.Fatalf("bad showcase spec %+v", s)
	}
	// Task gate: compare tolerance must be 12 / 0.5% / 64.
	if s.Tolerance.MaxDiff != 12 || s.Tolerance.BadFrac != 0.005 || s.Tolerance.HardCap != 64 {
		t.Fatalf("tolerance %+v want 12/0.005/64", s.Tolerance)
	}
	if s.CustomBar == "" || s.CustomHandle == "" {
		t.Fatalf("custom colors missing %+v", s)
	}
	return s
}

func loadShowcaseFace(t *testing.T) text.Face {
	t.Helper()
	text.ClearSystemFontPaths()
	face, desc, err := rendering.TryLoadDefaultFace(14)
	if err != nil || face == nil {
		t.Skipf("showcase needs a system face for real glyphs: %v", err)
	}
	t.Logf("showcase face: %s", desc)
	return face
}

// TestSplitter_Showcase_MainPaths lays §6.8 main paths on one big canvas:
// basic / controlled / vertical / collapsible / collapsibleIcon / multiple /
// group-nested / lazy-preview plus P1 custom-style / non-resizable-focus /
// collapsed-destroy rows. Three evidences: logic probe (orientation/sizes/
// bars), pixel assertions (red/blue panels, neutral bar, custom green bar,
// dark title ink), golden file compare (tolerance from
// testdata/showcase_spec.json). Regenerate with UPDATE_GOLDEN=1.
func TestSplitter_Showcase_MainPaths(t *testing.T) {
	spec := loadShowcaseSpec(t)
	face := loadShowcaseFace(t)

	W := spec.CanvasW
	margin, gap := spec.Margin, spec.Gap
	rowW := W - 2*margin
	rowH, tallH := spec.RowH, spec.TallH

	mkPanel := func(r, g, b float64) *splitter.SplitterPanel {
		return splitter.NewSplitterPanel(rendering.NewRenderColorBox(10, 10, r, g, b, 1))
	}
	mkTitle := func(s string) *rendering.RenderText {
		rt := rendering.NewRenderText(s)
		rt.FontSize = 14
		rt.R, rt.G, rt.B, rt.A = 0, 0, 0, 0.88
		rt.SetFace(face)
		return rt
	}
	toRGBA := func(c theme.Color) render.RGBA {
		return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
	}
	customBar := toRGBA(theme.Hex(spec.CustomBar))
	customHandle := toRGBA(theme.Hex(spec.CustomHandle))

	// R1 basic (size.tsx): 50/50 horizontal.
	basic := splitter.NewSplitter(mkPanel(0.9, 0.2, 0.2), mkPanel(0.2, 0.4, 0.9))
	basic.SetWidth(rowW)
	basic.SetHeight(rowH)

	// R2 controlled (control.tsx): percent controlled 40/60.
	ctrlL := mkPanel(0.13, 0.76, 0.76)
	ctrlR := mkPanel(0.98, 0.55, 0.09)
	ctrlL.SetSizePercent(40)
	ctrlR.SetSizePercent(60)
	controlled := splitter.NewSplitter(ctrlL, ctrlR)
	controlled.SetWidth(rowW)
	controlled.SetHeight(rowH)
	var ctrlNotes int
	controlled.OnResize(func(v []float64) { ctrlNotes++ })

	// R3 vertical (vertical.tsx).
	vert := splitter.NewSplitter(mkPanel(0.32, 0.77, 0.1), mkPanel(0.45, 0.18, 0.82))
	vert.SetOrientation(splitter.Vertical)
	vert.SetWidth(rowW)
	vert.SetHeight(tallH)

	// R4 collapsible (collapsible.tsx): left foldable.
	colL := mkPanel(0.98, 0.86, 0.08)
	colL.SetCollapsible(true)
	collapsible := splitter.NewSplitter(colL, mkPanel(0.4, 0.5, 0.7))
	collapsible.SetWidth(rowW)
	collapsible.SetHeight(rowH)

	// R5 collapsibleIcon (collapsibleIcon.tsx): both sides, always icons.
	iconL := mkPanel(0.96, 0.35, 0.67)
	iconR := mkPanel(0.13, 0.76, 0.76)
	iconL.SetCollapsible(true)
	iconL.SetShowCollapsibleIcon(splitter.CollapsibleIconAlways)
	iconR.SetCollapsible(true)
	iconR.SetShowCollapsibleIcon(splitter.CollapsibleIconAlways)
	collapsibleIcon := splitter.NewSplitter(iconL, iconR)
	collapsibleIcon.SetWidth(rowW)
	collapsibleIcon.SetHeight(rowH)

	// R6 multiple (multiple.tsx): three panels.
	multi := splitter.NewSplitter(
		mkPanel(0.9, 0.2, 0.2),
		mkPanel(0.2, 0.9, 0.2),
		mkPanel(0.2, 0.4, 0.9),
	)
	multi.SetWidth(rowW)
	multi.SetHeight(rowH)
	multi.SetBarHover(0, true)

	// R7 group nested (group.tsx): inner lives in outer left panel.
	inner := splitter.NewSplitter(mkPanel(0.9, 0.2, 0.2), mkPanel(0.2, 0.9, 0.2))
	inner.SetWidth(rowW / 2)
	inner.SetHeight(rowH)
	inner.Layout(rendering.Tight(rowW/2, rowH))
	outer := splitter.NewSplitter(
		splitter.NewSplitterPanel(inner.Node()),
		mkPanel(0.2, 0.4, 0.9),
	)
	outer.SetWidth(rowW)
	outer.SetHeight(rowH)

	// R8 lazy (lazy.tsx): preview line at +40, geometry frozen mid-drag.
	lazy := splitter.NewSplitter(mkPanel(0.9, 0.2, 0.2), mkPanel(0.2, 0.4, 0.9))
	lazy.SetLazy(true)
	lazy.SetWidth(rowW)
	lazy.SetHeight(rowH)

	// R9 P1 custom style (customize.tsx): green bar + thick handle + squares.
	cusL := mkPanel(0.85, 0.85, 0.85)
	cusR := mkPanel(0.75, 0.78, 0.85)
	cusL.SetCollapsible(true)
	cusL.SetShowCollapsibleIcon(splitter.CollapsibleIconAlways)
	cusR.SetCollapsible(true)
	cusR.SetShowCollapsibleIcon(splitter.CollapsibleIconAlways)
	custom := splitter.NewSplitter(cusL, cusR)
	custom.SetWidth(rowW)
	custom.SetHeight(rowH)
	custom.SetSemanticStyle(splitter.SemanticBar, splitter.Style{Bg: customBar, UseBg: true})
	custom.SetSemanticStyle(splitter.SemanticHandle, splitter.Style{Bg: customHandle, UseBg: true})
	custom.SetClassNames(map[splitter.SemanticKey]string{
		splitter.SemanticRoot: "my-splitter",
		splitter.SemanticBar:  "my-bar",
	})
	custom.SetDraggerIcon(rendering.NewRenderColorBox(4, 20, 0.2, 0.2, 0.2, 1))
	custom.SetCollapsibleIcons(
		rendering.NewRenderColorBox(6, 6, 0.2, 0.2, 0.2, 1),
		rendering.NewRenderColorBox(6, 6, 0.2, 0.2, 0.2, 1),
	)

	// R10 P0 resizable=false + focus ring (SPL-19/20 look).
	nrL := mkPanel(0.6, 0.6, 0.6)
	nrR := mkPanel(0.7, 0.7, 0.75)
	nrL.SetResizable(false)
	nrR.SetResizable(false)
	noresize := splitter.NewSplitter(nrL, nrR)
	noresize.SetWidth(rowW)
	noresize.SetHeight(rowH)
	noresize.SetAriaLabel("demo splitter")

	// R11 collapsed + destroyOnHidden (folded left, content gone).
	deL := mkPanel(0.9, 0.2, 0.2)
	deL.SetCollapsible(true)
	destroyed := splitter.NewSplitter(deL, mkPanel(0.2, 0.4, 0.9))
	destroyed.SetDestroyOnHidden(true)
	destroyed.SetWidth(rowW)
	destroyed.SetHeight(rowH)

	// Layout all (Exact). Lazy/collapsed/nested need staged gestures.
	type placedSplitter struct {
		s *splitter.Splitter
		x float64
		y float64
		w float64
		h float64
	}
	titles := []string{
		"basic 50/50 (size.tsx)",
		"controlled 40/60 (control.tsx)",
		"vertical (vertical.tsx)",
		"collapsible (collapsible.tsx)",
		"collapsible icons (collapsibleIcon.tsx)",
		"multiple 3 panels (multiple.tsx)",
		"group nested (group.tsx)",
		"lazy preview (lazy.tsx)",
		"P1 custom style (customize.tsx)",
		"resizable=false + focus (SPL-19/20)",
		"collapsed destroy (destroyOnHidden)",
	}
	splitters := []*splitter.Splitter{
		basic, controlled, vert, collapsible, collapsibleIcon,
		multi, outer, lazy, custom, noresize, destroyed,
	}
	heights := []float64{
		rowH, rowH, tallH, rowH, rowH, rowH, rowH, rowH, rowH, rowH, rowH,
	}
	if len(titles) != len(splitters) || len(heights) != len(splitters) {
		t.Fatal("showcase rows mismatch")
	}

	// Staged gestures before final layout.
	// Group: outer first, then sync inner to the real host width.
	outer.Layout(rendering.Tight(rowW, rowH))
	outerSizes := outer.PanelSizes()
	if len(outerSizes) == 2 && outerSizes[0] > 0 {
		inner.SetWidth(outerSizes[0])
		inner.Layout(rendering.Tight(outerSizes[0], rowH))
		outer.Layout(rendering.Tight(rowW, rowH))
	}
	// Lazy: hold a +40 preview (frozen geometry, preview line visible).
	lazy.Layout(rendering.Tight(rowW, rowH))
	lazyBefore := append([]float64(nil), lazy.PanelSizes()...)
	if !lazy.BeginDrag(0) {
		t.Fatal("lazy begin")
	}
	lazy.UpdateDrag(40)
	lazy.Layout(rendering.Tight(rowW, rowH))
	// Collapsed destroy row: fold left.
	destroyed.Layout(rendering.Tight(rowW, rowH))
	destroyed.CollapseAt(0, splitter.CollapseStart)
	destroyed.Layout(rendering.Tight(rowW, rowH))
	// Focus row: focus first bar for the ring.
	noresize.Layout(rendering.Tight(rowW, rowH))
	if !noresize.FocusBar(0) {
		t.Fatal("focus bar")
	}

	// Logic probe before paint: geometry real, bars visible, states wired.
	for i, s := range splitters {
		sz := s.Layout(rendering.Tight(rowW, heights[i]))
		if sz.Width <= 0 || sz.Height <= 0 {
			t.Fatalf("row %d layout=%v want non-zero", i, sz)
		}
		if s.Node() == nil || s.ChromeNode() == nil {
			t.Fatalf("row %d node nil", i)
		}
		if s.BarCount() < 1 {
			t.Fatalf("row %d bars=%d want >=1", i, s.BarCount())
		}
		bar := s.BarNode(0)
		if bar == nil {
			t.Fatalf("row %d bar nil", i)
		}
		if bar.Size().Width <= 0 || bar.Size().Height <= 0 {
			t.Fatalf("row %d bar size=%v want non-zero", i, bar.Size())
		}
	}
	if basic.EffectiveOrientation() != splitter.Horizontal {
		t.Fatal("basic must be horizontal")
	}
	if !vert.IsVertical() {
		t.Fatal("vertical probe")
	}
	if !controlled.IsControlled() {
		t.Fatal("controlled probe")
	}
	if math.Abs(controlled.PanelSizes()[0]-rowW*0.4) > 1 {
		t.Fatalf("controlled sizes=%v want 40 percent", controlled.PanelSizes())
	}
	if multi.BarCount() != 2 {
		t.Fatalf("multiple bars=%d want 2", multi.BarCount())
	}
	if !lazy.Lazy() || math.Abs(lazy.PreviewDelta()-40) > 0.5 {
		t.Fatalf("lazy preview=%v want 40", lazy.PreviewDelta())
	}
	if math.Abs(lazy.PanelSizes()[0]-lazyBefore[0]) > 0.5 {
		t.Fatalf("lazy frozen=%v want %v", lazy.PanelSizes(), lazyBefore)
	}
	if custom.ClassName(splitter.SemanticRoot) != "my-splitter" {
		t.Fatalf("semantic class=%q", custom.ClassName(splitter.SemanticRoot))
	}
	if got := custom.EffectiveBarColor(); got != customBar {
		t.Fatalf("custom bar=%+v want %+v", got, customBar)
	}
	if custom.DraggerIcon() == nil {
		t.Fatal("dragger icon missing")
	}
	if st, ok := custom.SemanticStyle(splitter.SemanticBar); !ok || !st.UseBg {
		t.Fatal("semantic bar style missing")
	}
	if noresize.BarResizable(0) {
		t.Fatal("resizable=false must report non-resizable")
	}
	if !noresize.FocusRingVisible(0) {
		t.Fatal("focus ring must show")
	}
	if !destroyed.IsCollapsed(0) || !destroyed.IsContentDestroyed(0) {
		t.Fatal("collapsed destroy probe")
	}
	if math.Abs(basic.SplitBarSize()-2) > 0.5 ||
		math.Abs(basic.SplitTriggerSize()-6) > 0.5 ||
		math.Abs(basic.SplitBarDraggableSize()-20) > 0.5 {
		t.Fatal("bar metrics 2/6/20 probe")
	}

	// Stack titles + splitters on one canvas.
	type placedTitle struct {
		n *rendering.RenderText
		x float64
		y float64
	}
	var titleItems []placedTitle
	var splitItems []placedSplitter
	y := margin
	for i, s := range splitters {
		rt := mkTitle(titles[i])
		tsz := rt.Layout(rendering.Loose(rowW, 200))
		if tsz.Width <= 0 || tsz.Height <= 0 {
			t.Fatalf("title %d layout=%v", i, tsz)
		}
		titleItems = append(titleItems, placedTitle{n: rt, x: margin, y: y})
		y += tsz.Height + 4
		h := heights[i]
		sz := s.Layout(rendering.Tight(rowW, h))
		splitItems = append(splitItems, placedSplitter{s: s, x: margin, y: y, w: sz.Width, h: sz.Height})
		y += h + gap
	}
	H := y - gap + margin

	dc := render.NewContext(int(W), int(H))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	for _, it := range titleItems {
		it.n.Paint(rendering.NewPaintContext(dc, 1).WithOrigin(it.x, it.y))
	}
	for _, it := range splitItems {
		it.s.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(it.x, it.y))
	}
	// End the held lazy gesture after paint so later tests start idle.
	lazy.EndDrag()
	got := dc.Image()

	// Pixel assertion 1: basic left red / right blue.
	b0 := splitItems[0]
	bsizes := basic.PanelSizes()
	lx := int(b0.x + bsizes[0]/2)
	rx := int(b0.x + bsizes[0] + basic.PanelSizes()[1]/2)
	midY := int(b0.y + b0.h/2)
	if r, g, b, _ := got.At(lx, midY).RGBA(); r < 0xC000 || g > 0x8000 || b > 0x8000 {
		t.Fatalf("basic left #%04x%04x%04x want red", r, g, b)
	}
	if r, g, b, _ := got.At(rx, midY).RGBA(); b < 0x8000 || r > 0x8000 {
		t.Fatalf("basic right #%04x%04x%04x want blue", r, g, b)
	}

	// Pixel assertion 2: basic bar seam is a visible neutral track.
	bx := int(b0.x + bsizes[0])
	if r, g, b, _ := got.At(bx, midY).RGBA(); r < 0x8000 || g < 0x8000 || b < 0x8000 {
		t.Fatalf("basic bar #%04x%04x%04x want neutral track", r, g, b)
	}

	// Pixel assertion 3: P1 custom bar carries the file-driven green.
	// Sample near the bar top (outside the 20px center handle) so the
	// track color shows instead of the handle purple.
	c0 := splitItems[8]
	cx := int(c0.x + custom.PanelSizes()[0])
	cy := int(c0.y + 6)
	cr, cg, cb, _ := got.At(cx, cy).RGBA()
	cr8, cg8, cb8 := cr/257, cg/257, cb/257
	// #52c41a -> (82,196,26); allow AA-adjacent sampling.
	if !(cg8 > 150 && cr8 < 130 && cb8 < 110) {
		t.Fatalf("custom bar #%02x%02x%02x want ~#52c41a", cr8, cg8, cb8)
	}

	// Pixel assertion 4: title zone carries dark glyph ink (real text).
	t0 := titleItems[0]
	dark := 0
	x0, x1 := int(t0.x+4), int(t0.x+320)
	if x1 > int(W)-4 {
		x1 = int(W) - 4
	}
	tsz := t0.n.Size()
	for yy := int(t0.y + 2); yy < int(t0.y+tsz.Height-2); yy++ {
		for xx := x0; xx < x1; xx++ {
			r, g, b, _ := got.At(xx, yy).RGBA()
			r8, g8, b8 := r/257, g/257, b/257
			if r8 < 110 && g8 < 110 && b8 < 110 {
				dark++
			}
		}
	}
	if dark < 30 {
		t.Fatalf("title zone dark=%d want >=30 (real glyphs missing?)", dark)
	}

	path := filepath.Join("testdata", "showcase_splitter.png")
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
			m := max4show(diffShow(r1, r2), diffShow(g1, g2), diffShow(b1, b2), diffShow(a1, a2))
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
	_ = ctrlNotes
}

func diffShow(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func max4show(vs ...uint32) uint32 {
	m := vs[0]
	for _, v := range vs[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
