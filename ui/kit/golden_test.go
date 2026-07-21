package kit_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/primitive"
)

// Golden layout snapshot: fixed tree → stable sizes/offsets (no GPU).
func TestGoldenLayout_ButtonRow(t *testing.T) {
	a := kit.NewButton("A")
	b := kit.NewButton("B")
	row := primitive.Row(a.Node(), b.Node())
	row.Gap = 8
	row.CrossAlign = core.CrossStart
	tree := core.NewTree(row)
	tree.Layout(core.Size{Width: 400, Height: 80})

	// Golden expectations (tolerance for font-less measure)
	if a.Root.Size().Height < 24 || a.Root.Size().Height > 48 {
		t.Fatalf("btn A height=%v", a.Root.Size().Height)
	}
	if b.Root.Offset().X <= a.Root.Size().Width {
		t.Fatalf("gap/offset: A.w=%v B.x=%v", a.Root.Size().Width, b.Root.Offset().X)
	}
	// deterministic re-layout
	tree.Layout(core.Size{Width: 400, Height: 80})
	x1 := b.Root.Offset().X
	tree.Layout(core.Size{Width: 400, Height: 80})
	if b.Root.Offset().X != x1 {
		t.Fatalf("layout unstable %v vs %v", x1, b.Root.Offset().X)
	}
}

// Golden paint: CPU context gets non-zero ink after frame.
func TestGoldenPaint_PrimaryButton(t *testing.T) {
	btn := kit.NewButton("OK")
	btn.SetType(kit.ButtonPrimary)
	root := primitive.NewBox(btn.Node())
	root.Width, root.Height = 200, 80
	root.Padding = primitive.All(16)
	root.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}

	tree := core.NewTree(root)
	dc := render.NewContext(200, 80)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.RGBA{R: 1, G: 1, B: 1, A: 1})
	pc := &core.PaintContext{DC: dc, Theme: kit.DefaultTheme()}
	tree.Frame(pc, core.Size{Width: 200, Height: 80})

	img := dc.Image()
	if img == nil {
		t.Fatal("nil image")
	}
	// count non-white-ish pixels
	n := countNonWhite(img, 250)
	if n < 50 {
		t.Fatalf("expected painted pixels, got %d non-white", n)
	}
}

func countNonWhite(img image.Image, thr uint32) int {
	b := img.Bounds()
	n := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			// RGBA is 16-bit
			if r>>8 < thr || g>>8 < thr || bl>>8 < thr {
				n++
			}
		}
	}
	return n
}

func TestGoldenHit_ClickCenter(t *testing.T) {
	clicks := 0
	btn := kit.NewButton("Go")
	btn.SetOnClick(func() { clicks++ })
	tree := core.NewTree(btn.Node())
	tree.Layout(core.Size{Width: 200, Height: 80})
	cx := btn.Root.Size().Width / 2
	cy := btn.Root.Size().Height / 2
	host := platform.NewHeadless(200, 80)
	defer host.Close()
	host.InjectClick(cx, cy)
	for _, ev := range host.PumpEvents() {
		platform.Dispatch(tree, ev)
	}
	if clicks != 1 {
		t.Fatalf("clicks=%d", clicks)
	}
}

func TestAntCoverageTable(t *testing.T) {
	entries := kit.AntCoverage()
	if len(entries) < 40 {
		t.Fatalf("expected broad Ant table, got %d", len(entries))
	}
	sum := kit.CoverageSummary(entries)
	ready := sum[kit.CovReady]
	partial := sum[kit.CovPartial]
	if ready+partial < 20 {
		t.Fatalf("too few covered: ready=%d partial=%d full=%v", ready, partial, sum)
	}
	// required ready set
	need := map[string]bool{"Button": false, "Input": false, "Checkbox": false, "Progress": false}
	for _, e := range entries {
		if _, ok := need[e.Ant]; ok && (e.Status == kit.CovReady || e.Status == kit.CovPartial) {
			need[e.Ant] = true
		}
	}
	for k, ok := range need {
		if !ok {
			t.Fatalf("missing coverage for %s", k)
		}
	}
	t.Logf("Ant coverage summary: %+v (entries=%d)", sum, len(entries))
}

func TestNewHostHeadless(t *testing.T) {
	h, err := platform.NewHost(platform.HostOptions{Width: 100, Height: 80, PreferHeadless: true})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	w, ht := h.Size()
	if w != 100 || ht != 80 {
		t.Fatalf("%dx%d", w, ht)
	}
	if platform.GPUPresentReady(h) {
		t.Fatal("headless should not be GPU-ready")
	}
}

func TestPlatformFactoryLinuxOrStub(t *testing.T) {
	// On Linux this opens X11 if DISPLAY set; may fail in pure CI — prefer headless path already tested.
	// Compile-only check for Windows/Darwin types:
	_ = platform.WindowsOptions{}
	_ = platform.DarwinOptions{}
}

// silence color import if unused on some go versions
var _ = color.RGBA{}
