package textinput

import (
	"strings"
	"testing"

	"github.com/energye/gpui/ui/rendering"
)

func TestTextLayout_StickyColumn(t *testing.T) {
	// Deep lines to test sticky column preservation
	txt := "0123456789\n0123456789\n0123456789"
	lay := rendering.BuildTextLayout(txt, nil, 14, 0, 1.2)
	ed := New()
	ed.SetText(txt, TextRange{Base: 5, Extent: 5}, TextRange{}, 0)
	// Move down twice, sticky should keep column 5
	if !ed.MoveVisualDown(lay) {
		t.Fatalf("down1 failed")
	}
	off1 := ed.GetCursorOffset()
	// line 1 start is 11 (10 +1), +5 =16? Let's compute: "0123456789"=10, +"\n"=1 => 11, second line offset 11+5=16
	if off1 != 16 {
		t.Fatalf("down1 off %d want 16", off1)
	}
	if !ed.MoveVisualDown(lay) {
		t.Fatalf("down2 failed")
	}
	off2 := ed.GetCursorOffset()
	// third line 22+5=27? Actually 11*2=22, +5=27
	if off2 != 27 {
		t.Fatalf("down2 off %d want 27", off2)
	}
	// Move up should return to 16 then 5
	if !ed.MoveVisualUp(lay) {
		t.Fatalf("up1 failed")
	}
	if ed.GetCursorOffset() != 16 {
		t.Fatalf("up1 off %d want 16", ed.GetCursorOffset())
	}
	if !ed.MoveVisualUp(lay) {
		t.Fatalf("up2 failed")
	}
	if ed.GetCursorOffset() != 5 {
		t.Fatalf("up2 off %d want 5", ed.GetCursorOffset())
	}
	// Horizontal move should clear sticky, next down goes from current x
	ed.SetCaretWithAffinity(5, rendering.AffinityDownstream)
	ed.MoveVisual(1, lay) // right to 6
	if ed.GetCursorOffset() != 6 {
		t.Fatalf("right off %d want 6", ed.GetCursorOffset())
	}
	if !ed.MoveVisualDown(lay) {
		t.Fatalf("down after horiz failed")
	}
	// sticky re-captured at 6, so down should be at line1 col 6 => 17
	if ed.GetCursorOffset() != 17 {
		t.Fatalf("sticky after horiz off %d want 17", ed.GetCursorOffset())
	}
	_ = strings.Contains
}
