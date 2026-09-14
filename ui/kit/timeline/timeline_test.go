package timeline_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/timeline"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
	"github.com/energye/gpui/ui/theme"
)

type timelineFile struct {
	Source  string `json:"source"`
	Metrics struct {
		DotSize                   float64 `json:"dotSize"`
		DotBorderWidth            float64 `json:"dotBorderWidth"`
		TailWidth                 float64 `json:"tailWidth"`
		ItemPaddingBottom         float64 `json:"itemPaddingBottom"`
		FontSize                  float64 `json:"fontSize"`
		TitleSpan                 float64 `json:"titleSpan"`
		CustomHeadPaddingVertical float64 `json:"customHeadPaddingVertical"`
	} `json:"metrics"`
	Colors []string `json:"colors"`
	Basic  []struct {
		Content string `json:"content"`
	} `json:"basic"`
	Layout struct {
		MaxWidth  float64 `json:"maxWidth"`
		MaxHeight float64 `json:"maxHeight"`
	} `json:"layout"`
}

func loadTimelineFile(t *testing.T) timelineFile {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "timeline.json"))
	if err != nil {
		t.Fatalf("read timeline.json: %v", err)
	}
	var f timelineFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("parse timeline.json: %v", err)
	}
	if f.Metrics.DotSize <= 0 || len(f.Colors) == 0 || len(f.Basic) == 0 {
		t.Fatal("timeline.json missing metrics/colors/basic")
	}
	return f
}

func layoutLoose(t *testing.T, tl *timeline.Timeline, max float64) rendering.Size {
	t.Helper()
	return tl.Layout(rendering.Loose(max, max))
}

func paintTimeline(tl *timeline.Timeline, w, h int) {
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	tl.Layout(rendering.Loose(float64(w), float64(h)))
	pc := rendering.NewPaintContext(dc, 1)
	tl.Node().Paint(pc)
}

func TestTimeline_PRD_TL01(t *testing.T) {
	tl := timeline.NewTimeline()
	if tl.ItemCount() != 0 {
		t.Fatalf("count=%d want 0", tl.ItemCount())
	}
	if tl.Mode() != timeline.TimelineModeStart {
		t.Fatalf("mode=%v want start", tl.Mode())
	}
	if tl.Orientation() != timeline.TimelineVertical {
		t.Fatalf("orientation=%v want vertical", tl.Orientation())
	}
	if tl.Variant() != timeline.TimelineOutlined {
		t.Fatalf("variant=%v want outlined", tl.Variant())
	}
	if tl.Reverse() {
		t.Fatal("reverse default false")
	}
	if tl.TitleSpan() != 12 {
		t.Fatalf("titleSpan=%v want 12", tl.TitleSpan())
	}
	if tl.Role() != "list" || tl.Focusable() {
		t.Fatalf("role=%q focusable=%v", tl.Role(), tl.Focusable())
	}
	if tl.HasLoadingSpinner() || tl.HasPending() {
		t.Fatal("no loading by default")
	}
	sz := layoutLoose(t, tl, 400)
	if sz.Width != 0 || sz.Height != 0 {
		t.Fatalf("empty size=%+v want 0", sz)
	}
	paintTimeline(tl, 64, 64)
}

func TestTimeline_PRD_TL02(t *testing.T) {
	tl := timeline.NewTimeline(
		timeline.TimelineItem{Content: "a"},
		timeline.TimelineItem{Content: "b"},
		timeline.TimelineItem{Content: "c"},
	)
	if tl.ItemCount() != 3 {
		t.Fatalf("count=%d want 3", tl.ItemCount())
	}
	if len(tl.DisplayItems()) != 3 {
		t.Fatal("display items should be 3")
	}
	sz := layoutLoose(t, tl, 400)
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("size=%+v want positive", sz)
	}
	paintTimeline(tl, 128, 128)
}

func TestTimeline_PRD_TL03(t *testing.T) {
	tl := timeline.NewTimeline(
		timeline.TimelineItem{Content: "a"},
		timeline.TimelineItem{Content: "b"},
		timeline.TimelineItem{Content: "c"},
	)
	tl.SetMode(timeline.TimelineModeAlternate)
	if tl.ItemPlacement(0) != timeline.TimelinePlacementStart {
		t.Fatalf("item0=%v want start", tl.ItemPlacement(0))
	}
	if tl.ItemPlacement(1) != timeline.TimelinePlacementEnd {
		t.Fatalf("item1=%v want end", tl.ItemPlacement(1))
	}
	if tl.ItemPlacement(2) != timeline.TimelinePlacementStart {
		t.Fatalf("item2=%v want start", tl.ItemPlacement(2))
	}
	// Placement override beats the alternate default.
	tl2 := timeline.NewTimeline(
		timeline.TimelineItem{Content: "a", Placement: timeline.TimelinePlacementEnd},
		timeline.TimelineItem{Content: "b"},
	)
	tl2.SetMode(timeline.TimelineModeAlternate)
	if tl2.ItemPlacement(0) != timeline.TimelinePlacementEnd {
		t.Fatalf("override=%v want end", tl2.ItemPlacement(0))
	}
	sz := layoutLoose(t, tl, 400)
	if sz.Width <= 0 {
		t.Fatalf("alternate width=%v", sz.Width)
	}
	paintTimeline(tl, 160, 160)
}

func TestTimeline_PRD_TL04(t *testing.T) {
	tl := timeline.NewTimeline(
		timeline.TimelineItem{Content: "a"},
		timeline.TimelineItem{Content: "b"},
		timeline.TimelineItem{Content: "Recording", Loading: true},
	)
	if !tl.ItemLoading(2) {
		t.Fatal("last item should load")
	}
	if tl.ItemLoading(0) {
		t.Fatal("first item should not load")
	}
	if !tl.HasLoadingSpinner() || !tl.HasPending() {
		t.Fatal("pending semantic needs spinner")
	}
	if !tl.ItemHasIcon(2) {
		t.Fatal("loading counts as custom point")
	}
	reg := &scheduler.TickerRegistry{}
	tl.AttachTicker(reg)
	if !reg.HasActive() {
		t.Fatal("ticker should register")
	}
	p0 := tl.Phase()
	tl.Tick(0.25)
	if tl.Phase() == p0 {
		t.Fatal("tick should advance phase")
	}
	if !tl.WantsFrame() {
		t.Fatal("loading should want frames")
	}
	tl.SetReduceMotion(true)
	if tl.WantsFrame() {
		t.Fatal("reduced motion stops frames")
	}
	tl.Detach()
	paintTimeline(tl, 128, 128)
}

func TestTimeline_PRD_TL05(t *testing.T) {
	tl := timeline.NewTimeline(
		timeline.TimelineItem{Content: "first"},
		timeline.TimelineItem{Content: "second"},
		timeline.TimelineItem{Content: "third"},
	)
	tl.SetReverse(true)
	if !tl.Reverse() {
		t.Fatal("reverse flag")
	}
	if tl.DisplayIndex(0) != 2 || tl.DisplayIndex(2) != 0 {
		t.Fatalf("displayIndex 0->%d 2->%d", tl.DisplayIndex(0), tl.DisplayIndex(2))
	}
	if tl.DisplayLogical(0) != 2 || tl.DisplayLogical(2) != 0 {
		t.Fatal("display logical mapping")
	}
	got := tl.DisplayItems()
	if got[0].Content != "third" || got[2].Content != "first" {
		t.Fatalf("display order %+v", got)
	}
	// Speech follows render order.
	ro := tl.ReadingOrder()
	if len(ro) != 3 || ro[0] != 2 || ro[2] != 0 {
		t.Fatalf("reading order %v", ro)
	}
	paintTimeline(tl, 128, 128)
}

func TestTimeline_PRD_TL06(t *testing.T) {
	f := loadTimelineFile(t)
	items := make([]timeline.TimelineItem, len(f.Colors))
	for i, c := range f.Colors {
		items[i] = timeline.TimelineItem{Content: "c", Color: c}
	}
	tl := timeline.NewTimeline(items...)
	tok := theme.Default.Current()
	toRGBA := func(c theme.Color) render.RGBA {
		return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
	}
	if got := tl.ItemColor(0); got != toRGBA(tok.ColorPrimary) {
		t.Fatalf("blue=%+v want primary %+v", got, toRGBA(tok.ColorPrimary))
	}
	if got := tl.ItemColor(1); got != toRGBA(tok.ColorError) {
		t.Fatalf("red=%+v", got)
	}
	if got := tl.ItemColor(2); got != toRGBA(tok.ColorSuccess) {
		t.Fatalf("green=%+v", got)
	}
	if got := tl.ItemColor(3); got != toRGBA(tok.ColorTextDisabled) {
		t.Fatalf("gray=%+v", got)
	}
	wantHex := render.Hex("#ff8800")
	if got := tl.ItemColor(4); got != wantHex {
		t.Fatalf("hex=%+v want %+v", got, wantHex)
	}
	// Empty color falls back to blue primary.
	tl2 := timeline.NewTimeline(timeline.TimelineItem{Content: "x"})
	if got := tl2.ItemColor(0); got != toRGBA(tok.ColorPrimary) {
		t.Fatalf("default=%+v", got)
	}
	paintTimeline(tl, 128, 128)
}

func TestTimeline_PRD_TL07(t *testing.T) {
	custom := rendering.NewRenderColorBox(12, 12, 0.2, 0.4, 0.8, 1)
	tl := timeline.NewTimeline(
		timeline.TimelineItem{Content: "icon-name", Icon: "smile"},
		timeline.TimelineItem{Content: "icon-node", IconNode: custom},
		timeline.TimelineItem{Content: "plain"},
	)
	if !tl.ItemHasIcon(0) || !tl.ItemHasIcon(1) {
		t.Fatal("custom points should report icon")
	}
	if tl.ItemHasIcon(2) {
		t.Fatal("plain dot is not custom")
	}
	sz := layoutLoose(t, tl, 400)
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("size=%+v", sz)
	}
	paintTimeline(tl, 128, 128)
}

func TestTimeline_PRD_TL08(t *testing.T) {
	f := loadTimelineFile(t)
	items := make([]timeline.TimelineItem, len(f.Basic))
	for i, b := range f.Basic {
		items[i] = timeline.TimelineItem{Content: b.Content}
	}
	tl := timeline.NewTimeline(items...)
	if tl.ItemCount() != 4 {
		t.Fatalf("basic count=%d want 4", tl.ItemCount())
	}
	sz := layoutLoose(t, tl, f.Layout.MaxWidth)
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("basic size=%+v", sz)
	}
	paintTimeline(tl, 200, 200)
}

func TestTimeline_PRD_TL09(t *testing.T) {
	mk := func(v timeline.TimelineVariant) *timeline.Timeline {
		tl := timeline.NewTimeline(timeline.TimelineItem{Content: "x", Color: "red"})
		tl.SetVariant(v)
		return tl
	}
	outlined, filled := mk(timeline.TimelineOutlined), mk(timeline.TimelineFilled)
	if outlined.Variant() != timeline.TimelineOutlined || filled.Variant() != timeline.TimelineFilled {
		t.Fatal("variant flag")
	}
	// Pixel proof: filled red dot center is red, outlined center stays light.
	checkCenter := func(tl *timeline.Timeline, wantRed bool) {
		t.Helper()
		const canvas = 64
		dc := render.NewContext(canvas, canvas)
		defer dc.Close()
		dc.BeginFrame()
		dc.ClearWithColor(render.White)
		tl.Layout(rendering.Loose(canvas, canvas))
		tl.Node().Paint(rendering.NewPaintContext(dc, 1))
		got := dc.Image()
		// Dot at rail center x=9, y=7 for single start-mode item.
		r, g, b, _ := got.At(9, 7).RGBA()
		isRed := r > 0xC000 && g < 0x6000 && b < 0x6000
		if wantRed && !isRed {
			t.Fatalf("filled center #%04x%04x%04x want red", r, g, b)
		}
		if !wantRed && isRed {
			t.Fatalf("outlined center #%04x%04x%04x want light", r, g, b)
		}
	}
	checkCenter(filled, true)
	checkCenter(outlined, false)
}

func TestTimeline_PRD_TL10(t *testing.T) {
	tl := timeline.NewTimeline(
		timeline.TimelineItem{Content: "a"},
		timeline.TimelineItem{Content: "b"},
		timeline.TimelineItem{Content: "Recording", Loading: true},
	)
	tl.SetReverse(true)
	if !tl.HasPending() {
		t.Fatal("pending demo needs spinner")
	}
	got := tl.DisplayItems()
	if got[0].Content != "Recording" {
		t.Fatalf("reversed pending first=%q", got[0].Content)
	}
	paintTimeline(tl, 128, 160)
}

func TestTimeline_PRD_TL11(t *testing.T) {
	tl := timeline.NewTimeline(
		timeline.TimelineItem{Content: "a"},
		timeline.TimelineItem{Content: "b"},
		timeline.TimelineItem{Content: "c"},
	)
	tl.SetMode(timeline.TimelineModeAlternate)
	single := timeline.NewTimeline(
		timeline.TimelineItem{Content: "a"},
		timeline.TimelineItem{Content: "b"},
	)
	singleW := layoutLoose(t, single, 400).Width
	altW := layoutLoose(t, tl, 400).Width
	if altW < singleW {
		t.Fatalf("alternate width=%v < single=%v", altW, singleW)
	}
	paintTimeline(tl, 200, 160)
}

func TestTimeline_PRD_TL12(t *testing.T) {
	tl := timeline.NewTimeline(
		timeline.TimelineItem{Content: "aaa"},
		timeline.TimelineItem{Content: "bbb"},
		timeline.TimelineItem{Content: "ccc"},
	)
	tl.SetOrientation(timeline.TimelineHorizontal)
	if tl.Orientation() != timeline.TimelineHorizontal {
		t.Fatal("orientation flag")
	}
	sz := layoutLoose(t, tl, 400)
	if sz.Width <= sz.Height {
		t.Fatalf("horizontal size=%+v want wide", sz)
	}
	// Placement picks above/below rail without crashing.
	tl2 := timeline.NewTimeline(
		timeline.TimelineItem{Content: "up", Placement: timeline.TimelinePlacementStart},
		timeline.TimelineItem{Content: "down", Placement: timeline.TimelinePlacementEnd},
	)
	tl2.SetOrientation(timeline.TimelineHorizontal)
	sz2 := layoutLoose(t, tl2, 400)
	if sz2.Height <= 0 {
		t.Fatalf("mixed size=%+v", sz2)
	}
	paintTimeline(tl, 240, 120)
	paintTimeline(tl2, 240, 160)
}

func TestTimeline_PRD_TL13(t *testing.T) {
	tl := timeline.NewTimeline(
		timeline.TimelineItem{Content: "a", Icon: "smile"},
		timeline.TimelineItem{Content: "b", Color: "green"},
	)
	if !tl.ItemHasIcon(0) {
		t.Fatal("custom dot should show")
	}
	paintTimeline(tl, 128, 128)
}

func TestTimeline_PRD_TL14(t *testing.T) {
	tl := timeline.NewTimeline(
		timeline.TimelineItem{Content: "a"},
		timeline.TimelineItem{Content: "b"},
	)
	tl.SetMode(timeline.TimelineModeEnd)
	if tl.ItemPlacement(0) != timeline.TimelinePlacementStart {
		t.Fatalf("end mode side=%v want start", tl.ItemPlacement(0))
	}
	tl.SetMode(timeline.TimelineModeStart)
	if tl.ItemPlacement(0) != timeline.TimelinePlacementEnd {
		t.Fatalf("start mode side=%v want end", tl.ItemPlacement(0))
	}
	paintTimeline(tl, 128, 128)
}

func TestTimeline_PRD_TL15(t *testing.T) {
	tl := timeline.NewTimeline(
		timeline.TimelineItem{Title: "2015-09-01", Content: "Create services site"},
		timeline.TimelineItem{Title: "2015-09-02", Content: "Solve network problems"},
	)
	if tl.ItemName(0) != "2015-09-01 Create services site" {
		t.Fatalf("name=%q", tl.ItemName(0))
	}
	plain := timeline.NewTimeline(timeline.TimelineItem{Content: "only"})
	if layoutLoose(t, tl, 400).Height <= layoutLoose(t, plain, 400).Height {
		t.Fatal("title should grow item height")
	}
	if tl.ItemRole(0) != "listitem" {
		t.Fatalf("item role=%q", tl.ItemRole(0))
	}
	paintTimeline(tl, 200, 160)
}

func TestTimeline_PRD_TL16(t *testing.T) {
	f := loadTimelineFile(t)
	tl := timeline.NewTimeline(timeline.TimelineItem{Content: "x"})
	eq := func(got, want float64, name string) {
		t.Helper()
		if math.Abs(got-want) > 0.5 {
			t.Fatalf("%s=%v want %v", name, got, want)
		}
	}
	eq(tl.DotSize(), f.Metrics.DotSize, "dotSize")
	eq(tl.TailWidth(), f.Metrics.TailWidth, "tailWidth")
	eq(tl.ItemPaddingBottom(), f.Metrics.ItemPaddingBottom, "itemPaddingBottom")
	eq(tl.FontSize(), f.Metrics.FontSize, "fontSize")
	eq(tl.TitleSpan(), f.Metrics.TitleSpan, "titleSpan")
	eq(tl.DotBorderWidth(), f.Metrics.DotBorderWidth, "dotBorderWidth")
	eq(tl.CustomHeadPaddingVertical(), f.Metrics.CustomHeadPaddingVertical, "customHeadPaddingVertical")
	// Theme Token wiring: bold line token drives the tail.
	tok := theme.Default.Current()
	eq(tok.LineWidthBold, f.Metrics.TailWidth, "theme LineWidthBold")
}

func TestTimeline_PRD_TL17(t *testing.T) {
	tl := timeline.NewTimeline(timeline.TimelineItem{Content: "x"})
	tok := theme.Default.Current()
	want := render.RGBA{R: tok.ColorPrimary.R, G: tok.ColorPrimary.G, B: tok.ColorPrimary.B, A: tok.ColorPrimary.A}
	if got := tl.ItemColor(0); got != want {
		t.Fatalf("default skin=%+v want Theme primary %+v", got, want)
	}
	// A Theme override must flow to dots (no hard-coded brand color).
	custom := tok
	custom.ColorPrimary = theme.Hex("#123456")
	tl.SetTheme(&custom)
	got := tl.ItemColor(0)
	if got.R == want.R && got.G == want.G && got.B == want.B {
		t.Fatal("Theme override did not change dot color")
	}
	if math.Abs(got.R-0x12/255.0) > 0.01 {
		t.Fatalf("custom primary=%+v", got)
	}
	tl.SetTheme(nil)
	if back := tl.ItemColor(0); back != want {
		t.Fatalf("clear Theme=%+v want %+v", back, want)
	}
	// Provider path also works.
	p := theme.NewProvider(theme.DefaultTokens())
	tl.SetProvider(p)
	if got := tl.ItemColor(0); got != want {
		t.Fatalf("provider color=%+v", got)
	}
}

func TestTimeline_LayoutMatrix(t *testing.T) {
	tl := timeline.NewTimeline(
		timeline.TimelineItem{Title: "t", Content: "content one"},
		timeline.TimelineItem{Content: "content two"},
	)
	exact := tl.Layout(rendering.Tight(300, 200))
	if exact.Width != 300 || exact.Height != 200 {
		t.Fatalf("exact=%+v want 300x200", exact)
	}
	loose := tl.Layout(rendering.Loose(500, 500))
	if loose.Width <= 0 || loose.Height <= 0 || loose.Width > 500 || loose.Height > 500 {
		t.Fatalf("loose=%+v", loose)
	}
	narrow := tl.Layout(rendering.Loose(80, 500))
	if narrow.Width > 80+0.5 {
		t.Fatalf("narrow width=%v exceeds 80", narrow.Width)
	}
	// Min constraints clamp small content up.
	minC := tl.Layout(rendering.Constraints{MinWidth: 200, MaxWidth: 500, MinHeight: 100, MaxHeight: 500})
	if minC.Width < 200 || minC.Height < 100 {
		t.Fatalf("min clamp=%+v", minC)
	}
}

func TestTimeline_DirtyLocality_PaintVisits(t *testing.T) {
	const vp = 240.0
	root := rendering.NewRenderBox()
	root.FixedWidth, root.FixedHeight = vp, vp
	tl := timeline.NewTimeline(
		timeline.TimelineItem{Content: "alpha one"},
		timeline.TimelineItem{Content: "beta two"},
		timeline.TimelineItem{Content: "gamma three"},
	)
	root.AddChild(tl.Node())
	for i := 0; i < 20; i++ {
		root.AddChild(rendering.NewRenderColorBox(4, 4, 0.3, 0.3, 0.3, 1))
	}
	owner := rendering.NewPipelineOwner(root)
	if !owner.FlushLayout(rendering.Size{Width: vp, Height: vp}, true) {
		t.Fatal("initial layout")
	}
	dc := render.NewContext(int(vp), int(vp))
	defer dc.Close()
	dc.BeginFrame()
	var visits int64
	owner.FlushPaint(&rendering.PaintContext{DC: dc, PaintVisits: &visits}, true)
	full := visits
	if full <= 0 {
		t.Fatal("full paint visited nothing")
	}
	layouts0 := owner.LayoutCount
	tl.SetItemColor(0, "red")
	visits = 0
	if !owner.FlushPaint(&rendering.PaintContext{DC: dc, PaintVisits: &visits}, false) {
		t.Fatal("expected partial paint after single item dirty")
	}
	if visits >= full/2 {
		t.Fatalf("PaintVisits=%d too high for single dirty item (full=%d)", visits, full)
	}
	if owner.LayoutCount != layouts0 {
		t.Fatalf("layout must not run on color-only dirty: %d -> %d", layouts0, owner.LayoutCount)
	}
}

func TestTimeline_A11y_RoleFocusAria(t *testing.T) {
	tl := timeline.NewTimeline(
		timeline.TimelineItem{Title: "day one", Content: "first event"},
		timeline.TimelineItem{Content: "second event"},
	)
	if tl.Role() != "list" {
		t.Fatalf("Role=%q want list", tl.Role())
	}
	if tl.ItemRole(0) != "listitem" || tl.ItemRole(1) != "listitem" {
		t.Fatal("each node needs listitem Role")
	}
	if tl.Focusable() {
		t.Fatal("body must not take Focus")
	}
	tl.SetAriaLabel("release history")
	if tl.AriaLabel() != "release history" {
		t.Fatalf("Aria label=%q", tl.AriaLabel())
	}
	before := tl.ReadingOrder()
	tl.SetReverse(true)
	after := tl.ReadingOrder()
	if len(before) != 2 || len(after) != 2 || before[0] == after[0] {
		t.Fatalf("reverse must flip speech order %v -> %v", before, after)
	}
	if tl.ItemName(0) != "day one first event" {
		t.Fatalf("name=%q", tl.ItemName(0))
	}
}
