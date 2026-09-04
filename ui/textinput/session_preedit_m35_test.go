package textinput

import (
	"testing"

	"github.com/energye/gpui/ui/input"
)

// M3.5: PreeditEvent.Cursor 是字节偏移（<0 = 末尾）。光标映射必须钳制到
// [0,len] 并对齐到 rune 边界，否则半个 CJK 字上的取值会静默错位。
func TestSessionPreeditCursorClamp_M35(t *testing.T) {
	// "ni你好": n(0) i(1) 你(2,3,4) 好(5,6,7)，共 8 字节、4 个 UTF16 单位。
	const preedit = "ni你好"
	cases := []struct {
		name   string
		cursor int
		want   int
	}{
		{"negative means end", -1, 4},
		{"mid-rune snaps down", 3, 2},
		{"rune boundary", 2, 2},
		{"second char start", 5, 3},
		{"at len means end", 8, 4},
		{"overlong clamps to end", 100, 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, ed, _ := newTestSession()
			s.AttachEditor(ed, fakeField{})
			s.PreeditChanged(input.PreeditEvent{Text: preedit, Cursor: c.cursor})
			if ed.GetText() != preedit {
				t.Fatalf("text=%q want %q", ed.GetText(), preedit)
			}
			// selection/composingRange 以 UTF16 计，cuOff 的直接体现。
			if got := ed.selection.Extent - ed.composingRange.Start(); got != c.want {
				t.Fatalf("cursor=%d utf16caret=%d want %d", c.cursor, got, c.want)
			}
		})
	}
}

// M3.5: ImeSession.DeleteSurrounding(before,after) 转 Editor 相对偏移时
// 不得静默丢弃（韩文等依赖该事件删词），after=0 与越界必须分别正确处理。
func TestSessionDeleteSurroundingCounts_M35(t *testing.T) {
	t.Run("before only", func(t *testing.T) {
		s, ed, _ := newTestSession()
		s.AttachEditor(ed, fakeField{})
		setText(t, ed, "hello")
		s.DeleteSurrounding(2, 0)
		if ed.GetText() != "hel" {
			t.Fatalf("text=%q want %q", ed.GetText(), "hel")
		}
	})
	t.Run("after only", func(t *testing.T) {
		s, ed, _ := newTestSession()
		s.AttachEditor(ed, fakeField{})
		setText(t, ed, "hello")
		ed.SetSelection(TextRange{Base: 3, Extent: 3})
		s.DeleteSurrounding(0, 2)
		if ed.GetText() != "hel" {
			t.Fatalf("text=%q want %q", ed.GetText(), "hel")
		}
	})
	t.Run("korean syllables", func(t *testing.T) {
		s, ed, _ := newTestSession()
		s.AttachEditor(ed, fakeField{})
		setText(t, ed, "한글ab")
		ed.SetSelection(TextRange{Base: 2, Extent: 2})
		s.DeleteSurrounding(1, 1)
		if ed.GetText() != "한b" {
			t.Fatalf("text=%q want %q", ed.GetText(), "한b")
		}
	})
	t.Run("zero counts are noop", func(t *testing.T) {
		s, ed, _ := newTestSession()
		s.AttachEditor(ed, fakeField{})
		setText(t, ed, "hello")
		s.DeleteSurrounding(0, 0)
		if ed.GetText() != "hello" {
			t.Fatalf("text=%q want unchanged", ed.GetText())
		}
	})
}
