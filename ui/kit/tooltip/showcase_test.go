package tooltip_test

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/kit/tooltip"
	"github.com/energye/gpui/ui/rendering"
)

type showcaseSpec struct {
	CanvasW   int     `json:"canvasW"`
	Margin    float64 `json:"margin"`
	Gap       float64 `json:"gap"`
	Tolerance struct {
		MaxDiff float64 `json:"maxDiff"`
		BadFrac float64 `json:"badFrac"`
		HardCap float64 `json:"hardCap"`
	} `json:"tolerance"`
}

func loadShowcaseSpec(t *testing.T) showcaseSpec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "showcase_spec.json"))
	if err != nil {
		t.Fatalf("read showcase_spec.json: %v", err)
	}
	var s showcaseSpec
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse showcase spec: %v", err)
	}
	if s.CanvasW <= 0 || s.Gap < 0 {
		t.Fatalf("bad showcase spec %+v", s)
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

// TestTooltip_Showcase_P0Rows lays §6.8 P0 rows on one big canvas: basic /
// trigger trio / 12 placements / arrow trio / shift / colorful / disabled
// pair / custom slots. Three evidences: logic probe (placement/arrow/color/
// disabled/custom), pixel assertions (dark default skin, red preset skin,
// light text ink, empty-title paints nothing, custom trigger blue), golden
// file compare (tolerance from testdata/showcase_spec.json).
// Regenerate with UPDATE_GOLDEN=1.
func TestTooltip_Showcase_P0Rows(t *testing.T) {
	spec := loadShowcaseSpec(t)
	face := loadShowcaseFace(t)

	W := float64(spec.CanvasW)
	margin, gap := spec.Margin, spec.Gap
	colGap := 12.0

	type demo struct {
		tp *tooltip.Tooltip
	}
	mk := func(title, trigger string) *tooltip.Tooltip {
		tp := tooltip.NewTooltip(title)
		tp.SetTriggerLabel(trigger)
		tp.SetFace(face)
		tp.SetMouseEnterDelay(0)
		return tp
	}
	var demos []demo
	// Basic.
	basic := mk("prompt text", "Hover me")
	demos = append(demos, demo{tp: basic})
	// Trigger trio.
	trH := mk("hover tip", "Hover")
	trF := mk("focus tip", "Focus")
	trF.SetTriggerModes(tooltip.TriggerFocus)
	trC := mk("click tip", "Click")
	trC.SetTriggerModes(tooltip.TriggerClick)
	demos = append(demos, demo{tp: trH}, demo{tp: trF}, demo{tp: trC})
	// Placement dozen.
	var placements []*tooltip.Tooltip
	for _, p := range tooltip.AllPlacements {
		tp := mk("tip "+string(p), string(p))
		tp.SetPlacement(p)
		tp.SetAnchor(400, 300)
		placements = append(placements, tp)
		demos = append(demos, demo{tp: tp})
	}
	// Arrow trio.
	arrOn := mk("arrow on", "Arrow")
	arrOff := mk("no arrow", "NoArrow")
	arrOff.SetArrow(false)
	arrCenter := mk("center arrow", "Center")
	arrCenter.SetPlacement(tooltip.TopLeft)
	arrCenter.SetArrowConfig(true, true)
	demos = append(demos, demo{tp: arrOn}, demo{tp: arrOff}, demo{tp: arrCenter})
	// Shift (edge-anchored, auto-adjust skin shown here; geometry in TIP-14).
	shift := mk("edge tip", "Edge")
	shift.SetAnchor(390, 150)
	shift.SetViewport(400, 300)
	demos = append(demos, demo{tp: shift})
	// Colorful presets + custom hex.
	colorNames := []string{"red", "green", "blue", "orange", "purple", "gold"}
	var colors []*tooltip.Tooltip
	for _, n := range colorNames {
		tp := mk(n+" tip", n)
		tp.SetColor(n)
		colors = append(colors, tp)
		demos = append(demos, demo{tp: tp})
	}
	hexTip := mk("hex tip", "#ff5500")
	hexTip.SetColor("#ff5500")
	colors = append(colors, hexTip)
	demos = append(demos, demo{tp: hexTip})
	// Disabled pair: disabled trigger keeps skin, empty title paints nothing.
	dis := mk("disabled tip", "Disabled")
	dis.SetDisabled(true)
	empty := mk("", "Empty title")
	demos = append(demos, demo{tp: dis}, demo{tp: empty})
	// Custom slots.
	custom := tooltip.NewTooltip("slot")
	custom.SetFace(face)
	customText := rendering.NewRenderText("custom title node")
	customText.FontSize = 14
	customText.SetFace(face)
	custom.SetTitleNode(customText)
	custom.SetTriggerNode(rendering.NewRenderColorBox(90, 32, 0.1, 0.4, 0.9, 1))
	custom.Layout(rendering.Loose(900, 200))
	demos = append(demos, demo{tp: custom})

	// Logic probe before paint.
	if basic.Placement() != tooltip.Top || !basic.HasArrow() {
		t.Fatal("basic placement/arrow probe")
	}
	if len(placements) != 12 {
		t.Fatalf("placements=%d want 12", len(placements))
	}
	if arrOff.HasArrow() || arrCenter.EffectivePlacement() != tooltip.Top {
		t.Fatal("arrow show/hide/center probe")
	}
	redBG := colors[0].Background()
	if !(redBG.R > 0.8 && redBG.R > redBG.G+0.3) {
		t.Fatalf("colorful red bg=%+v", redBG)
	}
	if !dis.Disabled() || dis.IsOpen() {
		t.Fatal("disabled probe")
	}
	if custom.TitleNode() == nil || custom.TriggerNode() == nil {
		t.Fatal("custom slot probe")
	}

	// Flow layout: trigger + panel side by side per demo.
	type placed struct {
		tp             *tooltip.Tooltip
		tx, ty, tw, th float64
		px, py, pw, ph float64
		hasPanel       bool
	}
	var items []placed
	x, y, rowH := margin, margin, 0.0
	flush := func() {
		if rowH > 0 {
			y += rowH + gap
			rowH = 0
		}
		x = margin
	}
	for _, d := range demos {
		sz := d.tp.Layout(rendering.Loose(900, 200))
		ps := d.tp.PanelSize()
		pw, ph := ps.Width, ps.Height
		// Empty titles keep their slot: Panel().Paint there must draw
		// nothing (no black bars), proven by pixel 4 below.
		hasPanel := true
		itemW := sz.Width + pw
		if hasPanel {
			itemW += colGap
		}
		itemH := sz.Height
		if ph > itemH {
			itemH = ph
		}
		if x > margin && x+itemW > W-margin {
			flush()
		}
		py := y
		if ph < itemH {
			py = y + (itemH-ph)/2
		}
		ty := y
		if sz.Height < itemH {
			ty = y + (itemH-sz.Height)/2
		}
		items = append(items, placed{tp: d.tp, tx: x, ty: ty, tw: sz.Width, th: sz.Height, px: x + sz.Width + colGap, py: py, pw: pw, ph: ph, hasPanel: hasPanel})
		x += itemW + colGap
		if itemH > rowH {
			rowH = itemH
		}
	}
	H := y + rowH + margin

	dc := render.NewContext(int(W), int(H))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	for _, it := range items {
		it.tp.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(it.tx, it.ty))
		if it.hasPanel {
			it.tp.Panel().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(it.px, it.py))
		}
	}
	got := dc.Image()

	at := func(fx, fy float64) (uint32, uint32, uint32) {
		r, g, b, _ := got.At(int(fx), int(fy)).RGBA()
		return r / 257, g / 257, b / 257
	}
	find := func(tp *tooltip.Tooltip) *placed {
		for i := range items {
			if items[i].tp == tp {
				return &items[i]
			}
		}
		return nil
	}
	// Pixel 1: default skin padding corner is dark spotlight, not white.
	bi := find(basic)
	if bi == nil {
		t.Fatal("basic item missing")
	}
	dr, dg, db := at(bi.px+3, bi.py+3)
	if !(dr < 110 && dg < 110 && db < 110) {
		t.Fatalf("default skin #%02x%02x%02x want dark spotlight", dr, dg, db)
	}
	// Pixel 2: red preset corner is red.
	ri := find(colors[0])
	rr, rg, rb := at(ri.px+3, ri.py+3)
	if !(rr > 180 && rr > rg+60 && rr > rb+60) {
		t.Fatalf("red skin #%02x%02x%02x want red preset", rr, rg, rb)
	}
	// Pixel 3: title zone carries light glyph ink (real text, no bars).
	light := 0
	x0, x1 := int(bi.px+8), int(bi.px+bi.pw-8)
	for yy := int(bi.py + 4); yy < int(bi.py+bi.ph-4); yy++ {
		for xx := x0; xx < x1; xx++ {
			r, g, b := at(float64(xx), float64(yy))
			if r > 200 && g > 200 && b > 200 {
				light++
			}
		}
	}
	if light < 20 {
		t.Fatalf("basic title light pixels=%d want >=20 (real glyphs missing?)", light)
	}
	// Pixel 4: empty title paints nothing (slot stays white, no black bars).
	ei := find(empty)
	er, eg, eb := at(ei.px+ei.pw/2, ei.py+ei.ph/2)
	if !(er > 240 && eg > 240 && eb > 240) {
		t.Fatalf("empty panel #%02x%02x%02x want white (no bars)", er, eg, eb)
	}
	// Pixel 5: custom trigger slot paints its blue box.
	ci := find(custom)
	cr, cg, cb := at(ci.tx+ci.tw/2, ci.ty+ci.th/2)
	if !(cb > 150 && cb > cr+50 && cb > cg+30) {
		t.Fatalf("custom trigger #%02x%02x%02x want blue box", cr, cg, cb)
	}

	path := filepath.Join("testdata", "showcase_tooltip.png")
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
			m := max4tip(diffTip(r1, r2), diffTip(g1, g2), diffTip(b1, b2), diffTip(a1, a2))
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

func diffTip(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func max4tip(vs ...uint32) uint32 {
	m := vs[0]
	for _, v := range vs[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
