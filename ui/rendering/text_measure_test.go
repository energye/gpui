package rendering_test

import (
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
