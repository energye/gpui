package textinput

import (
	"strings"
	"testing"

	"github.com/energye/gpui/ui/rendering"
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
	if end-start > rendering.DefaultMaxSingleLineBytes {
		t.Fatalf("window not bounded: %d bytes (cap %d)", end-start, rendering.DefaultMaxSingleLineBytes)
	}
}
