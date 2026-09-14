package popover_test

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/kit/popover"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type showcaseTol struct {
	CanvasW int     `json:"canvasW"`
	Margin  float64 `json:"margin"`
	GapCanvas float64 `json:"gapCanvas"`
	Tolerance struct {
		MaxDiff float64 `json:"maxDiff"`
		BadFrac float64 `json:"badFrac"`
		HardCap float64 `json:"hardCap"`
	} `json:"tolerance"`
}

func loadShowcaseTol(t *testing.T) showcaseTol {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "popover.json"))
	if err != nil {
		t.Fatalf("read popover.json: %v", err)
	}
	var s showcaseTol
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse tolerance: %v", err)
	}
	if s.Tolerance.MaxDiff <= 0 {
		t.Fatalf("bad tolerance %+v", s)
	}
	return s
}

func loadShowcaseFace(t *testing.T) text.Face {
	t.Helper()
	text.ClearSystemFontPaths()
	face, desc, err := rendering.TryLoadDefaultFace(14)
	if err != nil || face == nil {
		t.Skipf("showcase needs a system face for real glyphs: %v", err)
	}
	t.Logf("showcase face: %s", desc)
	return face
}

// TestPopover_Showcase_MainPaths lays §6.8 P0 on one canvas: basic, three
// triggers, placement, arrow, shift, control-inner-close, hover-click.
// Three evidences: logic probe, pixel assertions, golden file compare.
func TestPopover_Showcase_MainPaths(t *testing.T) {
	spec := loadShowcaseTol(t)
	face := loadShowcaseFace(t)

	W := float64(spec.CanvasW)
	if W <= 0 {
		W = 920
	}
	margin, gap := spec.Margin, spec.GapCanvas
	if margin <= 0 {
		margin = 16
	}
	if gap <= 0 {
		gap = 16
	}

	mk := func(label, title, content string) *popover.Popover {
		p := popover.NewPopover(label)
		p.SetTitle(title)
		p.SetContent(content)
		p.SetTextFace(face)
		p.Layout(rendering.Loose(2000, 2000))
		return p
	}

	basic := mk("Hover me", "Title", "Content here")
	basic.SetDefaultOpen(true)

	hoverT := mk("Hover", "Hover title", "Hover content")
	hoverT.SetDefaultOpen(true)
	focusT := mk("Focus", "Focus title", "Focus content")
	focusT.SetTrigger(popover.TriggerFocus)
	focusT.FocusTrigger()
	clickT := mk("Click", "Click title", "Click content")
	clickT.SetTrigger(popover.TriggerClick)
	clickT.ClickTrigger()

	placeTop := mk("Top", "Top title", "Top content")
	placeTop.SetPlacement(popover.Top)
	placeTop.SetDefaultOpen(true)
	placeBottom := mk("Bottom", "Bottom title", "Bottom content")
	placeBottom.SetPlacement(popover.Bottom)
	placeBottom.SetDefaultOpen(true)
	placeLeft := mk("Left", "Left title", "Left content")
	placeLeft.SetPlacement(popover.Left)
	placeLeft.SetDefaultOpen(true)
	placeRight := mk("Right", "Right title", "Right content")
	placeRight.SetPlacement(popover.Right)
	placeRight.SetDefaultOpen(true)

	arrowOn := mk("Arrow", "Arrow on", "Caret shown")
	arrowOn.SetArrow(true)
	arrowOn.SetDefaultOpen(true)
	arrowOff := mk("NoArrow", "Arrow off", "Caret hidden")
	arrowOff.SetArrow(false)
	arrowOff.SetDefaultOpen(true)
	arrowCenter := mk("Center", "Point center", "Arrow centered")
	arrowCenter.SetArrowConfig(true, true)
	arrowCenter.SetPlacement(popover.TopLeft)
	arrowCenter.SetDefaultOpen(true)

	shift := mk("Shift", "Shift title", "Flip near edge")
	shift.SetPlacement(popover.Top)
	shift.SetAutoAdjustOverflow(true)
	shift.SetViewport(900, 700)
	shift.SetDefaultOpen(true)

	control := mk("Control", "Control title", "Press inner close")
	control.SetTrigger(popover.TriggerClick)
	control.SetOpen(true)
	control.SetContentAction(func() {})

	hoverClick := mk("HoverClick", "Mixed title", "Hover plus click")
	hoverClick.SetTriggerModes(popover.TriggerHover, popover.TriggerClick)
	hoverClick.HoverEnter()

	// Logic probe before paint.
	if !basic.IsOpen() || !hoverT.IsOpen() || !focusT.IsOpen() || !clickT.IsOpen() {
		t.Fatal("trigger rows must open")
	}
	if placeTop.Placement() != popover.Top || placeBottom.Placement() != popover.Bottom {
		t.Fatal("placement probe")
	}
	if !arrowOn.Arrow() || arrowOff.Arrow() || !arrowCenter.PointAtCenter() {
		t.Fatal("arrow probe")
	}
	if !shift.AutoAdjustOverflow() {
		t.Fatal("shift probe")
	}
	if !control.IsOpen() || !control.Controlled() {
		t.Fatal("control probe")
	}
	if !hoverClick.IsOpen() || len(hoverClick.Triggers()) != 2 {
		t.Fatal("hoverclick probe")
	}

	type item struct {
		trig *popover.Popover
		tx   float64
		ty   float64
		px   float64
		py   float64
	}
	var items []item
	y := margin
	// One band per popover: trigger at margin, panel at overlay Resolve.
	// Band height covers both so neighboring panels never overlap.
	placeOne := func(p *popover.Popover) {
		tw, th := p.LaidOut().Width, p.LaidOut().Height
		pw, ph := p.PanelLaidOut().Width, p.PanelLaidOut().Height
		// Reserve space above for top placements.
		ty := y
		isTop := p.Placement() == popover.Top || p.Placement() == popover.TopLeft || p.Placement() == popover.TopRight
		if isTop {
			ty = y + ph + 8
		}
		isLeft := p.Placement() == popover.Left || p.Placement() == popover.LeftTop || p.Placement() == popover.LeftBottom
		tx := margin
		if isLeft {
			tx = margin + pw + 8
		}
		anchor := rendering.NewRect(tx, ty, tw, th)
		res := p.Resolve(anchor, pw, ph, 2000, 2000)
		items = append(items, item{trig: p, tx: tx, ty: ty, px: res.X, py: res.Y})
		bandBottom := ty + th
		if res.Y+ph > bandBottom {
			bandBottom = res.Y + ph
		}
		y = bandBottom + gap
	}
	for _, p := range []*popover.Popover{basic, hoverT, focusT, clickT, placeTop, placeBottom, placeLeft, placeRight, arrowOn, arrowOff, arrowCenter, shift, control, hoverClick} {
		placeOne(p)
	}

	H := y - gap + margin
	tok := theme.Default.Current()
	bgLayout := tok.ColorBgLayout

	dc := render.NewContext(int(W), int(H))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.RGBA{R: bgLayout.R, G: bgLayout.G, B: bgLayout.B, A: 1})
	for _, it := range items {
		it.trig.TriggerShell().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(it.tx, it.ty))
		it.trig.Panel().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(it.px, it.py))
	}
	got := dc.Image()

	// Pixel 1: panel interior is container white, canvas is layout gray.
	px0, py0 := items[0].px+8, items[0].py+8
	r, g, b, _ := got.At(int(px0), int(py0)).RGBA()
	r8, g8, b8 := r/257, g/257, b/257
	if !(r8 > 240 && g8 > 240 && b8 > 240) {
		t.Fatalf("panel interior #%02x%02x%02x want white container", r8, g8, b8)
	}
	cr, cg, cb, _ := got.At(4, 4).RGBA()
	cr8, cg8, cb8 := cr/257, cg/257, cb/257
	if !(cr8 > 230 && cr8 < 250 && cg8 > 230 && cg8 < 250) {
		t.Fatalf("canvas #%02x%02x%02x want layout gray", cr8, cg8, cb8)
	}

	// Pixel 2: title zone carries dark glyph ink (no black bars).
	dark := 0
	x0, x1 := int(items[0].px+12), int(items[0].px+180)
	yy0, yy1 := int(items[0].py+10), int(items[0].py+30)
	if x1 >= int(W) {
		x1 = int(W) - 1
	}
	for yy := yy0; yy < yy1; yy++ {
		for xx := x0; xx < x1; xx++ {
			rr, gg, bb, _ := got.At(xx, yy).RGBA()
			a8, b8x, c8 := rr/257, gg/257, bb/257
			if a8 < 110 && b8x < 110 && c8 < 110 {
				dark++
			}
		}
	}
	if dark < 30 {
		t.Fatalf("title zone dark=%d want >=30 (real glyphs missing?)", dark)
	}

	path := filepath.Join("testdata", "showcase_popover.png")
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
			m := max4u(diffu(r1, r2), diffu(g1, g2), diffu(b1, b2), diffu(a1, a2))
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

func diffu(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func max4u(vs ...uint32) uint32 {
	m := vs[0]
	for _, v := range vs[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
