package core_test

import (
	"fmt"
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Simulates "content mid-screen but hits at top": if Flexible Column packs with
// MainCenter or Decorated centers, AbsoluteBounds and paint Origin must still match.
func TestLayoutPaintOriginMatchesHit(t *testing.T) {
	// tall host + short child (like tab panel)
	child := primitive.NewDecorated()
	child.Width, child.Height = 100, 40
	child.Background = struct{ R, G, B, A float64 }{1, 0, 0, 1}

	// Wrong pattern: parent taller, if child centered, offset.Y != 0
	host := primitive.NewDecorated(child)
	host.Width, host.Height = 400, 400
	host.SetCenterContent(false) // Flutter Align top-left
	host.StretchChild = false

	// Wrap in Flexible Column like gallery
	flex := primitive.NewFlexible(1, host)
	col := primitive.Column(primitive.NewText("title"), flex)
	col.MainAlign = core.MainStart
	col.CrossAlign = core.CrossStretch
	root := primitive.NewBox(col)
	root.Width, root.Height = 800, 600

	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 800, Height: 600})

	abs := core.AbsoluteBounds(child)
	fmt.Printf("child abs=%v size=%v off=%v\n", abs, child.Size(), child.Base().Offset())
	fmt.Printf("host abs=%v size=%v\n", core.AbsoluteBounds(host), host.Size())

	// Hit center of child rect must hit child (or host)
	cx := (abs.Min.X + abs.Max.X) / 2
	cy := (abs.Min.Y + abs.Max.Y) / 2
	hit := tree.HitTest(core.Point{X: cx, Y: cy})
	fmt.Printf("hit(%.0f,%.0f)=%T\n", cx, cy, hit)

	// Child must be near top of host (not vertically centered in 400)
	hostAbs := core.AbsoluteBounds(host)
	relY := abs.Min.Y - hostAbs.Min.Y
	if relY > 5 {
		t.Fatalf("child Y relative to host=%v — content not top-aligned (centered?)", relY)
	}
	// Click near top of panel should not miss when content is top
	if hit == nil {
		t.Fatal("miss at child center")
	}
}
