package typography_test

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/kit/typography"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
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
	if s.CanvasW <= 0 || s.Gap < 0 || s.Tolerance.MaxDiff <= 0 {
		t.Fatalf("bad showcase spec %+v", s)
	}
	return s
}

func loadShowcaseFace(t *testing.T, pts float64) text.Face {
	t.Helper()
	face, _, err := text.LoadMultiFace(pts)
	if err == nil && face != nil {
		return face
	}
	face2, _, err2 := rendering.TryLoadDefaultFace(pts)
	if err2 != nil || face2 == nil {
		t.Skipf("showcase needs a system face @%.0fpt for real glyphs: %v / %v", pts, err, err2)
	}
	return face2
}

// TestTypography_Showcase_P0P1 lays §6.8 P0 rows plus P1 suffix/style rows on
// one canvas: Text/Title h1..h5/Paragraph/Link, four types, disabled, seven
// modifiers, copyable+icon, editable, ellipsis rows/expandable/controlled/
// middle/suffix, placement start/end. Three evidences: logic probe
// (ladder/colors/ellipsis/placement), pixel assertions (ink + washes),
// golden compare (tolerance from testdata/showcase_spec.json).
// Regenerate with UPDATE_GOLDEN=1. Real glyphs via SetFace; without a face
// the test Skips instead of drawing bars.
func TestTypography_Showcase_P0P1(t *testing.T) {
	spec := loadShowcaseSpec(t)
	f := loadTypo(t)
	text.ClearSystemFontPaths()

	faceCache := map[float64]text.Face{}
	faceFor := func(pts float64) text.Face {
		if ff, ok := faceCache[pts]; ok {
			return ff
		}
		ff := loadShowcaseFace(t, pts)
		faceCache[pts] = ff
		return ff
	}
	mk := func(tp *typography.Typography) *typography.Typography {
		tp.SetFace(faceFor(tp.EffectiveFontSize()))
		return tp
	}

	basic := mk(typography.NewText(f.Samples.Basic))
	h1 := mk(typography.NewTitle(f.Samples.Title, 1))
	h2 := mk(typography.NewTitle(f.Samples.Title, 2))
	h3 := mk(typography.NewTitle(f.Samples.Title, 3))
	h4 := mk(typography.NewTitle(f.Samples.Title, 4))
	h5 := mk(typography.NewTitle(f.Samples.Title, 5))
	para := mk(typography.NewParagraph(f.Samples.Paragraph))
	link := mk(typography.NewLink(f.Samples.Link))
	linkClicks := 0
	link.SetOnClick(func() { linkClicks++ })
	secondary := mk(typography.NewText(f.Samples.Basic))
	secondary.SetType(typography.TypeSecondary)
	success := mk(typography.NewText(f.Samples.Basic))
	success.SetType(typography.TypeSuccess)
	warning := mk(typography.NewText(f.Samples.Basic))
	warning.SetType(typography.TypeWarning)
	danger := mk(typography.NewText(f.Samples.Basic))
	danger.SetType(typography.TypeDanger)
	disabled := mk(typography.NewText(f.Samples.Basic))
	disabled.SetDisabled(true)
	strong := mk(typography.NewText(f.Samples.Basic))
	strong.SetStrong(true)
	italic := mk(typography.NewText(f.Samples.Basic))
	italic.SetItalic(true)
	underline := mk(typography.NewText(f.Samples.Basic))
	underline.SetUnderline(true)
	deleted := mk(typography.NewText(f.Samples.Basic))
	deleted.SetDelete(true)
	code := mk(typography.NewText(f.Samples.Basic))
	code.SetCode(true)
	marked := mk(typography.NewText(f.Samples.Basic))
	marked.SetMark(true)
	kbd := mk(typography.NewText(f.Samples.Basic))
	kbd.SetKeyboard(true)
	copyable := mk(typography.NewText(f.Samples.Copy))
	copyable.SetCopyable(true)
	copyable.SetCopyIcon("copy")
	var copied []string
	copyable.OnCopy = func(s string) { copied = append(copied, s) }
	editable := mk(typography.NewText(f.Samples.EditOld))
	editable.SetEditable(true)
	ell1 := mk(typography.NewText(f.Samples.Long))
	ell1.SetEllipsis(true)
	ell1.SetEllipsisRows(1)
	ell2 := mk(typography.NewText(f.Samples.Long))
	ell2.SetEllipsis(true)
	ell2.SetEllipsisRows(2)
	expandable := mk(typography.NewText(f.Samples.Long))
	expandable.SetEllipsis(true)
	expandable.SetExpandable(true)
	expandable.SetCollapsible(true)
	controlled := mk(typography.NewText(f.Samples.Long))
	controlled.SetEllipsis(true)
	controlled.SetExpandable(true)
	controlled.SetCollapsible(true)
	expandCalls := 0
	controlled.OnExpand = func(bool) { expandCalls++ }
	controlled.SetExpanded(true)
	middle := mk(typography.NewText(f.Samples.Long))
	middle.SetEllipsis(true)
	middle.SetEllipsisRows(1)
	middle.SetEllipsisMiddle(true)
	middle.SetSuffix(f.Samples.MiddleTail)
	suffixRow := mk(typography.NewText(f.Samples.Long))
	suffixRow.SetEllipsis(true)
	suffixRow.SetEllipsisRows(1)
	suffixRow.SetSuffix(f.Samples.MiddleTail)
	placeStart := mk(typography.NewText(f.Samples.Basic))
	placeStart.SetCopyable(true)
	placeStart.SetEditable(true)
	placeStart.SetActionsPlacement(typography.PlacementStart)
	placeEnd := mk(typography.NewText(f.Samples.Basic))
	placeEnd.SetCopyable(true)
	placeEnd.SetEditable(true)
	placeEnd.SetActionsPlacement(typography.PlacementEnd)
	styled := mk(typography.NewText(f.Samples.Basic))
	styled.SetStyle(typography.Style{Text: render.RGBA{R: 0.2, G: 0.4, B: 0.8, A: 1}, UseText: true})

	rows := []*typography.Typography{basic, h1, h2, h3, h4, h5, para, link,
		secondary, success, warning, danger, disabled, strong, italic,
		underline, deleted, code, marked, kbd, copyable, editable,
		ell1, ell2, expandable, controlled, middle, suffixRow,
		placeStart, placeEnd, styled}

	// Logic probe before layout: ladder, colors, flags, actions, click.
	titles := []*typography.Typography{h1, h2, h3, h4, h5}
	for i, tp := range titles {
		if !approx(tp.EffectiveFontSize(), f.TitleSizes[i], f.Tolerance) {
			t.Fatalf("h%d=%v want %v", i+1, tp.EffectiveFontSize(), f.TitleSizes[i])
		}
		if i > 0 && !(tp.EffectiveFontSize() < titles[i-1].EffectiveFontSize()) {
			t.Fatalf("h%d must be below h%d", i+1, i)
		}
	}
	if !approx(basic.EffectiveFontSize(), f.BodyFontSize, f.Tolerance) {
		t.Fatalf("body=%v want %v", basic.EffectiveFontSize(), f.BodyFontSize)
	}
	seen := map[render.RGBA]bool{}
	for _, tp := range []*typography.Typography{secondary, success, warning, danger} {
		seen[tp.EffectiveColor()] = true
	}
	if len(seen) != 4 {
		t.Fatalf("four types must differ, got %d", len(seen))
	}
	if def := basic.EffectiveColor(); seen[def] {
		t.Fatal("type colors must differ from body")
	}
	if strong.FontWeight() != 600 || basic.FontWeight() != 400 {
		t.Fatal("strong weight probe")
	}
	if !disabled.Disabled() || disabled.Focusable() {
		t.Fatal("disabled probe")
	}
	tok := theme.Default.Current()
	toRGBA := func(c theme.Color) render.RGBA { return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A} }
	if disabled.EffectiveColor() != toRGBA(tok.ColorTextDisabled) {
		t.Fatal("disabled color must follow token")
	}
	if copyable.CopyIcon() != "copy" || copyable.CopyPayload() != f.Samples.Copy {
		t.Fatalf("copy icon/payload %q/%q", copyable.CopyIcon(), copyable.CopyPayload())
	}
	if !copyable.PressCopy() || len(copied) != 1 || copied[0] != f.Samples.Copy {
		t.Fatalf("copy must fire once, got %v", copied)
	}
	if placeStart.ActionsPlacement() != typography.PlacementStart || placeEnd.ActionsPlacement() != typography.PlacementEnd {
		t.Fatal("placement probe")
	}
	if !placeStart.HasActions() || len(placeStart.ActionNames()) < 2 {
		t.Fatalf("start actions %v", placeStart.ActionNames())
	}
	foundCopy := false
	for _, n := range placeEnd.ActionNames() {
		if n == "复制" {
			foundCopy = true
		}
	}
	if !foundCopy {
		t.Fatalf("end names %v must carry 复制", placeEnd.ActionNames())
	}
	if !link.Click() || linkClicks != 1 {
		t.Fatalf("link onClick clicks=%d", linkClicks)
	}
	disClicks := 0
	disabled.SetOnClick(func() { disClicks++ })
	if disabled.Click() || disClicks != 0 {
		t.Fatal("disabled must swallow onClick")
	}
	if got := styled.EffectiveColor(); got != (render.RGBA{R: 0.2, G: 0.4, B: 0.8, A: 1}) {
		t.Fatalf("style override %+v", got)
	}
	if link.Role() != "link" || !link.Focusable() {
		t.Fatal("link a11y probe")
	}
	if expandCalls != 1 || !controlled.IsExpanded() {
		t.Fatalf("controlled expand calls=%d expanded=%v", expandCalls, controlled.IsExpanded())
	}

	W := float64(spec.CanvasW)
	margin, gap := spec.Margin, spec.Gap
	rowW := W - 2*margin
	type placed struct {
		tp       *typography.Typography
		x, y, w, h float64
	}
	var items []placed
	y := margin
	for i, tp := range rows {
		sz := tp.Layout(rendering.Constraints{MinWidth: rowW, MaxWidth: rowW, MaxHeight: rendering.Unbounded})
		if sz.Width != rowW || sz.Height <= 0 {
			t.Fatalf("row %d layout=%v want width %v", i, sz, rowW)
		}
		if tp.Node() == nil || tp.Node().Size().Width <= 0 || tp.Node().Size().Height <= 0 {
			t.Fatalf("row %d node size %+v must be non-zero", i, tp.Node().Size())
		}
		items = append(items, placed{tp: tp, x: margin, y: y, w: sz.Width, h: sz.Height})
		y += sz.Height + gap
	}
	H := y - gap + margin

	// Post-layout ellipsis probes need the exact-width constraint applied.
	if !ell1.IsEllipsized() || !strings.Contains(ell1.DisplayText(), typography.EllipsisMark) {
		t.Fatalf("ell1 %q must truncate", ell1.DisplayText())
	}
	if ell1.LineCount() != 1 {
		t.Fatalf("ell1 lines=%d want 1", ell1.LineCount())
	}
	if !ell2.IsEllipsized() || ell2.LineCount() != 2 {
		t.Fatalf("ell2 ell=%v lines=%d want 2", ell2.IsEllipsized(), ell2.LineCount())
	}
	if !expandable.IsEllipsized() || !strings.Contains(expandable.DisplayText(), expandable.ExpandSymbol()) {
		t.Fatalf("expandable %q must carry 展开", expandable.DisplayText())
	}
	if strings.Contains(controlled.DisplayText(), typography.EllipsisMark) {
		t.Fatalf("controlled open %q must be full", controlled.DisplayText())
	}
	md := middle.DisplayText()
	if !strings.Contains(md, typography.EllipsisMark) || !strings.HasSuffix(md, f.Samples.MiddleTail) || !strings.HasPrefix(md, f.Samples.Long[:8]) {
		t.Fatalf("middle %q must be head+…+tail", md)
	}
	sd := suffixRow.DisplayText()
	if !strings.Contains(sd, typography.EllipsisMark) || !strings.HasSuffix(sd, f.Samples.MiddleTail) {
		t.Fatalf("suffix %q must end with tail", sd)
	}

	dc := render.NewContext(int(W), int(H))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	for _, it := range items {
		it.tp.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(it.x, it.y))
	}
	got := dc.Image()

	darkIn := func(it placed, pad float64) int {
		n := 0
		x0, x1 := int(it.x+pad), int(it.x+it.w-pad)
		if x1 > int(W)-2 {
			x1 = int(W) - 2
		}
		for yy := int(it.y + 2); yy < int(it.y+it.h-2); yy++ {
			for xx := x0; xx < x1; xx++ {
				r, g, b, _ := got.At(xx, yy).RGBA()
				if r/257 < 110 && g/257 < 110 && b/257 < 110 {
					n++
				}
			}
		}
		return n
	}
	if n := darkIn(items[0], 4); n < 30 {
		t.Fatalf("basic text dark=%d want >=30 (real glyphs missing?)", n)
	}
	if n := darkIn(items[1], 4); n < 30 {
		t.Fatalf("h1 dark=%d want >=30", n)
	}
	// Mark wash at the right edge (away from glyphs) stays gold.
	mkIt := items[18]
	mr, mg, mb, _ := got.At(int(mkIt.x+mkIt.w-6), int(mkIt.y+mkIt.h/2)).RGBA()
	if mr/257 < 240 || mg/257 < 200 || mb/257 > 175 {
		t.Fatalf("mark edge #%02x%02x%02x want gold #ffe58f", mr/257, mg/257, mb/257)
	}
	// Code wash at the right edge is tinted, not pure white.
	cdIt := items[17]
	cr, cg, cb, _ := got.At(int(cdIt.x+cdIt.w-6), int(cdIt.y+cdIt.h/2)).RGBA()
	if cr/257 > 250 && cg/257 > 250 && cb/257 > 250 {
		t.Fatalf("code edge #%02x%02x%02x looks white, want wash", cr/257, cg/257, cb/257)
	}

	path := filepath.Join("testdata", "showcase_typography.png")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		fw, err := os.Create(path)
		if err != nil {
			t.Fatalf("write showcase: %v", err)
		}
		if err := png.Encode(fw, got); err != nil {
			fw.Close()
			t.Fatalf("encode showcase: %v", err)
		}
		fw.Close()
		t.Logf("showcase rewritten: %s (%dx%d)", path, int(W), int(H))
		return
	}
	fr, err := os.Open(path)
	if err != nil {
		t.Fatalf("open showcase %s (run once with UPDATE_GOLDEN=1): %v", path, err)
	}
	want, err := png.Decode(fr)
	fr.Close()
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
			m := maxShowcase(diffShowcase(r1, r2), diffShowcase(g1, g2), diffShowcase(b1, b2), diffShowcase(a1, a2))
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

func diffShowcase(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func maxShowcase(vs ...uint32) uint32 {
	m := vs[0]
	for _, v := range vs[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

// TestTypography_PRD_TYPP0_ExtraHeadless covers P0 seams with no font:
// New/Loose non-zero contract, no-face paints no black bar, copy icon,
// controlled editing/defaultExpanded, placement, suffix-end, Text onClick.
func TestTypography_PRD_TYPP0_ExtraHeadless(t *testing.T) {
	f := loadTypo(t)
	fresh := typography.NewText(f.Samples.Basic)
	if fresh.Node() == nil {
		t.Fatal("node nil")
	}
	if fresh.Node().Size().Width <= 0 || fresh.Node().Size().Height <= 0 {
		t.Fatalf("New size %+v must be non-zero", fresh.Node().Size())
	}
	sz := fresh.Layout(rendering.Loose(300, 100))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("loose=%v", sz)
	}

	// No face: heuristic width still lays out, paint leaves white (no bar).
	bare := typography.NewText(f.Samples.Basic)
	if bare.TextWidth(f.Samples.Basic) <= 0 {
		t.Fatal("heuristic width must be positive without face")
	}
	bsz := bare.Layout(rendering.Loose(300, 100))
	if bsz.Width <= 0 || bsz.Height <= 0 {
		t.Fatalf("bare layout=%v", bsz)
	}
	dc := render.NewContext(160, 40)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	bare.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(4, 4))
	img := dc.Image()
	black := 0
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r/257 < 30 && g/257 < 30 && b/257 < 30 {
				black++
			}
		}
	}
	if black != 0 {
		t.Fatalf("no-face paint has %d near-black pixels (black bar?)", black)
	}

	icon := typography.NewText(f.Samples.Copy)
	icon.SetCopyable(true)
	icon.SetCopyIcon("copy")
	if icon.CopyIcon() != "copy" {
		t.Fatalf("icon=%q", icon.CopyIcon())
	}
	icon.SetCopyText("pinned")
	if icon.CopyPayload() != "pinned" {
		t.Fatalf("payload=%q", icon.CopyPayload())
	}

	ctl := typography.NewText(f.Samples.EditOld)
	ctl.SetEditable(true)
	ctl.SetEditing(true)
	if !ctl.IsEditing() {
		t.Fatal("controlled editing true")
	}
	ctl.SetEditing(false)
	if ctl.IsEditing() {
		t.Fatal("controlled editing false")
	}

	dflt := typography.NewText(f.Samples.Long)
	dflt.SetEllipsis(true)
	dflt.SetMaxWidth(80)
	dflt.SetExpandable(true)
	dflt.SetDefaultExpanded(true)
	if !dflt.IsExpanded() {
		t.Fatal("defaultExpanded true must start open")
	}
	dflt.Layout(rendering.Loose(80, 100))
	if strings.Contains(dflt.DisplayText(), typography.EllipsisMark) {
		t.Fatalf("default open %q must be full", dflt.DisplayText())
	}

	start := typography.NewText(f.Samples.Basic)
	start.SetCopyable(true)
	start.SetEditable(true)
	start.SetActionsPlacement(typography.PlacementStart)
	if start.ActionsPlacement() != typography.PlacementStart || !start.HasActions() {
		t.Fatal("start placement")
	}
	end := typography.NewText(f.Samples.Basic)
	end.SetActionsPlacement(typography.PlacementEnd)
	if end.ActionsPlacement() != typography.PlacementEnd {
		t.Fatal("end placement")
	}

	tail := typography.NewText(f.Samples.Long)
	tail.SetEllipsis(true)
	tail.SetEllipsisRows(1)
	tail.SetMaxWidth(120)
	tail.SetSuffix(f.Samples.MiddleTail)
	tail.Layout(rendering.Loose(120, 100))
	td := tail.DisplayText()
	if !strings.Contains(td, typography.EllipsisMark) || !strings.HasSuffix(td, f.Samples.MiddleTail) {
		t.Fatalf("suffix-end %q must carry mark+tail", td)
	}

	clickable := typography.NewText(f.Samples.Basic)
	n := 0
	clickable.SetOnClick(func() { n++ })
	if !clickable.Click() || n != 1 {
		t.Fatalf("text onClick n=%d", n)
	}
	off := typography.NewText(f.Samples.Basic)
	off.SetDisabled(true)
	off.SetOnClick(func() { n++ })
	if off.Click() || n != 1 {
		t.Fatal("disabled must swallow text onClick")
	}
}

// TestTypography_PRD_TYP28_P1Staged records §6.8 P1: implemented parts pass,
// desktop-unmapped parts Skip with reasons (no silent drop).
func TestTypography_PRD_TYP28_P1Staged(t *testing.T) {
	f := loadTypo(t)
	t.Run("semanticStylePartial", func(t *testing.T) {
		tp := typography.NewText(f.Samples.Basic)
		tp.SetStyle(typography.Style{Text: render.RGBA{R: 0.2, G: 0.4, B: 0.8, A: 1}, UseText: true})
		if got := tp.EffectiveColor(); got != (render.RGBA{R: 0.2, G: 0.4, B: 0.8, A: 1}) {
			t.Fatalf("style %+v", got)
		}
		t.Skipf("P1 full classNames/styles object|function CSS mapping has no desktop equivalent; Style override is the staged surface")
	})
	t.Run("copyFormatTooltipsTabIndex", func(t *testing.T) {
		t.Skipf("P1 copyable.format/tooltips/tabIndex are browser hover/MIME/DOM order; host clipboard stand-in keeps text+onCopy only")
	})
	t.Run("editableFullMatrix", func(t *testing.T) {
		t.Skipf("P1 editable.autoSize/maxLength/enterIcon/triggerType full matrix needs host text-area widget; P0 covers icon trigger + Enter/Esc")
	})
	t.Run("ellipsisTooltip", func(t *testing.T) {
		tp := typography.NewText(f.Samples.Long)
		fired := -1
		tp.OnEllipsis = func(b bool) {
			if b {
				fired = 1
			} else {
				fired = 0
			}
		}
		tp.SetEllipsis(true)
		tp.SetMaxWidth(60)
		tp.Layout(rendering.Loose(60, 100))
		if fired != 1 {
			t.Fatalf("onEllipsis hook fired=%d want 1", fired)
		}
		t.Skipf("P1 ellipsis.tooltip pixel needs hover tooltip layer; onEllipsis hook is the staged signal")
	})
	t.Run("copySuccessMotion", func(t *testing.T) {
		t.Skipf("P1 copy-success ≈3s ticker/pixel animation is staged; P0 uses instant state + repaint")
	})
	t.Run("suffixtablePages", func(t *testing.T) {
		tp := typography.NewText(f.Samples.Long)
		tp.SetEllipsis(true)
		tp.SetSuffix(f.Samples.MiddleTail)
		tp.SetMaxWidth(120)
		tp.Layout(rendering.Loose(120, 100))
		if !strings.Contains(tp.DisplayText(), f.Samples.MiddleTail) {
			t.Fatalf("suffix %q", tp.DisplayText())
		}
		t.Skipf("P1 suffix/table full gallery pages are staged; suffix truncation capability is covered here + showcase")
	})
	t.Run("configProviderGlobal", func(t *testing.T) {
		tp := typography.NewText(f.Samples.Basic)
		tp.SetProvider(theme.Default)
		_ = tp.Theme()
		t.Skipf("P1 ConfigProvider global Typography defaults ride with config-provider wave; per-node SetProvider/SetTheme is the staged seam")
	})
	t.Run("browserOnly", func(t *testing.T) {
		t.Skipf("P1 browser-only (href nav semantics,逐像素官网哈希, debug pages) has no desktop mapping")
	})
}

// TestTypography_PRD_TYP27_L4Human needs a human side-by-side sign-off.
func TestTypography_PRD_TYP27_L4Human(t *testing.T) {
	t.Skipf("L4 human气质: needs side-by-side with ant.design sign-off at baseline build/change; not a CI pixel hash")
}
