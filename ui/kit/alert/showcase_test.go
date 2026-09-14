package alert_test

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/kit/alert"
	"github.com/energye/gpui/ui/kit/button"
	"github.com/energye/gpui/ui/kit/flex"
	"github.com/energye/gpui/ui/rendering"
)

type showcaseSpec struct {
	CanvasW int     `json:"canvasW"`
	Margin  float64 `json:"margin"`
	Gap     float64 `json:"gap"`
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

// TestAlert_Showcase_Official9 lays the §6.8 P0 official examples on one big
// canvas: basic / four types / filled / closable / description / icon /
// banner / loop-banner TitleNode / action single+double. Three evidences:
// logic probe (types/layout/banner fill), pixel assertions (four tints,
// dark text ink, banner edge-to-edge), golden file compare (tolerance from
// testdata/showcase_spec.json). Regenerate with UPDATE_GOLDEN=1.
func TestAlert_Showcase_Official9(t *testing.T) {
	spec := loadShowcaseSpec(t)
	face := loadShowcaseFace(t)
	alert.ResetAlertGlobals()

	W := float64(spec.CanvasW)
	margin, gap := spec.Margin, spec.Gap
	rowW := W - 2*margin

	mk := func(a *alert.Alert) *alert.Alert {
		a.SetTextFace(face)
		return a
	}

	// R1 basic (basic.tsx): success title only.
	basic := mk(alert.NewAlert("Success Text"))
	basic.SetType(alert.AlertSuccess)

	// R2-R5 four styles (style.tsx): four semantic types with icons.
	fourTypes := []alert.AlertType{alert.AlertSuccess, alert.AlertInfo, alert.AlertWarning, alert.AlertError}
	fourNames := []string{"Success", "Info", "Warning", "Error"}
	var fours []*alert.Alert
	for i, tp := range fourTypes {
		a := mk(alert.NewAlert(fourNames[i]))
		a.SetType(tp)
		a.SetShowIcon(true)
		fours = append(fours, a)
	}

	// R6 filled/borderless (filled.tsx): filled has no border.
	filled := mk(alert.NewAlert("Filled Info"))
	filled.SetType(alert.AlertInfo)
	filled.SetVariant(alert.AlertFilled)
	filled.SetShowIcon(true)

	// R7 closable (closable.tsx).
	closable := mk(alert.NewAlert("Closable Warning"))
	closable.SetType(alert.AlertWarning)
	closable.SetShowIcon(true)
	closable.SetClosable(true)

	// R8 description double row (description.tsx).
	desc := mk(alert.NewAlert("Success Title"))
	desc.SetType(alert.AlertSuccess)
	desc.SetDescription("Detailed description with two lines")
	desc.SetShowIcon(true)

	// R9 icon combos (icon.tsx): icon + description + closable.
	icon := mk(alert.NewAlert("Icon Info"))
	icon.SetType(alert.AlertInfo)
	icon.SetShowIcon(true)
	icon.SetDescription("Helper with icon")
	icon.SetClosable(true)

	// R10 banner (banner.tsx): full-bleed top announcement.
	banner := mk(alert.NewAlert("Banner Announcement"))
	banner.SetBanner(true)

	// R11 loop-banner (loop-banner.tsx): banner + long TitleNode slot.
	loop := mk(alert.NewAlert("loop"))
	loop.SetBanner(true)
	longTitle := rendering.NewRenderText("Very long scrolling announcement for loop-banner marquee slot")
	longTitle.FontSize = 14
	longTitle.R, longTitle.G, longTitle.B, longTitle.A = 0, 0, 0, 0.88
	longTitle.SetFace(face)
	loop.SetTitleNode(longTitle)

	// R12 action single (action.tsx): right slot with one real button.
	mkBtn := func(label string) *button.Button {
		b := button.NewButton(label)
		b.SetTextFace(face)
		b.Layout(rendering.Loose(rowW, 800))
		return b
	}
	single := mk(alert.NewAlert("Action Single"))
	single.SetType(alert.AlertInfo)
	single.SetShowIcon(true)
	single.SetAction(mkBtn("UNDO").Node())

	// R13 action double + description: Accept+Decline vertical pair of real
	// buttons (pixel-perfect vertical stacking is P1 staged; the slot,
	// coexistence with the double row, and hit box are P0 here).
	accept := mkBtn("Accept")
	decline := mkBtn("Decline")
	pair := flex.NewFlex(accept.Node(), decline.Node())
	pair.SetVertical(true)
	pair.Layout(rendering.Loose(rowW, 800))
	double := mk(alert.NewAlert("Action Double"))
	double.SetType(alert.AlertWarning)
	double.SetShowIcon(true)
	double.SetDescription("With double-row action")
	double.SetAction(pair.Node())

	rows := []*alert.Alert{basic}
	rows = append(rows, fours...)
	rows = append(rows, filled, closable, desc, icon, banner, loop, single, double)
	isBannerRow := func(i int) bool {
		a := rows[i]
		return a.IsBanner()
	}

	// Logic probe before paint: types, chrome, banner defaults.
	if basic.Type() != alert.AlertSuccess || !basic.Visible() {
		t.Fatal("basic logic probe")
	}
	seen := map[string]bool{}
	for _, a := range fours {
		seen[string(a.Type())] = true
	}
	if len(seen) != 4 {
		t.Fatalf("four styles want 4 types, got %v", seen)
	}
	if filled.HasBorder() || filled.LineWidth() != 0 {
		t.Fatal("filled must be borderless")
	}
	if !closable.Closable() || !icon.IconVisible() || !desc.HasDescription() {
		t.Fatal("closable/icon/description probe")
	}
	if banner.Type() != alert.AlertWarning || !banner.ShowIcon() || banner.Radius() != 0 || banner.HasBorder() {
		t.Fatal("banner defaults probe")
	}
	if loop.TitleNode() == nil || double.Action() == nil || !double.HasDescription() {
		t.Fatal("loop/action probe")
	}

	// Layout every row at exact width (banners edge-to-edge, others inset).
	type placed struct {
		a *alert.Alert
		x float64
		y float64
		w float64
		h float64
	}
	var items []placed
	y := margin
	bannerIdx := -1
	for i, a := range rows {
		w := rowW
		x := margin
		if isBannerRow(i) {
			w = W
			x = 0
			if bannerIdx < 0 && a == banner {
				bannerIdx = len(items)
			}
		}
		sz := a.Layout(rendering.Constraints{MinWidth: w, MaxWidth: w, MaxHeight: rendering.Unbounded})
		if sz.Width != w || sz.Height <= 0 {
			t.Fatalf("row %d layout=%v want width %v", i, sz, w)
		}
		items = append(items, placed{a: a, x: x, y: y, w: sz.Width, h: sz.Height})
		y += sz.Height + gap
	}
	H := y - gap + margin

	dc := render.NewContext(int(W), int(H))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	for _, it := range items {
		it.a.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(it.x, it.y))
	}
	got := dc.Image()

	// Pixel assertion 1: four semantic shells are tinted and distinct.
	shellAt := func(it placed) (uint32, uint32, uint32) {
		r, g, b, _ := got.At(int(it.x+6), int(it.y+6)).RGBA()
		return r / 257, g / 257, b / 257
	}
	fourSeen := map[string]bool{}
	for k := 1; k <= 4; k++ {
		r, g, b := shellAt(items[k])
		if r > 250 && g > 250 && b > 250 {
			t.Fatalf("four row %d shell #%02x%02x%02x looks white, want tint", k, r, g, b)
		}
		key := string(rune(r)) + string(rune(g)) + string(rune(b))
		_ = key
		fourSeen[itKey(r, g, b)] = true
	}
	if len(fourSeen) != 4 {
		t.Fatalf("four shells must differ, got %d distinct", len(fourSeen))
	}

	// Pixel assertion 2: title zone carries dark glyph ink (real text, no bars).
	b0 := items[0]
	dark := 0
	x0, x1 := int(b0.x+14), int(b0.x+220)
	if x1 > int(W)-4 {
		x1 = int(W) - 4
	}
	for yy := int(b0.y + 4); yy < int(b0.y+b0.h-4); yy++ {
		for xx := x0; xx < x1; xx++ {
			r, g, b, _ := got.At(xx, yy).RGBA()
			r8, g8, b8 := r/257, g/257, b/257
			if r8 < 110 && g8 < 110 && b8 < 110 {
				dark++
			}
		}
	}
	if dark < 30 {
		t.Fatalf("basic title zone dark pixels=%d want >=30 (real glyphs missing?)", dark)
	}

	// Pixel assertion 3: banner spans edge to edge.
	if bannerIdx < 0 {
		t.Fatal("banner row not found")
	}
	bb := items[bannerIdx]
	midY := int(bb.y + bb.h/2)
	lr, lg, lb, _ := got.At(2, midY).RGBA()
	rr, rg, rb, _ := got.At(int(W)-3, midY).RGBA()
	if lr/257 > 250 && lg/257 > 250 && lb/257 > 250 {
		t.Fatalf("banner left edge looks white, want tint to x=0")
	}
	if rr/257 > 250 && rg/257 > 250 && rb/257 > 250 {
		t.Fatalf("banner right edge looks white, want tint to full width")
	}

	path := filepath.Join("testdata", "showcase_alert.png")
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

func itKey(r, g, b uint32) string {
	return string([]byte{byte(r), byte(r >> 8), byte(g), byte(g >> 8), byte(b), byte(b >> 8)})
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
