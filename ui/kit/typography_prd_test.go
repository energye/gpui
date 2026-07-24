package kit_test

import (
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/typography.md §6.9 — P0 PRD cases (TYP-01…TYP-25).
// L3/L4 (TYP-26/27) and P1 (TYP-28) deferred.

func layoutTypo(t *testing.T, ty *kit.Typography, w, h float64) *core.Tree {
	t.Helper()
	tree := core.NewTree(ty.Node())
	tree.SetTheme(kit.DefaultTheme())
	tree.Layout(core.Size{Width: w, Height: h})
	return tree
}

func findByLabel(root core.Node, label string) core.Node {
	var found core.Node
	var walk func(core.Node)
	walk = func(n core.Node) {
		if n == nil || found != nil {
			return
		}
		if n.Base().Label == label {
			found = n
			return
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(root)
	return found
}

func clickLabel(t *testing.T, tree *core.Tree, root core.Node, label string) bool {
	t.Helper()
	n := findByLabel(root, label)
	if n == nil {
		return false
	}
	abs := core.AbsoluteBounds(n)
	x := abs.Min.X + abs.Size().Width/2
	y := abs.Min.Y + abs.Size().Height/2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
	return true
}

func TestTypography_PRD_01_Defaults(t *testing.T) {
	// TYP-01
	ty := kit.NewTypography("hello")
	if ty.Kind != kit.TypographyText {
		t.Fatalf("Kind=%v want Text", ty.Kind)
	}
	if ty.Disabled || ty.Copyable || ty.Editable || ty.Ellipsis {
		t.Fatalf("flags disabled=%v copyable=%v editable=%v ellipsis=%v",
			ty.Disabled, ty.Copyable, ty.Editable, ty.Ellipsis)
	}
	if ty.Type != kit.TypographyTypeDefault {
		t.Fatalf("Type=%v want default", ty.Type)
	}
	if ty.ActionsPlacement != kit.TypographyActionsEnd {
		t.Fatalf("placement=%v want end", ty.ActionsPlacement)
	}
	if ty.ValueOf() != "hello" {
		t.Fatalf("Value=%q", ty.ValueOf())
	}
	_ = ty.Node().Layout(core.Loose(200, 50))
}

func TestTypography_PRD_02_TitleLevels(t *testing.T) {
	// TYP-02 / TYP-S1
	want := []float64{
		1: kit.DefaultTitleFontSizeH1,
		2: kit.DefaultTitleFontSizeH2,
		3: kit.DefaultTitleFontSizeH3,
		4: kit.DefaultTitleFontSizeH4,
		5: kit.DefaultTitleFontSizeH5,
	}
	for level := 1; level <= 5; level++ {
		ty := kit.NewTitle("H", level)
		got := ty.ResolvedFontSize()
		if got < want[level]-0.5 || got > want[level]+0.5 {
			t.Fatalf("level %d fontSize=%v want %v", level, got, want[level])
		}
		_ = ty.Node().Layout(core.Loose(400, 100))
		if ty.Root.FontSize < want[level]-0.5 || ty.Root.FontSize > want[level]+0.5 {
			t.Fatalf("level %d Root.FontSize=%v want %v", level, ty.Root.FontSize, want[level])
		}
	}
}

func TestTypography_PRD_03_SemanticTypeColors(t *testing.T) {
	// TYP-03 / TYP-S2
	th := kit.DefaultTheme()
	cases := []struct {
		ty   kit.TypographyType
		tok  string
		want render.RGBA
	}{
		{kit.TypographyTypeSecondary, core.TokenColorTextSecondary, th.Color(core.TokenColorTextSecondary)},
		{kit.TypographyTypeSuccess, core.TokenColorSuccess, th.Color(core.TokenColorSuccess)},
		{kit.TypographyTypeWarning, core.TokenColorWarning, th.Color(core.TokenColorWarning)},
		{kit.TypographyTypeDanger, core.TokenColorError, th.Color(core.TokenColorError)},
	}
	for _, tc := range cases {
		tx := kit.NewText("x")
		tx.SetType(tc.ty)
		_ = layoutTypo(t, tx, 200, 50)
		got := tx.ResolvedColor()
		if !approxRGBA(got, tc.want, 0.02) {
			t.Fatalf("type %v color=%v want ~%v", tc.ty, got, tc.want)
		}
		if !approxRGBA(tx.Root.Color, tc.want, 0.02) {
			t.Fatalf("type %v Root.Color=%v want ~%v", tc.ty, tx.Root.Color, tc.want)
		}
	}
}

func TestTypography_PRD_04_Copyable(t *testing.T) {
	// TYP-04 / TYP-S3
	tx := kit.NewText("copy-me")
	tx.SetCopyable(true)
	copied := ""
	tx.OnCopy = func(s string) { copied = s }

	tree := layoutTypo(t, tx, 400, 80)
	clip := core.NewMemoryClipboard()
	tree.SetClipboard(clip)

	if !clickLabel(t, tree, tx.Node(), "复制") {
		t.Fatal("copy action not found")
	}
	if copied != "copy-me" {
		t.Fatalf("OnCopy got %q", copied)
	}
	if got, ok := clip.ReadText(); !ok || got != "copy-me" {
		t.Fatalf("clipboard=%q ok=%v", got, ok)
	}
}

func TestTypography_PRD_05_Ellipsis(t *testing.T) {
	// TYP-05 / TYP-S4
	long := strings.Repeat("超长文本内容", 20)
	tx := kit.NewText(long)
	tx.SetEllipsis(true)
	tx.SetMaxWidth(80)
	_ = layoutTypo(t, tx, 80, 40)
	lines := tx.Root.DisplayedLines()
	if len(lines) == 0 {
		t.Fatal("no displayed lines")
	}
	joined := strings.Join(lines, "")
	if !strings.Contains(joined, "…") && !strings.Contains(joined, "...") {
		t.Fatalf("expected ellipsis in %q", joined)
	}
	if joined == long {
		t.Fatal("text should be truncated")
	}
}

func TestTypography_PRD_06_Expandable(t *testing.T) {
	// TYP-06 / TYP-S5
	long := strings.Repeat("段落内容 ", 40)
	pg := kit.NewParagraph(long)
	pg.SetEllipsis(true)
	pg.SetEllipsisRows(2)
	pg.SetMaxWidth(160)
	pg.SetExpandable(true)
	pg.SetCollapsible(true)
	kit.TypographyTestForceEllipsis(pg, true)

	tree := layoutTypo(t, pg, 200, 200)
	if !clickLabel(t, tree, pg.Node(), "展开") {
		t.Fatal("expand action not found")
	}
	if !pg.Expanded() {
		t.Fatal("expected expanded after click")
	}
	if pg.Root.Ellipsis {
		t.Fatal("ellipsis should be off when expanded")
	}
}

func TestTypography_PRD_07_EditableEnter(t *testing.T) {
	// TYP-07 / TYP-S6
	tx := kit.NewText("old")
	tx.SetEditable(true)
	changed := ""
	ended := 0
	tx.OnChange = func(v string) { changed = v }
	tx.OnEnd = func() { ended++ }

	tree := layoutTypo(t, tx, 400, 80)
	if !clickLabel(t, tree, tx.Node(), "编辑") {
		t.Fatal("edit action not found")
	}
	if !tx.IsEditing() {
		t.Fatal("expected editing")
	}
	// type new value via editor
	ed, ok := tx.ContentNode().(*primitive.EditableText)
	if !ok || ed == nil {
		t.Fatal("editor missing")
	}
	ed.SetValue("new")
	tree.SetFocus(ed)
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	if changed != "new" {
		t.Fatalf("OnChange=%q want new", changed)
	}
	if ended != 1 {
		t.Fatalf("OnEnd=%d", ended)
	}
	if tx.IsEditing() {
		t.Fatal("should exit editing after Enter")
	}
	if tx.ValueOf() != "new" {
		t.Fatalf("Value=%q", tx.ValueOf())
	}
}

func TestTypography_PRD_08_EditableEsc(t *testing.T) {
	// TYP-08 / TYP-S7
	tx := kit.NewText("keep")
	tx.SetEditable(true)
	canceled := 0
	tx.OnCancel = func() { canceled++ }
	tx.OnChange = func(v string) { t.Fatalf("OnChange should not fire on Esc: %q", v) }

	tree := layoutTypo(t, tx, 400, 80)
	if !clickLabel(t, tree, tx.Node(), "编辑") {
		t.Fatal("edit action not found")
	}
	ed, ok := tx.ContentNode().(*primitive.EditableText)
	if !ok || ed == nil {
		t.Fatal("editor missing")
	}
	ed.SetValue("dirty")
	tree.SetFocus(ed)
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Escape"})
	if canceled != 1 {
		t.Fatalf("OnCancel=%d", canceled)
	}
	if tx.ValueOf() != "keep" {
		t.Fatalf("Value=%q want keep", tx.ValueOf())
	}
	if tx.IsEditing() {
		t.Fatal("should exit editing after Esc")
	}
}

func TestTypography_PRD_09_DisabledBlocksActions(t *testing.T) {
	// TYP-09 / TYP-S8
	tx := kit.NewText("x")
	tx.SetCopyable(true)
	tx.SetEditable(true)
	tx.SetDisabled(true)
	copied := 0
	tx.OnCopy = func(string) { copied++ }
	tx.OnStart = func() { t.Fatal("OnStart") }

	tree := layoutTypo(t, tx, 400, 80)
	if clickLabel(t, tree, tx.Node(), "复制") {
		t.Fatal("copy action should be hidden when disabled")
	}
	if clickLabel(t, tree, tx.Node(), "编辑") {
		t.Fatal("edit action should be hidden when disabled")
	}
	if copied != 0 {
		t.Fatalf("copied=%d", copied)
	}
}

func TestTypography_PRD_10_Decorations(t *testing.T) {
	// TYP-10 / TYP-S9
	base := kit.NewText("plain")
	_ = layoutTypo(t, base, 200, 40)

	strong := kit.NewText("strong")
	strong.SetStrong(true)
	_ = layoutTypo(t, strong, 200, 40)

	code := kit.NewText("code")
	code.SetCode(true)
	_ = layoutTypo(t, code, 200, 40)
	if code.ChromeNode() == nil || code.ChromeNode() == code.Root {
		t.Fatal("code should have chrome Decorated")
	}
	dec := code.ChromeNode().(*primitive.Decorated)
	if dec.Background.A < 0.01 {
		t.Fatal("code chrome needs background")
	}

	mark := kit.NewText("mark")
	mark.SetMark(true)
	_ = layoutTypo(t, mark, 200, 40)
	mdec := mark.ChromeNode().(*primitive.Decorated)
	if !approxRGBA(mdec.Background, render.Hex("#FFE58F"), 0.05) {
		t.Fatalf("mark bg=%v", mdec.Background)
	}

	del := kit.NewText("del")
	del.SetDelete(true)
	_ = layoutTypo(t, del, 200, 40)
	if del.Root.Decoration&render.TextDecorationLineThrough == 0 {
		t.Fatal("delete should set line-through")
	}

	und := kit.NewText("und")
	und.SetUnderline(true)
	_ = layoutTypo(t, und, 200, 40)
	if und.Root.Decoration&render.TextDecorationUnderline == 0 {
		t.Fatal("underline decoration missing")
	}
}

func TestTypography_PRD_11_Link(t *testing.T) {
	// TYP-11 / TYP-S10
	th := kit.DefaultTheme()
	primary := th.Color(core.TokenColorPrimary)
	link := kit.NewLink("docs")
	clicks := 0
	link.SetOnClick(func() { clicks++ })
	tree := layoutTypo(t, link, 200, 50)
	if !approxRGBA(link.ResolvedColor(), primary, 0.05) {
		t.Fatalf("link color=%v want primary %v", link.ResolvedColor(), primary)
	}
	// click content
	abs := core.AbsoluteBounds(link.Node())
	x := abs.Min.X + 4
	y := abs.Min.Y + abs.Size().Height/2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
	if clicks != 1 {
		t.Fatalf("clicks=%d", clicks)
	}
	// focusable
	list := core.CollectFocusables(link.Node())
	if len(list) == 0 {
		t.Fatal("link should be focusable")
	}
}

func TestTypography_PRD_12_ControlledExpanded(t *testing.T) {
	// TYP-12 / TYP-S11
	pg := kit.NewParagraph(strings.Repeat("字", 80))
	pg.SetEllipsis(true)
	pg.SetEllipsisRows(1)
	pg.SetMaxWidth(100)
	pg.SetExpandable(true)
	pg.SetCollapsible(true)
	kit.TypographyTestForceEllipsis(pg, true)

	// uncontrolled first: expands on click
	tree := layoutTypo(t, pg, 200, 100)
	if !clickLabel(t, tree, pg.Node(), "展开") {
		t.Fatal("expand not found (uncontrolled)")
	}
	if !pg.Expanded() {
		t.Fatal("uncontrolled should expand")
	}

	// switch to controlled collapsed
	expandCalls := 0
	var last bool
	pg.OnExpand = func(exp bool) {
		expandCalls++
		last = exp
	}
	pg.SetExpanded(false)
	if pg.Expanded() {
		t.Fatal("controlled false")
	}
	kit.TypographyTestForceEllipsis(pg, true)
	tree.Layout(core.Size{Width: 200, Height: 100})
	if !clickLabel(t, tree, pg.Node(), "展开") {
		t.Fatal("expand not found (controlled)")
	}
	if pg.Expanded() {
		t.Fatal("controlled should not auto-expand")
	}
	if expandCalls != 1 || !last {
		t.Fatalf("OnExpand calls=%d last=%v", expandCalls, last)
	}
	// external updates win
	pg.SetExpanded(true)
	if !pg.Expanded() {
		t.Fatal("SetExpanded(true) should win")
	}
}

func TestTypography_PRD_13_BodyFontSize(t *testing.T) {
	// TYP-13 / TYP-S12
	tx := kit.NewText("body")
	if fs := tx.ResolvedFontSize(); fs < 14-0.5 || fs > 14+0.5 {
		t.Fatalf("fontSize=%v want 14", fs)
	}
	pg := kit.NewParagraph("p")
	if fs := pg.ResolvedFontSize(); fs < 14-0.5 || fs > 14+0.5 {
		t.Fatalf("paragraph fontSize=%v want 14", fs)
	}
}

func TestTypography_PRD_14_BasicDemo(t *testing.T) {
	// TYP-14 basic.tsx — title + paragraph stack
	col := primitive.Column(
		kit.NewTitle("Introduction", 2).Node(),
		kit.NewParagraph("Ant Design, a design language for background applications.").Node(),
	)
	tree := core.NewTree(col)
	tree.SetTheme(kit.DefaultTheme())
	tree.Layout(core.Size{Width: 480, Height: 200})
}

func TestTypography_PRD_15_TitleDemo(t *testing.T) {
	// TYP-15
	for lv := 1; lv <= 5; lv++ {
		ty := kit.NewTitle("h", lv)
		_ = layoutTypo(t, ty, 400, 80)
	}
}

func TestTypography_PRD_16_TextAndLinkDemo(t *testing.T) {
	// TYP-16
	nodes := []core.Node{
		kit.NewText("default").Node(),
		func() core.Node {
			x := kit.NewText("secondary")
			x.SetType(kit.TypographyTypeSecondary)
			return x.Node()
		}(),
		func() core.Node { x := kit.NewText("success"); x.SetType(kit.TypographyTypeSuccess); return x.Node() }(),
		func() core.Node { x := kit.NewText("warning"); x.SetType(kit.TypographyTypeWarning); return x.Node() }(),
		func() core.Node { x := kit.NewText("danger"); x.SetType(kit.TypographyTypeDanger); return x.Node() }(),
		func() core.Node { x := kit.NewText("mark"); x.SetMark(true); return x.Node() }(),
		func() core.Node { x := kit.NewText("code"); x.SetCode(true); return x.Node() }(),
		func() core.Node { x := kit.NewText("del"); x.SetDelete(true); return x.Node() }(),
		func() core.Node { x := kit.NewText("u"); x.SetUnderline(true); return x.Node() }(),
		func() core.Node { x := kit.NewText("strong"); x.SetStrong(true); return x.Node() }(),
		kit.NewLink("Link").Node(),
	}
	row := primitive.Row(nodes...)
	row.Gap = 8
	tree := core.NewTree(row)
	tree.SetTheme(kit.DefaultTheme())
	tree.Layout(core.Size{Width: 900, Height: 80})
}

func TestTypography_PRD_17_EditableDemo(t *testing.T) {
	// TYP-17
	tx := kit.NewText("This is an editable text.")
	tx.SetEditable(true)
	_ = layoutTypo(t, tx, 400, 60)
	if findByLabel(tx.Node(), "编辑") == nil {
		t.Fatal("edit action missing")
	}
}

func TestTypography_PRD_18_CopyableDemo(t *testing.T) {
	// TYP-18
	tx := kit.NewText("This is a copyable text.")
	tx.SetCopyable(true)
	_ = layoutTypo(t, tx, 400, 60)
	if findByLabel(tx.Node(), "复制") == nil {
		t.Fatal("copy action missing")
	}
}

func TestTypography_PRD_19_EllipsisDemo(t *testing.T) {
	// TYP-19
	pg := kit.NewParagraph(strings.Repeat("Ant Design, a design language. ", 10))
	pg.SetEllipsis(true)
	pg.SetEllipsisRows(3)
	pg.SetMaxWidth(240)
	_ = layoutTypo(t, pg, 240, 120)
}

func TestTypography_PRD_20_EllipsisControlledDemo(t *testing.T) {
	// TYP-20
	pg := kit.NewParagraph(strings.Repeat("Controlled expand. ", 20))
	pg.SetEllipsis(true)
	pg.SetEllipsisRows(2)
	pg.SetMaxWidth(200)
	pg.SetExpandable(true)
	pg.SetCollapsible(true)
	pg.SetExpanded(true)
	if !pg.Expanded() {
		t.Fatal("want expanded")
	}
	_ = layoutTypo(t, pg, 240, 160)
	pg.SetExpanded(false)
	if pg.Expanded() {
		t.Fatal("want collapsed")
	}
}

func TestTypography_PRD_21_EllipsisMiddleDemo(t *testing.T) {
	// TYP-21
	full := "https://ant.design/components/typography-cn#components-typography-demo-ellipsis-middle"
	tx := kit.NewText(full)
	tx.SetEllipsis(true)
	tx.SetEllipsisMiddle(true)
	tx.SetMaxWidth(180)
	_ = layoutTypo(t, tx, 180, 40)
	disp := tx.Root.Value
	if !strings.Contains(disp, "…") {
		t.Fatalf("middle ellipsis display=%q", disp)
	}
	if disp == full {
		t.Fatal("should truncate")
	}
}

func TestTypography_PRD_22_Metrics(t *testing.T) {
	// TYP-22 L2
	if kit.DefaultTypographyFontSize != 14 {
		t.Fatal(kit.DefaultTypographyFontSize)
	}
	if kit.DefaultTitleFontSizeH1 != 38 || kit.DefaultTitleFontSizeH5 != 16 {
		t.Fatalf("title ladder %v…%v", kit.DefaultTitleFontSizeH1, kit.DefaultTitleFontSizeH5)
	}
	th := kit.DefaultTheme()
	if th.SizeOr(core.TokenFontSize, 0) != 14 {
		t.Fatalf("token fontSize=%v", th.Size(core.TokenFontSize))
	}
}

func TestTypography_PRD_23_TokenColors(t *testing.T) {
	// TYP-23 L2 — no hard-coded brand as only skin
	tx := kit.NewText("x")
	tx.SetType(kit.TypographyTypeDanger)
	th := kit.DefaultTheme()
	_ = layoutTypo(t, tx, 100, 40)
	if !approxRGBA(tx.Root.Color, th.Color(core.TokenColorError), 0.02) {
		t.Fatalf("danger should use TokenColorError, got %v", tx.Root.Color)
	}
	// custom theme override
	custom := kit.DefaultTheme()
	if custom.Tokens != nil {
		custom.Tokens = custom.Tokens.Clone()
		custom.Tokens.Colors[core.TokenColorError] = render.Hex("#112233")
	}
	tx2 := kit.NewText("y")
	tx2.SetTheme(custom)
	tx2.SetType(kit.TypographyTypeDanger)
	_ = layoutTypo(t, tx2, 100, 40)
	if custom.Tokens != nil && !approxRGBA(tx2.Root.Color, render.Hex("#112233"), 0.02) {
		t.Fatalf("custom theme not applied: %v", tx2.Root.Color)
	}
}

func TestTypography_PRD_24_DisabledAppearance(t *testing.T) {
	// TYP-24 L2
	th := kit.DefaultTheme()
	tx := kit.NewText("dis")
	tx.SetDisabled(true)
	_ = layoutTypo(t, tx, 100, 40)
	if !approxRGBA(tx.Root.Color, th.Color(core.TokenColorDisabledText), 0.05) {
		t.Fatalf("disabled color=%v want %v", tx.Root.Color, th.Color(core.TokenColorDisabledText))
	}
}

func TestTypography_PRD_25_KeyboardFocus(t *testing.T) {
	// TYP-25
	link := kit.NewLink("focus-me")
	clicks := 0
	link.SetOnClick(func() { clicks++ })
	tree := layoutTypo(t, link, 200, 50)
	list := core.CollectFocusables(link.Node())
	if len(list) == 0 {
		t.Fatal("no focusables")
	}
	tree.SetFocus(list[0])
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	if clicks != 1 {
		t.Fatalf("Enter clicks=%d", clicks)
	}
}
