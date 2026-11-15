//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package kit

import (
	"strings"
	"testing"

	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// ctx returns the default scope context.
func typoCtx() scope.Ctx { return DefaultScopeCtx() }

// TYP-01: default construction.
func TestTypography_PRD_01_Default(t *testing.T) {
	inst, node := BuildTypography(typoCtx(), DefaultTypographyProps())
	if inst == nil || node == nil {
		t.Fatal("default construction returned nil")
	}
	if node.Root == nil {
		t.Fatal("binding has no render root")
	}
	p := DefaultTypographyProps()
	if p.Kind != TypographyText || p.Level != 1 || p.ActionsPlacement != TypographyActionsEnd {
		t.Fatal("default props deviate from antd")
	}
}

// TYP-02: Title level 1..5 ladder = 38/30/24/20/16 (±0 tolerance; token).
func TestTypography_PRD_02_TitleLadder(t *testing.T) {
	want := []float64{38, 30, 24, 20, 16}
	for lv := 1; lv <= 5; lv++ {
		p := DefaultTypographyProps()
		p.Kind = TypographyTitle
		p.Level = lv
		res := ResolveTypography(typoCtx().Theme, p, false)
		if res.FontSize != want[lv-1] {
			t.Fatalf("level %d font = %v want %v", lv, res.FontSize, want[lv-1])
		}
	}
	// Out-of-range clamps to 1..5 ends.
	res := ResolveTypography(typoCtx().Theme, TypographyProps{Kind: TypographyTitle, Level: 9}, false)
	if res.FontSize != 38 {
		t.Fatalf("level 9 clamp = %v want 38", res.FontSize)
	}
}

// TYP-03: type semantic colors from tokens.
func TestTypography_PRD_03_SemanticColors(t *testing.T) {
	seed := typoCtx().Theme
	cases := []struct {
		typ TypographyType
		col theme.Color
	}{
		{TypographyTypeSuccess, seed.ColorSuccess},
		{TypographyTypeWarning, seed.ColorWarning},
		{TypographyTypeDanger, seed.ColorError},
		{TypographyTypeSecondary, seed.ColorTextSecondary},
	}
	for _, c := range cases {
		res := ResolveTypography(seed, TypographyProps{Type: c.typ}, false)
		if res.Color != c.col {
			t.Fatalf("type %s color mismatch", c.typ)
		}
	}
	// Link form uses the link color.
	res := ResolveTypography(seed, TypographyProps{Kind: TypographyLink}, false)
	if res.Color != seed.ColorLink {
		t.Fatal("link color mismatch")
	}
}

// TYP-08/09: disabled wins over type; Copy refuses.
func TestTypography_PRD_09_Disabled(t *testing.T) {
	seed := typoCtx().Theme
	res := ResolveTypography(seed, TypographyProps{Type: TypographyTypeDanger}, true)
	if res.Color != seed.ColorTextDisabled {
		t.Fatal("disabled color must win over type")
	}
	inst, _ := BuildTypography(typoCtx(), TypographyProps{Disabled: true, Value: "x", Copyable: TypographyCopyable{On: true}})
	if got := inst.Copy(); got != "" {
		t.Fatalf("disabled copy returned %q", got)
	}
}

// TYP-04: copy machine — payload and OnCopy once.
func TestTypography_PRD_04_Copyable(t *testing.T) {
	calls := 0
	var got string
	p := DefaultTypographyProps()
	p.Value = "hello"
	p.Copyable = TypographyCopyable{On: true, OnCopy: func(s string) { calls++; got = s }}
	inst, _ := BuildTypography(typoCtx(), p)
	payload := inst.Copy()
	if payload != "hello" || got != "hello" || calls != 1 {
		t.Fatalf("copy payload=%q cb=%q calls=%d", payload, got, calls)
	}
	if !inst.Copied() {
		t.Fatal("copied feedback state not set")
	}
	// Explicit copy text overrides.
	p2 := DefaultTypographyProps()
	p2.Value = "visible"
	p2.Copyable = TypographyCopyable{On: true, Text: "hidden-payload"}
	inst2, _ := BuildTypography(typoCtx(), p2)
	if inst2.Copy() != "hidden-payload" {
		t.Fatal("copy text override failed")
	}
}

// TYP-07/08: editable Enter commits, Esc cancels.
func TestTypography_PRD_07_Editable(t *testing.T) {
	var changed, ended, cancelled, started int
	var newVal string
	p := DefaultTypographyProps()
	p.Value = "old"
	p.Editable = TypographyEditable{
		On:       true,
		OnChange: func(s string) { changed++; newVal = s },
		OnEnd:    func() { ended++ },
		OnCancel: func() { cancelled++ },
		OnStart:  func() { started++ },
	}
	inst, _ := BuildTypography(typoCtx(), p)

	inst.StartEditing()
	if !inst.Editing() || started != 1 {
		t.Fatalf("start editing: editing=%v starts=%d", inst.Editing(), started)
	}
	inst.SetEditingValue("new text")
	inst.CommitEditing()
	if changed != 1 || newVal != "new text" || ended != 1 || inst.Editing() {
		t.Fatalf("commit: changed=%d val=%q ended=%d editing=%v", changed, newVal, ended, inst.Editing())
	}

	inst.StartEditing()
	inst.CancelEditing()
	if cancelled != 1 || inst.Editing() {
		t.Fatalf("cancel: cancelled=%d editing=%v", cancelled, inst.Editing())
	}
}

// TYP-12: controlled expanded — external wins, internal toggle mirrors.
func TestTypography_PRD_12_ControlledExpanded(t *testing.T) {
	ctl := false
	var events []bool
	p := DefaultTypographyProps()
	p.Ellipsis = TypographyEllipsis{
		On:          true,
		ExpandedCtl: &ctl,
		OnExpand:    func(v bool) { events = append(events, v) },
	}
	inst, _ := BuildTypography(typoCtx(), p)
	inst.ToggleExpand() // controlled: mirrors value, reports the same value
	if len(events) != 1 || events[0] != false {
		t.Fatalf("controlled toggle reported %v", events)
	}
	inst.SetExpanded(true)
	if len(events) != 2 || events[1] != true || !inst.Expanded() {
		t.Fatal("SetExpanded must drive the controlled state")
	}
}

// TYP-05 (S4): single-line ellipsis produces the mark.
func TestTypography_PRD_05_EllipsisMark(t *testing.T) {
	m := EstimateRows(100)
	long := strings.Repeat("abc ", 40) // definitely > 1 row at 100px
	cut := CutEllipsis(long, 100, 1, "", "…", m)
	if !cut.Ellipsized || !strings.HasSuffix(cut.Text, "…") {
		t.Fatalf("ellipsis cut missing mark: %q", cut.Text)
	}
	if m(cut.Text, 100) > 1 {
		t.Fatalf("cut still overflows: rows=%d", m(cut.Text, 100))
	}
	// Short text stays whole.
	short := "short"
	if cut := CutEllipsis(short, 100, 1, "", "…", m); cut.Ellipsized {
		t.Fatal("short text must not ellipsize")
	}
}

// S4: suffix is reserved — cut converges with the suffix visible.
func TestTypography_Ellipsis_SuffixReserved(t *testing.T) {
	m := EstimateRows(100)
	long := strings.Repeat("字", 60) // wide CJK
	suffix := "（后缀）"
	cut := CutEllipsis(long, 100, 2, suffix, "…", m)
	if !cut.Ellipsized {
		t.Fatal("long text must ellipsize")
	}
	// The suffix rows must fit inside the 2-row budget together with the cut.
	total := cut.Text + suffix
	if rows := m(total, 100); rows > 2 {
		t.Fatalf("cut+suffix rows=%d want <=2", rows)
	}
}

// S5 middle: head+mark+tail fixed cut, no search.
func TestTypography_Ellipsis_Middle(t *testing.T) {
	long := strings.Repeat("x", 100)
	cut := CutEllipsisMiddle(long, 10, 6, "…")
	want := strings.Repeat("x", 10) + "…" + strings.Repeat("x", 6)
	if cut.Text != want {
		t.Fatalf("middle cut = %q want %q", cut.Text, want)
	}
	// Not overflowing: no cut.
	if cut := CutEllipsisMiddle("abc", 10, 6, "…"); cut.Ellipsized {
		t.Fatal("short middle text must stay whole")
	}
}

// 件 1 binding: SetState before Mount is silent; after Mount it dirties.
func TestTypography_Binding_SetStateRelay(t *testing.T) {
	// Unmounted instance (build mounts by default, so unmount first).
	inst, node := BuildTypography(typoCtx(), TypographyProps{Value: "v"})
	// Settle the fresh node's initial dirty bits (Init marks layout+paint)
	// so dirty assertions below measure only SetState effects.
	node.Root.Layout(rendering.Tight(100, 40))
	node.Root.Paint(&rendering.PaintContext{})
	node.Unmount()
	inst.SetState(func(*TypographyInstance) {}, typographyStateCopied) // must not panic
	if node.Root.NeedsPaint() {
		t.Fatal("unmounted SetState dirtied the tree")
	}
	node.Mount(typoCtx())
	inst.SetState(func(*TypographyInstance) {}, typographyStateCopied)
	if !node.Root.NeedsPaint() {
		t.Fatal("mounted copied SetState did not mark paint")
	}
	inst.SetState(func(*TypographyInstance) {}, typographyStateExpanded)
	if !node.Root.NeedsLayout() {
		t.Fatal("expanded SetState did not mark layout")
	}
}

// 件 1 binding: Update mutates node properties, no tree rebuild.
func TestTypography_Binding_UpdateMutates(t *testing.T) {
	inst, node := BuildTypography(typoCtx(), TypographyProps{Value: "first"})
	root := node.Root
	if _, ok := root.(*rendering.RenderText); !ok {
		t.Fatalf("root type %T is not RenderText", root)
	}
	inst.Update(typoCtx(), TypographyProps{Value: "second", Kind: TypographyTitle, Level: 2})
	if node.Root != root {
		t.Fatal("Update rebuilt the render tree")
	}
	if root.NeedsPaint() == false && root.NeedsLayout() == false {
		t.Fatal("Update left the node clean")
	}
}
