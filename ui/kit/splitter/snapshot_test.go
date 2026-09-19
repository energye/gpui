package splitter

import (
	"image"
	"sync"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// R2-6 snapshot quartet (button snapshot paradigm): raster paint follows the
// frozen SplitterSnap, never the live splitter / panel fields.

func splitBarToImage(t *testing.T, s *Splitter, index int, w, h float64) image.Image {
	t.Helper()
	dc := render.NewContext(int(w), int(h))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	s.paintBar(rendering.NewPaintContext(dc, 1), rendering.Size{Width: w, Height: h}, index)
	return dc.Image()
}

func splitImagesEqual(a, b image.Image) bool {
	if a == nil || b == nil || !a.Bounds().Eq(b.Bounds()) {
		return false
	}
	for y := 0; y < a.Bounds().Dy(); y++ {
		for x := 0; x < a.Bounds().Dx(); x++ {
			if a.At(x, y) != b.At(x, y) {
				return false
			}
		}
	}
	return true
}

func newSplitterSnapped(t *testing.T) *Splitter {
	t.Helper()
	s := NewSplitter(
		NewSplitterPanel(rendering.NewRenderColorBox(20, 20, 0.9, 0.2, 0.2, 1)),
		NewSplitterPanel(rendering.NewRenderColorBox(20, 20, 0.2, 0.4, 0.9, 1)),
	)
	s.refreshSnapshot()
	return s
}

// TestSplitter_SnapshotIgnoresLivePoke: live pokes without refresh must not
// change dispatched paint; refresh takes them up.
func TestSplitter_SnapshotIgnoresLivePoke(t *testing.T) {
	s := newSplitterSnapped(t)
	before := splitBarToImage(t, s, 0, 40, 40)
	// Live pokes bypass every mark funnel on purpose.
	s.panels[0].resizable = false
	s.panels[1].resizable = false
	s.panels[1].collapsible = true
	s.panels[1].collapsibleStart = true
	s.orientationSet = true
	s.orientation = Vertical
	s.override = &theme.Tokens{
		ColorBorderSecondary: theme.Hex("#ff00ff"),
		ColorFill:            theme.Hex("#ff00ff"),
		ColorFillSecondary:   theme.Hex("#ff00ff"),
		ColorTextTertiary:    theme.Hex("#00ffff"),
	}
	again := splitBarToImage(t, s, 0, 40, 40)
	if !splitImagesEqual(before, again) {
		t.Fatal("dispatched bar paint changed after live poke without refresh: reads live state")
	}
	s.refreshSnapshot()
	fresh := splitBarToImage(t, s, 0, 40, 40)
	if splitImagesEqual(before, fresh) {
		t.Fatal("refreshSnapshot did not pick up the live poke (vertical + colors + collapse expected)")
	}
}

// TestSplitter_FrozenSnapshotPaintsDeterministic: painting the same frozen
// snapshot twice is stable, and it stays stable until refreshSnapshot runs.
func TestSplitter_FrozenSnapshotPaintsDeterministic(t *testing.T) {
	s := newSplitterSnapped(t)
	shot1 := splitBarToImage(t, s, 0, 48, 48)
	shot2 := splitBarToImage(t, s, 0, 48, 48)
	if !splitImagesEqual(shot1, shot2) {
		t.Fatal("frozen snapshot paint is not deterministic")
	}
	// Live poke without refresh: output must stay identical to shot2 ...
	s.panels[0].resizable = false
	middle := splitBarToImage(t, s, 0, 48, 48)
	if !splitImagesEqual(shot2, middle) {
		t.Fatal("frozen snapshot changed mid-frame without refresh")
	}
	// ... and refresh then settles on the new config deterministically.
	s.refreshSnapshot()
	settled1 := splitBarToImage(t, s, 0, 48, 48)
	settled2 := splitBarToImage(t, s, 0, 48, 48)
	if !splitImagesEqual(settled1, settled2) {
		t.Fatal("post-refresh paint is not deterministic")
	}
	if splitImagesEqual(middle, settled1) {
		t.Fatal("refresh did not change dispatched paint (resizable handle should vanish)")
	}
}

// TestSplitter_SnapshotPurity: bar dispatch depends only on the frozen bits.
func TestSplitter_SnapshotPurity(t *testing.T) {
	base := SplitterSnap{
		Vertical:      false,
		BarSize:       DefaultSplitBarSize,
		DraggableSize: 20,
		BaseColor:     render.RGBA{R: 0.5, G: 0.5, B: 0.5, A: 1},
		HoverColor:    render.RGBA{R: 0.6, G: 0.6, B: 0.6, A: 1},
		ActiveColor:   render.RGBA{R: 0.7, G: 0.7, B: 0.7, A: 1},
		HandleColor:   render.RGBA{R: 0.1, G: 0.1, B: 0.1, A: 1},
		FocusColor:    render.RGBA{R: 0, G: 0, B: 0.8, A: 1},
		HasDragger:    false,
		Resizable:     []bool{true},
	}
	harness := &Splitter{}
	store := func(sb SplitterSnap) { harness.snap.Store(sb) }

	store(base)
	horizontal := splitBarToImage(t, harness, 0, 32, 24)

	flat := base
	flat.Vertical = true
	store(flat)
	vertical := splitBarToImage(t, harness, 0, 32, 24)
	if splitImagesEqual(horizontal, vertical) {
		t.Fatal("orientation bit did not affect dispatch")
	}

	noHandle := base
	noHandle.Resizable = []bool{false}
	noHandle.CollapsibleLeft = []bool{true}
	noHandle.CollapsibleRight = []bool{true}
	store(noHandle)
	condensed := splitBarToImage(t, harness, 0, 32, 24)
	if splitImagesEqual(horizontal, condensed) {
		t.Fatal("resizable/collapsible bits did not affect dispatch")
	}

	custom := noHandle
	custom.HasCollapseStart = true
	store(custom)
	customIcons := splitBarToImage(t, harness, 0, 32, 24)
	if splitImagesEqual(condensed, customIcons) {
		t.Fatal("custom icon bit did not affect dispatch")
	}
}

// TestSplitter_SnapshotConcurrentRace: one writer displaces the snapshot while
// serial bar paints read it (run under -race).
func TestSplitter_SnapshotConcurrentRace(t *testing.T) {
	s := newSplitterSnapped(t)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			s.panels[0].resizable = i%2 == 0
			s.panels[1].collapsible = i%2 == 1
			s.panels[1].collapsibleStart = i%2 == 1
			s.override = &theme.Tokens{
				ColorBorderSecondary: theme.Hex("#102040"),
				ColorFill:            theme.Hex("#203040"),
				ColorFillSecondary:   theme.Hex("#304050"),
				ColorTextTertiary:    theme.Hex("#405060"),
			}
			s.refreshSnapshot()
		}
	}()
	for i := 0; i < 300; i++ {
		_ = splitBarToImage(t, s, 0, 32, 24)
	}
	wg.Wait()
}