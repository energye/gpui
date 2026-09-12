package textinput

import (
	"strings"
	"testing"

	"github.com/energye/gpui/ui/rendering"
)

// Window resize must propagate through the box into its clip/viewport
// shells: otherwise the border paints the new rect while content stays
// clipped to the construction size (stale clip), and wrap keeps the old
// break width. Scroll re-clamps to the new visible window.
func TestBoxResize_PropagatesToShells(t *testing.T) {
	t.Run("viewport", func(t *testing.T) {
		ed, box := newMatrixBox(t, "single")
		ed.SetTextSimple(strings.Repeat("ab", 2000))
		caretToEnd(ed)
		box.Sync()
		if box.ScrollX() <= 0 {
			t.Fatal("want scrolled tail before resize")
		}
		box.FixedWidth, box.FixedHeight = 600, 60
		box.Layout(rendering.Loose(1200, 800))
		if box.Viewport == nil {
			t.Fatal("viewport box lost its viewport")
		}
		if box.Viewport.FixedWidth != 598 || box.Viewport.FixedHeight != 58 {
			t.Fatalf("viewport=%vx%v want 598x58", box.Viewport.FixedWidth, box.Viewport.FixedHeight)
		}
		lay := box.TextLayout()
		x, _, _, ok := lay.GetOffsetForCaret(ed.GetCursorOffset(), 0, 1.5)
		if !ok {
			t.Fatal("no caret geometry after resize")
		}
		pad := box.Padding()
		visW := 600 - 2 - 2*pad
		if x-box.ScrollX() < -1 || x-box.ScrollX() > visW+1 {
			t.Fatalf("caret x=%.1f outside resized view (scroll=%.1f visW=%.1f)", x, box.ScrollX(), visW)
		}
	})
	t.Run("wrap", func(t *testing.T) {
		ed, box := newMatrixBox(t, "wrap")
		ed.SetTextSimple(strings.Repeat("hello world ", 40))
		box.FixedWidth, box.FixedHeight = 500, 200
		box.Layout(rendering.Loose(1200, 800))
		if box.clip == nil {
			t.Fatal("multi box lost its clip")
		}
		if box.clip.FixedWidth != 500 || box.clip.FixedHeight != 200 {
			t.Fatalf("clip=%vx%v want 500x200", box.clip.FixedWidth, box.clip.FixedHeight)
		}
		pad := box.Padding()
		if box.TextLayout() == nil {
			t.Fatal("no layout after resize")
		}
		if got := box.txt.MaxWidth; got != 500-2*pad {
			t.Fatalf("wrap width=%.1f want %.1f", got, 500-2*pad)
		}
	})
	t.Run("shrinkReclamps", func(t *testing.T) {
		ed, box := newMatrixBox(t, "wrap")
		ed.SetTextSimple("hello" + strings.Repeat("\n", 60) + "TAIL")
		caretToEnd(ed)
		box.Sync()
		maxBefore := box.ScrollY()
		if maxBefore <= 0 {
			t.Fatal("want scrolled tail before shrink")
		}
		box.FixedWidth, box.FixedHeight = 880, 60
		box.Layout(rendering.Loose(1200, 800))
		lay := box.TextLayout()
		totalH := lay.LineTop(lay.LineCount()-1) + lay.LineHeight(lay.LineCount()-1)
		wantMax := totalH - (60 - 2*box.Padding())
		if wantMax < 0 {
			wantMax = 0
		}
		if d := box.ScrollY() - wantMax; d > 1 || d < -1 {
			t.Fatalf("scrollY=%.1f want max %.1f after shrink", box.ScrollY(), wantMax)
		}
	})
}
