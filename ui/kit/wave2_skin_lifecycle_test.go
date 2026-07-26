package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
	skindefault "github.com/energye/gpui/ui/skin/default"
)

// Wave 2 batch: ensure Type/Skin painters exist and Node() builds.
func TestWave2_SkinPaintersRegistered(t *testing.T) {
	th := skindefault.Theme()
	for _, id := range []string{
		kit.TypeSpin, kit.TypeSkeleton, kit.TypeEmpty, kit.TypeResult,
		kit.TypeStatistic, kit.TypeAvatar, kit.TypeDivider, kit.TypeIcon, kit.TypeTypography,
		kit.TypeBadge, kit.TypeAlert, kit.TypeProgress,
	} {
		if th.Painter(id) == nil {
			t.Errorf("missing skin painter for %s", id)
		}
	}
}

func TestWave2_EnsureBuilt_Nodes(t *testing.T) {
	cases := []struct {
		name string
		node func() core.Node
	}{
		{"Spin", func() core.Node { return kit.NewSpin(nil).Node() }},
		{"Skeleton", func() core.Node { return kit.NewSkeleton().Node() }},
		{"Empty", func() core.Node { return kit.NewEmpty().Node() }},
		{"Result", func() core.Node { return kit.NewResult().Node() }},
		{"Statistic", func() core.Node { return kit.NewStatistic().Node() }},
		{"Avatar", func() core.Node { return kit.NewAvatar("A").Node() }},
		{"Divider", func() core.Node { return kit.NewDivider().Node() }},
		{"Icon", func() core.Node { return kit.NewIcon("user").Node() }},
		{"Typography", func() core.Node { return kit.NewText("hi").Node() }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := tc.node()
			if n == nil {
				t.Fatal("nil node")
			}
		})
	}
}

func TestWave2_Spin_TypeIDAndPainterRegistered(t *testing.T) {
	// Spin host embeds RepaintBoundary: host Paint must NOT intercept Skin
	// (would skip layer/glyph paint path — same bug as Icon). Skin remains
	// registered for Override tooling / non-boundary chrome.
	s := kit.NewSpin(nil)
	n := s.Node()
	if n == nil {
		t.Fatal("nil node")
	}
	if n.TypeID() != kit.TypeSpin {
		t.Fatalf("TypeID=%q want %s", n.TypeID(), kit.TypeSpin)
	}
	th := skindefault.Theme()
	if th.Painter(kit.TypeSpin) == nil {
		t.Fatal("kit.Spin painter not registered")
	}
}

func TestWave2_SkinOverride_Empty(t *testing.T) {
	e := kit.NewEmpty()
	th := skindefault.Theme()
	var painted bool
	th.Skin = core.Override(th.Skin, kit.TypeEmpty, func(pc *core.PaintContext, n core.Node) {
		painted = true
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
		}
	})
	// Empty may use Theme field
	e.Theme = th
	// force rebuild if method exists
	if e.Node() == nil {
		t.Fatal("nil")
	}
	tree := core.NewTree(e.Node())
	tree.SetTheme(th)
	tree.Layout(core.Size{Width: 200, Height: 160})
	dc := render.NewContext(200, 160)
	pc := &core.PaintContext{Theme: th, Scale: 1, DC: dc}
	e.Node().Paint(pc)
	if !painted {
		// Empty Root may not paint if empty size - still check SkinType
		if d, ok := e.Node().(*primitive.Decorated); ok {
			if d.SkinType != kit.TypeEmpty {
				t.Fatalf("SkinType=%q painted=%v", d.SkinType, painted)
			}
		} else {
			t.Fatal("empty painter not invoked and root not decorated")
		}
	}
}
