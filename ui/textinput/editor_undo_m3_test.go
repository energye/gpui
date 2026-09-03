package textinput

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/energye/gpui/ui/rendering"
)

func m3Doc(t *testing.T, targetBytes int) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/cjk3000.txt")
	if err != nil || len(raw) == 0 {
		t.Fatalf("read testdata/cjk3000.txt: %v", err)
	}
	base := strings.TrimSpace(string(raw))
	if base == "" {
		t.Fatal("cjk3000.txt empty")
	}
	var sb strings.Builder
	for sb.Len() < targetBytes {
		sb.WriteString(base)
	}
	s := sb.String()
	if len(s) > targetBytes {
		s = s[:targetBytes]
		for len(s) > 0 {
			r, size := utf8.DecodeLastRuneInString(s)
			if r != utf8.RuneError || size > 1 {
				break
			}
			s = s[:len(s)-1]
		}
	}
	if !utf8.ValidString(s) {
		t.Fatal("m3Doc produced invalid UTF-8")
	}
	return s
}

func endCaret(e *Editor) {
	n := utf16Len(e.GetText())
	e.SetSelection(TextRange{Base: n, Extent: n})
}

func TestUndoDelta_Roundtrip_M3(t *testing.T) {
	e := New()
	const steps = 100
	for i := 0; i < steps; i++ {
		endCaret(e)
		if !e.AddText(fmt.Sprintf("x%d ", i)) {
			t.Fatalf("AddText step %d failed", i)
		}
	}
	final := e.GetText()
	for i := steps - 1; i >= 0; i-- {
		if !e.Undo() {
			t.Fatalf("Undo step %d failed", i)
		}
	}
	if e.GetText() != "" {
		t.Fatalf("after 100 undos want empty, got %q", e.GetText())
	}
	for i := 0; i < steps; i++ {
		if !e.Redo() {
			t.Fatalf("Redo step %d failed", i)
		}
	}
	if e.GetText() != final {
		t.Fatalf("after redo mismatch:\nwant %q\n got %q", final, e.GetText())
	}
}

func TestUndoDelta_CompositionSingleStep_M3(t *testing.T) {
	e := New()
	mustSetText(t, e, "ab")
	e.SetSelection(TextRange{Base: 2, Extent: 2})
	e.BeginComposing()
	e.UpdateComposingText("ni", TextRange{Base: 2, Extent: 4})
	e.UpdateComposingText("nihao", TextRange{Base: 2, Extent: 7})
	e.CommitComposing()
	if e.GetText() != "abnihao" {
		t.Fatalf("composed text %q", e.GetText())
	}
	if !e.Undo() {
		t.Fatal("Undo composition failed")
	}
	if e.GetText() != "ab" {
		t.Fatalf("undo composition want %q got %q", "ab", e.GetText())
	}
	if !e.Redo() {
		t.Fatal("Redo composition failed")
	}
	if e.GetText() != "abnihao" {
		t.Fatalf("redo composition want %q got %q", "abnihao", e.GetText())
	}
}

func TestUndoDelta_MemoryCeiling_M3(t *testing.T) {
	doc := m3Doc(t, 100000)
	e := New()
	e.SetTextSimple(doc)
	before := e.HistoryBytes()
	const edits = 100
	for i := 0; i < edits; i++ {
		endCaret(e)
		if !e.AddText("x") {
			t.Fatalf("AddText %d failed", i)
		}
	}
	after := e.HistoryBytes()
	growth := after - before
	if growth > edits*64 {
		t.Fatalf("history growth %d bytes for %d 1-byte edits, want <= %d (delta model)", growth, edits, edits*64)
	}
	if after > len(doc)+edits*64 {
		t.Fatalf("history total %d bytes, doc %d bytes: not incremental", after, len(doc))
	}
	for i := 0; i < 50; i++ {
		endCaret(e)
		if !e.AddText("y") {
			t.Fatalf("AddText cap %d failed", i)
		}
	}
	if got := len(e.history); got > 100 {
		t.Fatalf("history groups %d, want <= 100", got)
	}
	if after150 := e.HistoryBytes(); after150 > len(doc)+150*64 {
		t.Fatalf("history total %d bytes after 150 edits: cap eviction broken", after150)
	}
	for i := 0; i < 60; i++ {
		if !e.Undo() {
			t.Fatalf("Undo cap %d failed", i)
		}
	}
	endCaret(e)
	if !e.AddText("z") {
		t.Fatal("AddText after undo failed")
	}
	if e.Redo() {
		t.Fatal("Redo should fail after new edit cleared redo stack")
	}
}

func BenchmarkPushHistory_M3(b *testing.B) {
	for _, n := range []int{1000, 100000} {
		b.Run(fmt.Sprintf("%d", n), func(b *testing.B) {
			doc := strings.Repeat("a", n)
			e := New()
			e.SetTextSimple(doc)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				e.pushDelta(len(e.text), "", "x",
					e.selection, e.composingRange, e.composing,
					TextRange{Base: 1, Extent: 1}, e.composingRange, e.composing)
			}
		})
	}
}

func BenchmarkEditorSpliceShare_M3(b *testing.B) {
	for _, n := range []int{1000, 100000} {
		b.Run(fmt.Sprintf("splice_%d", n), func(b *testing.B) {
			doc := strings.Repeat("a", n)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				e := New()
				e.SetTextSimple(doc)
				endCaret(e)
				b.StartTimer()
				e.AddText("x")
			}
		})
		b.Run(fmt.Sprintf("layout_%d", n), func(b *testing.B) {
			doc := strings.Repeat("a", n)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = rendering.BuildTextLayout(doc, nil, 16, 0, 1.25)
			}
		})
	}
}
