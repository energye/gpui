package textinput

import (
	"strings"
	"testing"
	"time"
)

// TestBoxVirtualSync_1e6 locks the V2b-2b-1 path: large multi-line no-wrap
// boxes sync via the viewport model (no GetText/full layout); non-collapsed
// selection falls back to the full path so selection/IME remain correct.
func TestBoxVirtualSync_1e6(t *testing.T) {
	ed := New()
	ed.SetSingleLine(false)
	ed.SetTextSimple(strings.Repeat("line content here\n", 1000000))
	t0 := time.Now()
	b := NewMultiLineInputBox(ed, 800, 600, 14)
	firstSync := time.Since(t0)
	t.Logf("1e6-line box first sync=%v", firstSync)
	if !b.virtualOn || !b.txt.Virtual() {
		t.Fatalf("large multi-line box did not enter virtual mode")
	}
	if b.txt.Text != "" {
		t.Fatalf("virtual sync materialized full text (len=%d)", len(b.txt.Text))
	}
	if firstSync > 200*time.Millisecond {
		t.Fatalf("first sync=%v, looks like a full layout", firstSync)
	}
	if b.vtext.CacheLen() > 64 {
		t.Fatalf("cache=%d, expected O(visible)", b.vtext.CacheLen())
	}
	if _, top, _, ok := b.caretAnchor(); !ok || top < 0 || top > 600 {
		t.Fatalf("caret anchor top=%.1f ok=%v, expected inside the scrolled viewport", top, ok)
	}
	if line := ed.ensureBuf().LineIndex(ed.GetCursorOffset()); line < 999000 {
		t.Fatalf("caret line=%d, expected near document end", line)
	}
	// 鼠标命中:第 50 万行的 x=0 应落在该行行首.
	line := 500000
	want := ed.ensureBuf().LineStart(line)
	got := b.virtualByteAt(0, b.vtext.LineTop(line))
	if got != want {
		t.Fatalf("virtualByteAt line %d = %d want %d", line, got, want)
	}
	// 打字后仍走虚拟,且只失效缓存.
	ed.AddText("x")
	b.sync()
	if !b.virtualOn || b.txt.Text != "" {
		t.Fatalf("typing dropped virtual mode or materialized text")
	}
	// 非折叠选区 → 退回全量路径,保证选区/IME 功能.
	n := ed.ensureBuf().UTF16Len()
	ed.SetSelection(TextRange{Base: 0, Extent: n / 2})
	b.sync()
	if b.virtualOn {
		t.Fatalf("non-collapsed selection must fall back to full layout")
	}
	if b.txt.Text == "" {
		t.Fatalf("fallback did not materialize text")
	}
}
