package masonry_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/kit/masonry"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type masonryFile struct {
	Tolerance         float64            `json:"tolerance"`
	ContainerW        float64            `json:"containerW"`
	Columns           int                `json:"columns"`
	Gutter            float64            `json:"gutter"`
	SingleColumn      int                `json:"singleColumn"`
	Heights           []float64          `json:"heights"`
	BasicHeights      []float64          `json:"basicHeights"`
	ImageHeights      []float64          `json:"imageHeights"`
	DynamicHeights    []float64          `json:"dynamicHeights"`
	ViewportSmall     float64            `json:"viewportSmall"`
	ViewportLarge     float64            `json:"viewportLarge"`
	ResponsiveColumns map[string]int     `json:"responsiveColumns"`
	ResponsiveGutter  map[string]float64 `json:"responsiveGutter"`
}

func loadMasonry(t *testing.T) masonryFile {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "masonry.json"))
	if err != nil {
		t.Fatalf("read masonry.json: %v", err)
	}
	var f masonryFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("parse masonry.json: %v", err)
	}
	if f.Tolerance <= 0 {
		t.Fatal("tolerance must be positive")
	}
	if len(f.Heights) < 3 || len(f.BasicHeights) < 6 {
		t.Fatal("fixture heights too short")
	}
	return f
}

func respColumns(src map[string]int) map[masonry.Breakpoint]int {
	out := map[masonry.Breakpoint]int{}
	for k, v := range src {
		out[masonry.Breakpoint(k)] = v
	}
	return out
}

func respGutter(src map[string]float64) map[masonry.Breakpoint]float64 {
	out := map[masonry.Breakpoint]float64{}
	for k, v := range src {
		out[masonry.Breakpoint(k)] = v
	}
	return out
}

func autoItems(prefix string, heights []float64) []masonry.MasonryItem {
	out := make([]masonry.MasonryItem, len(heights))
	for i, h := range heights {
		out[i] = masonry.MasonryItem{Key: prefix + string(rune('a'+i)), Column: -1, Height: h}
	}
	return out
}

func near(t *testing.T, got, want, tol float64, what string) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Fatalf("%s=%v want %v (tol %v)", what, got, want, tol)
	}
}

func TestMasonry_PRD_MAS01_Defaults(t *testing.T) {
	m := masonry.NewMasonry()
	if m == nil || m.Node() == nil {
		t.Fatal("NewMasonry must not crash")
	}
	if m.EffectiveColumnCount() != masonry.DefaultColumns {
		t.Fatalf("default columns=%d want %d", m.EffectiveColumnCount(), masonry.DefaultColumns)
	}
	h, v := m.EffectiveGutter()
	if h != 0 || v != 0 {
		t.Fatalf("default gutter=%v/%v want 0/0", h, v)
	}
	if m.Fresh() {
		t.Fatal("default fresh must be false")
	}
	if m.Focusable() {
		t.Fatal("container must not be Focusable")
	}
	if m.Role() != "" || m.AriaLabel() != "" {
		t.Fatalf("default role=%q label=%q want unnamed", m.Role(), m.AriaLabel())
	}
	if m.ColumnOf("missing") != -1 {
		t.Fatal("missing key ColumnOf must be -1")
	}
	sz := m.Layout(rendering.Loose(400, 200))
	if sz.Width != 400 && sz.Width != 0 {
		t.Fatalf("empty layout width=%v", sz.Width)
	}
	tok := theme.Default.Current()
	if tok.FontSize != 14 {
		t.Fatalf("theme FontSize=%v want 14", tok.FontSize)
	}
	_ = tok.ColorPrimary
}

func TestMasonry_PRD_MAS02_ThreeColumns(t *testing.T) {
	fx := loadMasonry(t)
	items := autoItems("c", []float64{fx.BasicHeights[0], fx.BasicHeights[1], fx.BasicHeights[2], fx.BasicHeights[3], fx.BasicHeights[4], fx.BasicHeights[5]})
	m := masonry.NewMasonry()
	m.SetColumns(fx.Columns)
	m.SetGutter(0, 0)
	m.SetItems(items)
	exact := m.Layout(rendering.Tight(fx.ContainerW, 600))
	near(t, exact.Width, fx.ContainerW, fx.Tolerance, "exact width")
	loose := m.Layout(rendering.Loose(fx.ContainerW, 2000))
	near(t, loose.Width, fx.ContainerW, fx.Tolerance, "loose width")
	mm := m.Layout(rendering.Constraints{MinWidth: 800, MaxWidth: fx.ContainerW, MinHeight: 0, MaxHeight: 2000})
	if mm.Width < 800-fx.Tolerance || mm.Width > fx.ContainerW+fx.Tolerance {
		t.Fatalf("minmax width=%v want 800..%v", mm.Width, fx.ContainerW)
	}
	m.Layout(rendering.Loose(fx.ContainerW, 2000))
	wantW := fx.ContainerW / float64(fx.Columns)
	for _, it := range items {
		bx := m.ItemBox(it.Key)
		near(t, bx.Size().Width, wantW, fx.Tolerance, "item width "+it.Key)
	}
	seen := map[int]int{}
	for _, it := range items {
		seen[m.ColumnOf(it.Key)]++
	}
	if len(seen) != fx.Columns {
		t.Fatalf("columns used=%v want %d", seen, fx.Columns)
	}
	for c, n := range seen {
		if n < 1 {
			t.Fatalf("column %d empty", c)
		}
	}
}

func TestMasonry_PRD_MAS03_Gutter16(t *testing.T) {
	fx := loadMasonry(t)
	m := masonry.NewMasonry()
	m.SetColumns(fx.Columns)
	m.SetGutter(fx.Gutter, fx.Gutter)
	m.SetItems(autoItems("g", []float64{100, 100, 100, 100}))
	m.Layout(rendering.Loose(fx.ContainerW, 2000))
	colW := (fx.ContainerW + fx.Gutter) / float64(fx.Columns)
	wantW := colW - fx.Gutter
	bx := m.ItemBox("ga")
	near(t, bx.Size().Width, wantW, fx.Tolerance, "gutter item width")
	// Vertical spacing: fourth item lands in column 0 on top of first + gutter.
	top0 := m.ItemBox("ga").Min.Y
	top3 := m.ItemBox("gd").Min.Y
	near(t, top3-top0, 100+fx.Gutter, fx.Tolerance, "row spacing")
}

func TestMasonry_PRD_MAS04_UnevenHeights(t *testing.T) {
	fx := loadMasonry(t)
	m := masonry.NewMasonry()
	m.SetColumns(fx.Columns)
	m.SetGutter(0, 0)
	m.SetItems(autoItems("u", fx.Heights))
	m.Layout(rendering.Loose(fx.ContainerW, 2000))
	if m.ColumnOf("ua") != 0 || m.ColumnOf("ub") != 1 || m.ColumnOf("uc") != 2 {
		t.Fatalf("shortest order cols %d %d %d want 0 1 2", m.ColumnOf("ua"), m.ColumnOf("ub"), m.ColumnOf("uc"))
	}
	hs := m.ColumnHeights()
	if len(hs) != fx.Columns {
		t.Fatalf("heights=%v", hs)
	}
	mn, mx := hs[0], hs[0]
	for _, h := range hs[1:] {
		if h < mn {
			mn = h
		}
		if h > mx {
			mx = h
		}
	}
	if mx-mn <= 0 {
		t.Fatalf("uneven heights must differ: %v", hs)
	}
}

func TestMasonry_PRD_MAS05_AddItemRelayout(t *testing.T) {
	fx := loadMasonry(t)
	m := masonry.NewMasonry()
	m.SetColumns(fx.Columns)
	m.SetGutter(0, 0)
	base := autoItems("m", []float64{100, 120, 140})
	calls := -1
	var last []masonry.MasonryColumn
	m.SetOnLayoutChange(func(items []masonry.MasonryColumn) {
		calls++
		last = append([]masonry.MasonryColumn(nil), items...)
	})
	m.SetItems(base)
	m.Layout(rendering.Loose(fx.ContainerW, 2000))
	if calls != 0 {
		t.Fatalf("initial callback=%d want 0", calls)
	}
	// Content wins over ItemRender.
	hitRender := false
	m.SetItemRender(func(item masonry.MasonryItem, index, column int) rendering.RenderObject {
		hitRender = true
		return rendering.NewRenderColorBox(10, 10, 0.5, 0.5, 0.5, 1)
	})
	withContent := masonry.MasonryItem{Key: "mc", Column: -1, Height: 50, Content: rendering.NewRenderColorBox(10, 50, 0.2, 0.3, 0.4, 1)}
	if got := m.ResolveItemNode(withContent, 0, 0); got != withContent.Content {
		t.Fatal("Content must win over ItemRender")
	}
	plain := masonry.MasonryItem{Key: "plain", Column: -1, Height: 10}
	if got := m.ResolveItemNode(plain, 0, 0); got == nil || !hitRender {
		t.Fatal("ItemRender must serve Content-less items")
	}
	calls = -1
	m.SetItems(append(base, masonry.MasonryItem{Key: "md", Column: -1, Height: 60}))
	m.Layout(rendering.Loose(fx.ContainerW, 2000))
	if calls != 0 {
		t.Fatalf("append callback=%d want 0 (one layout one call)", calls)
	}
	if m.ColumnOf("md") < 0 {
		t.Fatal("new item missing column")
	}
	if len(last) != 4 {
		t.Fatalf("callback items=%d want 4", len(last))
	}
}

func TestMasonry_PRD_MAS06_SingleColumn(t *testing.T) {
	fx := loadMasonry(t)
	m := masonry.NewMasonry()
	m.SetColumns(fx.SingleColumn)
	m.SetGutter(0, 0)
	m.SetItems(autoItems("s", []float64{80, 90}))
	m.Layout(rendering.Loose(fx.ContainerW, 2000))
	for _, k := range []string{"sa", "sb"} {
		bx := m.ItemBox(k)
		near(t, bx.Size().Width, fx.ContainerW, fx.Tolerance, "single width "+k)
		near(t, bx.Min.X, 0, fx.Tolerance, "single left "+k)
		if m.ColumnOf(k) != 0 {
			t.Fatalf("%s col=%d want 0", k, m.ColumnOf(k))
		}
	}
	topB := m.ItemBox("sb").Min.Y
	near(t, topB, 80, fx.Tolerance, "stacked top")
}

func TestMasonry_PRD_MAS07_BasicExample(t *testing.T) {
	fx := loadMasonry(t)
	m := masonry.NewMasonry()
	m.SetColumns(fx.Columns)
	m.SetGutter(0, 0)
	items := autoItems("b", fx.BasicHeights)
	m.SetItems(items)
	m.Layout(rendering.Loose(fx.ContainerW, 2000))
	wantW := fx.ContainerW / float64(fx.Columns)
	for _, it := range items {
		near(t, m.ItemBox(it.Key).Size().Width, wantW, fx.Tolerance, "basic "+it.Key)
	}
	seen := map[int]int{}
	for _, it := range items {
		seen[m.ColumnOf(it.Key)]++
	}
	if len(seen) != fx.Columns {
		t.Fatalf("basic columns=%v want 3", seen)
	}
	for c, n := range seen {
		if n < 1 {
			t.Fatalf("basic column %d empty", c)
		}
	}
}

func TestMasonry_PRD_MAS08_Responsive(t *testing.T) {
	fx := loadMasonry(t)
	m := masonry.NewMasonry()
	m.SetResponsiveColumns(respColumns(fx.ResponsiveColumns))
	m.SetGutter(0, 0)
	m.SetItems(autoItems("r", []float64{100, 100, 100, 100}))
	m.SetViewportWidth(fx.ViewportSmall)
	m.Layout(rendering.Loose(fx.ViewportSmall, 2000))
	if m.EffectiveColumnCount() != 1 {
		t.Fatalf("small columns=%d want 1", m.EffectiveColumnCount())
	}
	near(t, m.ItemBox("ra").Size().Width, fx.ViewportSmall, fx.Tolerance, "small width")
	m.SetViewportWidth(fx.ViewportLarge)
	m.Layout(rendering.Loose(fx.ViewportLarge, 2000))
	if m.EffectiveColumnCount() != 3 {
		t.Fatalf("large columns=%d want 3", m.EffectiveColumnCount())
	}
	near(t, m.ItemBox("ra").Size().Width, fx.ViewportLarge/3, fx.Tolerance, "large width")
}

func TestMasonry_PRD_MAS09_ImageExample(t *testing.T) {
	fx := loadMasonry(t)
	m := masonry.NewMasonry()
	m.SetColumns(fx.Columns)
	m.SetGutter(8, 8)
	m.SetItems(autoItems("p", fx.ImageHeights))
	m.Layout(rendering.Loose(fx.ContainerW, 2000))
	hs := m.ColumnHeights()
	mn, mx := hs[0], hs[0]
	for _, h := range hs[1:] {
		if h < mn {
			mn = h
		}
		if h > mx {
			mx = h
		}
	}
	if mx-mn <= 0 {
		t.Fatalf("image columns must differ: %v", hs)
	}
	colW := (fx.ContainerW + 8) / float64(fx.Columns)
	near(t, m.ItemBox("pa").Size().Width, colW-8, fx.Tolerance, "image width")
}

func TestMasonry_PRD_MAS10_DynamicExample(t *testing.T) {
	fx := loadMasonry(t)
	m := masonry.NewMasonry()
	m.SetColumns(fx.Columns)
	m.SetGutter(0, 0)
	m.SetItems(autoItems("d", fx.DynamicHeights))
	calls := 0
	m.SetOnLayoutChange(func(items []masonry.MasonryColumn) { calls++ })
	m.Layout(rendering.Loose(fx.ContainerW, 2000))
	if calls != 1 {
		t.Fatalf("first layout calls=%d want 1", calls)
	}
	calls = 0
	cur := m.Items()
	cur = append(cur, masonry.MasonryItem{Key: "dz", Column: -1, Height: 110})
	m.SetItems(cur)
	m.Layout(rendering.Loose(fx.ContainerW, 2000))
	if calls != 1 {
		t.Fatalf("dynamic calls=%d want 1", calls)
	}
	if m.ColumnOf("dz") < 0 {
		t.Fatal("dynamic item missing")
	}
}

func TestMasonry_PRD_MAS10b_FreshMeasure(t *testing.T) {
	fx := loadMasonry(t)
	content := rendering.NewRenderColorBox(100, 60, 0.3, 0.4, 0.8, 1)
	m := masonry.NewMasonry(masonry.MasonryItem{Key: "f1", Column: -1, Height: 0, Content: content})
	m.SetColumns(2)
	m.SetGutter(0, 0)
	m.SetViewportWidth(400)
	m.Layout(rendering.Loose(400, 400))
	h0 := m.ItemBox("f1").Size().Height
	near(t, h0, 60, fx.Tolerance, "measured height")
	// Grow content without fresh: cached height stays.
	content.Height = 120
	m.Layout(rendering.Loose(400, 400))
	near(t, m.ItemBox("f1").Size().Height, 60, fx.Tolerance, "stale without fresh")
	m.SetFresh(true)
	if !m.Fresh() {
		t.Fatal("fresh flag must stick")
	}
	m.Layout(rendering.Loose(400, 400))
	near(t, m.ItemBox("f1").Size().Height, 120, fx.Tolerance, "fresh follows")
	// Explicit column clamps, explicit height wins.
	m.SetFresh(false)
	m.SetItems([]masonry.MasonryItem{{Key: "fx", Column: 99, Height: 40}})
	m.Layout(rendering.Loose(400, 400))
	if m.ColumnOf("fx") != 1 {
		t.Fatalf("clamped col=%d want 1", m.ColumnOf("fx"))
	}
}

func TestMasonry_PRD_MAS13_Metrics(t *testing.T) {
	fx := loadMasonry(t)
	m := masonry.NewMasonry()
	m.SetColumns(fx.Columns)
	m.SetGutter(fx.Gutter, fx.Gutter)
	m.SetItems(autoItems("t", []float64{100, 100, 100}))
	m.Layout(rendering.Loose(fx.ContainerW, 2000))
	colW := (fx.ContainerW + fx.Gutter) / float64(fx.Columns)
	near(t, m.ItemBox("ta").Size().Width, colW-fx.Gutter, fx.Tolerance, "metric width")
	near(t, m.ItemBox("ta").Min.X, 0, fx.Tolerance, "metric left0")
	near(t, m.ItemBox("tb").Min.X, colW, fx.Tolerance, "metric left1")
	// Pair gutter: horizontal enters width, vertical accumulates height.
	m.SetGutter(10, 20)
	m.SetItems(autoItems("q", []float64{50, 50, 50, 50}))
	m.Layout(rendering.Loose(300, 1000))
	qw := (300+10)/3.0 - 10
	near(t, m.ItemBox("qa").Size().Width, qw, fx.Tolerance, "pair width")
	near(t, m.ItemBox("qd").Min.Y, 50+20, fx.Tolerance, "pair vertical")
	// Responsive gutter follows breakpoint.
	m.SetResponsiveGutter(respGutter(fx.ResponsiveGutter))
	m.SetViewportWidth(fx.ViewportSmall)
	h, _ := m.EffectiveGutter()
	near(t, h, 8, fx.Tolerance, "resp gutter small")
	m.SetViewportWidth(fx.ViewportLarge)
	h, _ = m.EffectiveGutter()
	near(t, h, 16, fx.Tolerance, "resp gutter large")
	// Breakpoint order: 500 xs, 600 sm, 800 md.
	m.SetResponsiveColumns(respColumns(fx.ResponsiveColumns))
	m.SetViewportWidth(500)
	if m.EffectiveColumnCount() != 1 {
		t.Fatalf("500 cols=%d want 1", m.EffectiveColumnCount())
	}
	m.SetViewportWidth(600)
	if m.EffectiveColumnCount() != 2 {
		t.Fatalf("600 cols=%d want 2", m.EffectiveColumnCount())
	}
	m.SetViewportWidth(800)
	if m.EffectiveColumnCount() != 3 {
		t.Fatalf("800 cols=%d want 3", m.EffectiveColumnCount())
	}
	// Zero columns fall back to default.
	m.SetColumns(0)
	if m.EffectiveColumnCount() != masonry.DefaultColumns {
		t.Fatalf("zero cols=%d want %d", m.EffectiveColumnCount(), masonry.DefaultColumns)
	}
	tok := theme.Default.Current()
	if tok.Padding != 16 {
		t.Fatalf("theme Padding=%v want 16", tok.Padding)
	}
}

func TestMasonry_PRD_MAS14_NoOwnColor(t *testing.T) {
	fx := loadMasonry(t)
	m := masonry.NewMasonry(autoItems("n", []float64{40, 40})...)
	m.SetColumns(2)
	m.SetGutter(8, 8)
	m.Layout(rendering.Loose(200, 200))
	before := m.ItemBox("na").Size()
	tok := theme.Default.Current()
	_ = tok.ColorPrimary
	_ = tok.ColorText
	alt := tok
	alt.ColorPrimary = theme.Hex("#ff4d4f")
	m.SetTheme(&alt)
	m.Layout(rendering.Loose(200, 200))
	after := m.ItemBox("na").Size()
	if math.Abs(before.Width-after.Width) > fx.Tolerance || math.Abs(before.Height-after.Height) > fx.Tolerance {
		t.Fatal("gap must not follow brand color")
	}
	m.SetTheme(nil)
	p := theme.NewProvider(theme.DefaultTokens())
	m.SetProvider(p)
	_ = p.Current()
	m.Layout(rendering.Loose(200, 200))
	if m.Focusable() {
		t.Fatal("no chrome focus")
	}
}

func TestMasonry_PRD_MAS15_DisabledNA(t *testing.T) {
	m := masonry.NewMasonry(autoItems("x", []float64{30, 30})...)
	if m.Focusable() {
		t.Fatal("disabled N/A: container never Focusable")
	}
	m.Layout(rendering.Loose(200, 100))
	if m.Role() != "" {
		t.Fatalf("disabled N/A: role=%q want empty", m.Role())
	}
	sz := m.Layout(rendering.Loose(200, 100))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("disabled N/A: layout must stay stable %+v", sz)
	}
}

func TestMasonry_PRD_MAS16_FocusNA(t *testing.T) {
	m := masonry.NewMasonry(autoItems("y", []float64{30, 30})...)
	if m.Focusable() {
		t.Fatal("container must not be Focusable")
	}
	m.SetAriaLabel("gallery")
	if m.AriaLabel() != "gallery" || m.Role() != "group" {
		t.Fatalf("aria role=%q label=%q", m.Role(), m.AriaLabel())
	}
	if m.Focusable() {
		t.Fatal("AriaLabel must not make container Focusable")
	}
	m.SetAriaLabel("")
	if m.Role() != "" {
		t.Fatalf("cleared role=%q want empty", m.Role())
	}
}
