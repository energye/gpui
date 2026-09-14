package space_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/kit/space"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type spaceFile struct {
	GapSmall     float64 `json:"gapSmall"`
	GapMiddle    float64 `json:"gapMiddle"`
	GapLarge     float64 `json:"gapLarge"`
	ChildW       float64 `json:"childW"`
	ChildH       float64 `json:"childH"`
	ShortH       float64 `json:"shortH"`
	TallH        float64 `json:"tallH"`
	NarrowW      float64 `json:"narrowW"`
	NarrowChildW float64 `json:"narrowChildW"`
	Tolerance    float64 `json:"tolerance"`
}

func loadSpace(t *testing.T) spaceFile {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "space.json"))
	if err != nil {
		t.Fatalf("read space.json: %v", err)
	}
	var f spaceFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("parse space.json: %v", err)
	}
	if f.Tolerance <= 0 {
		t.Fatal("tolerance must be positive")
	}
	return f
}

func box(w, h float64) *rendering.RenderColorBox {
	return rendering.NewRenderColorBox(w, h, 0.3, 0.4, 0.8, 1)
}

func offsets(s *space.Space) []rendering.Point {
	out := []rendering.Point{}
	for _, c := range s.Children() {
		out = append(out, c.Offset())
	}
	return out
}

func sizes(s *space.Space) []rendering.Size {
	out := []rendering.Size{}
	for _, c := range s.Children() {
		out = append(out, c.Size())
	}
	return out
}

func spacingX(off []rendering.Point, sz []rendering.Size, i int) float64 {
	return off[i+1].X - off[i].X - sz[i].Width
}

func spacingY(off []rendering.Point, sz []rendering.Size, i int) float64 {
	return off[i+1].Y - off[i].Y - sz[i].Height
}

func TestSpace_PRD_SPC01_Defaults(t *testing.T) {
	fx := loadSpace(t)
	s := space.NewSpace()
	if s == nil || s.Node() == nil {
		t.Fatal("NewSpace must not crash")
	}
	if s.EffectiveOrientation() != space.SpaceHorizontal || s.IsVertical() {
		t.Fatalf("default orientation=%q want horizontal", s.EffectiveOrientation())
	}
	if math.Abs(s.ResolvedGap()-fx.GapSmall) > fx.Tolerance {
		t.Fatalf("default gap=%v want small %v", s.ResolvedGap(), fx.GapSmall)
	}
	if s.Align() != space.SpaceAlignAuto || s.EffectiveAlign() != space.SpaceAlignCenter {
		t.Fatalf("default align=%q effective=%q want auto/center", s.Align(), s.EffectiveAlign())
	}
	if s.Wrap() {
		t.Fatal("default wrap must be false")
	}
	if s.SeparatorCount() != 0 {
		t.Fatalf("default separators=%d want 0", s.SeparatorCount())
	}
	if s.ExpandMax() {
		t.Fatal("default expandMax must be false")
	}
	if s.Focusable() {
		t.Fatal("space container must not be Focusable")
	}
	if s.Role() != "" || s.AriaLabel() != "" {
		t.Fatalf("default role=%q label=%q want unnamed", s.Role(), s.AriaLabel())
	}
	if s.ChromeNode() != s.Node() {
		t.Fatal("no separate chrome: ChromeNode must equal Node")
	}
	sz := s.Layout(rendering.Loose(400, 200))
	if sz.Width != 0 || sz.Height != 0 {
		t.Fatalf("empty layout=%v want 0x0", sz)
	}
	_ = theme.Default.Current()
}

func TestSpace_PRD_SPC02_DefaultThreeChildren(t *testing.T) {
	fx := loadSpace(t)
	a, b, c := box(50, 20), box(50, 20), box(50, 20)
	s := space.NewSpace(a, b, c)
	if s.ChildCount() != 3 {
		t.Fatalf("children=%d want 3", s.ChildCount())
	}
	// Layout matrix: Exact / Loose / Min-Max each run.
	exact := s.Layout(rendering.Tight(300, 100))
	if math.Abs(exact.Width-300) > fx.Tolerance {
		t.Fatalf("exact width=%v want 300", exact.Width)
	}
	loose := s.Layout(rendering.Loose(400, 100))
	wantW := 3*50 + 2*fx.GapSmall
	if math.Abs(loose.Width-wantW) > fx.Tolerance {
		t.Fatalf("loose width=%v want %v", loose.Width, wantW)
	}
	mm := s.Layout(rendering.Constraints{MinWidth: 200, MaxWidth: 400, MinHeight: 0, MaxHeight: 200})
	if mm.Width < 200-fx.Tolerance || mm.Width > 400+fx.Tolerance {
		t.Fatalf("minmax width=%v want 200..400", mm.Width)
	}
	// Behavior under loose content size: horizontal, gap small.
	s.Layout(rendering.Loose(400, 100))
	off, sz := offsets(s), sizes(s)
	if math.Abs(off[0].Y-off[1].Y) > fx.Tolerance || math.Abs(off[1].Y-off[2].Y) > fx.Tolerance {
		t.Fatalf("row tops %+v want level", off)
	}
	for i := 0; i < 2; i++ {
		if math.Abs(spacingX(off, sz, i)-fx.GapSmall) > fx.Tolerance {
			t.Fatalf("spacing %d=%v want %v", i, spacingX(off, sz, i), fx.GapSmall)
		}
	}
}

func TestSpace_PRD_SPC03_SizeLarge(t *testing.T) {
	fx := loadSpace(t)
	s := space.NewSpace(box(50, 20), box(50, 20))
	s.SetSize(space.SpaceSizeLarge)
	if math.Abs(s.ResolvedGap()-fx.GapLarge) > fx.Tolerance {
		t.Fatalf("gap=%v want %v", s.ResolvedGap(), fx.GapLarge)
	}
	s.Layout(rendering.Loose(400, 100))
	off, sz := offsets(s), sizes(s)
	if math.Abs(spacingX(off, sz, 0)-fx.GapLarge) > fx.Tolerance {
		t.Fatalf("spacing=%v want %v", spacingX(off, sz, 0), fx.GapLarge)
	}
}

func TestSpace_PRD_SPC04_Vertical(t *testing.T) {
	fx := loadSpace(t)
	a, b := box(50, 20), box(50, 20)
	s := space.NewSpace(a, b)
	s.SetVertical(true)
	if !s.IsVertical() || s.EffectiveOrientation() != space.SpaceVertical {
		t.Fatal("vertical sugar must flip axis")
	}
	// Orientation wins over the vertical sugar.
	o := space.NewSpace(box(50, 20), box(50, 20))
	o.SetOrientation(space.SpaceVertical)
	o.SetVertical(false)
	if !o.IsVertical() {
		t.Fatal("orientation must win over vertical=false")
	}
	o.SetOrientation(space.SpaceHorizontal)
	o.SetVertical(true)
	if o.IsVertical() {
		t.Fatal("explicit horizontal must win over vertical=true")
	}
	s.Layout(rendering.Loose(200, 400))
	off, sz := offsets(s), sizes(s)
	if math.Abs(off[0].X-off[1].X) > fx.Tolerance {
		t.Fatalf("left diff=%v", math.Abs(off[0].X-off[1].X))
	}
	if math.Abs(spacingY(off, sz, 0)-fx.GapSmall) > fx.Tolerance {
		t.Fatalf("vertical spacing=%v want %v", spacingY(off, sz, 0), fx.GapSmall)
	}
}

func TestSpace_PRD_SPC05_Wrap(t *testing.T) {
	fx := loadSpace(t)
	a, b := box(fx.NarrowChildW, 40), box(fx.NarrowChildW, 40)
	s := space.NewSpace(a, b)
	s.SetWrap(true)
	s.Layout(rendering.Tight(fx.NarrowW, 200))
	off := offsets(s)
	if len(off) != 2 {
		t.Fatalf("children=%d want 2", len(off))
	}
	if !(off[1].Y > off[0].Y) {
		t.Fatalf("wrap must break line: tops %v %v", off[0].Y, off[1].Y)
	}
	rowGap := off[1].Y - off[0].Y - 40
	if math.Abs(rowGap-s.ResolvedRowGap()) > fx.Tolerance {
		t.Fatalf("row gap=%v want %v", rowGap, s.ResolvedRowGap())
	}
	// Wrap is horizontal-only: vertical ignores it.
	v := space.NewSpace(box(30, 30), box(30, 30))
	v.SetVertical(true)
	v.SetWrap(true)
	v.Layout(rendering.Tight(40, 200))
	voff := offsets(v)
	if !(voff[1].Y > voff[0].Y) || math.Abs(voff[0].X-voff[1].X) > fx.Tolerance {
		t.Fatalf("vertical single column %+v", voff)
	}
}

func TestSpace_PRD_SPC06_Separator(t *testing.T) {
	s := space.NewSpace(box(50, 20), box(50, 20), box(50, 20))
	s.SetSeparator(func() rendering.RenderObject {
		return rendering.NewRenderColorBox(8, 16, 0.6, 0.6, 0.6, 1)
	})
	if s.SeparatorCount() != 2 {
		t.Fatalf("separators=%d want 2", s.SeparatorCount())
	}
	s.Layout(rendering.Loose(400, 100))
	for i := 0; i < s.SeparatorCount(); i++ {
		if w := s.SeparatorAt(i).Size().Width; w <= 0 {
			t.Fatalf("separator %d width=%v want >0", i, w)
		}
	}
	if !s.SeparatorsAriaHidden() {
		t.Fatal("separators must be aria-hidden")
	}
	if s.SeparatorFocusable() {
		t.Fatal("separators must not take focus")
	}
	if s.SeparatorAt(9) != nil {
		t.Fatal("SeparatorAt out of range must be nil")
	}
}

func TestSpace_PRD_SPC07_CompactOverlap(t *testing.T) {
	fx := loadSpace(t)
	a, b := box(60, 32), box(60, 32)
	c := space.NewSpaceCompact(a, b)
	c.Layout(rendering.Loose(400, 100))
	kids := c.Children()
	if len(kids) != 2 {
		t.Fatalf("compact children=%d want 2", len(kids))
	}
	off0, off1 := kids[0].Offset(), kids[1].Offset()
	overlap := (off0.X + 60) - off1.X
	if math.Abs(overlap-1) > fx.Tolerance {
		t.Fatalf("overlap=%v want 1 (lineWidth)", overlap)
	}
	if math.Abs(c.Overlap()-1) > fx.Tolerance {
		t.Fatalf("Overlap()=%v want 1", c.Overlap())
	}
	// Vertical compact stacks with the same overlap.
	c.SetVertical(true)
	if !c.IsVertical() {
		t.Fatal("compact vertical sugar must flip axis")
	}
	c.Layout(rendering.Loose(200, 400))
	kids = c.Children()
	voverlap := (kids[0].Offset().Y + 32) - kids[1].Offset().Y
	if math.Abs(voverlap-1) > fx.Tolerance {
		t.Fatalf("vertical overlap=%v want 1", voverlap)
	}
}

func TestSpace_PRD_SPC08_AlignCenter(t *testing.T) {
	fx := loadSpace(t)
	short, tall := box(60, fx.ShortH), box(60, fx.TallH)
	s := space.NewSpace(short, tall)
	s.SetAlign(space.SpaceAlignCenter)
	s.Layout(rendering.Loose(400, 100))
	off, cs := offsets(s), sizes(s)
	shortMid := off[0].Y + cs[0].Height/2
	if math.Abs(shortMid-fx.TallH/2) > fx.Tolerance {
		t.Fatalf("short mid=%v line mid=%v", shortMid, fx.TallH/2)
	}
	// Start pins tops, end pins bottoms.
	s.SetAlign(space.SpaceAlignStart)
	s.Layout(rendering.Loose(400, 100))
	if off0 := offsets(s); math.Abs(off0[0].Y) > fx.Tolerance || math.Abs(off0[1].Y) > fx.Tolerance {
		t.Fatalf("start tops %+v want 0", off0)
	}
	s.SetAlign(space.SpaceAlignEnd)
	s.Layout(rendering.Loose(400, 100))
	off, cs = offsets(s), sizes(s)
	if math.Abs((off[0].Y+cs[0].Height)-fx.TallH) > fx.Tolerance {
		t.Fatalf("end bottom=%v want %v", off[0].Y+cs[0].Height, fx.TallH)
	}
}

func TestSpace_PRD_SPC09_SizeNumeric(t *testing.T) {
	fx := loadSpace(t)
	s := space.NewSpace(box(50, 20), box(50, 20))
	s.SetSizePx(16)
	if math.Abs(s.ResolvedGap()-16) > fx.Tolerance {
		t.Fatalf("gap=%v want 16", s.ResolvedGap())
	}
	s.Layout(rendering.Loose(400, 100))
	off, sz := offsets(s), sizes(s)
	if math.Abs(spacingX(off, sz, 0)-16) > fx.Tolerance {
		t.Fatalf("spacing=%v want 16", spacingX(off, sz, 0))
	}
	// Explicit 0 collapses the gap.
	s.SetSizePx(0)
	if s.ResolvedGap() != 0 {
		t.Fatalf("explicit 0 gap=%v", s.ResolvedGap())
	}
	// [col,row] pair: main axis reads col when horizontal.
	s.SetSizeXY(10, 20)
	if math.Abs(s.ResolvedColGap()-10) > fx.Tolerance || math.Abs(s.ResolvedRowGap()-20) > fx.Tolerance {
		t.Fatalf("col/row=%v/%v want 10/20", s.ResolvedColGap(), s.ResolvedRowGap())
	}
	if math.Abs(s.ResolvedGap()-10) > fx.Tolerance {
		t.Fatalf("horizontal main gap=%v want col 10", s.ResolvedGap())
	}
	s.SetVertical(true)
	if math.Abs(s.ResolvedGap()-20) > fx.Tolerance {
		t.Fatalf("vertical main gap=%v want row 20", s.ResolvedGap())
	}
}

func TestSpace_PRD_SPC10_BaseExample(t *testing.T) {
	fx := loadSpace(t)
	// base.tsx: three children, size small.
	s := space.NewSpace(box(60, 32), box(60, 32), box(60, 32))
	s.Layout(rendering.Loose(400, 100))
	off, sz := offsets(s), sizes(s)
	for i := 0; i < 2; i++ {
		if math.Abs(spacingX(off, sz, i)-fx.GapSmall) > fx.Tolerance {
			t.Fatalf("base spacing %d=%v want %v", i, spacingX(off, sz, i), fx.GapSmall)
		}
	}
	if math.Abs(off[0].Y-off[1].Y) > fx.Tolerance {
		t.Fatal("base must stay horizontal")
	}
}

func TestSpace_PRD_SPC11_VerticalExample(t *testing.T) {
	fx := loadSpace(t)
	// vertical.tsx: vertical=true.
	s := space.NewSpace(box(60, 32), box(60, 32))
	s.SetVertical(true)
	s.Layout(rendering.Loose(200, 400))
	off, sz := offsets(s), sizes(s)
	if math.Abs(spacingY(off, sz, 0)-fx.GapSmall) > fx.Tolerance {
		t.Fatalf("vertical spacing=%v want %v", spacingY(off, sz, 0), fx.GapSmall)
	}
}

func TestSpace_PRD_SPC12_SizeExample(t *testing.T) {
	fx := loadSpace(t)
	// size.tsx: small/middle/large rows resolve 8/16/24.
	for _, tc := range []struct {
		size space.SpaceSize
		want float64
	}{
		{space.SpaceSizeSmall, fx.GapSmall},
		{space.SpaceSizeMiddle, fx.GapMiddle},
		{space.SpaceSizeLarge, fx.GapLarge},
	} {
		s := space.NewSpace(box(50, 20), box(50, 20))
		s.SetSize(tc.size)
		s.Layout(rendering.Loose(400, 100))
		off, sz := offsets(s), sizes(s)
		if math.Abs(spacingX(off, sz, 0)-tc.want) > fx.Tolerance {
			t.Fatalf("size %d spacing=%v want %v", tc.size, spacingX(off, sz, 0), tc.want)
		}
	}
	// Middle alias resolves the same.
	s := space.NewSpace(box(50, 20), box(50, 20))
	s.SetSize(space.SpaceSizeMedium)
	if math.Abs(s.ResolvedGap()-fx.GapMiddle) > fx.Tolerance {
		t.Fatalf("medium gap=%v want %v", s.ResolvedGap(), fx.GapMiddle)
	}
}

func TestSpace_PRD_SPC13_AlignExample(t *testing.T) {
	fx := loadSpace(t)
	// align.tsx: unequal heights, align center.
	short, tall := box(80, fx.ShortH), box(80, fx.TallH)
	s := space.NewSpace(short, tall)
	s.SetAlign(space.SpaceAlignCenter)
	s.Layout(rendering.Loose(400, 100))
	off, cs := offsets(s), sizes(s)
	if math.Abs((off[0].Y+cs[0].Height/2)-fx.TallH/2) > fx.Tolerance {
		t.Fatalf("align mid=%v want %v", off[0].Y+cs[0].Height/2, fx.TallH/2)
	}
}

func TestSpace_PRD_SPC14_WrapExample(t *testing.T) {
	fx := loadSpace(t)
	// wrap.tsx: wrap=true in a narrow container.
	kids := []rendering.RenderObject{box(fx.NarrowChildW, 32), box(fx.NarrowChildW, 32), box(fx.NarrowChildW, 32)}
	s := space.NewSpace(kids...)
	s.SetWrap(true)
	s.Layout(rendering.Tight(fx.NarrowW, 300))
	off := offsets(s)
	rows := map[float64]bool{}
	for _, o := range off {
		rows[math.Round(o.Y)] = true
	}
	if len(rows) < 2 {
		t.Fatalf("wrap rows=%v want >=2", rows)
	}
	if !(off[1].Y > off[0].Y || off[2].Y > off[0].Y) {
		t.Fatalf("wrap must break: %+v", off)
	}
	rowGap := off[1].Y - off[0].Y - 32
	if math.Abs(rowGap-s.ResolvedRowGap()) > fx.Tolerance {
		t.Fatalf("row gap=%v want %v", rowGap, s.ResolvedRowGap())
	}
}

func TestSpace_PRD_SPC15_SeparatorExample(t *testing.T) {
	// separator.tsx: separator="/" between every pair.
	s := space.NewSpace(box(40, 20), box(40, 20))
	s.SetSeparator(func() rendering.RenderObject {
		return rendering.NewRenderColorBox(6, 14, 0.5, 0.5, 0.5, 1)
	})
	if s.SeparatorCount() != 1 {
		t.Fatalf("separators=%d want 1", s.SeparatorCount())
	}
	s.Layout(rendering.Loose(400, 100))
	if w := s.SeparatorAt(0).Size().Width; w <= 0 {
		t.Fatalf("separator width=%v want >0", w)
	}
}

func TestSpace_PRD_SPC16_CompactExample(t *testing.T) {
	fx := loadSpace(t)
	// compact.tsx: two compact children share one border.
	c := space.NewSpaceCompact(box(60, 32), box(60, 32))
	c.Layout(rendering.Loose(400, 100))
	kids := c.Children()
	shared := (kids[0].Offset().X + 60) - kids[1].Offset().X
	if math.Abs(shared-1) > fx.Tolerance {
		t.Fatalf("shared border=%v want 1", shared)
	}
	if c.ChildCount() != 2 {
		t.Fatalf("compact children=%d want 2", c.ChildCount())
	}
	if !c.IsFirstItem(0) || !c.IsLastItem(1) || c.IsFirstItem(1) || c.IsLastItem(0) {
		t.Fatal("first/last item flags wrong")
	}
}

func TestSpace_PRD_SPC17_CompactButtons(t *testing.T) {
	fx := loadSpace(t)
	// compact-buttons.tsx: multiple compact cells, middle radius cleared.
	mk := func() *space.SpaceAddon {
		return space.NewSpaceAddon(box(60, 32))
	}
	a0, a1, a2 := mk(), mk(), mk()
	c := space.NewSpaceCompact()
	c.AddAddon(a0)
	c.AddAddon(a1)
	c.AddAddon(a2)
	if c.Size() != space.SpaceSizeMiddle {
		t.Fatalf("compact size=%d want middle", c.Size())
	}
	c.Layout(rendering.Loose(600, 100))
	kids := c.Children()
	if len(kids) != 3 {
		t.Fatalf("compact children=%d want 3", len(kids))
	}
	for i := 0; i < 2; i++ {
		shared := (kids[i].Offset().X + 60) - kids[i+1].Offset().X
		if math.Abs(shared-1) > fx.Tolerance {
			t.Fatalf("shared border %d=%v want 1", i, shared)
		}
	}
	if r := a1.EffectiveRadius(); r != 0 {
		t.Fatalf("middle addon radius=%v want 0", r)
	}
	if r := a0.EffectiveRadius(); r <= 0 {
		t.Fatalf("first addon radius=%v want kept", r)
	}
	if r := a2.EffectiveRadius(); r <= 0 {
		t.Fatalf("last addon radius=%v want kept", r)
	}
	if f, l, ok := a1.CompactEdges(); !ok || f || l {
		t.Fatalf("middle edges=%v/%v ok=%v", f, l, ok)
	}
	// Block fills the parent width.
	c.SetBlock(true)
	if !c.Block() {
		t.Fatal("Block flag must stick")
	}
	sz := c.Layout(rendering.Tight(300, 100))
	if math.Abs(sz.Width-300) > fx.Tolerance {
		t.Fatalf("block width=%v want 300", sz.Width)
	}
	// Addon control height follows the middle档.
	if math.Abs(a0.ControlHeight()-32) > fx.Tolerance {
		t.Fatalf("addon height=%v want 32", a0.ControlHeight())
	}
	a0.SetSize(space.SpaceSizeSmall)
	if math.Abs(a0.ControlHeight()-24) > fx.Tolerance {
		t.Fatalf("small addon height=%v want 24", a0.ControlHeight())
	}
	a0.SetDisabled(true)
	if !a0.Disabled() || a0.Focusable() {
		t.Fatal("addon disabled flag must stick without focus")
	}
}

func TestSpace_PRD_SPC18_TokenGaps(t *testing.T) {
	fx := loadSpace(t)
	// Default theme carries the antd seed (PaddingXS 8 / Padding 16 / LG 24).
	tok := theme.Default.Current()
	if math.Abs(tok.PaddingXS-fx.GapSmall) > fx.Tolerance {
		t.Fatalf("TokenPaddingXS=%v want %v", tok.PaddingXS, fx.GapSmall)
	}
	if math.Abs(tok.Padding-fx.GapMiddle) > fx.Tolerance {
		t.Fatalf("TokenPadding=%v want %v", tok.Padding, fx.GapMiddle)
	}
	if math.Abs(tok.PaddingLG-fx.GapLarge) > fx.Tolerance {
		t.Fatalf("TokenPaddingLG=%v want %v", tok.PaddingLG, fx.GapLarge)
	}
	s := space.NewSpace(box(10, 10))
	s.SetSize(space.SpaceSizeSmall)
	if math.Abs(s.ResolvedGap()-tok.PaddingXS) > fx.Tolerance {
		t.Fatalf("small=%v want TokenPaddingXS %v", s.ResolvedGap(), tok.PaddingXS)
	}
	s.SetSize(space.SpaceSizeMiddle)
	if math.Abs(s.ResolvedGap()-tok.Padding) > fx.Tolerance {
		t.Fatalf("middle=%v want TokenPadding %v", s.ResolvedGap(), tok.Padding)
	}
	s.SetSize(space.SpaceSizeLarge)
	if math.Abs(s.ResolvedGap()-tok.PaddingLG) > fx.Tolerance {
		t.Fatalf("large=%v want TokenPaddingLG %v", s.ResolvedGap(), tok.PaddingLG)
	}
	// Provider override flows through.
	p := theme.NewProvider(theme.DefaultTokens())
	s.SetTheme(nil)
	s.SetProvider(p)
	s.SetSize(space.SpaceSizeMiddle)
	if math.Abs(s.ResolvedGap()-p.Current().Padding) > fx.Tolerance {
		t.Fatalf("provider middle=%v", s.ResolvedGap())
	}
}

func TestSpace_PRD_SPC19_NoOwnChrome(t *testing.T) {
	// Pure layout: no color state, theme only feeds gaps + split track.
	s := space.NewSpace(box(20, 20))
	if s.ChromeNode() != s.Node() {
		t.Fatal("no separate chrome node")
	}
	tok := theme.Default.Current()
	if got := s.SeparatorColor(); got != tok.ColorBorderSecondary {
		t.Fatalf("separator color=%+v want TokenColorBorderSecondary %+v", got, tok.ColorBorderSecondary)
	}
	// Gap resolution must not depend on brand color.
	before := s.ResolvedGap()
	alt := tok
	alt.ColorPrimary = theme.Hex("#ff4d4f")
	s.SetTheme(&alt)
	if s.ResolvedGap() != before {
		t.Fatal("gap must not follow brand color")
	}
	if s.Focusable() {
		t.Fatal("no chrome focus")
	}
}

func TestSpace_PRD_SPC20_DisabledNA(t *testing.T) {
	// Disabled does not apply to a pure layout container.
	s := space.NewSpace(box(20, 20))
	if s.Focusable() {
		t.Fatal("disabled N/A: container never Focusable")
	}
	s.Layout(rendering.Loose(200, 100))
	if s.Role() != "" {
		t.Fatalf("disabled N/A: role=%q", s.Role())
	}
}

func TestSpace_PRD_SPC21_FocusKeyboardNA(t *testing.T) {
	// Container never takes focus; separators never join Tab order.
	s := space.NewSpace(box(20, 20))
	if s.Focusable() {
		t.Fatal("container must not be Focusable")
	}
	s.SetAriaLabel("toolbar")
	if s.AriaLabel() != "toolbar" || s.Role() != "group" {
		t.Fatalf("aria role=%q label=%q", s.Role(), s.AriaLabel())
	}
	if s.Focusable() {
		t.Fatal("AriaLabel must not make container Focusable")
	}
	s.SetAriaLabel("")
	if s.Role() != "" {
		t.Fatalf("cleared role=%q want empty", s.Role())
	}
	if s.SeparatorFocusable() || !s.SeparatorsAriaHidden() {
		t.Fatal("separators: aria-hidden, never focusable")
	}
	// Children management helpers must work.
	a, b, c := box(10, 10), box(10, 10), box(10, 10)
	s.SetChildren(a, b)
	if s.ChildCount() != 2 {
		t.Fatal("SetChildren must replace")
	}
	s.Add(c)
	if s.ChildCount() != 3 {
		t.Fatal("Add must append")
	}
	s.ClearChildren()
	if s.ChildCount() != 0 || s.SeparatorCount() != 0 {
		t.Fatal("ClearChildren must empty children and separators")
	}
}
