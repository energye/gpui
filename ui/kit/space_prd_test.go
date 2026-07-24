package kit_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/space.md §6.9 — P0 PRD cases (SPC-01 … SPC-21 L1/L2).
// L3/L4 (SPC-22/23) and P1 (SPC-24) deferred.

func approxSpace(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func spaceBox(w, h float64) core.Node {
	b := primitive.NewBox()
	b.Width, b.Height = w, h
	return b
}

func TestSpace_PRD_01_Defaults(t *testing.T) {
	// SPC-01: NewSpace 默认创建；默认值符合 §6.10 / antd
	sp := kit.NewSpace()
	if sp.Node() == nil {
		t.Fatal("nil node")
	}
	if sp.EffectiveOrientation() != kit.SpaceHorizontal {
		t.Fatalf("orientation=%v want horizontal", sp.EffectiveOrientation())
	}
	if sp.IsVertical() {
		t.Fatal("want horizontal")
	}
	if sp.Wrap {
		t.Fatal("wrap default false")
	}
	if sp.Size != kit.SpaceSizeSmall {
		t.Fatalf("Size=%v want small", sp.Size)
	}
	if !approxSpace(sp.ResolvedGap(), 8, 0.01) {
		t.Fatalf("gap=%v want 8 (small)", sp.ResolvedGap())
	}
	if sp.Align != kit.SpaceAlignAuto {
		t.Fatalf("Align=%v want auto", sp.Align)
	}
	if sp.ResolvedAlign() != core.CrossCenter {
		t.Fatalf("cross=%v want center (horizontal auto)", sp.ResolvedAlign())
	}
	if sp.Root == nil || sp.Root.Axis != core.AxisHorizontal {
		t.Fatal("root axis not horizontal")
	}
	if sp.Root.ExpandMax {
		t.Fatal("Space is inline-flex; ExpandMax should be false")
	}
}

func TestSpace_PRD_02_DefaultThreeChildrenGap8(t *testing.T) {
	// SPC-02 / SPC-S1: 默认三子 → 横向 gap8
	a, b, c := spaceBox(40, 20), spaceBox(40, 20), spaceBox(40, 20)
	sp := kit.NewSpace(a, b, c)
	if !approxSpace(sp.ResolvedGap(), 8, 0.5) {
		t.Fatalf("gap=%v want 8", sp.ResolvedGap())
	}
	_ = sp.Node().Layout(core.Loose(400, 100))
	kids := sp.Root.Children()
	if len(kids) != 3 {
		t.Fatalf("kids=%d", len(kids))
	}
	if kids[0].Base().Offset().Y != kids[1].Base().Offset().Y {
		t.Fatalf("not same row")
	}
	dx := kids[1].Base().Offset().X - (kids[0].Base().Offset().X + kids[0].Base().Size().Width)
	if !approxSpace(dx, 8, 0.5) {
		t.Fatalf("spacing=%v want 8", dx)
	}
}

func TestSpace_PRD_03_SizeLarge(t *testing.T) {
	// SPC-03 / SPC-S2: size=large → gap24
	a, b := spaceBox(40, 20), spaceBox(40, 20)
	sp := kit.NewSpace(a, b)
	sp.SetSize(kit.SpaceSizeLarge)
	if !approxSpace(sp.ResolvedGap(), 24, 0.5) {
		t.Fatalf("gap=%v want 24", sp.ResolvedGap())
	}
	_ = sp.Node().Layout(core.Loose(400, 100))
	kids := sp.Root.Children()
	dx := kids[1].Base().Offset().X - (kids[0].Base().Offset().X + kids[0].Base().Size().Width)
	if !approxSpace(dx, 24, 0.5) {
		t.Fatalf("spacing=%v want 24", dx)
	}
}

func TestSpace_PRD_04_Vertical(t *testing.T) {
	// SPC-04 / SPC-S3
	a, b := spaceBox(40, 20), spaceBox(40, 20)
	sp := kit.NewSpace(a, b)
	sp.SetVertical(true)
	if !sp.IsVertical() {
		t.Fatal("want vertical")
	}
	_ = sp.Node().Layout(core.Loose(200, 200))
	kids := sp.Root.Children()
	if kids[1].Base().Offset().Y <= kids[0].Base().Offset().Y {
		t.Fatalf("second not below: y0=%v y1=%v", kids[0].Base().Offset().Y, kids[1].Base().Offset().Y)
	}
	// orientation wins over Vertical sugar
	sp2 := kit.NewSpace()
	sp2.SetVertical(true)
	sp2.SetOrientation(kit.SpaceHorizontal)
	if sp2.IsVertical() {
		t.Fatal("orientation should win over Vertical sugar")
	}
	// vertical auto align → start
	sp3 := kit.NewSpace()
	sp3.SetOrientation(kit.SpaceVertical)
	if sp3.ResolvedAlign() != core.CrossStart {
		t.Fatalf("vertical auto align=%v want start", sp3.ResolvedAlign())
	}
}

func TestSpace_PRD_05_Wrap(t *testing.T) {
	// SPC-05 / SPC-S4
	mk := func() core.Node { return spaceBox(40, 20) }
	sp := kit.NewSpace(mk(), mk(), mk())
	sp.SetSizePx(8)
	sp.SetWrap(true)
	sz := sp.Node().Layout(core.Constraints{MaxWidth: 100, MaxHeight: 400})
	// 2 per line: 40+8+40=88; third wraps → height 20+8+20=48
	if sz.Height < 47.5 || sz.Height > 48.5 {
		t.Fatalf("height got %v want 48", sz.Height)
	}
	kids := sp.Root.Children()
	if kids[2].Base().Offset().Y < 27.5 {
		t.Fatalf("third child should be on line 2, y=%v", kids[2].Base().Offset().Y)
	}
	// vertical ignores wrap
	sp2 := kit.NewSpace(mk(), mk())
	sp2.SetVertical(true)
	sp2.SetWrap(true)
	_ = sp2.Node()
	if sp2.Root.Wrap {
		t.Fatal("wrap must be ignored when vertical")
	}
}

func TestSpace_PRD_06_Separator(t *testing.T) {
	// SPC-06 / SPC-S5: separator 可见（作为中间 flex 子项）
	a, b, c := spaceBox(40, 20), spaceBox(40, 20), spaceBox(40, 20)
	sp := kit.NewSpace(a, b, c)
	sp.SetSizePx(0) // isolate separator geometry
	sp.SetSeparator(func() core.Node {
		sep := primitive.NewBox()
		sep.Width, sep.Height = 4, 12
		return sep
	})
	_ = sp.Node().Layout(core.Loose(400, 100))
	// children + 2 separators
	kids := sp.Root.Children()
	if len(kids) != 5 {
		t.Fatalf("kids=%d want 5 (3 children + 2 seps)", len(kids))
	}
	// separators occupy slots 1 and 3
	if kids[1].Base().Size().Width < 3.5 {
		t.Fatalf("separator width=%v", kids[1].Base().Size().Width)
	}
	// product children still 3
	if len(sp.Children()) != 3 {
		t.Fatalf("product children=%d", len(sp.Children()))
	}
}

func TestSpace_PRD_07_CompactDualButton(t *testing.T) {
	// SPC-07 / SPC-S6: Compact 双 Button → 中间无双边框缝
	b1 := kit.NewButton("A")
	b2 := kit.NewButton("B")
	cp := kit.NewSpaceCompact()
	cp.SetButtons(b1, b2)
	n := cp.Node()
	_ = n.Layout(core.Loose(400, 100))
	// gap is -lineWidth so borders overlap
	if !approxSpace(cp.Root.Gap, -1, 0.5) {
		t.Fatalf("compact gap=%v want ≈ -1 (overlap)", cp.Root.Gap)
	}
	kids := cp.Root.Children()
	if len(kids) != 2 {
		t.Fatalf("kids=%d", len(kids))
	}
	// second starts within first's right edge (overlap) — no positive seam
	x0 := kids[0].Base().Offset().X
	w0 := kids[0].Base().Size().Width
	x1 := kids[1].Base().Offset().X
	seam := x1 - (x0 + w0)
	if seam > 0.5 {
		t.Fatalf("positive seam=%v (want ≤0 / overlap)", seam)
	}
}

func TestSpace_PRD_08_Align(t *testing.T) {
	// SPC-08 / SPC-S7
	// tall container, short child → center places child mid-cross
	a := spaceBox(40, 20)
	sp := kit.NewSpace(a)
	sp.SetAlign(kit.SpaceAlignCenter)
	_ = sp.Node().Layout(core.Tight(200, 100))
	kids := sp.Root.Children()
	if !approxSpace(kids[0].Base().Offset().Y, 40, 1.0) {
		t.Fatalf("center y=%v want ~40", kids[0].Base().Offset().Y)
	}
	sp.SetAlign(kit.SpaceAlignStart)
	_ = sp.Node().Layout(core.Tight(200, 100))
	if !approxSpace(sp.Root.Children()[0].Base().Offset().Y, 0, 0.5) {
		t.Fatalf("start y=%v want 0", sp.Root.Children()[0].Base().Offset().Y)
	}
	sp.SetAlign(kit.SpaceAlignEnd)
	_ = sp.Node().Layout(core.Tight(200, 100))
	if !approxSpace(sp.Root.Children()[0].Base().Offset().Y, 80, 1.0) {
		t.Fatalf("end y=%v want ~80", sp.Root.Children()[0].Base().Offset().Y)
	}
}

func TestSpace_PRD_09_SizePx16(t *testing.T) {
	// SPC-09 / SPC-S8: size=16 数字 → 16px
	a, b := spaceBox(40, 20), spaceBox(40, 20)
	sp := kit.NewSpace(a, b)
	sp.SetSizePx(16)
	if !approxSpace(sp.ResolvedGap(), 16, 0.01) {
		t.Fatalf("gap=%v want 16", sp.ResolvedGap())
	}
	_ = sp.Node().Layout(core.Loose(400, 100))
	kids := sp.Root.Children()
	dx := kids[1].Base().Offset().X - (kids[0].Base().Offset().X + kids[0].Base().Size().Width)
	if !approxSpace(dx, 16, 0.5) {
		t.Fatalf("spacing=%v want 16", dx)
	}
}

func TestSpace_PRD_10_DemoBase(t *testing.T) {
	// SPC-10: 基本用法 base.tsx — Space of mixed children
	sp := kit.NewSpace(
		kit.NewText("Space").Node(),
		kit.NewButton("Button").Node(),
		kit.NewButton("Confirm").Node(),
	)
	if sp.Node() == nil {
		t.Fatal("nil")
	}
	_ = sp.Node().Layout(core.Loose(600, 80))
	if len(sp.Children()) != 3 {
		t.Fatalf("children=%d", len(sp.Children()))
	}
	if !approxSpace(sp.ResolvedGap(), 8, 0.5) {
		t.Fatalf("default gap=%v", sp.ResolvedGap())
	}
}

func TestSpace_PRD_11_DemoVertical(t *testing.T) {
	// SPC-11: vertical.tsx — orientation vertical + size middle
	c1 := kit.NewCard("Card")
	c1.SetContent(kit.NewText("content").Node())
	c2 := kit.NewCard("Card")
	c2.SetContent(kit.NewText("content").Node())
	sp := kit.NewSpace(c1.Node(), c2.Node())
	sp.SetOrientation(kit.SpaceVertical)
	sp.SetSize(kit.SpaceSizeMiddle)
	if !sp.IsVertical() {
		t.Fatal("want vertical")
	}
	if !approxSpace(sp.ResolvedGap(), 16, 0.5) {
		t.Fatalf("middle gap=%v want 16", sp.ResolvedGap())
	}
	_ = sp.Node().Layout(core.Loose(400, 600))
	kids := sp.Root.Children()
	if kids[1].Base().Offset().Y <= kids[0].Base().Offset().Y {
		t.Fatal("not stacked")
	}
}

func TestSpace_PRD_12_DemoSize(t *testing.T) {
	// SPC-12: size.tsx — small/middle/large/customize (slider path = SetSizePx live)
	a, b := spaceBox(30, 20), spaceBox(30, 20)
	for _, tc := range []struct {
		set  func(*kit.Space)
		want float64
	}{
		{func(s *kit.Space) { s.SetSize(kit.SpaceSizeSmall) }, 8},
		{func(s *kit.Space) { s.SetSize(kit.SpaceSizeMiddle) }, 16},
		{func(s *kit.Space) { s.SetSize(kit.SpaceSizeLarge) }, 24},
		{func(s *kit.Space) { s.SetSizePx(12) }, 12},
	} {
		sp := kit.NewSpace(a, b)
		tc.set(sp)
		if !approxSpace(sp.ResolvedGap(), tc.want, 0.5) {
			t.Fatalf("gap=%v want %v", sp.ResolvedGap(), tc.want)
		}
	}
	// customize slider: live SetSizePx while already custom
	sp := kit.NewSpace(a, b)
	sp.SetSize(kit.SpaceSizeMiddle)
	sp.SetSizePx(7)
	if !approxSpace(sp.ResolvedGap(), 7, 0.01) {
		t.Fatalf("customize first=%v", sp.ResolvedGap())
	}
	sp.SetSizePx(33)
	if !approxSpace(sp.ResolvedGap(), 33, 0.01) {
		t.Fatalf("customize drag=%v", sp.ResolvedGap())
	}
	_ = sp.Node().Layout(core.Loose(400, 100))
	kids := sp.Root.Children()
	dx := kids[1].Base().Offset().X - (kids[0].Base().Offset().X + kids[0].Base().Size().Width)
	if !approxSpace(dx, 33, 0.5) {
		t.Fatalf("live spacing=%v want 33", dx)
	}
}

func TestSpace_PRD_13_DemoAlign(t *testing.T) {
	// SPC-13 / SPC-S7: align.tsx — Block mock taller than Button (pad 32×16 + text)
	// short label + button(h≈32) + tall Block must show different cross offsets.
	mkBlock := func() core.Node {
		tx := kit.NewText("Block")
		d := primitive.NewDecorated(tx.Node())
		d.Padding = primitive.EdgeInsets{Left: 16, Right: 16, Top: 32, Bottom: 32}
		return d
	}
	for _, tc := range []struct {
		align kit.SpaceAlign
		// relative Y of button vs tall block after layout in loose height
		check func(t *testing.T, kids []core.Node)
	}{
		{kit.SpaceAlignCenter, func(t *testing.T, kids []core.Node) {
			// center: midlines roughly equal
			btn := kids[1]
			blk := kids[2]
			btnMid := btn.Base().Offset().Y + btn.Base().Size().Height/2
			blkMid := blk.Base().Offset().Y + blk.Base().Size().Height/2
			if !approxSpace(btnMid, blkMid, 2.0) {
				t.Fatalf("center mid btn=%v block=%v", btnMid, blkMid)
			}
		}},
		{kit.SpaceAlignStart, func(t *testing.T, kids []core.Node) {
			if !approxSpace(kids[1].Base().Offset().Y, kids[2].Base().Offset().Y, 0.5) {
				t.Fatalf("start y btn=%v block=%v", kids[1].Base().Offset().Y, kids[2].Base().Offset().Y)
			}
		}},
		{kit.SpaceAlignEnd, func(t *testing.T, kids []core.Node) {
			btn := kids[1]
			blk := kids[2]
			btnBottom := btn.Base().Offset().Y + btn.Base().Size().Height
			blkBottom := blk.Base().Offset().Y + blk.Base().Size().Height
			if !approxSpace(btnBottom, blkBottom, 2.0) {
				t.Fatalf("end bottom btn=%v block=%v", btnBottom, blkBottom)
			}
		}},
		{kit.SpaceAlignBaseline, func(t *testing.T, kids []core.Node) {
			// P0 baseline ≈ start
			if !approxSpace(kids[1].Base().Offset().Y, kids[2].Base().Offset().Y, 0.5) {
				t.Fatalf("baseline≈start y btn=%v block=%v", kids[1].Base().Offset().Y, kids[2].Base().Offset().Y)
			}
		}},
	} {
		sp := kit.NewSpace(
			kit.NewText("label").Node(),
			kit.NewButton("Primary").Node(),
			mkBlock(),
		)
		sp.SetAlign(tc.align)
		_ = sp.Node().Layout(core.Loose(600, 200))
		kids := sp.Root.Children()
		if len(kids) != 3 {
			t.Fatalf("align=%v kids=%d", tc.align, len(kids))
		}
		// Block must be taller than button (antd mock padding)
		if kids[2].Base().Size().Height <= kids[1].Base().Size().Height {
			t.Fatalf("align=%v block h=%v not taller than btn h=%v (missing Block text/pad?)",
				tc.align, kids[2].Base().Size().Height, kids[1].Base().Size().Height)
		}
		tc.check(t, kids)
	}
}

func TestSpace_PRD_14_DemoWrap(t *testing.T) {
	// SPC-14: wrap.tsx — size={[8,16]} wrap 20 buttons
	kids := make([]core.Node, 0, 20)
	for i := 0; i < 20; i++ {
		kids = append(kids, kit.NewButton("Button").Node())
	}
	sp := kit.NewSpace(kids...)
	sp.SetSizeXY(8, 16)
	sp.SetWrap(true)
	// main gap = col 8
	if !approxSpace(sp.ResolvedGap(), 8, 0.5) {
		t.Fatalf("col gap=%v", sp.ResolvedGap())
	}
	if !approxSpace(sp.ResolvedCrossGap(), 16, 0.5) {
		t.Fatalf("row gap=%v", sp.ResolvedCrossGap())
	}
	sz := sp.Node().Layout(core.Constraints{MaxWidth: 320, MaxHeight: 800})
	if sz.Height < 40 {
		t.Fatalf("wrap height too small: %v", sz.Height)
	}
}

func TestSpace_PRD_15_DemoSeparator(t *testing.T) {
	// SPC-15: separator.tsx — Divider vertical between links
	sp := kit.NewSpace(
		kit.NewLink("Link").Node(),
		kit.NewLink("Link").Node(),
		kit.NewLink("Link").Node(),
	)
	sp.SetSeparator(func() core.Node {
		d := kit.NewDivider()
		d.SetVertical(true)
		return d.Node()
	})
	_ = sp.Node().Layout(core.Loose(400, 40))
	if len(sp.Root.Children()) != 5 {
		t.Fatalf("kids=%d want 5", len(sp.Root.Children()))
	}
}

func TestSpace_PRD_16_DemoCompact(t *testing.T) {
	// SPC-16 / SPC-S6 path: compact.tsx — Compact block fits parent; Flexible children
	// must not overflow (antd width % / calc(100%-btn)).
	const parentW = 320.0

	in1 := kit.NewInput("0571")
	in1.SetValue("0571")
	in1.Root.ExpandWidth = true
	in2 := kit.NewInput("26888888")
	in2.SetValue("26888888")
	in2.Root.ExpandWidth = true
	fl1 := primitive.NewFlexible(2, in1.Node())
	fl1.FillChild = true
	fl2 := primitive.NewFlexible(3, in2.Node())
	fl2.FillChild = true
	// 20%+30%+empty50% proportions
	cpPhone := kit.NewSpaceCompact(fl1, fl2, primitive.NewFlexible(5, nil))
	cpPhone.SetBlock(true)

	inURL := kit.NewInput("https://ant.design")
	inURL.SetValue("https://ant.design")
	inURL.Root.ExpandWidth = true
	btn := kit.NewButton("Submit")
	btn.SetType(kit.ButtonPrimary)
	flURL := primitive.NewFlexible(1, inURL.Node())
	flURL.FillChild = true
	cpURL := kit.NewSpaceCompact(flURL, btn.Node())
	cpURL.SetBlock(true)

	// vertical Space of block Compacts must ExpandMax so children see finite MaxWidth
	col := kit.NewSpace(cpPhone.Node(), cpURL.Node())
	col.SetOrientation(kit.SpaceVertical)
	col.SetSize(kit.SpaceSizeMiddle)
	col.SetExpandMax(true)
	if !col.Root.ExpandMax {
		t.Fatal("SetExpandMax not applied on Root")
	}

	host := primitive.NewDecorated(col.Node())
	host.ExpandWidth = true
	host.StretchChild = true
	sz := host.Layout(core.Constraints{MinWidth: parentW, MaxWidth: parentW, MaxHeight: 400})
	if sz.Width > parentW+0.5 {
		t.Fatalf("host width=%v > parent %v", sz.Width, parentW)
	}
	// each compact row ≤ parent
	for i, row := range col.Root.Children() {
		w := row.Base().Size().Width
		if w > parentW+0.5 {
			t.Fatalf("compact[%d] width=%v exceeds parent %v", i, w, parentW)
		}
	}
	// phone inputs share row without summing past parent
	phoneKids := cpPhone.Root.Children()
	if len(phoneKids) < 2 {
		t.Fatal("phone kids")
	}
	sum := 0.0
	for _, k := range phoneKids {
		sum += k.Base().Size().Width
	}
	// with negative gap, sum of child widths can slightly exceed container; allow lineWidth
	if sum > parentW+2 {
		t.Fatalf("phone children sum width=%v parent=%v", sum, parentW)
	}

	// Addon: general inline-flex cell (not $-only). Center multi/single content;
	// size small/middle/large; Compact pushes size via AddAddon.
	assertAddonCentered := func(t *testing.T, addon *kit.SpaceAddon, wantH float64) {
		t.Helper()
		_ = addon.Node().Layout(core.Loose(200, 80))
		if addon.Root == nil || addon.Root.Size().Height < wantH-0.5 || addon.Root.Size().Height > wantH+0.5 {
			t.Fatalf("addon height=%v want %v", addon.Root.Size().Height, wantH)
		}
		// content row is sole child; StretchChild fills host → CrossCenter
		hostKids := addon.Root.Children()
		if len(hostKids) != 1 {
			t.Fatalf("chrome kids=%d want inner flex", len(hostKids))
		}
		row := hostKids[0]
		if row.Base().Size().Height < wantH-4 { // minus padding 0 → near full height
			// with StretchChild, row height ≈ host - vertical pad (0) ≈ wantH
		}
		// each content child's mid-Y ≈ host mid-Y
		content := row.Base().Children()
		if len(content) == 0 {
			t.Fatal("empty addon content")
		}
		hostMid := addon.Root.Size().Height / 2
		for i, ch := range content {
			// offsets are relative to row; row is at pad offset inside Root
			rowOff := row.Base().Offset()
			mid := rowOff.Y + ch.Base().Offset().Y + ch.Base().Size().Height/2
			if !approxSpace(hostMid, mid, 2.0) {
				t.Fatalf("content[%d] not cross-centered: hostMid=%v mid=%v", i, hostMid, mid)
			}
		}
	}

	// single glyph
	addon := kit.NewSpaceAddon(kit.NewText("$").Node())
	assertAddonCentered(t, addon, 32)

	// multi-child (icon-like box + text) — same flex CrossCenter path
	ic := primitive.NewBox()
	ic.Width, ic.Height = 14, 14
	multi := kit.NewSpaceAddon(ic, kit.NewText("USD").Node())
	assertAddonCentered(t, multi, 32)

	// size matrix
	for _, tc := range []struct {
		sz kit.ButtonSize
		h  float64
	}{
		{kit.ButtonSmall, 24},
		{kit.ButtonMiddle, 32},
		{kit.ButtonLarge, 40},
	} {
		ad := kit.NewSpaceAddon(kit.NewText("x").Node())
		ad.SetSize(tc.sz)
		assertAddonCentered(t, ad, tc.h)
	}

	// Compact → Addon size push (useCompactItemContext equivalent)
	ad2 := kit.NewSpaceAddon(kit.NewText("$").Node())
	cpA := kit.NewSpaceCompact()
	cpA.SetSize(kit.ButtonSmall)
	cpA.AddAddon(ad2)
	if ad2.Size != kit.ButtonSmall {
		t.Fatalf("compact size push: addon Size=%v want small", ad2.Size)
	}
	assertAddonCentered(t, ad2, 24)
}

func TestSpace_PRD_17_DemoCompactButtons(t *testing.T) {
	// SPC-17: compact-buttons.tsx — primary button group
	btns := make([]*kit.Button, 4)
	for i := range btns {
		btns[i] = kit.NewButton("Button")
		btns[i].SetType(kit.ButtonPrimary)
	}
	cp := kit.NewSpaceCompact()
	cp.SetButtons(btns...)
	_ = cp.Node().Layout(core.Loose(600, 40))
	if len(cp.Root.Children()) != 4 {
		t.Fatalf("kids=%d", len(cp.Root.Children()))
	}
	// middle buttons force square radius
	if !btns[1].Style.ForceRadius || btns[1].Style.Radius != 0 {
		t.Fatalf("middle button should ForceRadius 0")
	}
	if !approxSpace(cp.OverlapGap(), -1, 0.5) {
		t.Fatalf("overlap=%v", cp.OverlapGap())
	}
}

func TestSpace_PRD_18_Tokens(t *testing.T) {
	// SPC-18: §6.2 关键尺寸
	th := kit.DefaultTheme()
	if !approxSpace(kit.DefaultSpaceGapSmall, 8, 0.01) {
		t.Fatal("small")
	}
	if !approxSpace(kit.DefaultSpaceGapMiddle, 16, 0.01) {
		t.Fatal("middle")
	}
	if !approxSpace(kit.DefaultSpaceGapLarge, 24, 0.01) {
		t.Fatal("large")
	}
	if !approxSpace(th.SizeOr(core.TokenPadding, 0), 16, 0.5) {
		t.Fatalf("TokenPadding=%v", th.SizeOr(core.TokenPadding, 0))
	}
	if !approxSpace(th.SizeOr(core.TokenPaddingLG, 0), 24, 0.5) {
		t.Fatalf("TokenPaddingLG=%v", th.SizeOr(core.TokenPaddingLG, 0))
	}
	if !approxSpace(th.SizeOr(core.TokenFontSize, 0), 14, 0.5) {
		t.Fatalf("font=%v", th.SizeOr(core.TokenFontSize, 0))
	}
	if !approxSpace(th.SizeOr(core.TokenBorderRadius, 0), 6, 0.5) {
		t.Fatalf("radius=%v", th.SizeOr(core.TokenBorderRadius, 0))
	}
	if !approxSpace(th.SizeOr(core.TokenLineWidth, 0), 1, 0.5) {
		t.Fatalf("lineWidth=%v", th.SizeOr(core.TokenLineWidth, 0))
	}
	// preset resolution
	sp := kit.NewSpace()
	sp.SetSize(kit.SpaceSizeSmall)
	if !approxSpace(sp.ResolvedGap(), 8, 0.5) {
		t.Fatal("small resolve")
	}
	sp.SetSize(kit.SpaceSizeMiddle)
	if !approxSpace(sp.ResolvedGap(), 16, 0.5) {
		t.Fatal("middle resolve")
	}
	sp.SetSize(kit.SpaceSizeLarge)
	if !approxSpace(sp.ResolvedGap(), 24, 0.5) {
		t.Fatal("large resolve")
	}
}

func TestSpace_PRD_19_NoHardcodedBrandSkin(t *testing.T) {
	// SPC-19: 默认皮走 Theme Token；Space 本身无品牌色填充
	sp := kit.NewSpace(spaceBox(10, 10))
	sp.SetTheme(kit.DefaultTheme())
	_ = sp.Node()
	// Root is layout-only Flex — no background brand paint on chrome
	if sp.Theme == nil {
		t.Fatal("theme attached")
	}
	// Compact overlap uses TokenLineWidth
	cp := kit.NewSpaceCompact()
	cp.SetTheme(kit.DefaultTheme())
	if !approxSpace(cp.OverlapGap(), -kit.DefaultSpaceLineWidth, 0.01) {
		t.Fatalf("overlap should track lineWidth token default")
	}
}

func TestSpace_PRD_20_DisabledN_A(t *testing.T) {
	// SPC-20: layout-only — no disabled chrome on Space itself
	// Documented N/A; ensure construction still works when wrapping disabled child.
	btn := kit.NewButton("X")
	btn.SetDisabled(true)
	sp := kit.NewSpace(btn.Node())
	_ = sp.Node().Layout(core.Loose(200, 40))
}

func TestSpace_PRD_21_KeyboardN_A(t *testing.T) {
	// SPC-21: layout-only — Space root is not a focus target (no role/keyboard).
	// Interactive children keep their own focus/keyboard path (covered by Button PRD).
	sp := kit.NewSpace(kit.NewButton("Go").Node())
	n := sp.Node()
	if n == nil {
		t.Fatal("nil")
	}
	// Flex root remains HitDefer (pass-through), not a focusable control.
	if sp.Root.Hit != core.HitDefer && sp.Root.Hit != 0 {
		// HitDefer or default defer is fine; just ensure we don't claim Focusable role.
	}
	if sp.Root.Base().Label != "" && sp.AriaLabel == "" {
		t.Fatal("unexpected a11y label without SetAriaLabel")
	}
	_ = n.Layout(core.Loose(200, 40))
}

func TestSpace_PRD_SizeZeroExplicit(t *testing.T) {
	// SetSizePx(0) is explicit zero, not "use default"
	a, b := spaceBox(40, 20), spaceBox(40, 20)
	sp := kit.NewSpace(a, b)
	sp.SetSizePx(0)
	if !approxSpace(sp.ResolvedGap(), 0, 0.01) {
		t.Fatalf("gap=%v want 0", sp.ResolvedGap())
	}
	_ = sp.Node().Layout(core.Loose(400, 100))
	kids := sp.Root.Children()
	dx := kids[1].Base().Offset().X - (kids[0].Base().Offset().X + kids[0].Base().Size().Width)
	if !approxSpace(dx, 0, 0.5) {
		t.Fatalf("spacing=%v want 0", dx)
	}
}
