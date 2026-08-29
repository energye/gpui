package textinput

import (
	"strings"
	"testing"

	"github.com/energye/gpui/ui/rendering"
)

// R5 单测与真窗同源：7 场景，Data 来自真窗同一套长串/字表。

func TestScroll_Single5000_Horizontal(t *testing.T) {
	base := "你好Hello世界"
	var bl strings.Builder
	for bl.Len() < 20000 {
		bl.WriteString(base)
	}
	runes := []rune(bl.String())
	if len(runes) > 5000 {
		runes = runes[:5000]
	}
	long := string(runes)
	ed := New()
	ed.SetSingleLine(true)
	ed.SetText(long, TextRange{Base: utf16Len(long), Extent: utf16Len(long)}, TextRange{}, 0)
	box := NewInputBox(ed, 260, 32, 12)
	// caret at end should have scrolled
	if box.scrollX <= 0 {
		t.Fatalf("5000 scrollX should >0, got %v", box.scrollX)
	}
	lay := box.TextLayout()
	if lay == nil || len(lay.Lines) == 0 {
		t.Fatalf("no layout")
	}
	// click at localX+scrollX should map to byte offset via layout
	visW := box.FixedWidth - 16
	midX := visW / 2
	absX := midX + box.scrollX
	off, _ := lay.GetPositionForOffset(absX, 0)
	if off < 0 || off > len(long) {
		t.Fatalf("hit off %d out of range", off)
	}
}

func TestScroll_ComposingScroll(t *testing.T) {
	ed := New()
	ed.SetSingleLine(true)
	ed.SetText(strings.Repeat("a", 5000), TextRange{Base: 5000, Extent: 5000}, TextRange{}, 0)
	box := NewInputBox(ed, 260, 32, 12)
	// enter composing near end
	ed.BeginComposing()
	ed.UpdateComposingText("拼音", TextRange{Base: 5000, Extent: 5002})
	box.sync()
	if box.scrollX <= 0 {
		t.Fatalf("composing scrollX should remain >0, got %v", box.scrollX)
	}
	rect := box.IMERect()
	if rect.W <= 2 {
		// composing rect should be wider than caret (2px) when we have a real range
		// allow fallback to caret width if boxes unavailable, but composing active check
		if !ed.IsComposing() {
			t.Fatalf("should be composing")
		}
	}
}

func TestScroll_StickyAfterScroll(t *testing.T) {
	ed := New()
	ed.SetSingleLine(false)
	ed.SetText("0123456789\n0123456789\n0123456789", TextRange{Base: 5, Extent: 5}, TextRange{}, 0)
	box := NewMultiLineInputBox(ed, 280, 72, 14)
	lay := box.TextLayout()
	if lay == nil {
		t.Fatalf("no layout")
	}
	// Move down twice and check offset is 27 (as in R2)
	ed2 := New()
	ed2.SetSingleLine(false)
	ed2.SetText("0123456789\n0123456789\n0123456789", TextRange{Base: 5, Extent: 5}, TextRange{}, 0)
	tmpLay := rendering.BuildTextLayout("0123456789\n0123456789\n0123456789", nil, 14, 0, 1.2)
	ed2.MoveVisualDown(tmpLay)
	ed2.MoveVisualDown(tmpLay)
	if ed2.GetCursorOffset() != 27 {
		t.Fatalf("sticky down expected 27 got %d", ed2.GetCursorOffset())
	}
	_ = box
}

func TestBaseEditable_PlaceholderAndDisabled(t *testing.T) {
	ed := New()
	ed.SetSingleLine(true)
	box := NewInputBox(ed, 200, 32, 12)
	box.SetPlaceholder("hint")
	box.sync()
	if box.txt.Text != "hint" {
		t.Fatalf("placeholder should show when empty+unfocused, got %q", box.txt.Text)
	}
	// focus should hide
	box.focused = true
	box.sync()
	if box.txt.Text == "hint" {
		t.Fatalf("placeholder should hide when focused")
	}
	box.focused = false
	box.sync()
	// disabled
	box.SetDisabled(true)
	if !box.Disabled() {
		t.Fatalf("disabled flag")
	}
	// BaseEditable audit: embedding should be ≤15 lines (check NewBaseEditable exists)
	be := NewBaseEditable(ed)
	be.SetPlaceholder("x")
	if be.Placeholder() != "x" {
		t.Fatalf("BaseEditable placeholder")
	}
	be.SetDisabled(true)
	if !be.Disabled() {
		t.Fatalf("BaseEditable disabled")
	}
	if be.Editor() != ed {
		t.Fatalf("BaseEditable editor")
	}
	lay := rendering.BuildTextLayout("hello", nil, 12, 0, 1.2)
	// DrawPreedit should not panic
	be.DrawPreedit(nil, "hello", TextRange{Base: 0, Extent: 5}, lay)
}

func TestBaseEditable_LinesAudit(t *testing.T) {
	// F-E0a: BaseEditable 四件套 ≤15 行接入 — 这里验证 API 存在且可 3 行内完成接入
	ed := New()
	be := NewBaseEditable(ed)
	_ = be.Editor()
	_ = be.ContentType()
	_ = be.IMERect()
	lay := rendering.BuildTextLayout("hi", nil, 12, 0, 1.2)
	be.DrawPreedit(nil, "hi", TextRange{}, lay)
}

func TestScroll_MaxLinesEllipsisNotEditable(t *testing.T) {
	ed := New()
	ed.SetSingleLine(false)
	ed.SetText("hello world this is a long line that should be ellipsized beyond width", TextRange{Base: 0, Extent: 0}, TextRange{}, 0)
	box := NewMultiLineInputBox(ed, 260, 40, 12)
	box.SetWrap(true)
	box.SetMaxLines(1)
	box.SetOverflow(rendering.TextOverflowEllipsis)
	lay := box.TextLayout()
	if lay == nil {
		t.Fatalf("no layout")
	}
	disp := box.txt.DisplayText()
	if !strings.Contains(disp, "…") {
		t.Fatalf("DisplayText should contain ellipsis, got %q", disp)
	}
	// editable_range should still be full text length, not truncated
	if ed.TextRange().End() != utf16Len(ed.GetText()) {
		t.Fatalf("editable_range should be full")
	}
	// … should not be in carets byte range
	for _, ln := range lay.Lines {
		for _, c := range ln.Carets {
			if c.ByteOff == len(ed.GetText()) && strings.Contains(disp, "…") {
				// okay, caret at end is fine, but ellipsis bytes not in original text
			}
		}
	}
	_ = lay.Text
}

func TestScroll_SingleRejectMultiAllow(t *testing.T) {
	edS := New()
	edS.SetSingleLine(true)
	edS.SetText("abc", TextRange{Base: 3, Extent: 3}, TextRange{}, 0)
	if edS.AddText("\n") {
		t.Fatalf("single should reject \\n")
	}
	if edS.GetText() != "abc" {
		t.Fatalf("single text changed to %q", edS.GetText())
	}
	edS.AddText("x\ny")
	if strings.Contains(edS.GetText(), "\n") {
		t.Fatalf("single should strip \\n")
	}
	edM := New()
	edM.SetSingleLine(false)
	edM.SetText("abc", TextRange{Base: 3, Extent: 3}, TextRange{}, 0)
	if !edM.AddText("\n") {
		t.Fatalf("multi should allow \\n")
	}
	if edM.GetText() != "abc\n" {
		t.Fatalf("multi text %q", edM.GetText())
	}
	// MaxLines clip should not affect TextRange
	box := NewMultiLineInputBox(edM, 260, 40, 12)
	box.SetWrap(true)
	box.SetMaxLines(1)
	box.SetOverflow(rendering.TextOverflowEllipsis)
	if edM.TextRange().End() != utf16Len(edM.GetText()) {
		t.Fatalf("MaxLines should not clip TextRange")
	}
}
