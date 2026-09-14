package tag_test

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/kit/tag"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type tagShowcaseSpec struct {
	CanvasW   int     `json:"canvasW"`
	Margin    float64 `json:"margin"`
	Gap       float64 `json:"gap"`
	ColGap    float64 `json:"colGap"`
	Tolerance struct {
		MaxDiff float64 `json:"maxDiff"`
		BadFrac float64 `json:"badFrac"`
		HardCap float64 `json:"hardCap"`
	} `json:"tolerance"`
}

func loadTagShowcaseSpec(t *testing.T) tagShowcaseSpec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "showcase_spec.json"))
	if err != nil {
		t.Fatalf("read showcase_spec.json: %v", err)
	}
	var s tagShowcaseSpec
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse showcase spec: %v", err)
	}
	if s.CanvasW <= 0 || s.Gap < 0 || s.Tolerance.MaxDiff <= 0 {
		t.Fatalf("bad showcase spec %+v", s)
	}
	return s
}

func loadTagShowcaseFace(t *testing.T) text.Face {
	t.Helper()
	text.ClearSystemFontPaths()
	face, desc, err := rendering.TryLoadDefaultFace(12)
	if err != nil || face == nil {
		t.Skipf("showcase needs a system face for real glyphs: %v", err)
	}
	t.Logf("showcase face: %s", desc)
	return face
}

// tagWidget is the layout/paint surface shared by Tag, CheckableTag and
// CheckableTagGroup rows on the showcase canvas.
type tagWidget interface {
	Layout(rendering.Constraints) rendering.Size
	Node() rendering.RenderObject
}

func faceTagWidget(w tagWidget, face text.Face) {
	switch v := w.(type) {
	case *tag.Tag:
		v.SetFace(face)
	case *tag.CheckableTag:
		v.SetFace(face)
	case *tag.CheckableTagGroup:
		for _, ct := range v.Tags() {
			ct.SetFace(face)
		}
	}
}

// TestTag_Showcase_MainPaths lays the §6.8 P0 main paths on one big canvas:
// Basic / Colorful / Closable / Icon / Status / Checkable / Disabled, one
// row each. Three evidences: logic probe (variants/chrome/selection before
// paint), pixel assertions (dark glyph ink in the basic title zone, tinted
// and distinct colorful shells), golden file compare (tolerance from
// testdata/showcase_spec.json). Regenerate with UPDATE_GOLDEN=1.
func TestTag_Showcase_MainPaths(t *testing.T) {
	spec := loadTagShowcaseSpec(t)
	face := loadTagShowcaseFace(t)

	W := float64(spec.CanvasW)
	margin, gap, colGap := spec.Margin, spec.Gap, spec.ColGap
	rowW := W - 2*margin

	// R1 Basic (basic.tsx): plain tags plus a clickable link tag.
	basicPlain := tag.NewTag("Tag 1")
	basicPlain2 := tag.NewTag("Tag 2")
	basicLink := tag.NewTag("Link")
	linkClicks := 0
	basicLink.SetOnClick(func() { linkClicks++ })
	basicRow := []tagWidget{basicPlain, basicPlain2, basicLink}

	// R2 Colorful (colorful.tsx): presets in filled plus solid/outlined
	// samples and custom hex tags.
	presetNames := []string{"red", "blue", "green", "orange", "purple", "gold"}
	var colorfulRow []tagWidget
	var colorfulFilled []*tag.Tag
	for _, p := range presetNames {
		tg := tag.NewTag(p)
		tg.SetColor(p)
		colorfulFilled = append(colorfulFilled, tg)
		colorfulRow = append(colorfulRow, tg)
	}
	solidRed := tag.NewTag("red solid")
	solidRed.SetVariant(tag.TagVariantSolid)
	solidRed.SetColor("red")
	outlinedBlue := tag.NewTag("blue line")
	outlinedBlue.SetVariant(tag.TagVariantOutlined)
	outlinedBlue.SetColor("blue")
	hexA := tag.NewTag("#f50")
	hexA.SetColor("#f50")
	hexB := tag.NewTag("#2db7f5")
	hexB.SetColor("#2db7f5")
	colorfulRow = append(colorfulRow, solidRed, outlinedBlue, hexA, hexB)

	// R3 Closable (control.tsx end-state): closable tags in three skins.
	closePlain := tag.NewTag("Deletable")
	closePlain.SetClosable(true)
	closeRed := tag.NewTag("Red")
	closeRed.SetColor("red")
	closeRed.SetClosable(true)
	closeLine := tag.NewTag("Outlined")
	closeLine.SetVariant(tag.TagVariantOutlined)
	closeLine.SetClosable(true)
	closableRow := []tagWidget{closePlain, closeRed, closeLine}

	// R4 Icon (icon.tsx): tag icons plus a checkable icon.
	iconTag := tag.NewTag("Icon tag")
	iconTag.SetIcon("check")
	iconColor := tag.NewTag("Color icon")
	iconColor.SetIcon("star")
	iconColor.SetColor("blue")
	iconCheck := tag.NewCheckableTag("Icon check")
	iconCheck.SetIcon("star")
	iconRow := []tagWidget{iconTag, iconColor, iconCheck}

	// R5 Status (status.tsx): statuses in filled plus solid/outlined samples.
	statusNames := []string{"success", "processing", "error", "warning"}
	var statusFilled []*tag.Tag
	var statusRow []tagWidget
	for _, s := range statusNames {
		tg := tag.NewTag(s)
		tg.SetColor(s)
		statusFilled = append(statusFilled, tg)
		statusRow = append(statusRow, tg)
	}
	statusSolid := tag.NewTag("error solid")
	statusSolid.SetVariant(tag.TagVariantSolid)
	statusSolid.SetColor("error")
	statusOutlined := tag.NewTag("success line")
	statusOutlined.SetVariant(tag.TagVariantOutlined)
	statusOutlined.SetColor("success")
	statusRow = append(statusRow, statusSolid, statusOutlined)

	// R6 Checkable (checkable.tsx): unchecked/checked plus single and
	// multi groups with deterministic pre-selection.
	checkOff := tag.NewCheckableTag("Unchecked")
	checkOn := tag.NewCheckableTag("Checked")
	checkOn.SetChecked(true)
	groupSingle := tag.NewCheckableTagGroup(
		tag.TagOption{Label: "Movies", Value: "movies"},
		tag.TagOption{Label: "Books", Value: "books"},
		tag.TagOption{Label: "Music", Value: "music"},
	)
	groupSingle.SetValue("books")
	groupMulti := tag.NewCheckableTagGroup(
		tag.TagOption{Label: "Apple", Value: "apple"},
		tag.TagOption{Label: "Pear", Value: "pear"},
		tag.TagOption{Label: "Orange", Value: "orange"},
	)
	groupMulti.SetMultiple(true)
	groupMulti.SetValues([]string{"apple", "orange"})
	checkableRow := []tagWidget{checkOff, checkOn, groupSingle, groupMulti}

	// R7 Disabled (disabled.tsx doc page, TAG-18 look): disabled tag and
	// disabled checkable swallow interaction.
	disabledTag := tag.NewTag("Disabled")
	disabledTag.SetDisabled(true)
	disabledTag.SetClosable(true)
	disabledCheck := tag.NewCheckableTag("No pick")
	disabledCheck.SetDisabled(true)
	disabledRow := []tagWidget{disabledTag, disabledCheck}

	rows := [][]tagWidget{basicRow, colorfulRow, closableRow, iconRow, statusRow, checkableRow, disabledRow}
	for _, r := range rows {
		for _, w := range r {
			faceTagWidget(w, face)
		}
	}

	// Logic probe before paint: variants, chrome, selection, disabled.
	if basicLink.Role() != "button" || !basicLink.Focusable() {
		t.Fatal("clickable basic tag must be button focusable")
	}
	if basicPlain.Role() != "" || basicPlain.Focusable() {
		t.Fatal("static basic tag carries no role")
	}
	basicLink.Click()
	if linkClicks != 1 {
		t.Fatalf("link clicks=%d want 1", linkClicks)
	}
	for _, tg := range colorfulFilled {
		if tg.EffectiveBorderWidth() != 0 {
			t.Fatalf("%s filled border=%v want 0", tg.Label(), tg.EffectiveBorderWidth())
		}
	}
	def := tag.NewTag("Tag").EffectiveChrome()
	for _, tg := range colorfulFilled {
		if got := tg.EffectiveChrome(); got.Bg == def.Bg && got.Text == def.Text {
			t.Fatalf("%s skin must differ from default gray", tg.Label())
		}
	}
	if got := solidRed.EffectiveChrome(); got.Bg != render.Hex("#f5222d") {
		t.Fatalf("solid red bg=%+v want preset base", got.Bg)
	}
	if outlinedBlue.EffectiveBorderWidth() != 1 {
		t.Fatalf("outlined border=%v want 1", outlinedBlue.EffectiveBorderWidth())
	}
	for _, tg := range []*tag.Tag{closePlain, closeRed, closeLine} {
		if !tg.HasClose() || tg.CloseNode() == nil {
			t.Fatalf("%s close affordance missing", tg.Label())
		}
	}
	for _, w := range []tagWidget{iconTag, iconColor, iconCheck} {
		var has bool
		switch v := w.(type) {
		case *tag.Tag:
			has = v.HasIcon()
		case *tag.CheckableTag:
			has = v.HasIcon()
		}
		if !has {
			t.Fatal("icon row widget missing icon")
		}
	}
	tok := theme.Default.Current()
	statusTone := map[string]render.RGBA{
		"success": {R: tok.ColorSuccess.R, G: tok.ColorSuccess.G, B: tok.ColorSuccess.B, A: tok.ColorSuccess.A},
		"error":   {R: tok.ColorError.R, G: tok.ColorError.G, B: tok.ColorError.B, A: tok.ColorError.A},
		"warning": {R: tok.ColorWarning.R, G: tok.ColorWarning.G, B: tok.ColorWarning.B, A: tok.ColorWarning.A},
	}
	for i, tg := range statusFilled {
		name := statusNames[i]
		if name == "processing" {
			continue
		}
		if ch := tg.EffectiveChrome(); ch.Text != statusTone[name] {
			t.Fatalf("%s filled text=%+v want %+v", name, ch.Text, statusTone[name])
		}
	}
	if !checkOn.Checked() || checkOff.Checked() {
		t.Fatal("checkable preselection probe")
	}
	if groupSingle.Value() != "books" {
		t.Fatalf("single value=%q want books", groupSingle.Value())
	}
	if got := groupMulti.Values(); len(got) != 2 || got[0] != "apple" || got[1] != "orange" {
		t.Fatalf("multi values=%v want [apple orange]", got)
	}
	// Toggle on throwaway instances so painted rows keep their skin.
	probeGroup := tag.NewCheckableTagGroup(tag.TagOption{Label: "A", Value: "a"})
	probeGroup.PressOption("a")
	if probeGroup.Value() != "a" {
		t.Fatal("group toggle probe")
	}
	if disabledTag.Focusable() || disabledCheck.Focusable() {
		t.Fatal("disabled must not take focus")
	}
	probeDisabled := tag.NewTag("Probe")
	probeDisabled.SetDisabled(true)
	probeDisabled.SetClosable(true)
	probeDisabled.Close()
	if probeDisabled.Hidden() {
		t.Fatal("disabled must swallow close")
	}
	_ = rowW

	// Layout every row left-aligned; contract: Loose layout leaves a
	// non-zero Node size on every widget.
	type placed struct {
		w tagWidget
		x float64
		y float64
		s rendering.Size
	}
	var items []placed
	y := margin
	for ri, r := range rows {
		x := margin
		maxH := 0.0
		var sizes []rendering.Size
		for _, w := range r {
			sz := w.Layout(rendering.Loose(2000, 2000))
			if sz.Width <= 0 || sz.Height <= 0 {
				t.Fatalf("row %d layout=%v must be non-zero", ri, sz)
			}
			sizes = append(sizes, sz)
			if sz.Height > maxH {
				maxH = sz.Height
			}
		}
		for i, w := range r {
			if ns := w.Node().Size(); ns.Width <= 0 || ns.Height <= 0 {
				t.Fatalf("row %d node size=%v must be non-zero after Layout", ri, ns)
			}
			if x+sizes[i].Width > W-margin+1 {
				t.Fatalf("row %d overflows canvas width (widen canvasW)", ri)
			}
			items = append(items, placed{w: w, x: x, y: y, s: sizes[i]})
			x += sizes[i].Width + colGap
		}
		y += maxH + gap
	}
	H := y - gap + margin

	dc := render.NewContext(int(W), int(H))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	for _, it := range items {
		it.w.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(it.x, it.y))
	}
	got := dc.Image()

	// Pixel assertion 1: basic title zone carries dark glyph ink
	// (real text, no black bars).
	b0 := items[0]
	dark := 0
	x0, x1 := int(b0.x+7), int(b0.x+b0.s.Width-7)
	for yy := int(b0.y + 3); yy < int(b0.y+b0.s.Height-3); yy++ {
		for xx := x0; xx < x1; xx++ {
			r, g, b, _ := got.At(xx, yy).RGBA()
			if r/257 < 110 && g/257 < 110 && b/257 < 110 {
				dark++
			}
		}
	}
	if dark < 30 {
		t.Fatalf("basic title zone dark pixels=%d want >=30 (real glyphs missing?)", dark)
	}

	// Pixel assertion 2: colorful shells are tinted and distinct.
	shellKey := func(tg *tag.Tag) string {
		var px, py float64
		for _, it := range items {
			if it.w == tg {
				px, py = it.x+3, it.y+it.s.Height/2
				break
			}
		}
		r, g, b, _ := got.At(int(px), int(py)).RGBA()
		r8, g8, b8 := r/257, g/257, b/257
		if r8 > 250 && g8 > 250 && b8 > 250 {
			t.Fatalf("%s shell #%02x%02x%02x looks white, want tint", tg.Label(), r8, g8, b8)
		}
		return string([]byte{byte(r8), byte(g8), byte(b8)})
	}
	seen := map[string]bool{}
	for _, tg := range colorfulFilled {
		seen[shellKey(tg)] = true
	}
	if len(seen) != len(colorfulFilled) {
		t.Fatalf("colorful shells distinct=%d want %d", len(seen), len(colorfulFilled))
	}

	path := filepath.Join("testdata", "showcase_tag.png")
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
			m := max4tag(diffTag(r1, r2), diffTag(g1, g2), diffTag(b1, b2), diffTag(a1, a2))
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

func diffTag(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func max4tag(vs ...uint32) uint32 {
	m := vs[0]
	for _, v := range vs[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
