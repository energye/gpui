package primitive_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/primitive"
)

func TestBoxPaddingLayout(t *testing.T) {
	inner := primitive.NewBox()
	inner.Width, inner.Height = 40, 20
	box := primitive.NewBox(inner)
	box.Padding = primitive.All(8)
	sz := box.Layout(core.Loose(200, 200))
	if sz.Width != 56 || sz.Height != 36 {
		t.Fatalf("box size=%+v want 56x36", sz)
	}
	if inner.Offset().X != 8 || inner.Offset().Y != 8 {
		t.Fatalf("inner offset=%v", inner.Offset())
	}
}

func TestFlexRowWithGap(t *testing.T) {
	a := primitive.NewBox()
	a.Width, a.Height = 30, 10
	b := primitive.NewBox()
	b.Width, b.Height = 50, 10
	row := primitive.Row(a, b)
	row.Gap = 4
	sz := row.Layout(core.Loose(400, 100))
	if sz.Width != 84 {
		t.Fatalf("width=%v want 84", sz.Width)
	}
	if b.Offset().X != 34 {
		t.Fatalf("b.X=%v want 34", b.Offset().X)
	}
}

func TestFlexibleSpacer(t *testing.T) {
	a := primitive.NewBox()
	a.Width, a.Height = 20, 10
	sp := primitive.Spacer()
	b := primitive.NewBox()
	b.Width, b.Height = 20, 10
	row := primitive.Row(a, sp, b)
	sz := row.Layout(core.Tight(200, 10))
	if sz.Width != 200 {
		t.Fatalf("width=%v", sz.Width)
	}
	// spacer should take 160
	if sp.Size().Width < 150 {
		t.Fatalf("spacer width=%v want ~160", sp.Size().Width)
	}
	if b.Offset().X < 170 {
		t.Fatalf("b.X=%v want ~180", b.Offset().X)
	}
}

func TestStackCenter(t *testing.T) {
	child := primitive.NewBox()
	child.Width, child.Height = 40, 20
	stack := primitive.NewStack(primitive.Positioned(core.AlignCenter, child))
	sz := stack.Layout(core.Tight(100, 80))
	if sz.Width != 100 || sz.Height != 80 {
		t.Fatalf("stack=%+v", sz)
	}
	// Positioned wrapper offset
	pos := stack.Children()[0]
	off := pos.Base().Offset()
	if off.X != 30 || off.Y != 30 {
		t.Fatalf("positioned offset=%v want 30,30", off)
	}
}

func TestTextMeasureApprox(t *testing.T) {
	tx := primitive.NewText("Hi")
	tx.FontSize = 20
	sz := tx.Layout(core.Loose(400, 100))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("text size=%+v", sz)
	}
}

func TestClipHitAndSize(t *testing.T) {
	inner := primitive.NewBox()
	inner.Width, inner.Height = 200, 200
	clip := primitive.NewClip(inner)
	clip.Width, clip.Height = 50, 50
	sz := clip.Layout(core.Loose(400, 400))
	if sz.Width != 50 || sz.Height != 50 {
		t.Fatalf("clip size=%+v", sz)
	}
	// Hit outside clip bounds should miss.
	if hit := clip.HitTest(core.Point{X: 80, Y: 80}); hit != nil {
		t.Fatalf("expected miss outside clip, got %v", hit)
	}
}

func TestPressableClickHeadless(t *testing.T) {
	clicks := 0
	label := primitive.NewText("Click")
	btn := primitive.NewPressable(label)
	btn.Padding = primitive.Symmetric(12, 8)
	btn.SetColors(
		render.RGBA{R: 0.09, G: 0.42, B: 0.93, A: 1},
		render.RGBA{R: 0.2, G: 0.5, B: 1, A: 1},
		render.RGBA{R: 0.05, G: 0.3, B: 0.8, A: 1},
	)
	btn.Click = func() { clicks++ }

	root := primitive.NewBox(btn)
	root.Width, root.Height = 320, 200
	root.Color = render.RGBA{R: 0.95, G: 0.95, B: 0.97, A: 1}
	root.Padding = primitive.All(24)

	tree := core.NewTree(root)
	host := platform.NewHeadless(320, 200)
	defer host.Close()

	w, h := host.Size()
	tree.Layout(core.Size{Width: float64(w), Height: float64(h)})

	// Button is at padding 24,24 — click center of pressable.
	bx := 24 + btn.Size().Width/2
	by := 24 + btn.Size().Height/2
	host.InjectClick(bx, by)
	for _, ev := range host.PumpEvents() {
		platform.Dispatch(tree, ev)
	}
	if clicks != 1 {
		t.Fatalf("clicks=%d want 1 (btn size=%+v @%.1f,%.1f)", clicks, btn.Size(), bx, by)
	}
	if !btn.State.Pressed {
		// after up, pressed should be false
	}
	if btn.State.Pressed {
		t.Fatal("pressed still true after up")
	}
}

func TestPluginEmptyRun(t *testing.T) {
	h := core.NewPluginHost()
	if err := h.RegisterService("test", struct{}{}); err != nil {
		t.Fatal(err)
	}
}
