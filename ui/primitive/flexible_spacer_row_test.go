package primitive_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Modal footer: Row(Spacer, btn, btn) under tall MaxHeight must stay ~button height.
func TestSpacerInRowDoesNotTakeMaxHeight(t *testing.T) {
	btn := primitive.NewBox()
	btn.Width, btn.Height = 60, 32
	row := primitive.Row(primitive.Spacer(), btn)
	row.Gap = 8
	row.CrossAlign = core.CrossCenter
	// Simulate Column giving the footer a huge MaxHeight (full panel).
	sz := row.Layout(core.Constraints{MaxWidth: 400, MaxHeight: 700})
	if sz.Height > 40 {
		t.Fatalf("row height=%.1f want ~32 (Spacer must not absorb MaxHeight=700)", sz.Height)
	}
	sp := row.Children()[0]
	if sp.Base().Size().Height > 1 {
		t.Fatalf("spacer height=%.1f want 0", sp.Base().Size().Height)
	}
	// Button must not be CrossCentered mid-row
	off := btn.Base().Offset()
	if off.Y > 2 {
		t.Fatalf("button offset.Y=%.1f want ~0", off.Y)
	}
}
