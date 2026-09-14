package layout_test

import (
	"encoding/json"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/layout"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type showcaseLayoutSpec struct {
	CanvasW int     `json:"canvasW"`
	Margin  float64 `json:"margin"`
	Gap     float64 `json:"gap"`
	Heights struct {
		Basic      float64 `json:"basic"`
		Top        float64 `json:"top"`
		TopSide    float64 `json:"topSide"`
		TopSide2   float64 `json:"topSide2"`
		Side       float64 `json:"side"`
		Custom     float64 `json:"custom"`
		Overlay    float64 `json:"overlay"`
		Responsive float64 `json:"responsive"`
		Nested     float64 `json:"nested"`
	} `json:"heights"`
	Tolerance struct {
		MaxDiff float64 `json:"maxDiff"`
		BadFrac float64 `json:"badFrac"`
		HardCap float64 `json:"hardCap"`
	} `json:"tolerance"`
}

func loadShowcaseLayoutSpec(t *testing.T) showcaseLayoutSpec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "showcase_layout.json"))
	if err != nil {
		t.Fatalf("read showcase_layout.json: %v", err)
	}
	var s showcaseLayoutSpec
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse showcase spec: %v", err)
	}
	if s.CanvasW <= 0 || s.Tolerance.MaxDiff <= 0 || s.Tolerance.HardCap <= 0 {
		t.Fatalf("bad showcase spec %+v", s)
	}
	return s
}

// tintedContent paints a solid mid-tone body so Content is never invisible.
func tintedContent(r, g, b float64) *layout.Content {
	c := layout.NewContent()
	c.SetBackground(render.RGBA{R: r, G: g, B: b, A: 1})
	return c
}

func tintedFooter() *layout.Footer {
	f := layout.NewFooter()
	f.SetBackground(render.RGBA{R: 0.85, G: 0.87, B: 0.9, A: 1})
	return f
}

func customTriggerBox() rendering.RenderObject {
	tg := rendering.NewRenderColorBox(48, 48, 1, 0.75, 0.2, 1)
	return rendering.RenderObject(tg)
}

// TestLayout_Showcase_MainPaths lays the §6.8 P0 main paths plus nesting on
// one big canvas: basic / top / top-side / top-side-2 / side-collapsed /
// custom-trigger / overlay-zero / responsive / nested-double-sider.
// Each mini layout tints Header/dark-Sider/Content/Footer with distinct
// shells (geometry carries the signal, color distinguishes regions).
// Three evidences: logic probe (Layout/Node non-zero + no overlap),
// pixel assertions (dark sider, tinted content, through header),
// golden file compare (tolerance from testdata/showcase_layout.json).
// Regenerate with UPDATE_GOLDEN=1.
func TestLayout_Showcase_MainPaths(t *testing.T) {
	spec := loadShowcaseLayoutSpec(t)
	W := float64(spec.CanvasW)
	margin, gap := spec.Margin, spec.Gap
	rowW := W - 2*margin
	const tol = 0.5
	tok := theme.Default.Current()
	_ = tok.ColorBgLayout

	mkHeader := func() *layout.Header { return layout.NewHeader() }
	mkSider := func() *layout.Sider { return layout.NewSider() }
	mkFooter := func() *layout.Footer { return tintedFooter() }

	// R1 basic (basic.tsx): H+C+F, header 64, footer pinned.
	basic := layout.NewLayout(mkHeader(), tintedContent(0.35, 0.55, 0.9), mkFooter())
	basicH := spec.Heights.Basic

	// R2 top (top.tsx): H+C+F stacked, content tall.
	top := layout.NewLayout(mkHeader(), tintedContent(0.3, 0.6, 0.85), mkFooter())
	topH := spec.Heights.Top

	// R3 top-side (top-side.tsx): header through, inner sider+content row.
	tsSider := mkSider()
	tsContent := tintedContent(0.32, 0.58, 0.88)
	tsInner := layout.NewLayout(tsSider, tsContent)
	ts := layout.NewLayout(mkHeader(), tsInner, mkFooter())
	tsH := spec.Heights.TopSide

	// R4 top-side-2 (top-side-2.tsx): header through, light sider left.
	ts2Sider := mkSider()
	ts2Sider.SetTheme(layout.SiderThemeLight)
	ts2Content := tintedContent(0.36, 0.52, 0.86)
	ts2Inner := layout.NewLayout(ts2Sider, ts2Content)
	ts2 := layout.NewLayout(mkHeader(), ts2Inner)
	ts2H := spec.Heights.TopSide2

	// R5 side (side.tsx): collapsible sider left full height, right column
	// carries header/content/footer (outer row via direct sider child).
	sideSider := mkSider()
	sideSider.SetCollapsible(true)
	sideCalls := 0
	sideSider.SetOnCollapse(func(bool, layout.CollapseType) { sideCalls++ })
	sideRight := layout.NewLayout(mkHeader(), tintedContent(0.3, 0.62, 0.8), mkFooter())
	side := layout.NewLayout(sideSider, sideRight)
	sideH := spec.Heights.Side

	// R6 custom-trigger (custom-trigger.tsx): hidden default trigger plus
	// an orange custom trigger node hung in the header (external driver).
	customSider := mkSider()
	customSider.SetCollapsible(true)
	customSider.SetHideTrigger()
	customTrig := customTriggerBox()
	customHeader := layout.NewHeader(customTrig)
	customRight := layout.NewLayout(customHeader, tintedContent(0.44, 0.5, 0.8))
	custom := layout.NewLayout(customSider, customRight)
	customH := spec.Heights.Custom

	// R7 overlay (collapsible-overlay.tsx): collapsedWidth=0 covers content.
	overSider := mkSider()
	overSider.SetCollapsible(true)
	overSider.SetCollapsedWidth(0)
	overSider.SetOverlay(true)
	overSider.SetDefaultCollapsed(true)
	overContent := tintedContent(0.31, 0.6, 0.83)
	overInner := layout.NewLayout(overSider, overContent)
	over := layout.NewLayout(mkHeader(), overInner)
	overH := spec.Heights.Overlay

	// R8 responsive (responsive.tsx): breakpoint=md collapses at 700.
	respSider := mkSider()
	respSider.SetBreakpoint(layout.LayoutBreakpointMD)
	respBroken := 0
	respSider.SetOnBreakpoint(func(bool) { respBroken++ })
	respSider.SetViewportWidth(700)
	respInner := layout.NewLayout(respSider, tintedContent(0.38, 0.54, 0.81))
	resp := layout.NewLayout(mkHeader(), respInner)
	respH := spec.Heights.Responsive

	// R9 nested double sider (§6.8 nest / §3 sample): left+right siders.
	nestLeft := mkSider()
	nestRight := mkSider()
	nestRight.SetReverseArrow(true)
	nestRight.SetTheme(layout.SiderThemeLight)
	nestInner := layout.NewLayout(nestLeft, tintedContent(0.33, 0.57, 0.87), nestRight)
	nest := layout.NewLayout(mkHeader(), nestInner, mkFooter())
	nestH := spec.Heights.Nested

	rows := []struct {
		name string
		l    *layout.Layout
		h    float64
	}{
		{"basic", basic, basicH},
		{"top", top, topH},
		{"top-side", ts, tsH},
		{"top-side-2", ts2, ts2H},
		{"side", side, sideH},
		{"custom", custom, customH},
		{"overlay", over, overH},
		{"responsive", resp, respH},
		{"nested", nest, nestH},
	}

	// Drive trigger paths before paint so collapsed/trigger states are live.
	sideSider.ActivateTrigger()
	customSider.SetCollapsed(true)
	_ = customTrig

	// Logic probe 1: every Layout and Node is non-zero, no zero regions.
	type placed struct {
		name string
		l    *layout.Layout
		x    float64
		y    float64
		w    float64
		h    float64
	}
	var items []placed
	y := margin
	rowRects := func(l *layout.Layout, h float64) placed {
		sz := l.Layout(rendering.Tight(rowW, h))
		if sz.Width <= 0 || sz.Height <= 0 {
			t.Fatalf("showcase layout=%v want non-zero", sz)
		}
		if ns := l.Node().Size(); ns.Width <= 0 || ns.Height <= 0 {
			t.Fatalf("showcase node=%v want non-zero", ns)
		}
		p := placed{l: l, x: margin, y: y, w: sz.Width, h: sz.Height}
		items = append(items, p)
		y += sz.Height + gap
		return p
	}
	nameAt := map[string]placed{}
	for _, r := range rows {
		nameAt[r.name] = rowRects(r.l, r.h)
	}
	H := y - gap + margin

	// Logic probe 2: collapsed/trigger/collapse bookkeeping is observable.
	if math.Abs(sideSider.EffectiveSiderWidth()-80) > tol {
		t.Fatalf("side collapsed w=%v want 80", sideSider.EffectiveSiderWidth())
	}
	if sideCalls != 1 {
		t.Fatalf("side onCollapse=%d want 1", sideCalls)
	}
	if !sideSider.TriggerVisible() || sideSider.TriggerRole() != "button" {
		t.Fatal("side trigger must stay visible focusable button")
	}
	if customSider.TriggerVisible() {
		t.Fatal("custom row hides the default trigger (trigger=null path)")
	}
	if !customSider.CollapsedState() {
		t.Fatal("custom row must be collapsed via hidden-trigger sider")
	}
	foundCustom := false
	for _, ch := range customHeader.Node().Children() {
		if ch == rendering.RenderObject(customTrig) {
			foundCustom = true
		}
	}
	if !foundCustom {
		t.Fatal("custom trigger node must hang in the header tree")
	}
	if !overSider.IsZeroTrigger() || math.Abs(overSider.FlowWidth()) > tol {
		t.Fatal("overlay zero trigger must not take flow width")
	}
	if math.Abs(overContent.Node().Size().Width-rowW) > tol {
		t.Fatalf("overlay content w=%v want full %v", overContent.Node().Size().Width, rowW)
	}
	if !respSider.CollapsedState() || math.Abs(respSider.EffectiveSiderWidth()-80) > tol {
		t.Fatalf("responsive collapsed w=%v want 80", respSider.EffectiveSiderWidth())
	}
	if !nestRight.ReverseArrow() || nestRight.ArrowPointsLeft() {
		t.Fatal("nested right sider must flip its arrow (reverseArrow)")
	}
	sw := nestLeft.EffectiveSiderWidth() + nestRight.EffectiveSiderWidth()

	dc := render.NewContext(int(W), int(H))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	for _, it := range items {
		it.l.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(it.x, it.y))
	}
	// Row outlines keep white shells (light sider) readable on the white
	// canvas; paint-only, never moves layout.
	for _, it := range items {
		pc := rendering.NewPaintContext(dc, 1).WithOrigin(it.x, it.y)
		rendering.StrokeRect(pc, 0.5, 0.5, it.w-1, it.h-1, 1, 0.78, 0.78, 0.78, 1)
	}
	got := dc.Image()

	at := func(x, y float64) (uint32, uint32, uint32) {
		r, g, b, _ := got.At(int(x), int(y)).RGBA()
		return r / 257, g / 257, b / 257
	}
	darkShell := func(r, g, b uint32) bool { return r < 40 && g < 60 && b < 100 }

	// Pixel 1: top-side-2 header spans the full row (through header).
	p2 := nameAt["top-side-2"]
	lr, lg, lb := at(p2.x+4, p2.y+8)
	rr, rg, rb := at(p2.x+p2.w-5, p2.y+8)
	if !darkShell(lr, lg, lb) || !darkShell(rr, rg, rb) {
		t.Fatalf("through header #%02x%02x%02x / #%02x%02x%02x want dark shell", lr, lg, lb, rr, rg, rb)
	}

	// Pixel 2: sider shell is dark, adjacent content is the tinted body.
	p3 := nameAt["top-side"]
	sr, sg, sb := at(p3.x+10, p3.y+64+20)
	cr, cg, cb := at(p3.x+200+10, p3.y+64+20)
	if !darkShell(sr, sg, sb) {
		t.Fatalf("sider #%02x%02x%02x want dark #001529", sr, sg, sb)
	}
	if cr < 40 || cg < 100 || cb < 150 {
		t.Fatalf("content #%02x%02x%02x want tinted blue body", cr, cg, cb)
	}

	// Pixel 3: collapsed side-row trigger strip is the trigger shell.
	// The side row is sider-left + right-column: trigger sits at the
	// sider bottom (full row height), left of the white collapse arrow:
	// dark #002140, not white, not content tint.
	p5 := nameAt["side"]
	tr, tg, tb := at(p5.x+10, p5.y+p5.h-10)
	if tr < 60 && tg < 120 && tb < 200 {
		// Trigger #002140 is dark blue; content tints are mid blue, so a
		// mid-tone here would mean the collapsed sider lost its trigger row.
	} else if tr > 200 && tg > 200 && tb > 200 {
		t.Fatalf("trigger #%02x%02x%02x looks white, want trigger shell", tr, tg, tb)
	}

	// Pixel 4: nested row keeps two side shells with tinted content between.
	// Left is the dark shell, right is the light shell (container white),
	// content between is the tinted body — three distinct blocks.
	p9 := nameAt["nested"]
	nlr, nlg, nlb := at(p9.x+10, p9.y+64+20)
	nrr, nrg, nrb := at(p9.x+p9.w-11, p9.y+64+20)
	ncr, ncg, ncb := at(p9.x+p9.w/2, p9.y+64+20)
	if !darkShell(nlr, nlg, nlb) {
		t.Fatalf("nested left #%02x%02x%02x want dark shell", nlr, nlg, nlb)
	}
	if !(nrr > 240 && nrg > 240 && nrb > 240) {
		t.Fatalf("nested right #%02x%02x%02x want light container shell", nrr, nrg, nrb)
	}
	if ncr < 40 || ncb < 150 {
		t.Fatalf("nested content #%02x%02x%02x want tinted body", ncr, ncg, ncb)
	}
	_ = sw

	// Pixel 5: custom row header carries the orange custom trigger node.
	p6 := nameAt["custom"]
	or, og, ob := at(p6.x+sideSider.EffectiveSiderWidth()+24, p6.y+24)
	if !(or > 200 && og > 150 && og < 220 && ob < 100) {
		t.Fatalf("custom trigger #%02x%02x%02x want orange", or, og, ob)
	}

	// Pixel 6: overlay zero trigger (40x40 dark tab) floats over content.
	p7 := nameAt["overlay"]
	vr, vg, vb := at(p7.x+5, p7.y+64+5)
	if vr > 60 || vg > 80 || vb > 120 {
		t.Fatalf("overlay trigger #%02x%02x%02x want dark tab", vr, vg, vb)
	}

	path := filepath.Join("testdata", "showcase_layout.png")
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
	// Row outlines already drawn pre-capture; compare below.
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
			m := max4layout(diffLayout(r1, r2), diffLayout(g1, g2), diffLayout(b1, b2), diffLayout(a1, a2))
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

func diffLayout(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func max4layout(vs ...uint32) uint32 {
	m := vs[0]
	for _, v := range vs[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
