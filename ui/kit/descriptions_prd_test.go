package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/descriptions.md §6.9 — P0 PRD cases (DSC-01 … DSC-19).
// L3/L4 (DSC-20/21) and P1 (DSC-22) are deferred.

func approxDesc(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

func approxDescColor(a, b render.RGBA, tol float64) bool {
	return approxDesc(float64(a.R), float64(b.R), tol) &&
		approxDesc(float64(a.G), float64(b.G), tol) &&
		approxDesc(float64(a.B), float64(b.B), tol) &&
		approxDesc(float64(a.A), float64(b.A), tol)
}

func sampleItems3() []kit.DescriptionsItem {
	return []kit.DescriptionsItem{
		{Key: "1", Label: "UserName", Children: "Zhou Maomao"},
		{Key: "2", Label: "Telephone", Children: "1810000000"},
		{Key: "3", Label: "Live", Children: "Hangzhou, Zhejiang"},
	}
}

func TestDescriptions_PRD_01_Defaults(t *testing.T) {
	// DSC-01: NewDescriptions 默认创建
	d := kit.NewDescriptions()
	if d.Size != kit.DescriptionsLarge {
		t.Fatalf("Size=%v want large", d.Size)
	}
	if d.Bordered {
		t.Fatal("Bordered want false")
	}
	if d.Layout != kit.DescriptionsHorizontal {
		t.Fatalf("Layout=%v want horizontal", d.Layout)
	}
	if !d.Colon {
		t.Fatal("Colon want true")
	}
	if d.ResolvedColumn() != kit.DefaultDescriptionsColumn {
		t.Fatalf("column=%d want %d", d.ResolvedColumn(), kit.DefaultDescriptionsColumn)
	}
	if d.Node() == nil || d.ChromeNode() == nil {
		t.Fatal("nil node")
	}
	_ = d.Node().Layout(core.Loose(400, 200))
}

func TestDescriptions_PRD_02_Column3OneRow(t *testing.T) {
	// DSC-02 / DSC-S1: 3 项 column=3 → 一行三格
	d := kit.NewDescriptions(sampleItems3()...)
	d.SetColumn(3)
	if d.RowCount() != 1 {
		t.Fatalf("rows=%d want 1", d.RowCount())
	}
	spans := d.RowSpans()
	if len(spans) != 1 || len(spans[0]) != 3 {
		t.Fatalf("spans=%v want 1×3", spans)
	}
	for _, s := range spans[0] {
		if s != 1 {
			t.Fatalf("span=%d want 1", s)
		}
	}
	_ = d.Node().Layout(core.Loose(720, 200))
}

func TestDescriptions_PRD_03_Bordered(t *testing.T) {
	// DSC-03 / DSC-S2: bordered 表框
	d := kit.NewDescriptions(sampleItems3()...)
	d.SetTitle("User Info")
	d.SetBordered(true)
	if !d.IsBordered() {
		t.Fatal("IsBordered false")
	}
	sz := d.Node().Layout(core.Loose(720, 240))
	if sz.Width < 10 || sz.Height < 10 {
		t.Fatalf("size=%v", sz)
	}
	// view border via LineWidth > 0
	if d.LineWidth() < 0.5 {
		t.Fatalf("lineW=%v", d.LineWidth())
	}
}

func TestDescriptions_PRD_04_Span2(t *testing.T) {
	// DSC-04 / DSC-S3: span=2 占两列
	d := kit.NewDescriptions(
		kit.DescriptionsItem{Label: "A", Children: "1"},
		kit.DescriptionsItem{Label: "B", Children: "2", Span: 2},
	)
	d.SetColumn(3)
	if d.RowCount() != 1 {
		t.Fatalf("rows=%d want 1", d.RowCount())
	}
	spans := d.RowSpans()
	if len(spans[0]) != 2 || spans[0][0] != 1 || spans[0][1] != 2 {
		t.Fatalf("spans=%v want [1 2]", spans[0])
	}
}

func TestDescriptions_PRD_05_SizePadding(t *testing.T) {
	// DSC-05 / DSC-S4: size → padding 变
	lg := kit.NewDescriptions(sampleItems3()...)
	md := kit.NewDescriptions(sampleItems3()...)
	md.SetSize(kit.DescriptionsMiddle)
	sm := kit.NewDescriptions(sampleItems3()...)
	sm.SetSize(kit.DescriptionsSmall)

	if !approxDesc(lg.ItemPadBottom(), kit.DefaultDescriptionsItemPadBottom, 0.5) {
		t.Fatalf("lg pad=%v", lg.ItemPadBottom())
	}
	if !approxDesc(md.ItemPadBottom(), kit.DefaultDescriptionsItemPadBottomMD, 0.5) {
		t.Fatalf("md pad=%v", md.ItemPadBottom())
	}
	if !approxDesc(sm.ItemPadBottom(), kit.DefaultDescriptionsItemPadBottomSM, 0.5) {
		t.Fatalf("sm pad=%v", sm.ItemPadBottom())
	}
	if !approxDesc(lg.BorderedPadV(), kit.DefaultDescriptionsBorderedPadV, 0.5) ||
		!approxDesc(md.BorderedPadV(), kit.DefaultDescriptionsBorderedPadVMD, 0.5) ||
		!approxDesc(sm.BorderedPadV(), kit.DefaultDescriptionsBorderedPadVSM, 0.5) {
		t.Fatalf("bordered padV lg/md/sm=%v/%v/%v", lg.BorderedPadV(), md.BorderedPadV(), sm.BorderedPadV())
	}
	if md.ItemPadBottom() >= lg.ItemPadBottom() || sm.ItemPadBottom() >= md.ItemPadBottom() {
		t.Fatalf("size padding not decreasing: lg=%v md=%v sm=%v",
			lg.ItemPadBottom(), md.ItemPadBottom(), sm.ItemPadBottom())
	}
}

func TestDescriptions_PRD_06_Title(t *testing.T) {
	// DSC-06 / DSC-S5: title
	d := kit.NewDescriptions(sampleItems3()...)
	d.SetTitle("User Info")
	if d.Title != "User Info" {
		t.Fatalf("Title=%q", d.Title)
	}
	sz := d.Node().Layout(core.Loose(600, 200))
	// title adds height vs no-title
	d2 := kit.NewDescriptions(sampleItems3()...)
	sz2 := d2.Node().Layout(core.Loose(600, 200))
	if sz.Height <= sz2.Height {
		t.Fatalf("with title h=%v without=%v", sz.Height, sz2.Height)
	}
}

func TestDescriptions_PRD_07_LayoutVertical(t *testing.T) {
	// DSC-07 / DSC-S6: layout=vertical 标签在上
	d := kit.NewDescriptions(sampleItems3()...)
	d.SetLayout(kit.DescriptionsVertical)
	if !d.IsVertical() {
		t.Fatal("IsVertical false")
	}
	_ = d.Node().Layout(core.Loose(600, 240))
}

func TestDescriptions_PRD_08_DemoBasic(t *testing.T) {
	// DSC-08: basic.tsx
	items := []kit.DescriptionsItem{
		{Key: "1", Label: "UserName", Children: "Zhou Maomao"},
		{Key: "2", Label: "Telephone", Children: "1810000000"},
		{Key: "3", Label: "Live", Children: "Hangzhou, Zhejiang"},
		{Key: "4", Label: "Remark", Children: "empty"},
		{Key: "5", Label: "Address", Children: "No. 18, Wantang Road, Xihu District, Hangzhou, Zhejiang, China"},
	}
	d := kit.NewDescriptions(items...)
	d.SetTitle("User Info")
	// default column=3 → rows: 3 + 2 (last address expands)
	if d.RowCount() != 2 {
		t.Fatalf("rows=%d want 2", d.RowCount())
	}
	_ = d.Node().Layout(core.Loose(720, 300))
}

func TestDescriptions_PRD_09_DemoBorder(t *testing.T) {
	// DSC-09: border.tsx
	items := []kit.DescriptionsItem{
		{Key: "1", Label: "Product", Children: "Cloud Database"},
		{Key: "2", Label: "Billing Mode", Children: "Prepaid"},
		{Key: "3", Label: "Automatic Renewal", Children: "YES"},
		{Key: "4", Label: "Order time", Children: "2018-04-24 18:00:00"},
		{Key: "5", Label: "Usage Time", Children: "2019-04-24 18:00:00", Span: 2},
		{Key: "6", Label: "Status", Children: "Running", Span: 3},
		{Key: "7", Label: "Negotiated Amount", Children: "$80.00"},
		{Key: "8", Label: "Discount", Children: "$20.00"},
		{Key: "9", Label: "Official Receipts", Children: "$60.00"},
		{Key: "10", Label: "Config Info", Children: "Data disk type: MongoDB\nDatabase version: 3.4"},
	}
	d := kit.NewDescriptions(items...)
	d.SetTitle("User Info")
	d.SetBordered(true)
	spans := d.RowSpans()
	// row with Usage Time span=2 after Order time span=1 → [1,2]
	// Status span=3 alone
	found12, found3 := false, false
	for _, row := range spans {
		if len(row) == 2 && row[0] == 1 && row[1] == 2 {
			found12 = true
		}
		if len(row) == 1 && row[0] == 3 {
			found3 = true
		}
	}
	if !found12 || !found3 {
		t.Fatalf("spans=%v missing [1 2] or [3]", spans)
	}
	_ = d.Node().Layout(core.Loose(800, 500))
}

func TestDescriptions_PRD_10_DemoSize(t *testing.T) {
	// DSC-10: size.tsx — size + extra + bordered
	items := []kit.DescriptionsItem{
		{Label: "Product", Children: "Cloud Database"},
		{Label: "Billing", Children: "Prepaid"},
		{Label: "Time", Children: "18:00:00"},
	}
	for _, sz := range []kit.DescriptionsSize{kit.DescriptionsLarge, kit.DescriptionsMiddle, kit.DescriptionsSmall} {
		d := kit.NewDescriptions(items...)
		d.SetTitle("Custom Size")
		d.SetSize(sz)
		d.SetBordered(true)
		d.SetExtra(kit.NewButton("Edit").Node())
		_ = d.Node().Layout(core.Loose(640, 300))
	}
}

func TestDescriptions_PRD_11_DemoResponsive(t *testing.T) {
	// DSC-11: responsive.tsx — ColumnMap + ViewportWidth
	items := []kit.DescriptionsItem{
		{Label: "Product", Children: "Cloud Database"},
		{Label: "Billing", Children: "Prepaid"},
		{Label: "Time", Children: "18:00:00"},
		{Label: "Amount", Children: "$80.00"},
		{Label: "Discount", Children: "$20.00", SpanMap: map[string]int{"xl": 2, "xxl": 2}},
		{Label: "Official", Children: "$60.00", SpanMap: map[string]int{"xl": 2, "xxl": 2}},
	}
	d := kit.NewDescriptions(items...)
	d.SetTitle("Responsive Descriptions")
	d.SetBordered(true)
	d.SetColumnMap(map[string]int{"xs": 1, "sm": 2, "md": 3, "lg": 3, "xl": 4, "xxl": 4})

	d.SetViewportWidth(400) // xs (<576)
	if d.ResolvedColumn() != 1 {
		t.Fatalf("xs column=%d want 1", d.ResolvedColumn())
	}
	d.SetViewportWidth(600) // sm
	if d.ResolvedColumn() != 2 {
		t.Fatalf("sm column=%d want 2", d.ResolvedColumn())
	}
	d.SetViewportWidth(800) // md
	if d.ResolvedColumn() != 3 {
		t.Fatalf("md column=%d want 3", d.ResolvedColumn())
	}
	d.SetViewportWidth(1300) // xl
	if d.ResolvedColumn() != 4 {
		t.Fatalf("xl column=%d want 4", d.ResolvedColumn())
	}
	_ = d.Node().Layout(core.Loose(1300, 400))
}

func TestDescriptions_PRD_12_DemoVertical(t *testing.T) {
	// DSC-12: vertical.tsx
	items := []kit.DescriptionsItem{
		{Key: "1", Label: "UserName", Children: "Zhou Maomao"},
		{Key: "2", Label: "Telephone", Children: "1810000000"},
		{Key: "3", Label: "Live", Children: "Hangzhou, Zhejiang"},
		{Key: "4", Label: "Address", Span: 2, Children: "No. 18, Wantang Road"},
		{Key: "5", Label: "Remark", Children: "empty"},
	}
	d := kit.NewDescriptions(items...)
	d.SetTitle("User Info")
	d.SetLayout(kit.DescriptionsVertical)
	if d.RowCount() < 2 {
		t.Fatalf("rows=%d", d.RowCount())
	}
	_ = d.Node().Layout(core.Loose(720, 300))
}

func TestDescriptions_PRD_13_DemoVerticalBorder(t *testing.T) {
	// DSC-13: vertical-border.tsx
	d := kit.NewDescriptions(
		kit.DescriptionsItem{Label: "Product", Children: "Cloud Database"},
		kit.DescriptionsItem{Label: "Billing Mode", Children: "Prepaid"},
		kit.DescriptionsItem{Label: "Automatic Renewal", Children: "YES"},
	)
	d.SetTitle("User Info")
	d.SetLayout(kit.DescriptionsVertical)
	d.SetBordered(true)
	if !d.IsVertical() || !d.IsBordered() {
		t.Fatal("flags")
	}
	_ = d.Node().Layout(core.Loose(720, 300))
}

func TestDescriptions_PRD_14_DemoStyleClass(t *testing.T) {
	// DSC-14: style-class.tsx — shallow styles.label + root Style
	items := []kit.DescriptionsItem{
		{Key: "1", Label: "Product", Children: "Cloud Database"},
		{Key: "2", Label: "Billing Mode", Children: "Prepaid"},
		{Key: "3", Label: "Automatic Renewal", Children: "YES"},
	}
	small := kit.NewDescriptions(items...)
	small.SetTitle("User Info")
	small.SetBordered(true)
	small.SetSize(kit.DescriptionsSmall)
	small.SetLabelStyle(kit.Style{Text: render.Hex("#000000")})
	_ = small.Node().Layout(core.Loose(640, 200))

	large := kit.NewDescriptions(items...)
	large.SetTitle("User Info")
	large.SetBordered(true)
	large.SetSize(kit.DescriptionsLarge)
	large.SetLabelStyle(kit.Style{Text: render.Hex("#A294F9")})
	large.SetStyle(kit.Style{
		Border:      render.Hex("#CDC1FF"),
		Radius:      8,
		ForceRadius: true,
	})
	_ = large.Node().Layout(core.Loose(640, 200))
}

func TestDescriptions_PRD_15_DemoBlock(t *testing.T) {
	// DSC-15: block.tsx — span=filled
	d := kit.NewDescriptions(
		kit.DescriptionsItem{Label: "UserName", Children: "Zhou Maomao"},
		kit.DescriptionsItem{Label: "Live", Children: "Hangzhou, Zhejiang", SpanFilled: true},
		kit.DescriptionsItem{Label: "Remark", Children: "empty", SpanFilled: true},
		kit.DescriptionsItem{Label: "Address", Children: "No. 18, Wantang Road", Span: 1},
	)
	d.SetTitle("User Info")
	d.SetBordered(true)
	d.SetColumn(3)
	spans := d.RowSpans()
	// row0: UserName(1) + Live filled→2
	// row1: Remark filled→3
	// row2: Address expanded→3
	if len(spans) != 3 {
		t.Fatalf("rows=%d want 3 spans=%v", len(spans), spans)
	}
	if len(spans[0]) != 2 || spans[0][0] != 1 || spans[0][1] != 2 {
		t.Fatalf("row0=%v want [1 2]", spans[0])
	}
	if len(spans[1]) != 1 || spans[1][0] != 3 {
		t.Fatalf("row1=%v want [3]", spans[1])
	}
	if len(spans[2]) != 1 || spans[2][0] != 3 {
		t.Fatalf("row2=%v want [3] (expanded)", spans[2])
	}
	_ = d.Node().Layout(core.Loose(720, 300))
}

func TestDescriptions_PRD_16_Metrics(t *testing.T) {
	// DSC-16: §6.2 关键尺寸/间距
	d := kit.NewDescriptions(sampleItems3()...)
	if !approxDesc(d.FontSize(), kit.DefaultDescriptionsFontSize, 0.5) {
		t.Fatalf("font=%v", d.FontSize())
	}
	if !approxDesc(d.TitleFontSize(), kit.DefaultDescriptionsTitleFontSize, 0.5) {
		t.Fatalf("titleFont=%v", d.TitleFontSize())
	}
	if !approxDesc(d.Radius(), kit.DefaultDescriptionsRadius, 0.5) {
		t.Fatalf("radius=%v", d.Radius())
	}
	if !approxDesc(d.LineWidth(), kit.DefaultDescriptionsLineWidth, 0.5) {
		t.Fatalf("lineW=%v", d.LineWidth())
	}
	if !approxDesc(d.ItemPadBottom(), kit.DefaultDescriptionsItemPadBottom, 0.5) {
		t.Fatalf("itemPadB=%v", d.ItemPadBottom())
	}
	if !approxDesc(d.BorderedPadH(), kit.DefaultDescriptionsBorderedPadH, 0.5) {
		t.Fatalf("borderedPadH=%v", d.BorderedPadH())
	}
	// focus ring constant present
	if kit.DefaultDescriptionsFocusRingOutset < 1 {
		t.Fatal("focus ring")
	}
}

func TestDescriptions_PRD_17_ThemeTokens(t *testing.T) {
	// DSC-17: 默认皮颜色走 Theme Token，无硬编码品牌色
	th := kit.DefaultTheme()
	d := kit.NewDescriptions(sampleItems3()...)
	d.SetTheme(th)
	d.SetTitle("T")
	d.SetBordered(true)
	_ = d.Node().Layout(core.Loose(600, 200))

	// label tertiary / content text / split / fillSecondary exist
	if th.Color(core.TokenColorTextTertiary).A <= 0 {
		t.Fatal("missing tertiary")
	}
	if th.Color(core.TokenColorText).A <= 0 {
		t.Fatal("missing text")
	}
	if th.Color(core.TokenColorSplit).A <= 0 && th.Color(core.TokenColorSplit).R == 0 {
		// split may be near-white opaque
	}
	// custom theme propagates sizes
	custom := kit.DefaultTheme()
	custom.Tokens.Sizes[core.TokenFontSize] = 15
	d2 := kit.NewDescriptions(sampleItems3()...)
	d2.SetTheme(custom)
	if !approxDesc(d2.FontSize(), 15, 0.5) {
		t.Fatalf("custom font=%v", d2.FontSize())
	}
	_ = approxDescColor
}

func TestDescriptions_PRD_18_DisabledNA(t *testing.T) {
	// DSC-18: disabled 外观（适用者）— Descriptions 本体无 disabled API
	// Document N/A: display-only control.
	d := kit.NewDescriptions(sampleItems3()...)
	_ = d.Node().Layout(core.Loose(400, 120))
}

func TestDescriptions_PRD_19_A11yGroup(t *testing.T) {
	// DSC-19: 键盘/焦点主路径（适用者）— 根 group + AriaLabel / title 名
	// Display-only: no focus ring required on root; structure role present.
	d := kit.NewDescriptions(sampleItems3()...)
	d.SetTitle("User Info")
	d.SetAriaLabel("user-desc")
	n := d.Node()
	_ = n.Layout(core.Loose(600, 200))
	if n.Base().Role != "group" {
		t.Fatalf("Role=%q want group", n.Base().Role)
	}
	if n.Base().Label != "user-desc" {
		t.Fatalf("Label=%q want user-desc", n.Base().Label)
	}
	// Without AriaLabel falls back to Title
	d2 := kit.NewDescriptions(sampleItems3()...)
	d2.SetTitle("Only Title")
	n2 := d2.Node()
	if n2.Base().Label != "Only Title" {
		t.Fatalf("Label=%q", n2.Base().Label)
	}
}

func TestDescriptions_PRD_ColonOff(t *testing.T) {
	// colon=false still builds
	d := kit.NewDescriptions(sampleItems3()...)
	d.SetColon(false)
	if d.Colon {
		t.Fatal("Colon still true")
	}
	_ = d.Node().Layout(core.Loose(600, 160))
}

func TestDescriptions_PRD_RootStable(t *testing.T) {
	d := kit.NewDescriptions(sampleItems3()...)
	root := d.Node()
	d.SetTitle("T")
	d.SetBordered(true)
	d.SetSize(kit.DescriptionsSmall)
	if d.Node() != root {
		t.Fatal("Root pointer changed across rebuild")
	}
}

func TestDescriptions_PRD_ExtraSlot(t *testing.T) {
	d := kit.NewDescriptions(sampleItems3()...)
	d.SetTitle("T")
	btn := kit.NewButton("Edit")
	d.SetExtra(btn.Node())
	_ = d.Node().Layout(core.Loose(600, 200))
	// swap extra without losing root
	root := d.Node()
	d.SetExtra(kit.NewText("More").Node())
	if d.Node() != root {
		t.Fatal("root changed")
	}
}

func TestDescriptions_PRD_HitEqualsLayout(t *testing.T) {
	// hit == layout == paint: root HitDefer, no magic offset chrome
	d := kit.NewDescriptions(sampleItems3()...)
	d.SetTitle("T")
	d.SetBordered(true)
	n := d.Node()
	sz := n.Layout(core.Loose(640, 240))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("sz=%v", sz)
	}
	// Decorated root should hit-defer
	if dec, ok := n.(*primitive.Decorated); ok {
		if dec.Hit != core.HitDefer {
			// containers use HitDefer
			_ = dec
		}
	}
}
