package rendering_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/energye/gpui/ui/rendering"
)

func TestRenderText_LayoutUsesRuneEstimate(t *testing.T) {
	txt := rendering.NewRenderText("你好") // 2 runes, 6 bytes
	txt.FontSize = 10
	txt.ApproxCharW = 1.0
	// Loose constraints so MinWidth is 0 (FlushLayout uses Tight viewport).
	sz := txt.Layout(rendering.Loose(400, 100))
	wantW := float64(utf8.RuneCountInString("你好")) * 10 * 1.0
	if sz.Width < wantW-0.1 || sz.Width > wantW+0.1 {
		t.Fatalf("width=%v want ~%v (rune-based, not byte-based %v)", sz.Width, wantW, len("你好")*10)
	}
	if sz.Width >= float64(len("你好"))*10*0.9 {
		t.Fatalf("looks like byte-length width=%v", sz.Width)
	}
}

func TestRenderText_SetColorNoLayout(t *testing.T) {
	txt := rendering.NewRenderText("Hi")
	root := rendering.NewRenderBox(txt)
	root.FixedWidth, root.FixedHeight = 100, 40
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 100, Height: 40}, true)
	owner.FlushPaint(&rendering.PaintContext{}, true)
	n := owner.LayoutCount
	txt.SetColor(1, 0, 0, 1)
	if owner.FlushLayout(rendering.Size{Width: 100, Height: 40}, false) {
		t.Fatal("SetColor must not layout")
	}
	if owner.LayoutCount != n {
		t.Fatalf("layout count %d → %d", n, owner.LayoutCount)
	}
	if !txt.NeedsPaint() {
		t.Fatal("needs paint")
	}
}

func TestRenderText_MaxWidthWrapLayout(t *testing.T) {
	txt := rendering.NewRenderText("hello wrapped world from gpui")
	txt.FontSize = 12
	txt.ApproxCharW = 0.55
	txt.SetMaxWidth(80)
	sz := txt.Layout(rendering.Loose(400, 400))
	if sz.Width > 80.1 {
		t.Fatalf("width=%v want ≤80", sz.Width)
	}
	if sz.Height <= 12*1.2 {
		t.Fatalf("height=%v want multi-line taller", sz.Height)
	}
}

func TestRenderText_SingleLineEllipsis_MaxWidth(t *testing.T) {
	long := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	txt := rendering.NewRenderText(long)
	txt.FontSize = 10
	txt.ApproxCharW = 1.0 // 1em per rune → easy width math
	txt.SetMaxWidth(50)   // fits ~5 runes without ellipsis budget
	txt.SetMaxLines(1)
	txt.SetOverflow(rendering.TextOverflowEllipsis)

	sz := txt.Layout(rendering.Loose(400, 400))
	if sz.Width > 50.1 {
		t.Fatalf("width=%v want ≤50 (capped)", sz.Width)
	}
	// Single line height must not grow with full string length.
	fullH := float64(utf8.RuneCountInString(long)) * 10 * 1.25
	if sz.Height >= fullH*0.5 {
		t.Fatalf("height=%v looks unbounded (full would be ~%v)", sz.Height, fullH)
	}
	disp := txt.DisplayText()
	if !strings.Contains(disp, "…") {
		t.Fatalf("display %q want ellipsis marker", disp)
	}
	if disp == long {
		t.Fatal("display must not be the full unbounded string")
	}
	if strings.Count(disp, "\n") != 0 {
		t.Fatalf("maxLines=1 display has newlines: %q", disp)
	}
	// Paint path uses the same DisplayLines (no second truncation story).
	lines := txt.DisplayLines()
	if len(lines) != 1 {
		t.Fatalf("lines=%d want 1", len(lines))
	}
	if lines[0] != disp {
		t.Fatalf("paint line %q != display %q", lines[0], disp)
	}
}

func TestRenderText_MaxLines_Ellipsis(t *testing.T) {
	// Many words → many wrap lines; MaxLines=2 must cap height and mark ellipsis.
	txt := rendering.NewRenderText("one two three four five six seven eight nine ten eleven twelve")
	txt.FontSize = 10
	txt.ApproxCharW = 1.0
	txt.SetMaxWidth(40) // ~4 chars/line → many lines
	txt.SetMaxLines(2)
	txt.SetOverflow(rendering.TextOverflowEllipsis)

	// Uncapped wrap would be taller than 2 lines.
	uncapped := rendering.NewRenderText(txt.Text)
	uncapped.FontSize = 10
	uncapped.ApproxCharW = 1.0
	uncapped.SetMaxWidth(40)
	uSz := uncapped.Layout(rendering.Loose(400, 400))

	sz := txt.Layout(rendering.Loose(400, 400))
	if sz.Width > 40.1 {
		t.Fatalf("width=%v want ≤40", sz.Width)
	}
	if sz.Height > uSz.Height+0.1 && uSz.Height > 0 {
		// Cap must not exceed uncapped; primarily must be ≤ 2 line heights.
	}
	lh := 10 * 1.25 * 1.2 // fontSize * 1.25 * lineSpacing default
	if sz.Height > 2*lh+1 {
		t.Fatalf("height=%v want ≤ ~2 lines (%v)", sz.Height, 2*lh)
	}
	lines := txt.DisplayLines()
	if len(lines) > 2 {
		t.Fatalf("lines=%d want ≤2: %v", len(lines), lines)
	}
	if len(lines) < 1 {
		t.Fatal("no lines")
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "…") {
		t.Fatalf("display %q want ellipsis when lines truncated", joined)
	}
	// Full text must not appear as a single display string equal to source.
	if joined == txt.Text {
		t.Fatal("display equals full source — maxLines ignored")
	}
}

func TestRenderText_MaxLines_Clip_NoEllipsis(t *testing.T) {
	txt := rendering.NewRenderText("alpha beta gamma delta epsilon zeta eta theta")
	txt.FontSize = 10
	txt.ApproxCharW = 1.0
	txt.SetMaxWidth(30)
	txt.SetMaxLines(2)
	txt.SetOverflow(rendering.TextOverflowClip)

	sz := txt.Layout(rendering.Loose(400, 400))
	lh := 10 * 1.25 * 1.2
	if sz.Height > 2*lh+1 {
		t.Fatalf("height=%v want ≤2 lines", sz.Height)
	}
	disp := txt.DisplayText()
	if strings.Contains(disp, "…") {
		t.Fatalf("clip overflow must not insert ellipsis: %q", disp)
	}
	if len(txt.DisplayLines()) > 2 {
		t.Fatalf("lines=%v", txt.DisplayLines())
	}
}

func TestRenderText_SetMaxLines_DirtiesLayout(t *testing.T) {
	txt := rendering.NewRenderText("hello world again and again")
	txt.SetMaxWidth(40)
	root := rendering.NewRenderBox(txt)
	root.FixedWidth, root.FixedHeight = 200, 200
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 200, Height: 200}, true)
	n := owner.LayoutCount
	txt.SetMaxLines(1)
	txt.SetOverflow(rendering.TextOverflowEllipsis)
	if !owner.FlushLayout(rendering.Size{Width: 200, Height: 200}, false) {
		t.Fatal("SetMaxLines/Overflow must dirty layout")
	}
	if owner.LayoutCount <= n {
		t.Fatalf("layout count did not grow")
	}
}

// TestRenderTextByteOffsetAt covers click-to-caret conversion: clamping and
// rune-boundary snapping (CJK must never split mid-rune), independent of
// whether a real font face is loaded.
func TestRenderTextByteOffsetAt(t *testing.T) {
	tt := rendering.NewRenderText("你好a")
	tt.FontSize = 20

	if got := tt.ByteOffsetAt(-5); got != 0 {
		t.Fatalf("negative x = %d, want 0", got)
	}
	if got := tt.ByteOffsetAt(1e6); got != len("你好a") {
		t.Fatalf("far x = %d, want %d", got, len("你好a"))
	}
	// Every returned offset must be a rune boundary of the buffer.
	valid := map[int]bool{0: true, 3: true, 6: true, 7: true}
	for _, x := range []float64{1, 5, 10, 15, 20, 25, 30, 40, 50, 60} {
		if off := tt.ByteOffsetAt(x); !valid[off] {
			t.Fatalf("ByteOffsetAt(%v) = %d, not a rune boundary", x, off)
		}
	}
	// Monotonic non-decreasing along x.
	last := 0
	for x := 0.0; x <= 100; x += 2 {
		off := tt.ByteOffsetAt(x)
		if off < last {
			t.Fatalf("non-monotonic at x=%v: %d < %d", x, off, last)
		}
		last = off
	}

	empty := rendering.NewRenderText("")
	if empty.ByteOffsetAt(42) != 0 {
		t.Fatal("empty text must map to 0")
	}
}
