package typography_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/typography"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type typoFile struct {
	BodyFontSize float64   `json:"bodyFontSize"`
	TitleSizes   []float64 `json:"titleSizes"`
	ActionIcon   float64   `json:"actionIcon"`
	ActionGap    float64   `json:"actionGap"`
	CodeRadius   float64   `json:"codeRadius"`
	MarkBg       string    `json:"markBg"`
	Radius       float64   `json:"radius"`
	LineWidth    float64   `json:"lineWidth"`
	FocusRing    float64   `json:"focusRing"`
	Tolerance    float64   `json:"tolerance"`
	Samples      struct {
		Basic     string `json:"basic"`
		Title     string `json:"title"`
		Link      string `json:"link"`
		Paragraph string `json:"paragraph"`
		Long      string `json:"long"`
		MiddleTail string `json:"middleTail"`
		EditOld   string `json:"editOld"`
		EditNew   string `json:"editNew"`
		Copy      string `json:"copy"`
	} `json:"samples"`
}

func loadTypo(t *testing.T) typoFile {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "typography.json"))
	if err != nil {
		t.Fatalf("read typography.json: %v", err)
	}
	var f typoFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("parse typography.json: %v", err)
	}
	if len(f.TitleSizes) != 5 || f.Tolerance <= 0 {
		t.Fatal("bad typography.json")
	}
	return f
}

func approx(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func paintTypo(t *testing.T, n rendering.RenderObject) {
	t.Helper()
	dc := render.NewContext(64, 64)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	n.Paint(rendering.NewPaintContext(dc, 1))
}

func TestTypography_PRD_TYP01_Defaults(t *testing.T) {
	f := loadTypo(t)
	tp := typography.NewTypography(f.Samples.Basic)
	if tp.Kind() != typography.KindText || tp.Level() != 1 {
		t.Fatalf("kind=%v level=%v", tp.Kind(), tp.Level())
	}
	if tp.Type() != typography.TypeDefault || tp.Disabled() || tp.Copyable() || tp.Editable() || tp.Ellipsis() {
		t.Fatal("defaults: type default, no disabled/copy/edit/ellipsis")
	}
	if tp.ActionsPlacement() != typography.PlacementEnd {
		t.Fatalf("placement=%q want end", tp.ActionsPlacement())
	}
	if tp.ExpandSymbol() != "展开" || tp.CollapseSymbol() != "收起" {
		t.Fatalf("symbols %q/%q", tp.ExpandSymbol(), tp.CollapseSymbol())
	}
	if !approx(tp.EffectiveFontSize(), f.BodyFontSize, f.Tolerance) {
		t.Fatalf("body=%v want %v", tp.EffectiveFontSize(), f.BodyFontSize)
	}
	if tp.Node() == nil || tp.ContentNode() == nil || tp.ChromeNode() == nil {
		t.Fatal("nodes nil")
	}
	sz := tp.Layout(rendering.Loose(300, 100))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("layout=%v", sz)
	}
	paintTypo(t, tp.Node())
	_ = theme.Default.Current()
}

func TestTypography_PRD_TYP02_TitleLevels(t *testing.T) {
	f := loadTypo(t)
	var prev float64
	for i := 1; i <= 5; i++ {
		tp := typography.NewTitle(f.Samples.Title, i)
		if tp.Level() != i {
			t.Fatalf("level=%v want %v", tp.Level(), i)
		}
		if !approx(tp.EffectiveFontSize(), f.TitleSizes[i-1], f.Tolerance) {
			t.Fatalf("h%d=%v want %v", i, tp.EffectiveFontSize(), f.TitleSizes[i-1])
		}
		if i > 1 && !(tp.EffectiveFontSize() < prev) {
			t.Fatalf("h%d=%v not below h%d=%v", i, tp.EffectiveFontSize(), i-1, prev)
		}
		prev = tp.EffectiveFontSize()
		tp.Layout(rendering.Loose(400, 200))
		paintTypo(t, tp.Node())
	}
	_ = theme.Default.Current()
}

func TestTypography_PRD_TYP03_SemanticType(t *testing.T) {
	f := loadTypo(t)
	tok := theme.Default.Current()
	toRGBA := func(c theme.Color) render.RGBA { return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A} }
	cases := map[typography.TextType]render.RGBA{
		typography.TypeSecondary: toRGBA(tok.ColorTextSecondary),
		typography.TypeSuccess:   toRGBA(tok.ColorSuccess),
		typography.TypeWarning:   toRGBA(tok.ColorWarning),
		typography.TypeDanger:    toRGBA(tok.ColorError),
	}
	for ty, want := range cases {
		tp := typography.NewText(f.Samples.Basic)
		tp.SetType(ty)
		if got := tp.EffectiveColor(); got != want {
			t.Fatalf("type %v color=%+v want %+v", ty, got, want)
		}
		tp.Layout(rendering.Loose(300, 100))
		paintTypo(t, tp.Node())
	}
	def := typography.NewText(f.Samples.Basic).EffectiveColor()
	if def != toRGBA(tok.ColorText) {
		t.Fatalf("default=%+v want ColorText %+v", def, toRGBA(tok.ColorText))
	}
}

func TestTypography_PRD_TYP04_Copyable(t *testing.T) {
	f := loadTypo(t)
	tp := typography.NewText(f.Samples.Copy)
	tp.SetCopyable(true)
	var got []string
	tp.OnCopy = func(s string) { got = append(got, s) }
	if !tp.Copy() {
		t.Fatal("copy should succeed")
	}
	if tp.CopiedText() != f.Samples.Copy || len(got) != 1 || got[0] != f.Samples.Copy {
		t.Fatalf("copied=%q calls=%v", tp.CopiedText(), got)
	}
	tp.SetCopyText("pinned")
	tp.Copy()
	if tp.CopiedText() != "pinned" || len(got) != 2 {
		t.Fatalf("pinned=%q calls=%v", tp.CopiedText(), got)
	}
	tp.Layout(rendering.Loose(300, 100))
	paintTypo(t, tp.Node())
	_ = theme.Default.Current()
}

func TestTypography_PRD_TYP05_EllipsisOverflow(t *testing.T) {
	f := loadTypo(t)
	tp := typography.NewText(f.Samples.Long)
	tp.SetEllipsis(true)
	tp.SetEllipsisRows(1)
	tp.SetMaxWidth(80)
	tp.Layout(rendering.Loose(80, 100))
	if !tp.IsEllipsized() {
		t.Fatal("narrow long line must ellipsize")
	}
	d := tp.DisplayText()
	if !strings.Contains(d, typography.EllipsisMark) {
		t.Fatalf("display %q must carry ellipsis", d)
	}
	if tp.LineCount() != 1 {
		t.Fatalf("rows=%v want 1", tp.LineCount())
	}
	paintTypo(t, tp.Node())
}

func TestTypography_PRD_TYP06_ExpandableFull(t *testing.T) {
	f := loadTypo(t)
	tp := typography.NewText(f.Samples.Long)
	tp.SetEllipsis(true)
	tp.SetMaxWidth(80)
	tp.SetExpandable(true)
	tp.SetCollapsible(true)
	tp.Layout(rendering.Loose(80, 100))
	if !tp.IsEllipsized() {
		t.Fatal("must start ellipsized")
	}
	if !tp.ToggleExpand() || !tp.IsExpanded() {
		t.Fatal("toggle must expand")
	}
	d := tp.DisplayText()
	if !strings.Contains(d, f.Samples.Long[:16]) || strings.Contains(d, typography.EllipsisMark) {
		t.Fatalf("expanded %q must be full without mark", d)
	}
	tp.Layout(rendering.Loose(400, 200))
	paintTypo(t, tp.Node())
}

func TestTypography_PRD_TYP07_EditableEnter(t *testing.T) {
	f := loadTypo(t)
	tp := typography.NewText(f.Samples.EditOld)
	tp.SetEditable(true)
	var changed []string
	tp.OnChange = func(v string) { changed = append(changed, v) }
	ends := 0
	tp.OnEnd = func() { ends++ }
	starts := 0
	tp.OnStart = func() { starts++ }
	if !tp.StartEdit() || !tp.IsEditing() || starts != 1 {
		t.Fatal("start edit")
	}
	tp.SetPendingEdit(f.Samples.EditNew)
	if !tp.PressKey("Enter") {
		t.Fatal("enter must submit")
	}
	if tp.Value() != f.Samples.EditNew || len(changed) != 1 || changed[0] != f.Samples.EditNew || ends != 1 || tp.IsEditing() {
		t.Fatalf("value=%q changed=%v ends=%d editing=%v", tp.Value(), changed, ends, tp.IsEditing())
	}
	tp.Layout(rendering.Loose(300, 100))
	paintTypo(t, tp.Node())
}

func TestTypography_PRD_TYP08_EditableEsc(t *testing.T) {
	f := loadTypo(t)
	tp := typography.NewText(f.Samples.EditOld)
	tp.SetEditable(true)
	cancels := 0
	tp.OnCancel = func() { cancels++ }
	if !tp.StartEdit() {
		t.Fatal("start edit")
	}
	tp.SetPendingEdit("junk")
	if !tp.PressKey("Escape") {
		t.Fatal("esc must cancel")
	}
	if tp.Value() != f.Samples.EditOld || cancels != 1 || tp.IsEditing() {
		t.Fatalf("value=%q cancels=%d editing=%v", tp.Value(), cancels, tp.IsEditing())
	}
}

func TestTypography_PRD_TYP09_DisabledSwallows(t *testing.T) {
	f := loadTypo(t)
	tp := typography.NewText(f.Samples.Basic)
	tp.SetCopyable(true)
	tp.SetEditable(true)
	clicks := 0
	tp.SetOnClick(func() { clicks++ })
	copies := 0
	tp.OnCopy = func(string) { copies++ }
	tp.SetDisabled(true)
	if tp.Copy() || copies != 0 {
		t.Fatal("disabled must swallow copy")
	}
	if tp.StartEdit() || tp.IsEditing() {
		t.Fatal("disabled must swallow edit")
	}
	if !tp.Click() {
		// Click returns false when no handler? disabled must not call handler.
	}
	if clicks != 0 {
		t.Fatal("disabled must swallow onClick")
	}
	if tp.Focusable() {
		t.Fatal("disabled must not focus")
	}
	_ = theme.Default.Current()
}

func TestTypography_PRD_TYP10_ModifiersDistinct(t *testing.T) {
	tp := typography.NewText("mods")
	if tp.FontWeight() != 400 {
		t.Fatalf("plain weight=%v", tp.FontWeight())
	}
	tp.SetStrong(true)
	if !tp.Strong() || tp.FontWeight() != 600 {
		t.Fatal("strong weight 600")
	}
	tp.SetCode(true)
	if !tp.Code() {
		t.Fatal("code flag")
	}
	tp.SetMark(true)
	if !tp.Mark() {
		t.Fatal("mark flag")
	}
	tp.SetDelete(true)
	if !tp.Deleted() {
		t.Fatal("delete flag")
	}
	tp.SetUnderline(true)
	if !tp.Underline() {
		t.Fatal("underline flag")
	}
	tp.SetItalic(true)
	if !tp.Italic() {
		t.Fatal("italic flag")
	}
	tp.SetKeyboard(true)
	if !tp.Keyboard() {
		t.Fatal("keyboard flag")
	}
	tp.Layout(rendering.Loose(300, 100))
	paintTypo(t, tp.Node())
}

func TestTypography_PRD_TYP11_LinkFocus(t *testing.T) {
	f := loadTypo(t)
	tok := theme.Default.Current()
	tp := typography.NewLink(f.Samples.Link)
	if tp.Role() != "link" {
		t.Fatalf("role=%q want link", tp.Role())
	}
	if !tp.Focusable() {
		t.Fatal("link must take Tab")
	}
	want := render.RGBA{R: tok.ColorLink.R, G: tok.ColorLink.G, B: tok.ColorLink.B, A: tok.ColorLink.A}
	if got := tp.EffectiveColor(); got != want {
		t.Fatalf("link color=%+v want %+v", got, want)
	}
	if tp.FocusRingVisible() {
		t.Fatal("ring off before focus")
	}
	tp.Focus()
	if !tp.Focused() || !tp.FocusRingVisible() {
		t.Fatal("ring must show after Focus")
	}
	if tp.AriaLabel() != f.Samples.Link {
		t.Fatalf("aria=%q", tp.AriaLabel())
	}
	tp.Layout(rendering.Loose(300, 100))
	paintTypo(t, tp.Node())
	tp.Blur()
	if tp.FocusRingVisible() {
		t.Fatal("ring off after Blur")
	}
}

func TestTypography_PRD_TYP12_ControlledExpanded(t *testing.T) {
	f := loadTypo(t)
	tp := typography.NewText(f.Samples.Long)
	tp.SetEllipsis(true)
	tp.SetMaxWidth(80)
	tp.SetExpandable(true)
	calls := 0
	tp.OnExpand = func(bool) { calls++ }
	tp.SetExpanded(false)
	if tp.IsExpanded() || !tp.IsEllipsized() {
		t.Fatal("controlled false must stay collapsed")
	}
	if !strings.Contains(tp.DisplayText(), typography.EllipsisMark) {
		t.Fatal("collapsed must carry mark")
	}
	tp.SetExpanded(true)
	if !tp.IsExpanded() || calls != 1 {
		t.Fatalf("expanded=%v calls=%d want 1 (false->true fires once)", tp.IsExpanded(), calls)
	}
	if !strings.Contains(tp.DisplayText(), f.Samples.Long[:16]) {
		t.Fatal("controlled true must show full")
	}
}

func TestTypography_PRD_TYP13_BodySize(t *testing.T) {
	f := loadTypo(t)
	tp := typography.NewText(f.Samples.Basic)
	if !approx(tp.EffectiveFontSize(), f.BodyFontSize, f.Tolerance) {
		t.Fatalf("body=%v want %v", tp.EffectiveFontSize(), f.BodyFontSize)
	}
	para := typography.NewParagraph(f.Samples.Paragraph)
	if !approx(para.EffectiveFontSize(), f.BodyFontSize, f.Tolerance) {
		t.Fatalf("para=%v", para.EffectiveFontSize())
	}
	tp.Layout(rendering.Loose(300, 100))
	_ = theme.Default.Current()
}

func TestTypography_PRD_TYP14_BasicFourKinds(t *testing.T) {
	f := loadTypo(t)
	tx := typography.NewText(f.Samples.Basic)
	ti := typography.NewTitle(f.Samples.Title, 1)
	pa := typography.NewParagraph(f.Samples.Paragraph)
	li := typography.NewLink(f.Samples.Link)
	for _, tp := range []*typography.Typography{tx, ti, pa, li} {
		tp.Layout(rendering.Loose(400, 200))
		paintTypo(t, tp.Node())
		if tp.DisplayText() == "" || tp.Value() == "" {
			t.Fatal("each kind must carry a line")
		}
	}
	if !(ti.EffectiveFontSize() > tx.EffectiveFontSize()) {
		t.Fatalf("title %v must exceed body %v", ti.EffectiveFontSize(), tx.EffectiveFontSize())
	}
}

func TestTypography_PRD_TYP15_TitleExample(t *testing.T) {
	f := loadTypo(t)
	var prev float64
	for i := 1; i <= 5; i++ {
		tp := typography.NewTitle(f.Samples.Title, i)
		sz := tp.Layout(rendering.Loose(500, 200))
		if sz.Width <= 0 || sz.Height <= 0 {
			t.Fatalf("h%d layout=%v", i, sz)
		}
		if !approx(tp.EffectiveFontSize(), f.TitleSizes[i-1], f.Tolerance) {
			t.Fatalf("h%d=%v want %v", i, tp.EffectiveFontSize(), f.TitleSizes[i-1])
		}
		if i > 1 && !(tp.EffectiveFontSize() < prev-f.Tolerance/2) {
			t.Fatalf("h%d not decreasing", i)
		}
		prev = tp.EffectiveFontSize()
		paintTypo(t, tp.Node())
	}
}

func TestTypography_PRD_TYP16_TextLinkClick(t *testing.T) {
	f := loadTypo(t)
	tok := theme.Default.Current()
	toRGBA := func(c theme.Color) render.RGBA { return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A} }
	def := typography.NewText(f.Samples.Basic).EffectiveColor()
	for _, ty := range []typography.TextType{typography.TypeSecondary, typography.TypeSuccess, typography.TypeWarning, typography.TypeDanger} {
		tp := typography.NewText(f.Samples.Basic)
		tp.SetType(ty)
		if tp.EffectiveColor() == def {
			t.Fatalf("type %v must differ from body", ty)
		}
		tp.Layout(rendering.Loose(300, 100))
		paintTypo(t, tp.Node())
	}
	_ = toRGBA(tok.ColorText)
	li := typography.NewLink(f.Samples.Link)
	clicks := 0
	li.SetOnClick(func() { clicks++ })
	if !li.Click() || clicks != 1 {
		t.Fatalf("link click=%v clicks=%d", true, clicks)
	}
}

func TestTypography_PRD_TYP17_EditableExample(t *testing.T) {
	f := loadTypo(t)
	tp := typography.NewText(f.Samples.EditOld)
	tp.SetEditable(true)
	var changed []string
	tp.OnChange = func(v string) { changed = append(changed, v) }
	if !tp.StartEdit() {
		t.Fatal("enter edit")
	}
	// Change one word then Enter.
	if !tp.SubmitEdit(f.Samples.EditNew) {
		t.Fatal("enter submit")
	}
	if len(changed) != 1 || changed[0] != f.Samples.EditNew || tp.Value() != f.Samples.EditNew {
		t.Fatalf("changed=%v value=%q", changed, tp.Value())
	}
	if !strings.Contains(tp.DisplayText(), f.Samples.EditNew) {
		t.Fatal("content row must update")
	}
	tp.Layout(rendering.Loose(300, 100))
	paintTypo(t, tp.Node())
}

func TestTypography_PRD_TYP18_CopyableExample(t *testing.T) {
	f := loadTypo(t)
	tp := typography.NewText(f.Samples.Copy)
	tp.SetCopyable(true)
	calls := 0
	tp.OnCopy = func(string) { calls++ }
	if !tp.PressCopy() || calls != 1 || tp.CopiedText() != f.Samples.Copy {
		t.Fatalf("copied=%q calls=%d", tp.CopiedText(), calls)
	}
	tp.Layout(rendering.Loose(300, 100))
	paintTypo(t, tp.Node())
}

func TestTypography_PRD_TYP19_EllipsisRows(t *testing.T) {
	f := loadTypo(t)
	tp := typography.NewText(f.Samples.Long)
	tp.SetEllipsis(true)
	tp.SetEllipsisRows(2)
	tp.SetMaxWidth(60)
	tp.Layout(rendering.Loose(60, 200))
	if !tp.IsEllipsized() {
		t.Fatal("must ellipsize")
	}
	if !strings.Contains(tp.DisplayText(), typography.EllipsisMark) {
		t.Fatal("tail must carry mark")
	}
	if tp.LineCount() != 2 {
		t.Fatalf("lines=%v want 2", tp.LineCount())
	}
	paintTypo(t, tp.Node())
}

func TestTypography_PRD_TYP20_ControlledEllipsisExample(t *testing.T) {
	f := loadTypo(t)
	tp := typography.NewText(f.Samples.Long)
	tp.SetEllipsis(true)
	tp.SetMaxWidth(80)
	tp.SetExpandable(true)
	tp.SetCollapsible(true)
	calls := 0
	var last bool
	tp.OnExpand = func(b bool) { calls++; last = b }
	tp.SetExpanded(false)
	tp.Layout(rendering.Loose(80, 100))
	if !strings.Contains(tp.DisplayText(), typography.EllipsisMark) {
		t.Fatal("must start truncated")
	}
	tp.SetExpanded(true)
	if calls != 1 || !last || !strings.Contains(tp.DisplayText(), f.Samples.Long[:16]) {
		t.Fatalf("calls=%d last=%v display truncated (false->true fires once)", calls, last)
	}
	if strings.Contains(tp.DisplayText(), typography.EllipsisMark) {
		t.Fatal("expanded must be full")
	}
}

func TestTypography_PRD_TYP21_MiddleEllipsis(t *testing.T) {
	f := loadTypo(t)
	tp := typography.NewText(f.Samples.Long)
	tp.SetEllipsis(true)
	tp.SetEllipsisMiddle(true)
	tp.SetEllipsisRows(1)
	tp.SetSuffix(f.Samples.MiddleTail)
	tp.SetMaxWidth(200)
	tp.Layout(rendering.Loose(200, 100))
	d := tp.DisplayText()
	if !strings.Contains(d, typography.EllipsisMark) {
		t.Fatalf("middle %q must carry mark", d)
	}
	if !strings.HasSuffix(d, f.Samples.MiddleTail) {
		t.Fatalf("middle %q must end with fixed tail %q", d, f.Samples.MiddleTail)
	}
	if !strings.HasPrefix(d, f.Samples.Long[:8]) {
		t.Fatalf("middle %q must keep head", d)
	}
	paintTypo(t, tp.Node())
}

func TestTypography_PRD_TYP22_MetricsMatrix(t *testing.T) {
	f := loadTypo(t)
	tp := typography.NewText(f.Samples.Basic)
	if !approx(tp.EffectiveFontSize(), f.BodyFontSize, f.Tolerance) {
		t.Fatalf("body=%v", tp.EffectiveFontSize())
	}
	for i := 1; i <= 5; i++ {
		ti := typography.NewTitle("h", i)
		if !approx(ti.EffectiveFontSize(), f.TitleSizes[i-1], f.Tolerance) {
			t.Fatalf("h%d=%v", i, ti.EffectiveFontSize())
		}
	}
	if typography.ActionIconSize != f.ActionIcon || typography.ActionGap != f.ActionGap {
		t.Fatalf("action %v+%v", typography.ActionIconSize, typography.ActionGap)
	}
	if typography.CodeRadius != f.CodeRadius || typography.ContainerRadius != f.Radius || typography.LineWidth != f.LineWidth || typography.FocusRingOutset != f.FocusRing {
		t.Fatal("chrome mismatch")
	}
	if typography.MarkBg() != render.Hex(f.MarkBg) {
		t.Fatalf("mark=%+v", typography.MarkBg())
	}
	// Layout matrix: Exact forces, Loose contents, Min/Max clamps.
	exact := tp.Layout(rendering.Tight(200, 40))
	if math.Abs(exact.Width-200) > f.Tolerance || math.Abs(exact.Height-40) > f.Tolerance {
		t.Fatalf("exact=%v want 200x40", exact)
	}
	loose := tp.Layout(rendering.Loose(400, 100))
	if loose.Width <= 0 || loose.Height <= 0 {
		t.Fatalf("loose=%v", loose)
	}
	mm := tp.Layout(rendering.Constraints{MinWidth: 200, MaxWidth: 400, MinHeight: 0, MaxHeight: 100})
	if mm.Width < 200-f.Tolerance || mm.Width > 400+f.Tolerance {
		t.Fatalf("minmax=%v want 200..400", mm)
	}
	tok := theme.Default.Current()
	_ = tok.ColorText
	_ = tok.ColorPrimary
}

func TestTypography_PRD_TYP23_ThemeDefault(t *testing.T) {
	tok := theme.Default.Current()
	def := typography.NewText("body").EffectiveColor()
	want := render.RGBA{R: tok.ColorText.R, G: tok.ColorText.G, B: tok.ColorText.B, A: tok.ColorText.A}
	if def != want {
		t.Fatalf("default=%+v want ColorText %+v", def, want)
	}
	brand := render.RGBA{R: tok.ColorPrimary.R, G: tok.ColorPrimary.G, B: tok.ColorPrimary.B, A: tok.ColorPrimary.A}
	if def == brand {
		t.Fatal("default must not hardcode brand primary")
	}
	codeBg := typography.NewText("c").CodeBg()
	if codeBg == brand {
		t.Fatal("code wash must not be brand")
	}
	alt := tok
	alt.ColorPrimary = theme.Hex("#ff4d4f")
	tp := typography.NewText("body")
	tp.SetTheme(&alt)
	if got := tp.EffectiveColor(); got != want {
		t.Fatalf("default must ignore brand %+v", got)
	}
	_ = theme.Default.Current()
}

func TestTypography_PRD_TYP24_DisabledLook(t *testing.T) {
	tok := theme.Default.Current()
	tp := typography.NewText("off")
	tp.SetDisabled(true)
	want := render.RGBA{R: tok.ColorTextDisabled.R, G: tok.ColorTextDisabled.G, B: tok.ColorTextDisabled.B, A: tok.ColorTextDisabled.A}
	if got := tp.EffectiveColor(); got != want {
		t.Fatalf("disabled=%+v want %+v", got, want)
	}
	if tp.Focusable() || tp.FocusRingVisible() {
		t.Fatal("disabled must not focus or ring")
	}
	li := typography.NewLink("off")
	li.SetDisabled(true)
	if li.Focusable() {
		t.Fatal("disabled link must not focus")
	}
	tp.Layout(rendering.Loose(200, 100))
	paintTypo(t, tp.Node())
	_ = theme.Default.Current()
}

func TestTypography_PRD_TYP25_KeyboardFocus(t *testing.T) {
	tp := typography.NewText("acts")
	tp.SetCopyable(true)
	if !tp.Focusable() {
		t.Fatal("action row must take Tab")
	}
	if tp.Role() != "button" {
		t.Fatalf("role=%q want button", tp.Role())
	}
	if tp.FocusRingVisible() {
		t.Fatal("ring off before focus")
	}
	tp.Focus()
	if !tp.Focused() || !tp.FocusRingVisible() {
		t.Fatal("ring must show after Focus")
	}
	if !tp.PressCopy() || tp.CopiedText() != "acts" {
		t.Fatal("Enter/Space copy must activate")
	}
	names := tp.ActionNames()
	found := false
	for _, n := range names {
		if n == "复制" {
			found = true
		}
	}
	if !found {
		t.Fatalf("names=%v must carry 复制", names)
	}
	tp.SetAriaLabel("custom name")
	if tp.AriaLabel() != "custom name" {
		t.Fatalf("aria=%q", tp.AriaLabel())
	}
	tp.Blur()
	if tp.FocusRingVisible() {
		t.Fatal("ring off after Blur")
	}
	tp.Layout(rendering.Loose(300, 100))
	paintTypo(t, tp.Node())
}
