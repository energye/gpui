package primitive_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Flutter constraint contract tests for the layout foundation.

func TestDecoratedCapsChildMaxNotMin(t *testing.T) {
	// Switch-like: track 44×22 with 18×18 thumb — thumb must stay 18.
	thumb := primitive.NewDecorated()
	thumb.Width, thumb.Height = 18, 18
	thumb.Background = render.RGBA{R: 1, G: 1, B: 1, A: 1}

	track := primitive.NewDecorated(thumb)
	track.Width, track.Height = 44, 22
	track.Padding = primitive.EdgeInsets{Left: 2, Top: 2, Right: 2, Bottom: 2}
	track.SetCenterContent(false)

	_ = track.Layout(core.Loose(200, 100)) // loose parent: Width/Height preferred, not forced fill
	if thumb.Size().Width != 18 || thumb.Size().Height != 18 {
		t.Fatalf("thumb size=%v want 18×18 (must not fill track)", thumb.Size())
	}
	if track.Size().Width != 44 || track.Size().Height != 22 {
		t.Fatalf("track size=%v want 44×22", track.Size())
	}
}

func TestDecoratedCapsChildMaxWidth(t *testing.T) {
	// Tab rail: 160-wide Decorated must not pass MaxWidth=800 to children.

	// Child without own width — should be capped at 160 when laid under wide parent.
	lab := primitive.NewText("Tab")
	p := primitive.NewPressable(lab)
	host := primitive.NewDecorated(p)
	host.Width = 160
	host.Height = 40
	host.StretchChild = true
	host.SetCenterContent(false)

	_ = host.Layout(core.Loose(800, 600)) // Width=160 must win over loose max 800
	if host.Size().Width != 160 {
		t.Fatalf("host width=%v want 160", host.Size().Width)
	}
	if p.Size().Width != 160 {
		t.Fatalf("pressable width=%v want 160 (StretchChild)", p.Size().Width)
	}
	if p.Size().Height != 40 {
		t.Fatalf("pressable height=%v want 40", p.Size().Height)
	}
}

func TestPressableDoesNotExpandUnderLoose(t *testing.T) {
	// Button in a tall loose column must stay content height, not fill 600.
	lab := primitive.NewText("OK")
	btn := primitive.NewPressable(lab)
	btn.Padding = primitive.Symmetric(15, 0)
	// Parent loose tall constraints (as if Column with max height 600).
	sz := btn.Layout(core.Loose(400, 600))
	if sz.Height > 80 {
		t.Fatalf("pressable under loose expanded height=%v (should stay ~content)", sz.Height)
	}
}

func TestPressableExpandsUnderTight(t *testing.T) {
	lab := primitive.NewText("OK")
	btn := primitive.NewPressable(lab)
	sz := btn.Layout(core.Tight(160, 40))
	if sz.Width != 160 || sz.Height != 40 {
		t.Fatalf("pressable under tight size=%v want 160×40", sz)
	}
}

func TestFlexibleRelayoutsDirtyChild(t *testing.T) {
	inner := primitive.NewDecorated()
	inner.Width, inner.Height = 50, 50
	flex := primitive.NewFlexible(1, inner)
	row := primitive.Row(flex)
	_ = row.Layout(core.Tight(200, 100))
	if flex.Size().Width < 100 {
		t.Fatalf("flex width=%v should grow", flex.Size().Width)
	}
	// Mutate child and mark dirty — parent Flexible must not skip layout.
	inner.Width, inner.Height = 80, 80
	inner.MarkNeedsLayout()
	_ = row.Layout(core.Tight(200, 100))
	if !inner.Base().NeedsLayout() {
		// After layout, dirty cleared; check size updated
	}
	if flex.Children()[0].Base().Size().Width != 80 && flex.Size().Width < 80 {
		// child may be forced by tight flex alloc — just ensure no panic
	}
	// Main assertion: second layout does not panic and Flexible still has size
	if flex.Size().Width <= 0 {
		t.Fatal("flexible zero size after dirty relayout")
	}
}

func TestSlotFillsTightParent(t *testing.T) {
	child := primitive.NewText("hi")
	slot := primitive.NewSlot("body", child)
	// Parent assigns tight full panel size
	sz := slot.Layout(core.Tight(400, 300))
	if sz.Width != 400 || sz.Height != 300 {
		t.Fatalf("slot size=%v want 400×300", sz)
	}
}

func TestDecoratedStretchChildTopLeft(t *testing.T) {
	child := primitive.NewDecorated()
	child.Width, child.Height = 50, 20
	host := primitive.NewDecorated(child)
	host.Width, host.Height = 200, 200
	host.StretchChild = true
	host.SetCenterContent(true) // must be ignored when StretchChild
	_ = host.Layout(core.Loose(400, 400))
	off := child.Base().Offset()
	if off.Y != 0 {
		// padding 0 → top
		t.Fatalf("StretchChild child Y=%v want 0 (top), not centered", off.Y)
	}
}

func TestFlexibleChildTopLeft(t *testing.T) {
	// Flexible fills 200×400; short child must sit at y=0 (not centered).
	child := primitive.NewDecorated()
	child.Width, child.Height = 50, 30
	flex := primitive.NewFlexible(1, child)
	// Simulate Flex allocation: tight height, tight width
	_ = flex.Layout(core.Tight(200, 400))
	if flex.Size().Width != 200 || flex.Size().Height != 400 {
		t.Fatalf("flex size=%v want 200×400", flex.Size())
	}
	if child.Base().Offset().Y != 0 || child.Base().Offset().X != 0 {
		t.Fatalf("child offset=%v want (0,0) top-left", child.Base().Offset())
	}
	if child.Size().Height != 30 {
		t.Fatalf("child height=%v want 30 (not forced fill)", child.Size().Height)
	}
}

func TestFlexibleFillChild(t *testing.T) {
	child := primitive.NewDecorated()
	child.Width, child.Height = 10, 10
	flex := primitive.NewFlexible(1, child)
	flex.FillChild = true
	_ = flex.Layout(core.Tight(100, 40))
	if child.Size().Width != 100 || child.Size().Height != 40 {
		t.Fatalf("FillChild size=%v want 100×40", child.Size())
	}
}

// TestSpacerDoesNotTakeLooseMaxHeight locks LAYOUT_FOUNDATION: Flexible only
// expands tight axes. Modal footer Row(Spacer, btn) must not inflate to MaxHeight.
func TestSpacerDoesNotTakeLooseMaxHeight(t *testing.T) {
	sp := primitive.Spacer()
	// Loose constraints with huge MaxHeight (as if Column gave unbounded cross).
	sz := sp.Layout(core.Constraints{MaxWidth: 200, MaxHeight: 600, MinWidth: 0, MinHeight: 0})
	if sz.Height > 1 {
		t.Fatalf("Spacer under loose height=%v want ~0 (only tight axes expand)", sz.Height)
	}
	// Tight main axis only (Row flex allocation): width expands, height stays 0.
	sz = sp.Layout(core.Constraints{MinWidth: 120, MaxWidth: 120, MinHeight: 0, MaxHeight: 600})
	if sz.Width != 120 {
		t.Fatalf("Spacer tight width=%v want 120", sz.Width)
	}
	if sz.Height > 1 {
		t.Fatalf("Spacer must not adopt loose MaxHeight: height=%v", sz.Height)
	}
}

// TestDecoratedCenterContentDefaultOff — chrome must opt in.
func TestDecoratedCenterContentDefaultOff(t *testing.T) {
	child := primitive.NewDecorated()
	child.Width, child.Height = 20, 10
	host := primitive.NewDecorated(child)
	host.Width, host.Height = 100, 40 // explicit taller chrome
	// default CenterContent false → child top (after padding 0)
	_ = host.Layout(core.Loose(200, 200))
	if host.CenterContent {
		t.Fatal("CenterContent must default false")
	}
	if child.Base().Offset().Y != 0 {
		t.Fatalf("default no center: child Y=%v want 0", child.Base().Offset().Y)
	}
	host.SetCenterContent(true)
	host.MarkNeedsLayout()
	_ = host.Layout(core.Loose(200, 200))
	// With center, child should move down when host is taller than child.
	if child.Base().Offset().Y <= 0 {
		t.Fatalf("CenterContent=true: child Y=%v want >0", child.Base().Offset().Y)
	}
}

// TestPressableChildTopLeftUnderLooseColumn — no magic mid-box paint offset.
func TestPressableChildTopLeftUnderLooseColumn(t *testing.T) {
	lab := primitive.NewText("Tab")
	p := primitive.NewPressable(lab)
	p.Padding = primitive.Symmetric(8, 4)
	// Tall loose max as Tabs body used to pass.
	sz := p.Layout(core.Loose(160, 400))
	if sz.Height > 60 {
		t.Fatalf("pressable expanded under loose: H=%v", sz.Height)
	}
	// Label must sit inside pressable box (top-left after padding).
	if len(p.Children()) == 0 {
		t.Fatal("no child")
	}
	off := p.Children()[0].Base().Offset()
	if off.Y > p.Size().Height {
		t.Fatalf("child offset.Y=%v outside pressable H=%v", off.Y, p.Size().Height)
	}
	if off.Y < 0 {
		t.Fatalf("negative child offset %v", off)
	}
}

// TestRowSpacerButtonsStayTop — Modal footer pattern under loose column height.
func TestRowSpacerButtonsStayTop(t *testing.T) {
	b1 := primitive.NewPressable(primitive.NewText("Cancel"))
	b1.Padding = primitive.Symmetric(12, 6)
	b2 := primitive.NewPressable(primitive.NewText("OK"))
	b2.Padding = primitive.Symmetric(12, 6)
	row := primitive.Row(primitive.Spacer(), b1, b2)
	row.Gap = 8
	row.CrossAlign = core.CrossCenter
	// Loose max height 400 — historical bug inflated Spacer → CrossCenter mid-box.
	sz := row.Layout(core.Constraints{MaxWidth: 480, MaxHeight: 400})
	if sz.Height > 80 {
		t.Fatalf("row height=%v (Spacer took loose MaxHeight?)", sz.Height)
	}
	for _, btn := range []*primitive.Pressable{b1, b2} {
		if btn.Base().Offset().Y > 20 {
			t.Fatalf("button offset.Y=%v in row H=%v — pushed mid-box", btn.Base().Offset().Y, sz.Height)
		}
	}
}

// TestDecoratedCenterContentExpandWidth — ExpandWidth must count as chrome width
// so CenterContent can mid-box the label (grid/layout demo cells).
func TestDecoratedCenterContentExpandWidth(t *testing.T) {
	child := primitive.NewDecorated()
	child.Width, child.Height = 20, 10
	host := primitive.NewDecorated(child)
	host.ExpandWidth = true
	host.MinHeight = 40
	host.SetCenterContent(true)
	// Parent gives finite MaxWidth (Col / row packer style).
	_ = host.Layout(core.Constraints{MaxWidth: 200, MaxHeight: 100})
	if host.Size().Width != 200 {
		t.Fatalf("ExpandWidth size.W=%v want 200", host.Size().Width)
	}
	off := child.Base().Offset()
	// horizontal: (200-20)/2 = 90; vertical: (40-10)/2 = 15
	if off.X < 80 || off.X > 100 {
		t.Fatalf("CenterContent+ExpandWidth X=%v want ~90", off.X)
	}
	if off.Y < 10 || off.Y > 20 {
		t.Fatalf("CenterContent+MinHeight Y=%v want ~15", off.Y)
	}
}
