package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// Cards: 2x2 grid inside the shell body panel (body-relative coords).
const (
	cardW, cardH = 430.0, 232.0
)

var cardAt = [4][2]float64{{12, 44}, {462, 44}, {12, 296}, {462, 296}}

var (
	colCardBG = [3]float64{0.12, 0.135, 0.165}
	colTitle  = [3]float64{0.80, 0.86, 0.94}
	colSub    = [3]float64{0.64, 0.72, 0.84}
	colText   = [3]float64{0.94, 0.96, 1.00}
)

const (
	cardContentX = 16.0
	aSampleX     = 128.0
	bcdSampleX   = 146.0
	dSampleX     = 132.0
)

// vfRow is one visual line in a card and the matching ink-count region.
type vfRow struct {
	key, label, sample string
	face               text.Face
	baseline, sampleX  float64
	inkX, inkY         float64
	inkW, inkH         float64
}

type vfLayout struct {
	path       string
	source     *text.FontSource
	axis       text.VariationAxis
	axisTag    string
	axisFound  bool
	instances  []text.NamedInstance
	labelFace  text.Face
	aRows      []vfRow
	bRows      []vfRow
	cRows      []vfRow
	dRows      []vfRow
	fontSHA256 string
}

var lay vfLayout

// resolveVFFont keeps the window self-contained when run from the repository
// root (the documented command) and also works when run from this directory.
func resolveVFFont() (string, error) {
	candidates := []string{
		filepath.Join("examples", "ui_render_base_variablefont", "testdata", "SourceSans3VF-Upright.otf"),
		filepath.Join("testdata", "SourceSans3VF-Upright.otf"),
		filepath.Join("render", "text", "testdata", "source-sans", "VF", "SourceSans3VF-Upright.otf"),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("SourceSans3VF-Upright.otf not found (looked in %v)", candidates)
}

func buildVFLayout() error {
	path, err := resolveVFFont()
	if err != nil {
		return err
	}
	src, err := text.NewFontSourceFromFile(path)
	if err != nil {
		return fmt.Errorf("load %s: %w", path, err)
	}
	lay.path = path
	lay.source = src

	for _, ax := range src.VariationAxes() {
		if string(ax.Tag[:]) == "wght" {
			lay.axis, lay.axisTag, lay.axisFound = ax, "wght", true
			break
		}
	}
	if !lay.axisFound {
		return fmt.Errorf("%s has no wght axis (axes=%v)", path, src.VariationAxes())
	}
	lay.instances = src.NamedInstances()
	if len(lay.instances) == 0 {
		return fmt.Errorf("%s has no named instances", path)
	}

	lay.labelFace = src.Face(16)
	if lay.labelFace == nil {
		return fmt.Errorf("source.Face(16) returned nil")
	}

	// A: LoadFontFaceWithVariations contract is exercised in the logic probes;
	// the card uses the equivalent prebuilt faces so a persistent window does
	// not re-open the font file every frame.
	weightRows := []float32{
		clampWeight(lay.axis.Minimum, lay.axis),
		clampWeight(400, lay.axis),
		clampWeight(600, lay.axis),
		clampWeight(lay.axis.Maximum, lay.axis),
	}
	for i, w := range weightRows {
		baseline := 64.0 + float64(i)*40
		lay.aRows = append(lay.aRows, vfRow{
			key:      fmt.Sprintf("A_wght%.0f", w),
			label:    fmt.Sprintf("wght %.0f", w),
			sample:   "Variable",
			face:     src.Face(34, text.WithVariations(text.NewFontVariation("wght", w))),
			baseline: baseline, sampleX: aSampleX,
			inkX: aSampleX, inkY: baseline - 33, inkW: 290, inkH: 38,
		})
	}

	// B: named instances from the font's fvar table.
	selected := selectInstances(lay.instances, []string{"ExtraLight", "Regular", "Semibold", "Black"}, 4)
	for i, inst := range selected {
		baseline := 64.0 + float64(i)*42
		lay.bRows = append(lay.bRows, vfRow{
			key:      "B_" + sanitizeKey(inst.Name),
			label:    inst.Name,
			sample:   "Variable",
			face:     src.Face(30, text.WithVariations(inst.Variations...)),
			baseline: baseline, sampleX: bcdSampleX,
			inkX: bcdSampleX, inkY: baseline - 29, inkW: 270, inkH: 35,
		})
	}

	// C: per-face independence and the explicit axis-default equality.
	cRows := []struct {
		key, label string
		face       text.Face
	}{
		{"C_default", fmt.Sprintf("default %.0f", lay.axis.Default), src.Face(30)},
		{"C_explicit", fmt.Sprintf("explicit %.0f", lay.axis.Default), src.Face(30,
			text.WithVariations(text.NewFontVariation(lay.axisTag, lay.axis.Default)))},
		{"C_black", fmt.Sprintf("black %.0f", lay.axis.Maximum), src.Face(30,
			text.WithVariations(text.NewFontVariation(lay.axisTag, lay.axis.Maximum)))},
	}
	for i, r := range cRows {
		baseline := 76.0 + float64(i)*48
		lay.cRows = append(lay.cRows, vfRow{
			key: r.key, label: r.label, sample: "Variable", face: r.face,
			baseline: baseline, sampleX: bcdSampleX,
			inkX: bcdSampleX, inkY: baseline - 29, inkW: 270, inkH: 35,
		})
	}

	// D: spelling variants around the axis bounds and an unknown axis.
	dRows := []struct {
		key, label string
		face       text.Face
	}{
		{"D_min", fmt.Sprintf("min %.0f", lay.axis.Minimum), src.Face(27,
			text.WithVariations(text.NewFontVariation(lay.axisTag, lay.axis.Minimum)))},
		{"D_clamp_low", "clamp low", src.Face(27,
			text.WithVariations(text.NewFontVariation(lay.axisTag, lay.axis.Minimum-100)))},
		{"D_max", fmt.Sprintf("max %.0f", lay.axis.Maximum), src.Face(27,
			text.WithVariations(text.NewFontVariation(lay.axisTag, lay.axis.Maximum)))},
		{"D_clamp_high", "clamp high", src.Face(27,
			text.WithVariations(text.NewFontVariation(lay.axisTag, lay.axis.Maximum+100)))},
		{"D_default", fmt.Sprintf("default %.0f", lay.axis.Default), src.Face(27)},
		{"D_wdth", "wdth 75", src.Face(27,
			text.WithVariations(text.NewFontVariation("wdth", 75)))},
	}
	for i, r := range dRows {
		baseline := 58.0 + float64(i)*28
		lay.dRows = append(lay.dRows, vfRow{
			key: r.key, label: r.label, sample: "Variable", face: r.face,
			baseline: baseline, sampleX: dSampleX,
			inkX: dSampleX, inkY: baseline - 25, inkW: 290, inkH: 31,
		})
	}

	return nil
}

func clampWeight(v float32, ax text.VariationAxis) float32 {
	if v < ax.Minimum {
		return ax.Minimum
	}
	if v > ax.Maximum {
		return ax.Maximum
	}
	return v
}

func selectInstances(all []text.NamedInstance, names []string, want int) []text.NamedInstance {
	used := make(map[int]bool)
	out := make([]text.NamedInstance, 0, want)
	for _, name := range names {
		for i, inst := range all {
			if !used[i] && inst.Name == name {
				out = append(out, inst)
				used[i] = true
				break
			}
		}
		if len(out) >= want {
			return out
		}
	}
	for i, inst := range all {
		if len(out) >= want {
			break
		}
		if !used[i] {
			out = append(out, inst)
			used[i] = true
		}
	}
	return out
}

func sanitizeKey(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			out = append(out, r)
		} else {
			out = append(out, '_')
		}
	}
	return string(out)
}

// ---------------------------------------------------------------------------
// Logic probes
// ---------------------------------------------------------------------------

var vfProbeKeys = []string{
	"font_path_exists",
	"source_is_variable",
	"axis_wght_range",
	"axis_default_in_range",
	"named_instances_present",
	"instance_values_in_range",
	"loadface_sets_axes",
	"loadface_no_vars_axes",
	"face_variations_exact",
	"face_source_identity",
	"advance_monotonic",
	"default_matches_axis_default",
	"empty_variations_matches_default",
	"clamp_low_matches_min",
	"clamp_high_matches_max",
	"unknown_axis_ignored",
	"named_instance_faces",
	"glyph_coverage",
}

func runVFLogic() (ran, failed []string) {
	add := func(key string, ok bool) {
		ran = append(ran, key)
		if !ok {
			failed = append(failed, key)
		}
	}
	add("font_path_exists", lay.path != "" && fileExists(lay.path))
	add("source_is_variable", lay.source != nil && lay.source.IsVariable())
	add("axis_wght_range", lay.axisFound && lay.axisTag == "wght" &&
		near32(lay.axis.Minimum, 200) && near32(lay.axis.Default, 200) && near32(lay.axis.Maximum, 900))
	add("axis_default_in_range", lay.axisFound &&
		lay.axis.Minimum <= lay.axis.Default && lay.axis.Default <= lay.axis.Maximum)

	namesOK := len(lay.instances) >= 4
	if namesOK {
		for _, inst := range lay.instances {
			if len(inst.Variations) == 0 {
				namesOK = false
			}
		}
	}
	add("named_instances_present", namesOK)

	inRange := true
	for _, inst := range lay.instances {
		for _, v := range inst.Variations {
			if string(v.Tag[:]) == lay.axisTag && (v.Value < lay.axis.Minimum || v.Value > lay.axis.Maximum) {
				inRange = false
			}
		}
	}
	add("instance_values_in_range", inRange)

	dc := render.NewContext(16, 16)
	defer dc.Close()
	err := dc.LoadFontFaceWithVariations(lay.path, 24,
		text.NewFontVariation(lay.axisTag, 700))
	axes := dc.FontVariationAxes()
	add("loadface_sets_axes", err == nil && len(axes) == 1 &&
		string(axes[0].Tag[:]) == lay.axisTag &&
		near32(axes[0].Minimum, lay.axis.Minimum) &&
		near32(axes[0].Default, lay.axis.Default) &&
		near32(axes[0].Maximum, lay.axis.Maximum))

	dcNoVars := render.NewContext(16, 16)
	defer dcNoVars.Close()
	err = dcNoVars.LoadFontFaceWithVariations(lay.path, 24)
	axesNoVars := dcNoVars.FontVariationAxes()
	add("loadface_no_vars_axes", err == nil && len(axesNoVars) == 1 &&
		string(axesNoVars[0].Tag[:]) == lay.axisTag)

	face700 := lay.source.Face(24, text.WithVariations(text.NewFontVariation(lay.axisTag, 700)))
	add("face_variations_exact", face700 != nil &&
		sameVariations(face700.Variations(), []text.FontVariation{{Tag: lay.axis.Tag, Value: 700}}))
	add("face_source_identity", face700 != nil && face700.Source() == lay.source)

	adv := func(v float32) float64 {
		f := lay.source.Face(24, text.WithVariations(text.NewFontVariation(lay.axisTag, v)))
		if f == nil {
			return math.NaN()
		}
		return f.Advance("Variable")
	}
	aMin, aMid1, aMid2, aMax := adv(lay.axis.Minimum), adv(400), adv(600), adv(lay.axis.Maximum)
	add("advance_monotonic", aMin < aMid1 && aMid1 < aMid2 && aMid2 < aMax)

	defAdv := lay.source.Face(24).Advance("Variable")
	expDefAdv := adv(lay.axis.Default)
	add("default_matches_axis_default", math.Abs(defAdv-expDefAdv) < 0.01)
	add("empty_variations_matches_default",
		math.Abs(lay.source.Face(24, text.WithVariations()).Advance("Variable")-defAdv) < 0.01)
	add("clamp_low_matches_min", math.Abs(adv(lay.axis.Minimum-100)-aMin) < 0.01)
	add("clamp_high_matches_max", math.Abs(adv(lay.axis.Maximum+100)-aMax) < 0.01)
	add("unknown_axis_ignored",
		math.Abs(lay.source.Face(24, text.WithVariations(text.NewFontVariation("wdth", 75))).
			Advance("Variable")-defAdv) < 0.01)

	namedOK := len(selectInstances(lay.instances, []string{"ExtraLight", "Regular", "Semibold", "Black"}, 4)) == 4
	for _, inst := range lay.instances {
		f := lay.source.Face(24, text.WithVariations(inst.Variations...))
		if f == nil || !sameVariations(f.Variations(), inst.Variations) {
			namedOK = false
		}
	}
	add("named_instance_faces", namedOK)

	glyphOK := true
	for _, r := range "Variable" {
		if !lay.source.Face(24).HasGlyph(r) {
			glyphOK = false
		}
	}
	add("glyph_coverage", glyphOK)

	return ran, failed
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func near32(a, b float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 0.01
}

func sameVariations(a, b []text.FontVariation) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Tag != b[i].Tag || math.Abs(float64(a[i].Value-b[i].Value)) > 0.001 {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Pixel probes and scene
// ---------------------------------------------------------------------------

type pointProbe struct {
	key, desc string
	card      int
	rect      rect
	want      [3]float64
	tol       float64
}

type inkProbe struct {
	key, desc string
	card      int
	rect      rect
	min       int
	// greaterThan requires n > previous count + margin.
	greaterThan string
	margin      int
	// equalTo requires |n - previous count| <= equalTol.
	equalTo  string
	equalTol int
}

func vfPointProbes() []pointProbe {
	var out []pointProbe
	keys := []string{"A_bg", "B_bg", "C_bg", "D_bg"}
	desc := []string{"A blank card BG", "B blank card BG", "C blank card BG", "D blank card BG"}
	for i := range keys {
		out = append(out, pointProbe{
			key: keys[i], desc: desc[i], card: i,
			rect: rect{x: 416, y: 224, w: 1, h: 1},
			want: colCardBG, tol: 8.0 / 255,
		})
	}
	return out
}

func vfInkProbes() []inkProbe {
	var out []inkProbe
	prev := ""
	for _, r := range lay.aRows {
		out = append(out, inkProbe{
			key: r.key, desc: "wght ramp ink", card: 0,
			rect: rect{x: r.inkX, y: r.inkY, w: r.inkW, h: r.inkH},
			min:  60, greaterThan: prev, margin: 5,
		})
		prev = r.key
	}
	prev = ""
	for _, r := range lay.bRows {
		out = append(out, inkProbe{
			key: r.key, desc: "named instance ink", card: 1,
			rect: rect{x: r.inkX, y: r.inkY, w: r.inkW, h: r.inkH},
			min:  60, greaterThan: prev, margin: 5,
		})
		prev = r.key
	}
	out = append(out,
		inkProbe{key: "C_default", desc: "axis default ink", card: 2,
			rect: rect{x: lay.cRows[0].inkX, y: lay.cRows[0].inkY, w: lay.cRows[0].inkW, h: lay.cRows[0].inkH}, min: 60},
		inkProbe{key: "C_explicit", desc: "explicit default identical", card: 2,
			rect: rect{x: lay.cRows[1].inkX, y: lay.cRows[1].inkY, w: lay.cRows[1].inkW, h: lay.cRows[1].inkH}, min: 60,
			equalTo: "C_default", equalTol: 3},
		inkProbe{key: "C_black", desc: "black heavier than default", card: 2,
			rect: rect{x: lay.cRows[2].inkX, y: lay.cRows[2].inkY, w: lay.cRows[2].inkW, h: lay.cRows[2].inkH}, min: 60,
			greaterThan: "C_default", margin: 5},
	)
	dByKey := map[string]vfRow{}
	for _, r := range lay.dRows {
		dByKey[r.key] = r
	}
	addD := func(key, desc, equalTo string) {
		r := dByKey[key]
		out = append(out, inkProbe{
			key: key, desc: desc, card: 3,
			rect: rect{x: r.inkX, y: r.inkY, w: r.inkW, h: r.inkH},
			min:  40, equalTo: equalTo, equalTol: 3,
		})
	}
	addD("D_min", "axis minimum ink", "")
	addD("D_clamp_low", "below-min clamp identical", "D_min")
	addD("D_max", "axis maximum ink", "")
	addD("D_clamp_high", "above-max clamp identical", "D_max")
	addD("D_default", "axis default ink", "")
	addD("D_wdth", "unknown axis ignored", "D_default")
	return out
}

func buildVFScene(shell *wrkit.ShellChrome) {
	cardWithBoundary := func(i int, title string, paint func(*rendering.PaintContext, rendering.Size)) {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.SetRepaintBoundary(true)
		box.OnPaint = paint
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, colTitle[0], colTitle[1], colTitle[2]),
			cardAt[i][0]+12, cardAt[i][1]+10)
	}
	cardWithBoundary(0, "A  LoadFontFaceWithVariations — wght ramp", paintVFA)
	cardWithBoundary(1, "B  named instances — source.Face + WithVariations", paintVFB)
	cardWithBoundary(2, "C  per-face independence / axis default", paintVFC)
	cardWithBoundary(3, "D  clamp below/above · unknown axis", paintVFD)

}

func paintVFA(pc *rendering.PaintContext, sz rendering.Size) {
	paintVFRows(pc, lay.aRows)
}

func paintVFB(pc *rendering.PaintContext, sz rendering.Size) {
	paintVFRows(pc, lay.bRows)
}

func paintVFC(pc *rendering.PaintContext, sz rendering.Size) {
	paintVFRows(pc, lay.cRows)
}

func paintVFD(pc *rendering.PaintContext, sz rendering.Size) {
	paintVFRows(pc, lay.dRows)
}

func paintVFRows(pc *rendering.PaintContext, rows []vfRow) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	dc.SetRGBA(colCardBG[0], colCardBG[1], colCardBG[2], 1)
	dc.DrawRectangle(ox, oy, cardW, cardH)
	_ = dc.Fill()

	for _, row := range rows {
		dc.SetFont(lay.labelFace)
		dc.SetRGBA(colSub[0], colSub[1], colSub[2], 1)
		dc.DrawString(row.label, ox+cardContentX, oy+row.baseline)

		dc.SetFont(row.face)
		dc.SetRGBA(colText[0], colText[1], colText[2], 1)
		dc.DrawString(row.sample, ox+row.sampleX, oy+row.baseline)
	}
}
