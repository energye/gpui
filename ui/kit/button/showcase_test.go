package button_test

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/kit/button"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type showcaseTol struct {
	Tolerance struct {
		MaxDiff float64 `json:"maxDiff"`
		BadFrac float64 `json:"badFrac"`
		HardCap float64 `json:"hardCap"`
	} `json:"tolerance"`
}

func loadShowcaseTol(t *testing.T) showcaseTol {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "buttons.json"))
	if err != nil {
		t.Fatalf("read buttons.json: %v", err)
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

func loadShowcaseFaces(t *testing.T) (sm, md, lg text.Face) {
	t.Helper()
	text.ClearSystemFontPaths()
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
	return mk(12), mk(14), mk(16)
}

// TestButton_Showcase_MainPaths lays §6.8 main paths on one big canvas:
// Type row, Size row, Disabled / Loading / Icon / multiple / Ghost /
// Danger / Block / variant rows, plus P1 gradient / wave / spacing blocks.
// Three evidences: logic probe (variants/layout), pixel assertions (blue
// primary center, white center, dark text ink), golden file compare
// (tolerance from testdata/buttons.json). Regenerate with UPDATE_GOLDEN=1.
func TestButton_Showcase_MainPaths(t *testing.T) {
	tol := loadShowcaseTol(t)
	faceSM, faceMD, faceLG := loadShowcaseFaces(t)
	button.ResetGlobalConfig()
	defer button.ResetGlobalConfig()

	faceFor := func(b *button.Button) {
		switch b.Size() {
		case button.ButtonSmall:
			b.SetTextFace(faceSM)
		case button.ButtonLarge:
			b.SetTextFace(faceLG)
		default:
			b.SetTextFace(faceMD)
		}
	}
	mk := func(label string) *button.Button {
		b := button.NewButton(label)
		faceFor(b)
		return b
	}

	// Type row (5). Default/primary use two-Han labels so the button
	// center is the inter-char gap (background) for center probes.
	typeDefault := button.NewButton("确定")
	typeDefault.SetTextFace(faceMD)
	typePrimary := button.NewButton("确定")
	typePrimary.SetTextFace(faceMD)
	typePrimary.SetType(button.ButtonPrimary)
	typeDashed := mk("Dashed")
	typeDashed.SetType(button.ButtonDashed)
	typeText := mk("Text")
	typeText.SetType(button.ButtonText)
	typeLink := mk("Link")
	typeLink.SetType(button.ButtonLink)
	typeRow := []*button.Button{typeDefault, typePrimary, typeDashed, typeText, typeLink}

	// Size row (3).
	sizeSM := mk("Small")
	sizeSM.SetSize(button.ButtonSmall)
	sizeMD := mk("Middle")
	sizeLG := mk("Large")
	sizeLG.SetSize(button.ButtonLarge)
	faceFor(sizeSM)
	faceFor(sizeMD)
	faceFor(sizeLG)
	sizeRow := []*button.Button{sizeSM, sizeMD, sizeLG}

	// Disabled row (2).
	disDefault := mk("Disabled")
	disDefault.SetDisabled(true)
	disPrimary := mk("Disabled")
	disPrimary.SetType(button.ButtonPrimary)
	disPrimary.SetDisabled(true)
	disabledRow := []*button.Button{disDefault, disPrimary}

	// Loading row (2).
	loadSpin := mk("Loading")
	loadSpin.SetLoading(true)
	loadCustom := mk("Loading")
	loadCustom.SetLoadingConfig(button.LoadingConfig{Icon: "custom-spin"})
	loadingRow := []*button.Button{loadSpin, loadCustom}

	// Icon row (2).
	iconStart := mk("Search")
	iconStart.SetIcon("search")
	iconStart.SetIconPlacement(button.IconStart)
	iconEnd := mk("Search")
	iconEnd.SetIcon("search")
	iconEnd.SetIconPlacement(button.IconEnd)
	iconRow := []*button.Button{iconStart, iconEnd}

	// Multiple row (3, multiple.tsx horizontal combo).
	multiCancel := mk("Cancel")
	multiMore := mk("More")
	multiSubmit := mk("Submit")
	multiSubmit.SetType(button.ButtonPrimary)
	multiRow := []*button.Button{multiCancel, multiMore, multiSubmit}

	// Ghost row (2, dark strip behind).
	ghostDefault := mk("Ghost")
	ghostDefault.SetGhost(true)
	ghostPrimary := mk("Ghost")
	ghostPrimary.SetType(button.ButtonPrimary)
	ghostPrimary.SetGhost(true)
	ghostRow := []*button.Button{ghostDefault, ghostPrimary}

	// Danger row (2).
	dangerSolid := mk("Danger")
	dangerSolid.SetType(button.ButtonPrimary)
	dangerSolid.SetDanger(true)
	dangerPlain := mk("Danger")
	dangerPlain.SetDanger(true)
	dangerRow := []*button.Button{dangerSolid, dangerPlain}

	// Block row (1, full width).
	blockBtn := mk("Block Button")
	blockBtn.SetType(button.ButtonPrimary)
	blockBtn.SetBlock(true)

	// Variant row (6, colorVariant main path).
	varSolid := mk("Solid")
	varSolid.SetVariant(button.VariantSolid)
	varSolid.SetColor(button.ColorPrimary)
	varOutlined := mk("Outlined")
	varOutlined.SetVariant(button.VariantOutlined)
	varOutlined.SetColor(button.ColorPrimary)
	varDashed := mk("Dashed")
	varDashed.SetVariant(button.VariantDashed)
	varFilled := mk("Filled")
	varFilled.SetVariant(button.VariantFilled)
	varFilled.SetColor(button.ColorPrimary)
	varTextV := mk("Text")
	varTextV.SetVariant(button.VariantText)
	varLinkV := mk("Link")
	varLinkV.SetVariant(button.VariantLink)
	variantRow := []*button.Button{varSolid, varOutlined, varDashed, varFilled, varTextV, varLinkV}

	// P1 gradient block (1).
	gradBtn := mk("Gradient")
	gradBtn.SetType(button.ButtonPrimary)
	tok := theme.Default.Current()
	gradBtn.SetGradient(
		render.RGBA{R: tok.ColorPrimary.R, G: tok.ColorPrimary.G, B: tok.ColorPrimary.B, A: 1},
		render.RGBA{R: 0x72 / 255.0, G: 0x2e / 255.0, B: 0xd1 / 255.0, A: 1},
	)

	// P1 wave block (2, pressed with/without ripple).
	waveOn := mk("Wave On")
	waveOn.SetType(button.ButtonPrimary)
	waveOff := mk("Wave Off")
	waveOff.SetType(button.ButtonPrimary)
	waveOff.SetWaveDisabled(true)

	// P1 spacing block (2, 确定 spaced vs plain).
	spaceOn := button.NewButton("确定")
	spaceOn.SetTextFace(faceMD)
	spaceOff := button.NewButton("确定")
	spaceOff.SetTextFace(faceMD)
	spaceOff.SetAutoInsertSpace(false)
	spacingRow := []*button.Button{spaceOn, spaceOff}

	// Logic probe before paint.
	if typePrimary.EffectiveVariant() != button.VariantSolid {
		t.Fatal("primary type must be solid")
	}
	if sizeSM.Height() >= sizeMD.Height() || sizeMD.Height() >= sizeLG.Height() {
		t.Fatalf("size heights %v %v %v must ascend", sizeSM.Height(), sizeMD.Height(), sizeLG.Height())
	}
	if !disDefault.Disabled() || loadSpin.HasSpinner() != true {
		t.Fatal("disabled/loading probe")
	}
	if !iconStart.IconFirst() || iconEnd.IconFirst() {
		t.Fatal("icon placement probe")
	}
	if ghostDefault.Fill().A != 0 || !dangerSolid.Danger() {
		t.Fatal("ghost/danger probe")
	}
	if spaceOn.DisplayLabel() == spaceOff.DisplayLabel() {
		t.Fatal("spacing on/off must differ")
	}
	if !gradBtn.HasGradient() || waveOff.WaveDisabled() != true || waveOn.WaveActive() != true {
		t.Fatal("gradient/wave probe")
	}

	// Layout rows left-aligned with gaps; block spans full width.
	const W = 920.0
	const margin = 20.0
	const rowGap = 18.0
	const colGap = 12.0
	rowW := W - 2*margin

	type placed struct {
		b *button.Button
		x float64
		y float64
		w float64
		h float64
	}
	var items []placed
	var ghostBg struct{ x, y, w, h float64 }
	y := margin
	layoutRow := func(row []*button.Button) float64 {
		x := margin
		maxH := 0.0
		var sizes []struct {
			w, h float64
		}
		for _, b := range row {
			sz := b.Layout(rendering.Loose(1000, 1000))
			sizes = append(sizes, struct{ w, h float64 }{sz.Width, sz.Height})
			if sz.Height > maxH {
				maxH = sz.Height
			}
		}
		for i, b := range row {
			items = append(items, placed{b: b, x: x, y: y, w: sizes[i].w, h: sizes[i].h})
			x += sizes[i].w + colGap
		}
		y += maxH + rowGap
		return maxH
	}

	rows := [][]*button.Button{typeRow, sizeRow, disabledRow, loadingRow, iconRow, multiRow, ghostRow, dangerRow, variantRow, {gradBtn}, spacingRow}
	ghostRowIdx := 6
	for i, r := range rows {
		if i == ghostRowIdx {
			// Reserve dark strip behind the ghost row.
			maxH := 0.0
			for _, b := range r {
				sz := b.Layout(rendering.Loose(1000, 1000))
				if sz.Height > maxH {
					maxH = sz.Height
				}
			}
			ghostBg = struct{ x, y, w, h float64 }{0, y - 8, W, maxH + 16}
		}
		layoutRow(r)
	}
	// Block row spans.
	bsz := blockBtn.Layout(rendering.Constraints{MinWidth: rowW, MaxWidth: rowW, MaxHeight: rendering.Unbounded})
	items = append(items, placed{b: blockBtn, x: margin, y: y, w: bsz.Width, h: bsz.Height})
	y += bsz.Height + rowGap
	// Wave row needs pressed state before paint.
	waveRow := []*button.Button{waveOn, waveOff}
	x := margin
	maxH := 0.0
	var wsizes []struct {
		w, h float64
	}
	for _, b := range waveRow {
		sz := b.Layout(rendering.Loose(1000, 1000))
		wsizes = append(wsizes, struct{ w, h float64 }{sz.Width, sz.Height})
		if sz.Height > maxH {
			maxH = sz.Height
		}
	}
	for i, b := range waveRow {
		sz := rendering.Size{Width: wsizes[i].w, Height: wsizes[i].h}
		b.PointerDown(sz.Width/2, sz.Height/2)
		items = append(items, placed{b: b, x: x, y: y, w: sz.Width, h: sz.Height})
		x += sz.Width + colGap
	}
	y += maxH + rowGap
	H := y - rowGap + margin

	dc := render.NewContext(int(W), int(H))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	// Ghost dark strip (complex backdrop stand-in).
	dc.SetRGBA(0.12, 0.13, 0.16, 1)
	dc.DrawRectangle(ghostBg.x, ghostBg.y, ghostBg.w, ghostBg.h)
	_ = dc.Fill()
	for _, it := range items {
		it.b.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(it.x, it.y))
	}
	got := dc.Image()

	// Pixel assertion 1: blue primary center is the primary color.
	primAt := func(b *button.Button, px, py float64) (uint32, uint32, uint32) {
		r, g, bl, _ := got.At(int(px), int(py)).RGBA()
		return r / 257, g / 257, bl / 257
	}
	var primItem *placed
	for i := range items {
		if items[i].b == typePrimary {
			primItem = &items[i]
			break
		}
	}
	if primItem == nil {
		t.Fatal("primary item missing")
	}
	pr, pg, pb := primAt(typePrimary, primItem.x+primItem.w/2, primItem.y+primItem.h/2)
	// #1677ff -> (22,119,255); allow AA-free solid center tolerance.
	if !(pr < 60 && pg > 90 && pg < 150 && pb > 220) {
		t.Fatalf("blue primary center #%02x%02x%02x want ~#1677ff", pr, pg, pb)
	}

	// Pixel assertion 2: white (default) center is white/container.
	var defItem *placed
	for i := range items {
		if items[i].b == typeDefault {
			defItem = &items[i]
			break
		}
	}
	if defItem == nil {
		t.Fatal("default item missing")
	}
	dr, dg, db := primAt(typeDefault, defItem.x+defItem.w/2, defItem.y+defItem.h/2)
	if !(dr > 240 && dg > 240 && db > 240) {
		t.Fatalf("white button center #%02x%02x%02x want white", dr, dg, db)
	}

	// Pixel assertion 3: text zone carries dark glyph ink (real text).
	// Default button has dark text on white; primary text is inverse white.
	dark := 0
	x0, x1 := int(defItem.x+8), int(defItem.x+defItem.w-8)
	for yy := int(defItem.y + 4); yy < int(defItem.y+defItem.h-4); yy++ {
		for xx := x0; xx < x1; xx++ {
			r, g, bl, _ := got.At(xx, yy).RGBA()
			r8, g8, b8 := r/257, g/257, bl/257
			if r8 < 80 && g8 < 80 && b8 < 80 {
				dark++
			}
		}
	}
	if dark < 20 {
		t.Fatalf("default text zone dark=%d want >=20 (real glyphs missing?)", dark)
	}

	path := filepath.Join("testdata", "showcase_button.png")
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
	maxDiff := uint32(tol.Tolerance.MaxDiff * 257)
	hardCap := uint32(tol.Tolerance.HardCap * 257)
	bad := 0
	total := got.Bounds().Dx() * got.Bounds().Dy()
	for yy := 0; yy < got.Bounds().Dy(); yy++ {
		for xx := 0; xx < got.Bounds().Dx(); xx++ {
			r1, g1, b1, a1 := got.At(xx, yy).RGBA()
			r2, g2, b2, a2 := want.At(xx, yy).RGBA()
			m := max4s(diffS(r1, r2), diffS(g1, g2), diffS(b1, b2), diffS(a1, a2))
			if m > hardCap {
				t.Fatalf("showcase pixel (%d,%d) diff %d exceeds hard cap", xx, yy, m/257)
			}
			if m > maxDiff {
				bad++
			}
		}
	}
	if float64(bad)/float64(total) > tol.Tolerance.BadFrac {
		t.Fatalf("showcase bad pixels %d/%d exceed %.1f%%", bad, total, tol.Tolerance.BadFrac*100)
	}
}

func diffS(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func max4s(vs ...uint32) uint32 {
	m := vs[0]
	for _, v := range vs[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
