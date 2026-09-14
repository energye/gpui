package timeline_test

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/kit/timeline"
	"github.com/energye/gpui/ui/rendering"
)

type timelineShowcaseSpec struct {
	CanvasW int     `json:"canvasW"`
	Margin  float64 `json:"margin"`
	Gap     float64 `json:"gap"`
	Tolerance struct {
		MaxDiff float64 `json:"maxDiff"`
		BadFrac float64 `json:"badFrac"`
		HardCap float64 `json:"hardCap"`
	} `json:"tolerance"`
}

func loadTimelineShowcaseSpec(t *testing.T) timelineShowcaseSpec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "showcase_spec.json"))
	if err != nil {
		t.Fatalf("read showcase_spec.json: %v", err)
	}
	var s timelineShowcaseSpec
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse showcase spec: %v", err)
	}
	if s.CanvasW <= 0 || s.Gap < 0 {
		t.Fatalf("bad showcase spec %+v", s)
	}
	return s
}

func loadTimelineShowcaseFace(t *testing.T) text.Face {
	t.Helper()
	mk := func(pts float64) text.Face {
		f, _, err := text.LoadMultiFace(pts)
		if err != nil || f == nil {
			f2, _, err2 := rendering.TryLoadDefaultFace(pts)
			if err2 != nil || f2 == nil {
				t.Skipf("showcase needs a system face for real glyphs: %v / %v", err, err2)
			}
			return f2
		}
		return f
	}
	return mk(14)
}

// TestTimeline_PRD_TL20_Showcase_MainPaths lays the §6.8 P0 official examples
// on one big canvas: basic / variant outlined+filled / pending+reverse /
// alternate / horizontal / custom dots / end mode / title rows.
// Three evidences: logic probe (mode/placement/color/loading/reverse/variant/
// orientation), pixel assertions (tail axis ink + text zone ink + filled dot
// center), golden file compare (tolerance from testdata/showcase_spec.json).
// Regenerate with UPDATE_GOLDEN=1.
func TestTimeline_PRD_TL20_Showcase_MainPaths(t *testing.T) {
	spec := loadTimelineShowcaseSpec(t)
	face := loadTimelineShowcaseFace(t)

	W := float64(spec.CanvasW)
	margin, gap := spec.Margin, spec.Gap
	rowW := W - 2*margin

	withFace := func(tl *timeline.Timeline) *timeline.Timeline {
		tl.SetFace(face)
		return tl
	}

	// R1 basic (basic.tsx): 4 plain contents, default start/outlined/blue.
	basic := withFace(timeline.NewTimeline(
		timeline.TimelineItem{Content: "Create a services site"},
		timeline.TimelineItem{Content: "Solve initial network problems"},
		timeline.TimelineItem{Content: "Technical testing"},
		timeline.TimelineItem{Content: "Network problems solved"},
	))

	// R2+R3 variant (variant.tsx): outlined hollow vs filled solid dots.
	outlined := withFace(timeline.NewTimeline(
		timeline.TimelineItem{Content: "Outlined hollow dot", Color: "red"},
		timeline.TimelineItem{Content: "Outlined second", Color: "red"},
	))
	outlined.SetVariant(timeline.TimelineOutlined)
	filled := withFace(timeline.NewTimeline(
		timeline.TimelineItem{Content: "Filled solid dot", Color: "red"},
		timeline.TimelineItem{Content: "Filled second", Color: "red"},
	))
	filled.SetVariant(timeline.TimelineFilled)

	// R4+R5 pending + reverse (pending.tsx): trailing loading node, then
	// the same list flipped so the spinner leads.
	pending := withFace(timeline.NewTimeline(
		timeline.TimelineItem{Content: "Create a services site"},
		timeline.TimelineItem{Content: "Solve initial network problems"},
		timeline.TimelineItem{Content: "Recording", Loading: true},
	))
	reversed := withFace(timeline.NewTimeline(
		timeline.TimelineItem{Content: "Create a services site"},
		timeline.TimelineItem{Content: "Solve initial network problems"},
		timeline.TimelineItem{Content: "Recording", Loading: true},
	))
	reversed.SetReverse(true)

	// R6 alternate (alternate.tsx): content zig-zags across the rail.
	alternate := withFace(timeline.NewTimeline(
		timeline.TimelineItem{Title: "2015-09-01", Content: "Create a services site", Color: "green"},
		timeline.TimelineItem{Title: "2015-09-02", Content: "Solve network problems", Color: "green"},
		timeline.TimelineItem{Title: "2015-09-03", Content: "Technical testing", Color: "green"},
		timeline.TimelineItem{Title: "2015-09-04", Content: "Network problems solved", Color: "green"},
	))
	alternate.SetMode(timeline.TimelineModeAlternate)

	// R7 horizontal (horizontal.tsx): rail runs left to right.
	horizontal := withFace(timeline.NewTimeline(
		timeline.TimelineItem{Content: "Step one", Color: "blue"},
		timeline.TimelineItem{Content: "Step two", Color: "green"},
		timeline.TimelineItem{Content: "Step three", Color: "gray"},
	))
	horizontal.SetOrientation(timeline.TimelineHorizontal)

	// R8 custom (custom.tsx): icon dots replace the default circles.
	customIcon := rendering.NewRenderColorBox(12, 12, 0.2, 0.4, 0.8, 1)
	custom := withFace(timeline.NewTimeline(
		timeline.TimelineItem{Content: "Named icon dot", Icon: "smile", Color: "blue"},
		timeline.TimelineItem{Content: "Node icon dot", IconNode: customIcon, Color: "green"},
		timeline.TimelineItem{Content: "Plain dot", Color: "gray"},
	))

	// R9 end mode (end.tsx): rail on the right, content on the left.
	endMode := withFace(timeline.NewTimeline(
		timeline.TimelineItem{Content: "Right rail one", Color: "blue"},
		timeline.TimelineItem{Content: "Right rail two", Color: "#ff8800"},
	))
	endMode.SetMode(timeline.TimelineModeEnd)

	// R10 title (title.tsx): time labels above each content.
	titled := withFace(timeline.NewTimeline(
		timeline.TimelineItem{Title: "2015-09-01", Content: "Create services site"},
		timeline.TimelineItem{Title: "2015-09-02", Content: "Solve network problems"},
		timeline.TimelineItem{Title: "2015-09-03", Content: "Technical testing"},
	))

	rows := []*timeline.Timeline{basic, outlined, filled, pending, reversed,
		alternate, horizontal, custom, endMode, titled}

	// Logic probe before paint: every §6.8 P0 axis in one place.
	if basic.ItemCount() != 4 || basic.Mode() != timeline.TimelineModeStart {
		t.Fatal("basic probe")
	}
	if outlined.Variant() != timeline.TimelineOutlined || filled.Variant() != timeline.TimelineFilled {
		t.Fatal("variant probe")
	}
	if !pending.HasPending() || !pending.ItemLoading(2) || pending.ItemLoading(0) {
		t.Fatal("pending probe")
	}
	if got := reversed.DisplayItems(); len(got) != 3 || got[0].Content != "Recording" {
		t.Fatalf("reverse probe %+v", got)
	}
	if alternate.ItemPlacement(0) != timeline.TimelinePlacementStart ||
		alternate.ItemPlacement(1) != timeline.TimelinePlacementEnd {
		t.Fatal("alternate probe")
	}
	if horizontal.Orientation() != timeline.TimelineHorizontal {
		t.Fatal("horizontal probe")
	}
	if !custom.ItemHasIcon(0) || !custom.ItemHasIcon(1) || custom.ItemHasIcon(2) {
		t.Fatal("custom probe")
	}
	if endMode.ItemPlacement(0) != timeline.TimelinePlacementStart {
		t.Fatal("end mode probe")
	}
	if titled.ItemName(0) != "2015-09-01 Create services site" {
		t.Fatalf("title probe %q", titled.ItemName(0))
	}

	// Layout every row; Layout/Node must stay non-zero (true-size chain).
	type placed struct {
		tl *timeline.Timeline
		x  float64
		y  float64
		w  float64
		h  float64
	}
	var items []placed
	y := margin
	for i, tl := range rows {
		sz := tl.Layout(rendering.Loose(rowW, 4000))
		if sz.Width <= 0 || sz.Height <= 0 || sz.Width > rowW+0.5 {
			t.Fatalf("row %d layout=%v want positive within %v", i, sz, rowW)
		}
		if tl.Node() == nil || tl.ChromeNode() == nil {
			t.Fatalf("row %d node nil", i)
		}
		if ns := tl.Node().Size(); ns.Width <= 0 || ns.Height <= 0 {
			t.Fatalf("row %d node size=%v want >0", i, ns)
		}
		items = append(items, placed{tl: tl, x: margin, y: y, w: sz.Width, h: sz.Height})
		y += sz.Height + gap
	}
	H := y - gap + margin

	dc := render.NewContext(int(W), int(H))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	for _, it := range items {
		it.tl.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(it.x, it.y))
	}
	got := dc.Image()

	// Pixel assertion 1: the basic rail carries tail axis ink (not blank).
	// Single-side start mode pins the rail at x=0..18, tail center x=9.
	b0 := items[0]
	tailInk := 0
	for yy := int(b0.y + 14); yy < int(b0.y+b0.h-4); yy++ {
		r, g, b, _ := got.At(int(b0.x+9), yy).RGBA()
		if r/257 < 250 || g/257 < 250 || b/257 < 250 {
			tailInk++
		}
	}
	if tailInk < 30 {
		t.Fatalf("basic tail ink pixels=%d want >=30 (axis missing?)", tailInk)
	}

	// Pixel assertion 2: the basic text zone carries dark glyph ink.
	dark := 0
	tx0, tx1 := int(b0.x+26), int(b0.x+b0.w-4)
	for yy := int(b0.y + 2); yy < int(b0.y+b0.h-2); yy++ {
		for xx := tx0; xx < tx1; xx++ {
			r, g, b, _ := got.At(xx, yy).RGBA()
			if r/257 < 110 && g/257 < 110 && b/257 < 110 {
				dark++
			}
		}
	}
	if dark < 30 {
		t.Fatalf("basic text dark pixels=%d want >=30 (real glyphs missing?)", dark)
	}

	// Pixel assertion 3: filled red dot center is red, outlined stays light.
	// First dot of a single-side row sits at rail center (x=+9, y=+7).
	dotCenter := func(it placed) (uint32, uint32, uint32) {
		r, g, b, _ := got.At(int(it.x+9), int(it.y+7)).RGBA()
		return r / 257, g / 257, b / 257
	}
	fr, fg, fb := dotCenter(items[2])
	if !(fr > 192 && fg < 96 && fb < 96) {
		t.Fatalf("filled dot center #%02x%02x%02x want red", fr, fg, fb)
	}
	or, og, ob := dotCenter(items[1])
	if or > 192 && og < 96 && ob < 96 {
		t.Fatalf("outlined dot center #%02x%02x%02x want light hollow", or, og, ob)
	}

	path := filepath.Join("testdata", "showcase_timeline.png")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("write showcase: %v", err)
		}
		if err := png.Encode(f, got); err != nil {
			f.Close()
			t.Fatalf("encode showcase: %v", err)
		}
		f.Close()
		t.Logf("showcase rewritten: %s (%dx%d)", path, int(W), int(H))
		return
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open showcase %s (run once with UPDATE_GOLDEN=1): %v", path, err)
	}
	want, err := png.Decode(f)
	f.Close()
	if err != nil {
		t.Fatalf("decode showcase: %v", err)
	}
	if !want.Bounds().Eq(got.Bounds()) {
		t.Fatalf("showcase bounds %v want %v (spec drift? regen with UPDATE_GOLDEN=1)", got.Bounds(), want.Bounds())
	}
	maxDiff := uint32(spec.Tolerance.MaxDiff * 257)
	hardCap := uint32(spec.Tolerance.HardCap * 257)
	bad := 0
	total := got.Bounds().Dx() * got.Bounds().Dy()
	for yy := 0; yy < got.Bounds().Dy(); yy++ {
		for xx := 0; xx < got.Bounds().Dx(); xx++ {
			r1, g1, b1, a1 := got.At(xx, yy).RGBA()
			r2, g2, b2, a2 := want.At(xx, yy).RGBA()
			m := max4tl(diffTl(r1, r2), diffTl(g1, g2), diffTl(b1, b2), diffTl(a1, a2))
			if m > hardCap {
				t.Fatalf("showcase pixel (%d,%d) diff %d exceeds hard cap", xx, yy, m/257)
			}
			if m > maxDiff {
				bad++
			}
		}
	}
	if float64(bad)/float64(total) > spec.Tolerance.BadFrac {
		t.Fatalf("showcase bad pixels %d/%d exceed %.1f%%", bad, total, spec.Tolerance.BadFrac*100)
	}
}

func diffTl(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func max4tl(vs ...uint32) uint32 {
	m := vs[0]
	for _, v := range vs[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
