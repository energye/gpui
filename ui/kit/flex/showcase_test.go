package flex_test

import (
	"encoding/json"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/flex"
	"github.com/energye/gpui/ui/rendering"
)

type showcaseFlexSpec struct {
	CanvasW int     `json:"canvasW"`
	Margin  float64 `json:"margin"`
	RowGap  float64 `json:"rowGap"`
	Tolerance struct {
		MaxDiff float64 `json:"maxDiff"`
		BadFrac float64 `json:"badFrac"`
		HardCap float64 `json:"hardCap"`
	} `json:"tolerance"`
}

func loadShowcaseFlexSpec(t *testing.T) showcaseFlexSpec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "showcase_flex.json"))
	if err != nil {
		t.Fatalf("read showcase_flex.json: %v", err)
	}
	var s showcaseFlexSpec
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse showcase spec: %v", err)
	}
	if s.CanvasW <= 0 || s.Tolerance.MaxDiff <= 0 || s.Tolerance.HardCap <= 0 {
		t.Fatalf("bad showcase spec %+v", s)
	}
	return s
}

func cbox(w, h, r, g, b float64) *rendering.RenderColorBox {
	return rendering.NewRenderColorBox(w, h, r, g, b, 1)
}

// TestFlex_Showcase_MainPaths lays §6.8 P0 main paths on one big canvas:
// basic horizontal / vertical / justify×4 / align×3 / gap×3 / wrap /
// combination. Flex is a no-chrome layout box, so every child is a solid
// color block with a real size; geometry (not text) carries the signal.
// Three evidences: logic probe (offsets/gaps/rows), pixel assertions
// (distinct block colors + white gaps), golden file compare (tolerance
// from testdata/showcase_flex.json). Regenerate with UPDATE_GOLDEN=1.
func TestFlex_Showcase_MainPaths(t *testing.T) {
	spec := loadShowcaseFlexSpec(t)
	W := float64(spec.CanvasW)
	margin, rowGap := spec.Margin, spec.RowGap
	const tol = 0.5

	// Basic horizontal: 3 blocks 72x36, gap small 8.
	basic := flex.NewFlex(
		cbox(72, 36, 0.9, 0.2, 0.2),
		cbox(72, 36, 0.2, 0.7, 0.3),
		cbox(72, 36, 0.2, 0.4, 0.9),
	)
	basic.SetGapSize(flex.FlexGapSmall)

	// Vertical: 3 blocks 72x28, gap small 8.
	vertical := flex.NewFlex(
		cbox(72, 28, 0.95, 0.55, 0.15),
		cbox(72, 28, 0.55, 0.3, 0.85),
		cbox(72, 28, 0.15, 0.65, 0.65),
	)
	vertical.SetOrientation(flex.FlexOrientationVertical)
	vertical.SetGapSize(flex.FlexGapSmall)

	// Justify rows: same children, fixed 560x48 stage, gap 0.
	justifyKinds := []flex.FlexJustify{
		flex.FlexJustifyStart,
		flex.FlexJustifyCenter,
		flex.FlexJustifyEnd,
		flex.FlexJustifySpaceBetween,
	}
	var justifies []*flex.Flex
	for _, j := range justifyKinds {
		f := flex.NewFlex(
			cbox(80, 32, 0.9, 0.2, 0.2),
			cbox(80, 32, 0.2, 0.4, 0.9),
		)
		f.SetJustify(j)
		justifies = append(justifies, f)
	}

	// Align rows: short 80x20 + tall 80x44, fixed 560x64 stage, gap 8.
	alignKinds := []flex.FlexAlign{
		flex.FlexAlignStart,
		flex.FlexAlignCenter,
		flex.FlexAlignEnd,
	}
	var aligns []*flex.Flex
	for _, a := range alignKinds {
		f := flex.NewFlex(
			cbox(80, 20, 0.2, 0.7, 0.3),
			cbox(80, 44, 0.55, 0.3, 0.85),
		)
		f.SetAlign(a)
		f.SetGap(8)
		aligns = append(aligns, f)
	}

	// Gap rows: same children 64x32, small/medium/large.
	gapKinds := []flex.FlexGapSize{
		flex.FlexGapSmall,
		flex.FlexGapMedium,
		flex.FlexGapLarge,
	}
	var gaps []*flex.Flex
	for _, g := range gapKinds {
		f := flex.NewFlex(
			cbox(64, 32, 0.9, 0.2, 0.2),
			cbox(64, 32, 0.2, 0.4, 0.9),
		)
		f.SetGapSize(g)
		gaps = append(gaps, f)
	}

	// Wrap: 3 blocks 80x32 in a 200-wide stage, gap 8.
	wrap := flex.NewFlex(
		cbox(80, 32, 0.9, 0.2, 0.2),
		cbox(80, 32, 0.2, 0.7, 0.3),
		cbox(80, 32, 0.2, 0.4, 0.9),
	)
	wrap.SetGap(8)
	wrap.SetWrap(true)

	// Combination: vertical 220x170, center/center, gap small.
	combo := flex.NewFlex(
		cbox(64, 28, 0.95, 0.55, 0.15),
		cbox(64, 28, 0.55, 0.3, 0.85),
		cbox(64, 28, 0.15, 0.65, 0.65),
	)
	combo.SetOrientation(flex.FlexOrientationVertical)
	combo.SetJustify(flex.FlexJustifyCenter)
	combo.SetAlign(flex.FlexAlignCenter)
	combo.SetGapSize(flex.FlexGapSmall)

	type placed struct {
		f *flex.Flex
		x float64
		y float64
		w float64
		h float64
	}
	var items []placed
	y := margin
	put := func(f *flex.Flex, c rendering.Constraints) placed {
		sz := f.Layout(c)
		if sz.Width <= 0 || sz.Height <= 0 {
			t.Fatalf("showcase layout=%v want non-zero", sz)
		}
		if ns := f.Node().Size(); ns.Width <= 0 || ns.Height <= 0 {
			t.Fatalf("showcase node=%v want non-zero", ns)
		}
		for i, ch := range f.Children() {
			if cs := ch.Size(); cs.Width <= 0 || cs.Height <= 0 {
				t.Fatalf("showcase child %d size=%v want non-zero", i, cs)
			}
		}
		p := placed{f: f, x: margin, y: y, w: sz.Width, h: sz.Height}
		items = append(items, p)
		y += sz.Height + rowGap
		return p
	}

	basicP := put(basic, rendering.Loose(W-2*margin, 800))
	vertP := put(vertical, rendering.Loose(W-2*margin, 800))
	var justifyP []placed
	for _, f := range justifies {
		justifyP = append(justifyP, put(f, rendering.Tight(560, 48)))
	}
	var alignP []placed
	for _, f := range aligns {
		alignP = append(alignP, put(f, rendering.Tight(560, 64)))
	}
	var gapP []placed
	for _, f := range gaps {
		gapP = append(gapP, put(f, rendering.Loose(W-2*margin, 800)))
	}
	wrapP := put(wrap, rendering.Tight(200, 140))
	comboP := put(combo, rendering.Tight(220, 170))
	H := y - rowGap + margin

	// Logic probe: basic row tops equal, gaps 8.
	bo := []rendering.Point{}
	for _, ch := range basic.Children() {
		bo = append(bo, ch.Offset())
	}
	bs := []rendering.Size{}
	for _, ch := range basic.Children() {
		bs = append(bs, ch.Size())
	}
	if math.Abs(bo[0].Y-bo[1].Y) > tol || math.Abs(bo[1].Y-bo[2].Y) > tol {
		t.Fatalf("basic tops %+v want equal", bo)
	}
	if math.Abs((bo[1].X-bo[0].X-bs[0].Width)-8) > tol {
		t.Fatalf("basic spacing=%v want 8", bo[1].X-bo[0].X-bs[0].Width)
	}

	// Logic probe: vertical lefts equal, gaps 8.
	vo := []rendering.Point{}
	for _, ch := range vertical.Children() {
		vo = append(vo, ch.Offset())
	}
	vs := []rendering.Size{}
	for _, ch := range vertical.Children() {
		vs = append(vs, ch.Size())
	}
	if math.Abs(vo[0].X-vo[1].X) > tol || math.Abs(vo[1].X-vo[2].X) > tol {
		t.Fatalf("vertical lefts %+v want equal", vo)
	}
	if math.Abs((vo[1].Y-vo[0].Y-vs[0].Height)-8) > tol {
		t.Fatalf("vertical spacing=%v want 8", vo[1].Y-vo[0].Y-vs[0].Height)
	}

	// Logic probe: justify offsets diverge across the 560 stage.
	jo := func(f *flex.Flex) []rendering.Point {
		out := []rendering.Point{}
		for _, ch := range f.Children() {
			out = append(out, ch.Offset())
		}
		return out
	}
	js := func(f *flex.Flex) []rendering.Size {
		out := []rendering.Size{}
		for _, ch := range f.Children() {
			out = append(out, ch.Size())
		}
		return out
	}
	if math.Abs(jo(justifies[0])[0].X) > tol {
		t.Fatalf("justify start first X=%v want 0", jo(justifies[0])[0].X)
	}
	if math.Abs(jo(justifies[1])[0].X-200) > tol {
		t.Fatalf("justify center first X=%v want 200", jo(justifies[1])[0].X)
	}
	if math.Abs(jo(justifies[2])[0].X-400) > tol {
		t.Fatalf("justify end first X=%v want 400", jo(justifies[2])[0].X)
	}
	sbOff, sbSize := jo(justifies[3]), js(justifies[3])
	if math.Abs(sbOff[0].X) > tol || math.Abs((sbOff[1].X+sbSize[1].Width)-560) > tol {
		t.Fatalf("justify between edges %+v sizes %+v", sbOff, sbSize)
	}

	// Logic probe: align center short mid matches tall mid (64/2=32 in the
	// 64-high stage; single-line center uses the container cross size).
	acOff := jo(aligns[1])
	acSize := js(aligns[1])
	if math.Abs((acOff[0].Y+acSize[0].Height/2)-32) > tol {
		t.Fatalf("align center short mid=%v want 32", acOff[0].Y+acSize[0].Height/2)
	}
	if math.Abs(jo(aligns[0])[0].Y) > tol {
		t.Fatalf("align start short Y=%v want 0", jo(aligns[0])[0].Y)
	}
	if math.Abs((jo(aligns[2])[0].Y+js(aligns[2])[0].Height)-64) > tol {
		t.Fatalf("align end short bottom=%v want 64", jo(aligns[2])[0].Y+js(aligns[2])[0].Height)
	}

	// Logic probe: gap rows space 8/16/24.
	for i, want := range []float64{8, 16, 24} {
		off := jo(gaps[i])
		sz := js(gaps[i])
		if math.Abs((off[1].X-off[0].X-sz[0].Width)-want) > tol {
			t.Fatalf("gap row %d spacing=%v want %v", i, off[1].X-off[0].X-sz[0].Width, want)
		}
	}

	// Logic probe: wrap breaks into >=2 rows.
	wo := jo(wrap)
	if !(wo[1].Y > wo[0].Y || wo[2].Y > wo[0].Y) {
		t.Fatalf("wrap must break: %+v", wo)
	}

	// Logic probe: combination stays vertical and centered.
	co := jo(combo)
	cs := js(combo)
	if !combo.IsVertical() {
		t.Fatal("combo must stay vertical")
	}
	if math.Abs(co[0].X-co[1].X) > tol || math.Abs(co[1].X-co[2].X) > tol {
		t.Fatalf("combo lefts %+v want equal (align center)", co)
	}
	if math.Abs(co[0].X-78) > tol {
		t.Fatalf("combo X=%v want 78 (centered in 220)", co[0].X)
	}
	if math.Abs(co[0].Y-35) > tol {
		t.Fatalf("combo first Y=%v want 35 (centered in 170)", co[0].Y)
	}
	if math.Abs((co[1].Y-co[0].Y-cs[0].Height)-8) > tol {
		t.Fatalf("combo spacing=%v want 8", co[1].Y-co[0].Y-cs[0].Height)
	}

	dc := render.NewContext(int(W), int(H))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	for _, it := range items {
		it.f.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(it.x, it.y))
	}
	got := dc.Image()

	at := func(x, y float64) (uint32, uint32, uint32) {
		r, g, b, _ := got.At(int(x), int(y)).RGBA()
		return r / 257, g / 257, b / 257
	}
	isWhite := func(r, g, b uint32) bool { return r > 240 && g > 240 && b > 240 }

	// Pixel 1: basic first block red, its gap white, third block blue.
	r, g, b := at(basicP.x+36, basicP.y+18)
	if !(r > 180 && g < 110 && b < 110) {
		t.Fatalf("basic red #%02x%02x%02x want red", r, g, b)
	}
	r, g, b = at(basicP.x+72+4, basicP.y+18)
	if !isWhite(r, g, b) {
		t.Fatalf("basic gap #%02x%02x%02x want white", r, g, b)
	}
	r, g, b = at(basicP.x+72+8+72+36, basicP.y+18)
	if !(b > 180 && r < 110) {
		t.Fatalf("basic blue #%02x%02x%02x want blue", r, g, b)
	}

	// Pixel 2: vertical first block orange, inter-row gap white.
	r, g, b = at(vertP.x+36, vertP.y+14)
	if !(r > 220 && g > 110 && g < 170 && b < 90) {
		t.Fatalf("vertical orange #%02x%02x%02x want orange", r, g, b)
	}
	r, g, b = at(vertP.x+36, vertP.y+28+4)
	if !isWhite(r, g, b) {
		t.Fatalf("vertical gap #%02x%02x%02x want white", r, g, b)
	}

	// Pixel 3: space-between row touches both stage edges, middle white.
	sbP := justifyP[3]
	r, g, b = at(sbP.x+4, sbP.y+24)
	if !(r > 180 && g < 110 && b < 110) {
		t.Fatalf("between left #%02x%02x%02x want red", r, g, b)
	}
	r, g, b = at(sbP.x+560-4, sbP.y+24)
	if !(b > 180 && r < 110) {
		t.Fatalf("between right #%02x%02x%02x want blue", r, g, b)
	}
	r, g, b = at(sbP.x+280, sbP.y+24)
	if !isWhite(r, g, b) {
		t.Fatalf("between middle #%02x%02x%02x want white", r, g, b)
	}

	// Pixel 4: align-center short block green at its center.
	acP := alignP[1]
	r, g, b = at(acP.x+40, acP.y+acOff[0].Y+10)
	if !(g > 140 && r < 110 && b < 110) {
		t.Fatalf("align green #%02x%02x%02x want green", r, g, b)
	}

	// Pixel 5: large-gap row gap middle white, blocks red/blue.
	lgP := gapP[2]
	lgOff := jo(gaps[2])
	r, g, b = at(lgP.x+lgOff[0].X+32, lgP.y+16)
	if !(r > 180 && g < 110) {
		t.Fatalf("large-gap red #%02x%02x%02x want red", r, g, b)
	}
	r, g, b = at(lgP.x+lgOff[0].X+64+12, lgP.y+16)
	if !isWhite(r, g, b) {
		t.Fatalf("large-gap middle #%02x%02x%02x want white", r, g, b)
	}

	// Pixel 6: wrap first block red, wrapped block blue below.
	r, g, b = at(wrapP.x+wo[0].X+40, wrapP.y+wo[0].Y+16)
	if !(r > 180 && g < 110 && b < 110) {
		t.Fatalf("wrap first #%02x%02x%02x want red", r, g, b)
	}
	last := wo[2]
	if !(last.Y > wo[0].Y) {
		last = wo[1]
	}
	r, g, b = at(wrapP.x+last.X+40, wrapP.y+last.Y+16)
	if !(b > 140 && r < 140) {
		t.Fatalf("wrap broken #%02x%02x%02x want blue/green below", r, g, b)
	}

	// Pixel 7: combination middle block purple at its center.
	r, g, b = at(comboP.x+co[1].X+32, comboP.y+co[1].Y+14)
	if !(r > 100 && r < 180 && b > 150) {
		t.Fatalf("combo purple #%02x%02x%02x want purple", r, g, b)
	}

	path := filepath.Join("testdata", "showcase_flex.png")
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
			m := max4(diff(r1, r2), diff(g1, g2), diff(b1, b2), diff(a1, a2))
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
