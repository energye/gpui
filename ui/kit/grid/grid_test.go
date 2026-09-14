package grid_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/kit/grid"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type gridFile struct {
	Columns     int              `json:"columns"`
	Breakpoints map[string]float64 `json:"breakpoints"`
	Gutter      float64          `json:"gutter"`
	Container   float64          `json:"container"`
	Half        int              `json:"half"`
	Third       int              `json:"third"`
	Offset6     int              `json:"offset6"`
}

func loadGrid(t *testing.T) gridFile {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "grid.json"))
	if err != nil {
		t.Fatalf("read grid.json: %v", err)
	}
	var f gridFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("parse grid.json: %v", err)
	}
	if f.Columns == 0 || f.Container == 0 {
		t.Fatal("empty grid file")
	}
	return f
}

func tight(w, h float64) rendering.Constraints { return rendering.Tight(w, h) }

func colSize(c *grid.Col) rendering.Size { return c.Node().Size() }

func colOffset(c *grid.Col) rendering.Point { return c.Node().Offset() }

func TestGrid_PRD_GRD01(t *testing.T) {
	f := loadGrid(t)
	_ = f
	r := grid.NewGrid()
	if r.Align() != grid.RowAlignTop || r.Justify() != grid.RowJustifyStart || !r.Wrap() {
		t.Fatalf("defaults align=%s justify=%s wrap=%v", r.Align(), r.Justify(), r.Wrap())
	}
	if r.GutterH() != 0 || r.GutterV() != 0 {
		t.Fatalf("default gutter %v/%v", r.GutterH(), r.GutterV())
	}
	c := grid.NewCol()
	if c.Span() != -1 || c.Offset() != 0 || c.Order() != 0 || c.Push() != 0 || c.Pull() != 0 {
		t.Fatalf("col defaults span=%d offset=%d order=%d push=%d pull=%d", c.Span(), c.Offset(), c.Order(), c.Push(), c.Pull())
	}
	if r.Focusable() || r.Role() != "" {
		t.Fatal("container defaults: no focus, no forced role")
	}
	sz := r.Layout(tight(1200, 800))
	if sz.Width != 1200 {
		t.Fatalf("empty row width=%v", sz.Width)
	}
}

func TestGrid_PRD_GRD02(t *testing.T) {
	f := loadGrid(t)
	W := f.Container
	a, b := grid.NewCol(), grid.NewCol()
	a.SetSpan(f.Half)
	b.SetSpan(f.Half)
	r := grid.NewRow(a, b)
	r.Layout(tight(W, 800))
	for i, c := range []*grid.Col{a, b} {
		if got := colSize(c).Width; math.Abs(got-W/2) > 0.5 {
			t.Fatalf("col%d width=%v want %v", i, got, W/2)
		}
	}
}

func TestGrid_PRD_GRD03(t *testing.T) {
	f := loadGrid(t)
	W := f.Container
	cols := []*grid.Col{grid.NewCol(), grid.NewCol(), grid.NewCol()}
	for _, c := range cols {
		c.SetSpan(f.Third)
	}
	r := grid.NewRow(cols...)
	r.Layout(tight(W, 800))
	for i, c := range cols {
		if got := colSize(c).Width; math.Abs(got-W/3) > 0.5 {
			t.Fatalf("col%d width=%v want %v", i, got, W/3)
		}
	}
}

func TestGrid_PRD_GRD04(t *testing.T) {
	f := loadGrid(t)
	W := f.Container
	c := grid.NewCol()
	c.SetSpan(f.Half)
	c.SetOffset(f.Offset6)
	r := grid.NewRow(c)
	r.Layout(tight(W, 800))
	if got := colOffset(c).X; math.Abs(got-W*6/24) > 0.5 {
		t.Fatalf("offset x=%v want %v", got, W*6/24)
	}
}

func TestGrid_PRD_GRD05(t *testing.T) {
	f := loadGrid(t)
	W := f.Container
	mk := func() (*grid.Col, *rendering.RenderColorBox) {
		box := rendering.NewRenderColorBox(10, 10, 0.2, 0.3, 0.4, 1)
		c := grid.NewCol(box)
		c.SetSpan(f.Half)
		return c, box
	}
	a, ab := mk()
	b, _ := mk()
	_ = ab
	r := grid.NewRow(a, b)
	r.SetGutter(f.Gutter)
	r.Layout(tight(W, 800))
	// Outer boxes abut; content boxes inset by gutter/2 each side.
	if got := colOffset(b).X - (colOffset(a).X + colSize(a).Width); math.Abs(got) > 0.5 {
		t.Fatalf("outer gap=%v want 0", got)
	}
	ax := colOffset(a).X + colSize(a).Width - f.Gutter/2
	bx := colOffset(b).X + f.Gutter/2
	if math.Abs(bx-ax-f.Gutter) > 0.5 {
		t.Fatalf("content gap=%v want %v", bx-ax, f.Gutter)
	}
	// Child proves half padding.
	kids := a.Node().Children()
	if len(kids) == 0 {
		t.Fatal("no content child")
	}
	if got := kids[0].Offset().X; math.Abs(got-f.Gutter/2) > 0.5 {
		t.Fatalf("child pad=%v want %v", got, f.Gutter/2)
	}
}

func TestGrid_PRD_GRD06(t *testing.T) {
	f := loadGrid(t)
	W := f.Container
	c := grid.NewCol()
	c.SetXs(24)
	c.SetMd(12)
	r := grid.NewRow(c)
	r.SetViewportWidth(500)
	r.Layout(tight(W, 800))
	if got := colSize(c).Width; math.Abs(got-W) > 0.5 {
		t.Fatalf("xs width=%v want %v", got, W)
	}
	r.SetViewportWidth(800)
	r.Layout(tight(W, 800))
	if got := colSize(c).Width; math.Abs(got-W/2) > 0.5 {
		t.Fatalf("md width=%v want %v", got, W/2)
	}
}

func TestGrid_PRD_GRD07(t *testing.T) {
	f := loadGrid(t)
	W := f.Container
	cols := []*grid.Col{grid.NewCol(), grid.NewCol(), grid.NewCol()}
	for _, c := range cols {
		c.SetSpan(f.Half)
	}
	r := grid.NewRow(cols...)
	r.SetWrap(false)
	r.Layout(tight(W, 800))
	for i, c := range cols {
		if got := colOffset(c).Y; math.Abs(got) > 0.5 {
			t.Fatalf("col%d y=%v want 0 (no wrap)", i, got)
		}
	}
	last := cols[2]
	right := colOffset(last).X + colSize(last).Width
	if right <= W+0.5 {
		t.Fatalf("right=%v should exceed %v", right, W)
	}
}

func TestGrid_PRD_GRD08(t *testing.T) {
	f := loadGrid(t)
	W := f.Container
	// Mount basic.tsx: span 12+12, gutter 0.
	a, b := grid.NewCol(), grid.NewCol()
	a.SetSpan(12)
	b.SetSpan(12)
	r := grid.NewRow(a, b)
	r.Layout(tight(W, 800))
	for i, c := range []*grid.Col{a, b} {
		if got := colSize(c).Width; math.Abs(got-600) > 0.5 {
			t.Fatalf("basic col%d=%v want 600", i, got)
		}
	}
}

func TestGrid_PRD_GRD09(t *testing.T) {
	f := loadGrid(t)
	W := f.Container
	// Mount gutter.tsx: gutter 16.
	a, b := grid.NewCol(), grid.NewCol()
	a.SetSpan(12)
	b.SetSpan(12)
	r := grid.NewRow(a, b)
	r.SetGutter(16)
	r.Layout(tight(W, 800))
	ax := colOffset(a).X + colSize(a).Width - 8
	bx := colOffset(b).X + 8
	if math.Abs(bx-ax-16) > 0.5 {
		t.Fatalf("gutter gap=%v want 16", bx-ax)
	}
}

func TestGrid_PRD_GRD10(t *testing.T) {
	f := loadGrid(t)
	W := f.Container
	// Mount offset.tsx: offset 6.
	c := grid.NewCol()
	c.SetSpan(12)
	c.SetOffset(6)
	r := grid.NewRow(c)
	r.Layout(tight(W, 800))
	if got := colOffset(c).X; math.Abs(got-300) > 0.5 {
		t.Fatalf("offset x=%v want 300", got)
	}
}

func TestGrid_PRD_GRD11(t *testing.T) {
	f := loadGrid(t)
	W := f.Container
	// Mount sort.tsx: push/pull pair swaps visuals, widths stay.
	a, b := grid.NewCol(), grid.NewCol()
	a.SetSpan(12)
	b.SetSpan(12)
	a.SetPush(12)
	b.SetPull(12)
	r := grid.NewRow(a, b)
	r.Layout(tight(W, 800))
	if got := colSize(a).Width; math.Abs(got-600) > 0.5 {
		t.Fatalf("push width=%v want 600", got)
	}
	if got := colSize(b).Width; math.Abs(got-600) > 0.5 {
		t.Fatalf("pull width=%v want 600", got)
	}
	if math.Abs(colOffset(a).X-600) > 0.5 || math.Abs(colOffset(b).X-0) > 0.5 {
		t.Fatalf("swapped ax=%v bx=%v", colOffset(a).X, colOffset(b).X)
	}
}

func TestGrid_PRD_GRD12(t *testing.T) {
	f := loadGrid(t)
	W := f.Container
	// Mount flex.tsx: justify center groups columns in the middle.
	a, b := grid.NewCol(), grid.NewCol()
	a.SetSpan(6)
	b.SetSpan(6)
	r := grid.NewRow(a, b)
	r.SetJustify(grid.RowJustifyCenter)
	r.Layout(tight(W, 800))
	if got := colOffset(a).X; got <= 0.5 || math.Abs(got-300) > 0.5 {
		t.Fatalf("center first x=%v want 300", got)
	}
}

func TestGrid_PRD_GRD13(t *testing.T) {
	f := loadGrid(t)
	W := f.Container
	// Mount flex-align.tsx: align middle centers the short column.
	shortBox := rendering.NewRenderColorBox(100, 20, 0.2, 0.3, 0.4, 1)
	tallBox := rendering.NewRenderColorBox(100, 40, 0.4, 0.3, 0.2, 1)
	a := grid.NewCol(shortBox)
	b := grid.NewCol(tallBox)
	a.SetSpan(12)
	b.SetSpan(12)
	r := grid.NewRow(a, b)
	r.SetAlign(grid.RowAlignMiddle)
	r.Layout(tight(W, 800))
	ay := colOffset(a).Y
	by := colOffset(b).Y
	if math.Abs(ay-10) > 0.5 || math.Abs(by) > 0.5 {
		t.Fatalf("align ay=%v by=%v want 10/0", ay, by)
	}
}

func TestGrid_PRD_GRD14(t *testing.T) {
	f := loadGrid(t)
	W := f.Container
	// Mount flex-order.tsx: larger order goes later.
	a := grid.NewCol()
	b := grid.NewCol()
	a.SetSpan(12)
	b.SetSpan(12)
	a.SetOrder(2)
	b.SetOrder(1)
	r := grid.NewRow(a, b)
	r.Layout(tight(W, 800))
	if !(colOffset(b).X < colOffset(a).X) {
		t.Fatalf("order bx=%v ax=%v want b first", colOffset(b).X, colOffset(a).X)
	}
}

func TestGrid_PRD_GRD15(t *testing.T) {
	f := loadGrid(t)
	W := f.Container
	// Mount flex-stretch.tsx: fill column takes the remainder.
	fixed := grid.NewCol()
	fill := grid.NewCol()
	fixed.SetSpan(6)
	fill.SetFlexAuto()
	r := grid.NewRow(fixed, fill)
	r.Layout(tight(W, 800))
	if got := colSize(fill).Width; math.Abs(got-(W-300)) > 0.5 {
		t.Fatalf("fill=%v want %v", got, W-300)
	}
}

func TestGrid_PRD_GRD16(t *testing.T) {
	f := loadGrid(t)
	if grid.GridColumns != f.Columns {
		t.Fatalf("columns=%d want %d", grid.GridColumns, f.Columns)
	}
	want := map[string]float64{
		"sm": grid.ScreenSM, "md": grid.ScreenMD, "lg": grid.ScreenLG,
		"xl": grid.ScreenXL, "xxl": grid.ScreenXXL, "xxxl": grid.ScreenXXXL,
	}
	for k, v := range want {
		if math.Abs(f.Breakpoints[k]-v) > 0.5 {
			t.Fatalf("breakpoint %s=%v want %v", k, f.Breakpoints[k], v)
		}
	}
	// Theme tokens carry the shared seeds grid reads.
	tok := theme.Default.Current()
	if tok.ControlHeight != 32 || tok.FontSize != 14 {
		t.Fatalf("theme seeds control=%v font=%v", tok.ControlHeight, tok.FontSize)
	}
	r := grid.NewRow()
	if got := r.EffectiveTokens().ControlHeight; got != tok.ControlHeight {
		t.Fatalf("row tokens control=%v want %v", got, tok.ControlHeight)
	}
}

func TestGrid_PRD_GRD17(t *testing.T) {
	// No own colors: tokens come from theme, container paints nothing itself.
	r := grid.NewRow(grid.NewCol())
	tok := theme.Default.Current()
	if got := r.EffectiveTokens(); got.ColorPrimary != tok.ColorPrimary {
		t.Fatalf("primary %+v want theme %+v", got.ColorPrimary, tok.ColorPrimary)
	}
	custom := tok
	custom.ColorPrimary = theme.Hex("#123456")
	r.SetTheme(&custom)
	if got := r.EffectiveTokens().ColorPrimary; got != custom.ColorPrimary {
		t.Fatalf("override %+v want %+v", got, custom.ColorPrimary)
	}
	r.SetTheme(nil)
	if got := r.EffectiveTokens().ColorPrimary; got != tok.ColorPrimary {
		t.Fatalf("cleared %+v want %+v", got, tok.ColorPrimary)
	}
}

func TestGrid_PRD_GRD18(t *testing.T) {
	// Disabled N/A: pure layout has no disabled chrome; geometry stays stable.
	r := grid.NewRow(grid.NewCol())
	if r.Focusable() {
		t.Fatal("container must not take focus")
	}
	if r.Role() != "" {
		t.Fatalf("role=%q want empty (no forced role)", r.Role())
	}
	c := grid.NewCol()
	c.SetSpan(12)
	rr := grid.NewRow(c)
	rr.Layout(tight(1200, 800))
	if got := colSize(c).Width; math.Abs(got-600) > 0.5 {
		t.Fatalf("width=%v want 600", got)
	}
}

func TestGrid_PRD_GRD19(t *testing.T) {
	// Keyboard N/A: container not focusable; children own focus. Aria names only.
	r := grid.NewRow()
	if r.Focusable() {
		t.Fatal("Focus: container must stay unfocusable")
	}
	if r.Role() != "" {
		t.Fatalf("Role=%q want empty", r.Role())
	}
	if r.AriaLabel() != "" {
		t.Fatalf("Aria default %q want empty", r.AriaLabel())
	}
	r.SetAriaLabel("grid demo")
	if r.AriaLabel() != "grid demo" {
		t.Fatalf("Aria=%q", r.AriaLabel())
	}
	if r.Role() != "" {
		t.Fatalf("named Role=%q still empty (no forced role)", r.Role())
	}
}
