package grid_test

import (
	"encoding/json"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/grid"
	"github.com/energye/gpui/ui/rendering"
)

type showcaseGridSpec struct {
	CanvasW int     `json:"canvasW"`
	Margin  float64 `json:"margin"`
	RowGap  float64 `json:"rowGap"`
	BlockH  float64 `json:"blockH"`
	SmallH  float64 `json:"smallH"`
	ShortH  float64 `json:"shortH"`
	TallH   float64 `json:"tallH"`
	Gutter  float64 `json:"gutter"`
	GutterV float64 `json:"gutterV"`
	Tolerance struct {
		MaxDiff float64 `json:"maxDiff"`
		BadFrac float64 `json:"badFrac"`
		HardCap float64 `json:"hardCap"`
	} `json:"tolerance"`
}

func loadShowcaseGridSpec(t *testing.T) showcaseGridSpec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "showcase_grid.json"))
	if err != nil {
		t.Fatalf("read showcase_grid.json: %v", err)
	}
	var s showcaseGridSpec
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse showcase spec: %v", err)
	}
	if s.CanvasW <= 0 || s.Tolerance.MaxDiff <= 0 || s.Tolerance.HardCap <= 0 {
		t.Fatalf("bad showcase spec %+v", s)
	}
	return s
}

func gbox(w, h, r, g, b float64) *rendering.RenderColorBox {
	return rendering.NewRenderColorBox(w, h, r, g, b, 1)
}

// TestGrid_Showcase_MainPaths lays §6.8 P0 main paths on one big canvas:
// basic 12+12 / thirds 8x3 / gutter 16 / gutter HV / offset 6 /
// push+pull sort / justify center+space-between / align middle /
// order / flex fill / wrap / responsive xs24->md12 / hidden span0.
// Grid is a no-chrome layout box, so every cell is a solid color block
// with a real size; geometry (not text) carries the signal.
// Three evidences: logic probe (widths/offsets/gaps), pixel assertions
// (block colors + white gaps), golden file compare (tolerance from
// testdata/showcase_grid.json). Regenerate with UPDATE_GOLDEN=1.
func TestGrid_Showcase_MainPaths(t *testing.T) {
	spec := loadShowcaseGridSpec(t)
	W := float64(spec.CanvasW)
	margin, rowGap := spec.Margin, spec.RowGap
	blockH, smallH := spec.BlockH, spec.SmallH
	shortH, tallH := spec.ShortH, spec.TallH
	rowW := W - 2*margin
	const tol = 0.5

	// Basic 12+12 (basic.tsx) at half canvas.
	const basicW = 560.0
	basic := grid.NewRow(
		grid.NewCol(gbox(60, blockH, 0.09, 0.47, 1)),
		grid.NewCol(gbox(60, blockH, 1, 0.3, 0.2)),
	)
	basic.Cols()[0].SetSpan(12)
	basic.Cols()[1].SetSpan(12)

	// Thirds 8x3 (basic.tsx) at half canvas.
	thirds := grid.NewRow(
		grid.NewCol(gbox(40, smallH, 0.09, 0.47, 1)),
		grid.NewCol(gbox(40, smallH, 0.2, 0.7, 0.3)),
		grid.NewCol(gbox(40, smallH, 1, 0.3, 0.2)),
	)
	for _, c := range thirds.Cols() {
		c.SetSpan(8)
	}

	// Gutter 16 (gutter.tsx): halves stay adjacent, contents inset 8/side.
	gutter := grid.NewRow(
		grid.NewCol(gbox(40, smallH, 0.09, 0.47, 1)),
		grid.NewCol(gbox(40, smallH, 1, 0.3, 0.2)),
	)
	gutter.Cols()[0].SetSpan(12)
	gutter.Cols()[1].SetSpan(12)
	gutter.SetGutter(spec.Gutter)

	// Gutter HV: 12+12 x2 wrapped, vertical gap gutterV.
	gutterHV := grid.NewRow(
		grid.NewCol(gbox(40, smallH, 0.09, 0.47, 1)),
		grid.NewCol(gbox(40, smallH, 0.2, 0.7, 0.3)),
		grid.NewCol(gbox(40, smallH, 1, 0.3, 0.2)),
		grid.NewCol(gbox(40, smallH, 0.55, 0.3, 0.85)),
	)
	for _, c := range gutterHV.Cols() {
		c.SetSpan(12)
	}
	gutterHV.SetGutterHV(spec.Gutter, spec.GutterV)

	// Offset 6+12 (offset.tsx) at half canvas.
	offset := grid.NewRow(grid.NewCol(gbox(40, smallH, 0.2, 0.4, 0.9)))
	offset.Cols()[0].SetSpan(12)
	offset.Cols()[0].SetOffset(6)

	// Push/pull sort (sort.tsx): visuals swap, widths stay.
	sort := grid.NewRow(
		grid.NewCol(gbox(40, smallH, 0.09, 0.47, 1)),
		grid.NewCol(gbox(40, smallH, 1, 0.3, 0.2)),
	)
	sort.Cols()[0].SetSpan(12)
	sort.Cols()[1].SetSpan(12)
	sort.Cols()[0].SetPush(12)
	sort.Cols()[1].SetPull(12)

	// Justify center (flex.tsx): 6+6 grouped in the middle.
	center := grid.NewRow(
		grid.NewCol(gbox(40, smallH, 0.09, 0.47, 1)),
		grid.NewCol(gbox(40, smallH, 1, 0.3, 0.2)),
	)
	center.Cols()[0].SetSpan(6)
	center.Cols()[1].SetSpan(6)
	center.SetJustify(grid.RowJustifyCenter)

	// Justify space-between (flex.tsx): 6+6 touch both edges.
	between := grid.NewRow(
		grid.NewCol(gbox(40, smallH, 0.09, 0.47, 1)),
		grid.NewCol(gbox(40, smallH, 1, 0.3, 0.2)),
	)
	between.Cols()[0].SetSpan(6)
	between.Cols()[1].SetSpan(6)
	between.SetJustify(grid.RowJustifySpaceBetween)

	// Align middle (flex-align.tsx): short 20 + tall 44.
	align := grid.NewRow(
		grid.NewCol(gbox(40, shortH, 0.2, 0.7, 0.3)),
		grid.NewCol(gbox(40, tallH, 0.55, 0.3, 0.85)),
	)
	align.Cols()[0].SetSpan(12)
	align.Cols()[1].SetSpan(12)
	align.SetAlign(grid.RowAlignMiddle)

	// Order (flex-order.tsx): larger order goes later.
	orderA := grid.NewCol(gbox(40, smallH, 0.09, 0.47, 1))
	orderB := grid.NewCol(gbox(40, smallH, 1, 0.3, 0.2))
	order := grid.NewRow(orderA, orderB)
	order.Cols()[0].SetSpan(12)
	order.Cols()[1].SetSpan(12)
	order.Cols()[0].SetOrder(2)
	order.Cols()[1].SetOrder(1)

	// Flex fill (flex-stretch.tsx): span6 + auto takes the remainder.
	fillFixed := grid.NewCol(gbox(40, smallH, 0.95, 0.55, 0.15))
	fillFixed.SetSpan(6)
	fillAuto := grid.NewCol(gbox(40, smallH, 0.15, 0.65, 0.65))
	fillAuto.SetFlexAuto()
	stretch := grid.NewRow(fillFixed, fillAuto)

	// Wrap=false (GRD-07): 3x half overflow one line.
	wrapA := grid.NewCol(gbox(40, smallH, 0.09, 0.47, 1))
	wrapB := grid.NewCol(gbox(40, smallH, 0.2, 0.7, 0.3))
	wrapC := grid.NewCol(gbox(40, smallH, 1, 0.3, 0.2))
	wrap := grid.NewRow(wrapA, wrapB, wrapC)
	for _, c := range wrap.Cols() {
		c.SetSpan(12)
	}
	wrap.SetWrap(false)

	// Responsive xs24/md12 (GRD-06) painted at 500 -> full width.
	resp := grid.NewCol(gbox(40, smallH, 0.09, 0.47, 1))
	resp.SetXs(24)
	resp.SetMd(12)
	respRow := grid.NewRow(resp)
	respRow.SetViewportWidth(500)

	// Hidden span0 + 24 sibling (display:none). Both hold color-free /
	// colored content in this row; the verified contract is that the
	// 24-span sibling keeps the full row (w=600) while the hidden column
	// contributes no visible block (painted red only from the sibling).
	hid, vis := grid.NewCol(), grid.NewCol(gbox(40, smallH, 1, 0.3, 0.2))
	hid.SetSpan(0)
	vis.SetSpan(24)
	hidden := grid.NewRow(hid, vis)
	hiddenH := smallH

	type placed struct {
		r *grid.Row
		x float64
		y float64
		w float64
		h float64
		c rendering.Constraints
	}
	var items []placed
	y := margin
	put := func(r *grid.Row, c rendering.Constraints) placed {
		sz := r.Layout(c)
		if sz.Width <= 0 || sz.Height <= 0 {
			t.Fatalf("showcase layout=%v want non-zero", sz)
		}
		if ns := r.Node().Size(); ns.Width <= 0 || ns.Height <= 0 {
			t.Fatalf("showcase node=%v want non-zero", ns)
		}
		for i, col := range r.Cols() {
			if col.Span() == 0 {
				continue
			}
			if cs := col.Node().Size(); cs.Width <= 0 || cs.Height <= 0 {
				t.Fatalf("showcase col %d size=%v want non-zero", i, cs)
			}
		}
		p := placed{r: r, x: margin, y: y, w: sz.Width, h: sz.Height, c: c}
		items = append(items, p)
		y += sz.Height + rowGap
		if got := r.Size(); got.Width <= 0 || got.Height <= 0 {
			t.Fatalf("showcase Size()=%v want non-zero", got)
		}
		return p
	}

	basicP := put(basic, rendering.Tight(basicW, blockH))
	thirdsP := put(thirds, rendering.Tight(basicW, smallH))
	gutterP := put(gutter, rendering.Tight(basicW, smallH))
	gutterHVP := put(gutterHV, rendering.Tight(basicW, smallH*2+spec.GutterV))
	offsetP := put(offset, rendering.Tight(basicW, smallH))
	sortP := put(sort, rendering.Tight(basicW, smallH))
	centerP := put(center, rendering.Tight(basicW, smallH))
	betweenP := put(between, rendering.Tight(basicW, smallH))
	alignP := put(align, rendering.Tight(basicW, tallH))
	orderP := put(order, rendering.Tight(600, smallH))
	stretchP := put(stretch, rendering.Tight(600, smallH))
	wrapP := put(wrap, rendering.Tight(600, smallH))
	respP := put(respRow, rendering.Tight(600, smallH))
	hiddenP := put(hidden, rendering.Tight(600, hiddenH))
	_ = rowW
	H := y - rowGap + margin

	// Logic probe: basic halves 280 each at 560.
	bc := basic.Cols()
	if math.Abs(bc[0].Node().Size().Width-280) > tol || math.Abs(bc[1].Node().Size().Width-280) > tol {
		t.Fatalf("basic widths %v/%v want 280/280", bc[0].Node().Size().Width, bc[1].Node().Size().Width)
	}
	// Thirds split visibly: widths equal and pairwise distinct X.
	tc := thirds.Cols()
	if math.Abs(tc[0].Node().Size().Width-tc[1].Node().Size().Width) > tol {
		t.Fatalf("thirds widths %v/%v want equal", tc[0].Node().Size().Width, tc[1].Node().Size().Width)
	}
	if !(tc[0].Node().Offset().X < tc[1].Node().Offset().X && tc[1].Node().Offset().X < tc[2].Node().Offset().X) {
		t.Fatalf("thirds order %+v", []float64{tc[0].Node().Offset().X, tc[1].Node().Offset().X, tc[2].Node().Offset().X})
	}
	// Gutter: outer boxes abut, child pads prove half-gutter.
	gc := gutter.Cols()
	if math.Abs(gc[1].Node().Offset().X-(gc[0].Node().Offset().X+gc[0].Node().Size().Width)) > tol {
		t.Fatalf("gutter outer gap")
	}
	if math.Abs(gc[0].Node().Children()[0].Offset().X-spec.Gutter/2) > tol {
		t.Fatalf("gutter pad=%v want %v", gc[0].Node().Children()[0].Offset().X, spec.Gutter/2)
	}
	gox0 := gc[0].Node().Offset().X + gc[0].Node().Size().Width - spec.Gutter/2
	gox1 := gc[1].Node().Offset().X + spec.Gutter/2
	if math.Abs(gox1-gox0-spec.Gutter) > tol {
		t.Fatalf("gutter content gap=%v want %v", gox1-gox0, spec.Gutter)
	}
	// GutterHV vertical gap: second line starts smallH+gutterV below first.
	hvc := gutterHV.Cols()
	if math.Abs((hvc[2].Node().Offset().Y-hvc[0].Node().Offset().Y)-smallH-spec.GutterV) > tol {
		t.Fatalf("gutterV gap=%v want %v", hvc[2].Node().Offset().Y-hvc[0].Node().Offset().Y, smallH+spec.GutterV)
	}
	// Offset: left gap 6/24 of 560 = 140.
	if math.Abs(offset.Cols()[0].Node().Offset().X-140) > tol {
		t.Fatalf("offset x=%v want 140", offset.Cols()[0].Node().Offset().X)
	}
	// Push/pull: visuals swap, widths stay.
	sc := sort.Cols()
	if math.Abs(sc[0].Node().Size().Width-280) > tol || math.Abs(sc[1].Node().Size().Width-280) > tol {
		t.Fatalf("sort widths")
	}
	if math.Abs(sc[0].Node().Offset().X-280) > tol || math.Abs(sc[1].Node().Offset().X) > tol {
		t.Fatalf("sort swapped ax=%v bx=%v", sc[0].Node().Offset().X, sc[1].Node().Offset().X)
	}
	// Justify center: first 6-span starts at 140 in 560.
	if math.Abs(center.Cols()[0].Node().Offset().X-140) > tol {
		t.Fatalf("center x=%v want 140", center.Cols()[0].Node().Offset().X)
	}
	// Justify between: touches both edges of 560.
	btw := between.Cols()
	if math.Abs(btw[0].Node().Offset().X) > tol {
		t.Fatalf("between left=%v", btw[0].Node().Offset().X)
	}
	if math.Abs((btw[1].Node().Offset().X+btw[1].Node().Size().Width)-basicW) > tol {
		t.Fatalf("between right=%v", btw[1].Node().Offset().X+btw[1].Node().Size().Width)
	}
	// Align middle: short column sits 12 below the tall top ((44-20)/2).
	ac := align.Cols()
	if math.Abs(ac[0].Node().Offset().Y-12) > tol || math.Abs(ac[1].Node().Offset().Y) > tol {
		t.Fatalf("align ay=%v by=%v want 12/0", ac[0].Node().Offset().Y, ac[1].Node().Offset().Y)
	}
	// Order: order1 column comes first.
	oc := order.Cols()
	if !(oc[1].Node().Offset().X < oc[0].Node().Offset().X) {
		t.Fatalf("order bx=%v ax=%v want b first", oc[1].Node().Offset().X, oc[0].Node().Offset().X)
	}
	// Flex fill: 600 - 150 = 450.
	if math.Abs(fillAuto.Node().Size().Width-450) > tol {
		t.Fatalf("fill=%v want 450", fillAuto.Node().Size().Width)
	}
	// Wrap=false: single line, right edge overflows 600.
	wc := wrap.Cols()
	for i, c := range wc {
		if math.Abs(c.Node().Offset().Y) > tol {
			t.Fatalf("wrap col%d y=%v want 0", i, c.Node().Offset().Y)
		}
	}
	if last := wc[2]; last.Node().Offset().X+last.Node().Size().Width <= 600+tol {
		t.Fatalf("wrap right=%v should exceed 600", last.Node().Offset().X+last.Node().Size().Width)
	}
	// Responsive at 500: full 600 width.
	if math.Abs(resp.Node().Size().Width-600) > tol {
		t.Fatalf("responsive width=%v want 600", resp.Node().Size().Width)
	}
	// Hidden: sibling takes the full row. The hidden column paints
	// nothing visible; geometry keeps the row line (engine behavior),
	// so only the sibling width/slot is asserted, not a zero height.
	if math.Abs(vis.Node().Size().Width-600) > tol || math.Abs(vis.Node().Offset().X) > tol {
		t.Fatalf("hidden sibling w=%v x=%v want 600/0", vis.Node().Size().Width, vis.Node().Offset().X)
	}

	dc := render.NewContext(int(W), int(H))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	for _, it := range items {
		it.r.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(it.x, it.y))
	}
	got := dc.Image()

	at := func(x, y float64) (uint32, uint32, uint32) {
		r, g, b, _ := got.At(int(x), int(y)).RGBA()
		return r / 257, g / 257, b / 257
	}
	isWhite := func(r, g, b uint32) bool { return r > 240 && g > 240 && b > 240 }

	// Pixel 1: basic left blue, right red. Content boxes keep their
	// intrinsic 60px at the column start (no stretch), so probe near
	// the column origins: left blue at +30, right red at 280+30.
	r, g, b := at(basicP.x+30, basicP.y+blockH/2)
	if !(b > 150 && r < 110) {
		t.Fatalf("basic blue #%02x%02x%02x want blue", r, g, b)
	}
	r, g, b = at(basicP.x+280+30, basicP.y+blockH/2)
	if !(r > 180 && g < 120 && b < 120) {
		t.Fatalf("basic red #%02x%02x%02x want red", r, g, b)
	}

	// Pixel 2: thirds columns visibly distinct (blue/green/red).
	// Content boxes sit at each column origin (intrinsic 40px).
	tw := thirds.Cols()[0].Node().Size().Width
	r0, g0, b0 := at(thirdsP.x+20, thirdsP.y+smallH/2)
	r1, g1, b1 := at(thirdsP.x+tw+20, thirdsP.y+smallH/2)
	r2, g2, b2 := at(thirdsP.x+2*tw+20, thirdsP.y+smallH/2)
	if !(b0 > 150 && r0 < 110) {
		t.Fatalf("thirds blue #%02x%02x%02x", r0, g0, b0)
	}
	if !(g1 > 140 && r1 < 110) {
		t.Fatalf("thirds green #%02x%02x%02x", r1, g1, b1)
	}
	if !(r2 > 180) {
		t.Fatalf("thirds red #%02x%02x%02x", r2, g2, b2)
	}

	// Pixel 3: gutter white slot between the two content boxes.
	gox := gutterP.x + gc[0].Node().Offset().X + gc[0].Node().Size().Width - spec.Gutter/2
	r, g, b = at(gox+spec.Gutter/2, gutterP.y+smallH/2)
	if !isWhite(r, g, b) {
		t.Fatalf("gutter gap #%02x%02x%02x want white", r, g, b)
	}

	// Pixel 4: gutterV band white across the row width.
	r, g, b = at(gutterHVP.x+basicW/2, gutterHVP.y+smallH+spec.GutterV/2)
	if !isWhite(r, g, b) {
		t.Fatalf("gutterV gap #%02x%02x%02x want white", r, g, b)
	}

	// Pixel 5: offset leaves white left of the block. Content box sits
	// at the column origin (offset+8 pad): white at +70, blue at 140+8+20.
	r, g, b = at(offsetP.x+70, offsetP.y+smallH/2)
	if !isWhite(r, g, b) {
		t.Fatalf("offset gap #%02x%02x%02x want white", r, g, b)
	}
	r, g, b = at(offsetP.x+140+8+20, offsetP.y+smallH/2)
	if !(b > 150 && r < 110) {
		t.Fatalf("offset block #%02x%02x%02x want blue", r, g, b)
	}

	// Pixel 6: push/pull swap: pulled red column sits at x=0, pushed
	// blue column at x=280; each paints its 40px box at its own origin.
	r, g, b = at(sortP.x+20, sortP.y+smallH/2)
	if !(r > 180 && g < 120 && b < 120) {
		t.Fatalf("sort left #%02x%02x%02x want red (pulled to 0)", r, g, b)
	}
	r, g, b = at(sortP.x+280+20, sortP.y+smallH/2)
	if !(b > 150 && r < 110) {
		t.Fatalf("sort right #%02x%02x%02x want blue (pushed to 280)", r, g, b)
	}

	// Pixel 7: center row margins white, first block blue at 140+20.
	r, g, b = at(centerP.x+70, centerP.y+smallH/2)
	if !isWhite(r, g, b) {
		t.Fatalf("center margin #%02x%02x%02x want white", r, g, b)
	}
	r, g, b = at(centerP.x+140+20, centerP.y+smallH/2)
	if !(b > 150 && r < 110) {
		t.Fatalf("center block #%02x%02x%02x want blue", r, g, b)
	}

	// Pixel 8: between row middle white, edges colored.
	r, g, b = at(betweenP.x+basicW/2, betweenP.y+smallH/2)
	if !isWhite(r, g, b) {
		t.Fatalf("between middle #%02x%02x%02x want white", r, g, b)
	}

	// Pixel 9: align short green at its column origin +20, mid-height.
	ay := align.Cols()[0].Node().Offset().Y
	ax := align.Cols()[0].Node().Offset().X
	r, g, b = at(alignP.x+ax+20, alignP.y+ay+shortH/2)
	if !(g > 140 && r < 110) {
		t.Fatalf("align short #%02x%02x%02x want green", r, g, b)
	}

	// Pixel 10: stretch fill block teal at its column origin +20.
	// fillFixed holds an orange box at x=0; fillAuto teal follows.
	fx := stretch.Cols()[1].Node().Offset().X
	r, g, b = at(stretchP.x+fx+20, stretchP.y+smallH/2)
	if !(g > 140 && b > 120 && r < 110) {
		t.Fatalf("fill #%02x%02x%02x want teal", r, g, b)
	}
	// Pixel 10b: fixed orange block at x=0+20 proves the 6-span slot.
	r, g, b = at(stretchP.x+20, stretchP.y+smallH/2)
	if !(r > 220 && g > 110 && g < 170 && b < 90) {
		t.Fatalf("fixed #%02x%02x%02x want orange", r, g, b)
	}

	// Pixel 11: responsive full-width block blue at x=20; hidden
	// sibling red at x=20 (both rows paint from the column origin).
	r, g, b = at(respP.x+20, respP.y+smallH/2)
	if !(b > 150 && r < 110) {
		t.Fatalf("responsive #%02x%02x%02x want blue", r, g, b)
	}
	r, g, b = at(hiddenP.x+20, hiddenP.y+smallH/2)
	if !(r > 180 && g < 120 && b < 120) {
		t.Fatalf("hidden sibling #%02x%02x%02x want red", r, g, b)
	}

	// Geometry non-zero for showcase nodes explicitly.
	for i, it := range items {
		if it.w <= 0 || it.h <= 0 {
			t.Fatalf("showcase item %d size=%vx%v want non-zero", i, it.w, it.h)
		}
	}
	_ = orderP
	_ = wrapP

	path := filepath.Join("testdata", "showcase_grid.png")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		fh, err := os.Create(path)
		if err != nil {
			t.Fatalf("write showcase: %v", err)
		}
		if err := png.Encode(fh, got); err != nil {
			fh.Close()
			t.Fatalf("encode showcase: %v", err)
		}
		fh.Close()
		t.Logf("showcase rewritten: %s (%dx%d)", path, int(W), int(H))
		return
	}
	fh, err := os.Open(path)
	if err != nil {
		t.Fatalf("open showcase %s (run once with UPDATE_GOLDEN=1): %v", path, err)
	}
	want, err := png.Decode(fh)
	fh.Close()
	if err != nil {
		t.Fatalf("decode showcase: %v", err)
	}
	if !want.Bounds().Eq(got.Bounds()) {
		t.Fatalf("showcase bounds %v want %v (regen with UPDATE_GOLDEN=1)", got.Bounds(), want.Bounds())
	}
	maxDiff := uint32(spec.Tolerance.MaxDiff * 257)
	hardCap := uint32(spec.Tolerance.HardCap * 257)
	bad := 0
	total := got.Bounds().Dx() * got.Bounds().Dy()
	for yy := 0; yy < got.Bounds().Dy(); yy++ {
		for xx := 0; xx < got.Bounds().Dx(); xx++ {
			r1, g1, b1, a1 := got.At(xx, yy).RGBA()
			r2, g2, b2, a2 := want.At(xx, yy).RGBA()
			m := max4g(diffg(r1, r2), diffg(g1, g2), diffg(b1, b2), diffg(a1, a2))
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

func diffg(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func max4g(vs ...uint32) uint32 {
	m := vs[0]
	for _, v := range vs[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
