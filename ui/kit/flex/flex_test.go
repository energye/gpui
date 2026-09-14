package flex_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/kit/flex"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type flexFile struct {
	GapSmall      float64 `json:"gapSmall"`
	GapMedium     float64 `json:"gapMedium"`
	GapLarge      float64 `json:"gapLarge"`
	ChildW        float64 `json:"childW"`
	ChildH        float64 `json:"childH"`
	ShortH        float64 `json:"shortH"`
	TallH         float64 `json:"tallH"`
	ContainerW    float64 `json:"containerW"`
	NarrowW       float64 `json:"narrowW"`
	NarrowChildW  float64 `json:"narrowChildW"`
	Tolerance     float64 `json:"tolerance"`
}

func loadFlex(t *testing.T) flexFile {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "flex.json"))
	if err != nil {
		t.Fatalf("read flex.json: %v", err)
	}
	var f flexFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("parse flex.json: %v", err)
	}
	if f.Tolerance <= 0 {
		t.Fatal("tolerance must be positive")
	}
	return f
}

func box(w, h float64) *rendering.RenderColorBox {
	return rendering.NewRenderColorBox(w, h, 0.3, 0.4, 0.8, 1)
}

func offsets(f *flex.Flex) []rendering.Point {
	out := []rendering.Point{}
	for _, c := range f.Children() {
		out = append(out, c.Offset())
	}
	return out
}

func sizes(f *flex.Flex) []rendering.Size {
	out := []rendering.Size{}
	for _, c := range f.Children() {
		out = append(out, c.Size())
	}
	return out
}

func TestFlex_PRD_FLX01_Defaults(t *testing.T) {
	f := flex.NewFlex()
	if f == nil || f.Node() == nil {
		t.Fatal("NewFlex must not crash")
	}
	if f.EffectiveOrientation() != flex.FlexOrientationHorizontal || f.IsVertical() {
		t.Fatalf("default orientation=%q want horizontal", f.EffectiveOrientation())
	}
	if f.Justify() != flex.FlexJustifyStart {
		t.Fatalf("default justify=%q want start", f.Justify())
	}
	if f.Align() != flex.FlexAlignAuto || f.EffectiveAlign() != flex.FlexAlignStart {
		t.Fatalf("default align=%q effective=%q", f.Align(), f.EffectiveAlign())
	}
	if f.Wrap() {
		t.Fatal("default wrap must be false")
	}
	if math.Abs(f.ResolvedGap()) > 1e-9 {
		t.Fatalf("default gap=%v want 0", f.ResolvedGap())
	}
	if f.Focusable() {
		t.Fatal("flex container must not be Focusable")
	}
	if f.Role() != "" || f.AriaLabel() != "" {
		t.Fatalf("default role=%q label=%q want unnamed", f.Role(), f.AriaLabel())
	}
	sz := f.Layout(rendering.Loose(400, 200))
	if sz.Width != 0 || sz.Height != 0 {
		t.Fatalf("empty layout=%v want 0x0", sz)
	}
	_ = theme.Default.Current()
}

func TestFlex_PRD_FLX02_HorizontalDefault(t *testing.T) {
	fx := loadFlex(t)
	a, b := box(50, 20), box(50, 20)
	f := flex.NewFlex(a, b)
	// Layout matrix: Exact / Loose / Min-Max each run.
	exact := f.Layout(rendering.Tight(300, 100))
	if math.Abs(exact.Width-300) > fx.Tolerance {
		t.Fatalf("exact width=%v want 300", exact.Width)
	}
	loose := f.Layout(rendering.Loose(400, 100))
	if loose.Width < 100-fx.Tolerance {
		t.Fatalf("loose width=%v too small", loose.Width)
	}
	mm := f.Layout(rendering.Constraints{MinWidth: 200, MaxWidth: 400, MinHeight: 0, MaxHeight: 200})
	if mm.Width < 200-fx.Tolerance || mm.Width > 400+fx.Tolerance {
		t.Fatalf("minmax width=%v want 200..400", mm.Width)
	}
	// Behavior under loose content size.
	f.Layout(rendering.Loose(400, 100))
	off := offsets(f)
	if math.Abs(off[0].Y-off[1].Y) > fx.Tolerance {
		t.Fatalf("top diff=%v", math.Abs(off[0].Y-off[1].Y))
	}
	sz := sizes(f)
	if math.Abs(off[1].X-(off[0].X+sz[0].Width)) > fx.Tolerance {
		t.Fatalf("second left=%v want first right=%v", off[1].X, off[0].X+sz[0].Width)
	}
}

func TestFlex_PRD_FLX03_Vertical(t *testing.T) {
	fx := loadFlex(t)
	a, b := box(50, 20), box(50, 20)
	f := flex.NewFlex(a, b)
	f.SetVertical(true)
	if !f.IsVertical() || f.EffectiveOrientation() != flex.FlexOrientationVertical {
		t.Fatal("vertical sugar must flip axis")
	}
	// Orientation wins over the vertical sugar.
	f.SetOrientation(flex.FlexOrientationVertical)
	f.SetVertical(false)
	if !f.IsVertical() {
		t.Fatal("orientation must win over vertical=false")
	}
	f.SetOrientation(flex.FlexOrientationHorizontal)
	f.SetVertical(true)
	f.SetOrientation(flex.FlexOrientationVertical)
	if !f.IsVertical() {
		t.Fatal("explicit orientation must stick")
	}
	f.Layout(rendering.Loose(200, 400))
	off := offsets(f)
	sz := sizes(f)
	if math.Abs(off[0].X-off[1].X) > fx.Tolerance {
		t.Fatalf("left diff=%v", math.Abs(off[0].X-off[1].X))
	}
	if math.Abs(off[1].Y-(off[0].Y+sz[0].Height)) > fx.Tolerance {
		t.Fatalf("second top=%v want first bottom=%v", off[1].Y, off[0].Y+sz[0].Height)
	}
}

func TestFlex_PRD_FLX04_GapMiddle(t *testing.T) {
	fx := loadFlex(t)
	a, b := box(50, 20), box(50, 20)
	f := flex.NewFlex(a, b)
	f.SetGapSize(flex.FlexGapMedium)
	if math.Abs(f.ResolvedGap()-fx.GapMedium) > fx.Tolerance {
		t.Fatalf("gap=%v want %v", f.ResolvedGap(), fx.GapMedium)
	}
	f.Layout(rendering.Loose(400, 100))
	off := offsets(f)
	sz := sizes(f)
	if math.Abs((off[1].X-off[0].X-sz[0].Width)-fx.GapMedium) > fx.Tolerance {
		t.Fatalf("spacing=%v want %v", off[1].X-off[0].X-sz[0].Width, fx.GapMedium)
	}
	// Middle alias resolves the same.
	f.SetGapSize(flex.FlexGapMiddle)
	if math.Abs(f.ResolvedGap()-fx.GapMedium) > fx.Tolerance {
		t.Fatalf("middle gap=%v want %v", f.ResolvedGap(), fx.GapMedium)
	}
}

func TestFlex_PRD_FLX05_GapLarge(t *testing.T) {
	fx := loadFlex(t)
	a, b := box(50, 20), box(50, 20)
	f := flex.NewFlex(a, b)
	f.SetGapSize(flex.FlexGapLarge)
	if math.Abs(f.ResolvedGap()-fx.GapLarge) > fx.Tolerance {
		t.Fatalf("gap=%v want %v", f.ResolvedGap(), fx.GapLarge)
	}
	f.Layout(rendering.Loose(400, 100))
	off := offsets(f)
	sz := sizes(f)
	if math.Abs((off[1].X-off[0].X-sz[0].Width)-fx.GapLarge) > fx.Tolerance {
		t.Fatalf("spacing=%v want %v", off[1].X-off[0].X-sz[0].Width, fx.GapLarge)
	}
}

func TestFlex_PRD_FLX06_SpaceBetween(t *testing.T) {
	fx := loadFlex(t)
	a, b := box(fx.ChildW, fx.ChildH), box(fx.ChildW, fx.ChildH)
	f := flex.NewFlex(a, b)
	f.SetJustify(flex.FlexJustifySpaceBetween)
	sz := f.Layout(rendering.Tight(fx.ContainerW, 100))
	if math.Abs(sz.Width-fx.ContainerW) > fx.Tolerance {
		t.Fatalf("container width=%v want %v", sz.Width, fx.ContainerW)
	}
	off := offsets(f)
	cs := sizes(f)
	if math.Abs(off[0].X) > fx.Tolerance {
		t.Fatalf("first left=%v want 0", off[0].X)
	}
	if math.Abs((off[1].X+cs[1].Width)-fx.ContainerW) > fx.Tolerance {
		t.Fatalf("last right=%v want %v", off[1].X+cs[1].Width, fx.ContainerW)
	}
	// Other justifies must not crash and keep order.
	for _, j := range []flex.FlexJustify{flex.FlexJustifyStart, flex.FlexJustifyCenter, flex.FlexJustifyEnd, flex.FlexJustifySpaceAround, flex.FlexJustifySpaceEvenly} {
		f.SetJustify(j)
		f.Layout(rendering.Tight(fx.ContainerW, 100))
	}
}

func TestFlex_PRD_FLX07_AlignCenter(t *testing.T) {
	fx := loadFlex(t)
	short, tall := box(60, fx.ShortH), box(60, fx.TallH)
	f := flex.NewFlex(short, tall)
	f.SetAlign(flex.FlexAlignCenter)
	f.Layout(rendering.Loose(400, 100))
	off := offsets(f)
	cs := sizes(f)
	shortMid := off[0].Y + cs[0].Height/2
	lineMid := fx.TallH / 2
	if math.Abs(shortMid-lineMid) > fx.Tolerance {
		t.Fatalf("short mid=%v line mid=%v", shortMid, lineMid)
	}
}

func TestFlex_PRD_FLX08_Wrap(t *testing.T) {
	fx := loadFlex(t)
	a, b := box(fx.NarrowChildW, 40), box(fx.NarrowChildW, 40)
	f := flex.NewFlex(a, b)
	f.SetWrap(true)
	f.Layout(rendering.Tight(fx.NarrowW, 200))
	off := offsets(f)
	if len(off) != 2 {
		t.Fatalf("children=%d want 2", len(off))
	}
	if !(off[1].Y > off[0].Y) {
		t.Fatalf("wrap must break line: tops %v %v", off[0].Y, off[1].Y)
	}
}

func TestFlex_PRD_FLX09_GapNumber(t *testing.T) {
	fx := loadFlex(t)
	a, b := box(50, 20), box(50, 20)
	f := flex.NewFlex(a, b)
	f.SetGap(8)
	if math.Abs(f.ResolvedGap()-8) > fx.Tolerance {
		t.Fatalf("gap=%v want 8", f.ResolvedGap())
	}
	f.Layout(rendering.Loose(400, 100))
	off := offsets(f)
	sz := sizes(f)
	if math.Abs((off[1].X-off[0].X-sz[0].Width)-8) > fx.Tolerance {
		t.Fatalf("spacing=%v want 8", off[1].X-off[0].X-sz[0].Width)
	}
}

func TestFlex_PRD_FLX10_BasicExample(t *testing.T) {
	fx := loadFlex(t)
	a, b := box(60, 32), box(60, 32)
	f := flex.NewFlex(a, b)
	f.SetGapSize(flex.FlexGapSmall)
	f.Layout(rendering.Loose(400, 100))
	off := offsets(f)
	sz := sizes(f)
	if math.Abs(off[0].Y-off[1].Y) > fx.Tolerance {
		t.Fatalf("basic top diff=%v", math.Abs(off[0].Y-off[1].Y))
	}
	if math.Abs((off[1].X-off[0].X-sz[0].Width)-fx.GapSmall) > fx.Tolerance {
		t.Fatalf("basic spacing=%v want %v", off[1].X-off[0].X-sz[0].Width, fx.GapSmall)
	}
}

func TestFlex_PRD_FLX11_AlignExample(t *testing.T) {
	fx := loadFlex(t)
	short, tall := box(80, fx.ShortH), box(80, fx.TallH)
	f := flex.NewFlex(short, tall)
	f.SetAlign(flex.FlexAlignCenter)
	f.Layout(rendering.Loose(400, 100))
	off := offsets(f)
	cs := sizes(f)
	if math.Abs((off[0].Y+cs[0].Height/2)-fx.TallH/2) > fx.Tolerance {
		t.Fatalf("align example mid=%v want %v", off[0].Y+cs[0].Height/2, fx.TallH/2)
	}
}

func TestFlex_PRD_FLX12_GapExample(t *testing.T) {
	fx := loadFlex(t)
	for _, tc := range []struct {
		size flex.FlexGapSize
		want float64
	}{
		{flex.FlexGapSmall, fx.GapSmall},
		{flex.FlexGapMedium, fx.GapMedium},
		{flex.FlexGapLarge, fx.GapLarge},
	} {
		a, b := box(50, 20), box(50, 20)
		f := flex.NewFlex(a, b)
		f.SetGapSize(tc.size)
		f.Layout(rendering.Loose(400, 100))
		off := offsets(f)
		sz := sizes(f)
		if math.Abs((off[1].X-off[0].X-sz[0].Width)-tc.want) > fx.Tolerance {
			t.Fatalf("gap example %d spacing=%v want %v", tc.size, off[1].X-off[0].X-sz[0].Width, tc.want)
		}
	}
}

func TestFlex_PRD_FLX13_WrapExample(t *testing.T) {
	fx := loadFlex(t)
	kids := []rendering.RenderObject{box(fx.NarrowChildW, 32), box(fx.NarrowChildW, 32), box(fx.NarrowChildW, 32)}
	f := flex.NewFlex(kids...)
	f.SetWrap(true)
	f.SetGap(8)
	f.Layout(rendering.Tight(fx.NarrowW, 300))
	off := offsets(f)
	rows := map[float64]bool{}
	for _, o := range off {
		rows[math.Round(o.Y)] = true
	}
	if len(rows) < 2 {
		t.Fatalf("wrap example rows=%v want >=2", rows)
	}
	if !(off[1].Y > off[0].Y || off[2].Y > off[0].Y) {
		t.Fatalf("wrap example must break: %+v", off)
	}
}

func TestFlex_PRD_FLX14_CombinationExample(t *testing.T) {
	fx := loadFlex(t)
	a, b, c := box(60, 24), box(60, 24), box(60, 24)
	f := flex.NewFlex(a, b, c)
	f.SetOrientation(flex.FlexOrientationVertical)
	f.SetJustify(flex.FlexJustifyCenter)
	f.SetAlign(flex.FlexAlignCenter)
	f.SetGapSize(flex.FlexGapSmall)
	if !f.IsVertical() {
		t.Fatal("combination must stay vertical")
	}
	f.Layout(rendering.Tight(200, 300))
	off := offsets(f)
	cs := sizes(f)
	if math.Abs(off[0].X-off[1].X) > fx.Tolerance || math.Abs(off[1].X-off[2].X) > fx.Tolerance {
		t.Fatalf("vertical lefts %+v", off)
	}
	if math.Abs((off[1].Y-off[0].Y-cs[0].Height)-fx.GapSmall) > fx.Tolerance {
		t.Fatalf("vertical spacing=%v want %v", off[1].Y-off[0].Y-cs[0].Height, fx.GapSmall)
	}
	// Children helpers must work.
	if f.ChildCount() != 3 {
		t.Fatalf("children=%d want 3", f.ChildCount())
	}
	f.ClearChildren()
	if f.ChildCount() != 0 {
		t.Fatal("ClearChildren must empty")
	}
	f.SetChildren(a, b)
	if f.ChildCount() != 2 {
		t.Fatal("SetChildren must replace")
	}
	f.Add(c)
	if f.ChildCount() != 3 {
		t.Fatal("Add must append")
	}
}

func TestFlex_PRD_FLX15_TokenGaps(t *testing.T) {
	fx := loadFlex(t)
	// Default theme carries the antd seed (Padding 16 / PaddingLG 24).
	tok := theme.Default.Current()
	if math.Abs(tok.Padding-fx.GapMedium) > fx.Tolerance {
		t.Fatalf("TokenPadding=%v want %v", tok.Padding, fx.GapMedium)
	}
	if math.Abs(tok.PaddingLG-fx.GapLarge) > fx.Tolerance {
		t.Fatalf("TokenPaddingLG=%v want %v", tok.PaddingLG, fx.GapLarge)
	}
	f := flex.NewFlex(box(10, 10))
	f.SetGapSize(flex.FlexGapSmall)
	if math.Abs(f.ResolvedGap()-fx.GapSmall) > fx.Tolerance {
		t.Fatalf("small=%v want %v", f.ResolvedGap(), fx.GapSmall)
	}
	f.SetGapSize(flex.FlexGapMedium)
	if math.Abs(f.ResolvedGap()-tok.Padding) > fx.Tolerance {
		t.Fatalf("medium=%v want TokenPadding %v", f.ResolvedGap(), tok.Padding)
	}
	f.SetGapSize(flex.FlexGapLarge)
	if math.Abs(f.ResolvedGap()-tok.PaddingLG) > fx.Tolerance {
		t.Fatalf("large=%v want TokenPaddingLG %v", f.ResolvedGap(), tok.PaddingLG)
	}
	// Small must stay 8 even when PaddingXS drifts to 4.
	drift := tok
	drift.PaddingXS = 4
	f.SetTheme(&drift)
	f.SetGapSize(flex.FlexGapSmall)
	if math.Abs(f.ResolvedGap()-8) > fx.Tolerance {
		t.Fatalf("small with drifted PaddingXS=%v want 8", f.ResolvedGap())
	}
	// Provider override flows through.
	p := theme.NewProvider(theme.DefaultTokens())
	f.SetTheme(nil)
	f.SetProvider(p)
	f.SetGapSize(flex.FlexGapMedium)
	if math.Abs(f.ResolvedGap()-p.Current().Padding) > fx.Tolerance {
		t.Fatalf("provider medium=%v", f.ResolvedGap())
	}
}

func TestFlex_PRD_FLX16_NoChromeColor(t *testing.T) {
	// Pure layout: no color state, theme only feeds gaps.
	f := flex.NewFlex(box(20, 20))
	tok := theme.Default.Current()
	_ = tok.ColorPrimary
	_ = tok.ColorText
	if f.Focusable() {
		t.Fatal("no chrome focus")
	}
	if f.Role() != "" {
		t.Fatalf("unnamed role=%q want empty", f.Role())
	}
	// Gap resolution must not depend on brand color.
	before := f.ResolvedGap()
	alt := tok
	alt.ColorPrimary = theme.Hex("#ff4d4f")
	f.SetTheme(&alt)
	if f.ResolvedGap() != before {
		t.Fatal("gap must not follow brand color")
	}
}

func TestFlex_PRD_FLX17_DisabledNA(t *testing.T) {
	// Disabled does not apply to a pure layout container.
	f := flex.NewFlex(box(20, 20))
	if f.Focusable() {
		t.Fatal("disabled N/A: container never Focusable")
	}
	f.Layout(rendering.Loose(200, 100))
	if f.Role() != "" {
		t.Fatalf("disabled N/A: role=%q", f.Role())
	}
}

func TestFlex_PRD_FLX18_FocusNA(t *testing.T) {
	// Container never takes focus; children own it. Aria only names.
	f := flex.NewFlex(box(20, 20))
	if f.Focusable() {
		t.Fatal("container must not be Focusable")
	}
	f.SetAriaLabel("toolbar")
	if f.AriaLabel() != "toolbar" || f.Role() != "group" {
		t.Fatalf("aria role=%q label=%q", f.Role(), f.AriaLabel())
	}
	if f.Focusable() {
		t.Fatal("AriaLabel must not make container Focusable")
	}
	f.SetAriaLabel("")
	if f.Role() != "" {
		t.Fatalf("cleared role=%q want empty", f.Role())
	}
}
