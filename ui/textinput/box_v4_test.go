package textinput

import (
	"strings"
	"testing"
)

// TestV4_LongLineStaysOnRenderTextWindow locks the V4/V2 boundary: an
// over-long SINGLE logical line must not enter the V2 TextViewport virtual
// mode (which would materialize the whole line and bypass the D1 window),
// and must instead expose a bounded RenderText truncation window.
func TestV4_LongLineStaysOnRenderTextWindow(t *testing.T) {
	ed := New()
	ed.SetSingleLine(false)
	// Pressure data (not golden/standard data): one 5e6-byte logical line.
	ed.SetTextSimple(strings.Repeat("a", 5000000))
	b := NewMultiLineInputBox(ed, 200, 600, 14)

	if b.virtualOn || b.txt.Virtual() {
		t.Fatalf("over-long single line entered TextViewport virtual mode; D1 window would be bypassed")
	}
	start, end, fullLen, truncated := b.LongLineWindow()
	if !truncated || fullLen != 5000000 {
		t.Fatalf("no truncation window: (%d,%d,%d,%v)", start, end, fullLen, truncated)
	}
	// The window must be a small visible band, never anywhere near the full
	// 5e6-byte line (plan §3 layer 4 "只整形可见水平窗口").
	if end-start >= fullLen/10 {
		t.Fatalf("window not bounded: %d bytes of a %d-byte line", end-start, fullLen)
	}
}
