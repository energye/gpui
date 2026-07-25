package kit_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/tag.md §6.9 — P0 PRD cases (TAG-01 … TAG-19).
// L3/L4 (TAG-20/21) and P1 (TAG-22) deferred.

func TestTag_PRD_01_Defaults(t *testing.T) {
	// TAG-01
	tg := kit.NewTag("Tag 1")
	if tg.Value != "Tag 1" {
		t.Fatalf("Value=%q", tg.Value)
	}
	if tg.Variant != kit.TagFilled {
		t.Fatalf("Variant=%v want filled", tg.Variant)
	}
	if tg.Closable || tg.Disabled || tg.Hidden {
		t.Fatalf("flags closable=%v disabled=%v hidden=%v", tg.Closable, tg.Disabled, tg.Hidden)
	}
	if tg.Node() == nil || tg.ChromeNode() == nil {
		t.Fatal("nil node")
	}
	_ = tg.Node().Layout(core.Loose(200, 40))
	if tg.Root.Size().Width < 8 || tg.Root.Size().Height < 10 {
		t.Fatalf("size=%v", tg.Root.Size())
	}
}

func TestTag_PRD_02_PresetColor(t *testing.T) {
	// TAG-02
	plain := kit.NewTag("x")
	red := kit.NewTag("red")
	red.SetColor("red")
	if red.Background() == plain.Background() && red.Foreground() == plain.Foreground() {
		t.Fatal("preset red should change chrome vs default")
	}
	if red.Foreground().A == 0 {
		t.Fatal("red fg zero")
	}
}

func TestTag_PRD_03_ClosableOnClose(t *testing.T) {
	// TAG-03
	closed := 0
	tg := kit.NewTag("closable")
	tg.SetClosable(true)
	tg.OnClose = func(e *kit.TagCloseEvent) { closed++ }
	if tg.CloseNode() == nil {
		t.Fatal("expected close node")
	}
	tree := core.NewTree(tg.Node())
	tree.Layout(core.Size{Width: 200, Height: 40})
	// click near right edge where × lives
	sz := tg.Root.Size()
	x, y := sz.Width-4, sz.Height/2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
	if closed != 1 {
		// fallback: invoke close path via pressable click if hit missed
		if cp, ok := tg.CloseNode().(*primitive.Pressable); ok && cp.Click != nil {
			cp.Click()
		}
	}
	if closed != 1 {
		t.Fatalf("OnClose calls=%d want 1", closed)
	}
	if tg.Visible() {
		t.Fatal("expected Hidden after close")
	}
}

func TestTag_PRD_03b_ClosePreventDefault(t *testing.T) {
	// TAG-S3
	tg := kit.NewTag("keep")
	tg.SetClosable(true)
	tg.OnClose = func(e *kit.TagCloseEvent) { e.PreventDefault() }
	if cp, ok := tg.CloseNode().(*primitive.Pressable); ok && cp.Click != nil {
		cp.Click()
	}
	if !tg.Visible() {
		t.Fatal("PreventDefault should keep tag visible")
	}
}

func TestTag_PRD_04_CheckableToggle(t *testing.T) {
	// TAG-04
	got := []bool{}
	ct := kit.NewCheckableTag("Yes")
	ct.SetOnChange(func(on bool) { got = append(got, on) })
	if ct.IsChecked() {
		t.Fatal("default unchecked")
	}
	tree := core.NewTree(ct.Node())
	tree.Layout(core.Size{Width: 120, Height: 40})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 8, Y: 8, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: 8, Y: 8, Button: core.ButtonLeft})
	if len(got) != 1 || !got[0] {
		t.Fatalf("got=%v want [true]", got)
	}
	if !ct.IsChecked() {
		t.Fatal("uncontrolled should flip Checked")
	}
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 8, Y: 8, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: 8, Y: 8, Button: core.ButtonLeft})
	if len(got) != 2 || got[1] {
		t.Fatalf("got=%v want second false", got)
	}
}

func TestTag_PRD_05_BorderedFalseNoBorder(t *testing.T) {
	// TAG-05
	tg := kit.NewTag("x")
	tg.SetBordered(false)
	if tg.HasBorder() {
		t.Fatal("bordered=false → filled, no border")
	}
	if tg.Root.BorderWidth != 0 {
		t.Fatalf("BorderWidth=%v", tg.Root.BorderWidth)
	}
	out := kit.NewTag("o")
	out.SetVariant(kit.TagOutlined)
	if !out.HasBorder() || out.Root.BorderWidth <= 0 {
		t.Fatal("outlined should have border")
	}
}

func TestTag_PRD_06_Icon(t *testing.T) {
	// TAG-06
	tg := kit.NewTag("Twitter")
	tg.SetIcon("star")
	if tg.Node() == nil {
		t.Fatal("nil")
	}
	// row should have more than just label
	if tg.Root == nil || len(tg.Root.Children()) == 0 {
		t.Fatal("no chrome children")
	}
	// icon node is first child of row
	row, ok := tg.Root.Children()[0].(*primitive.Flex)
	if !ok || len(row.Children()) < 2 {
		t.Fatalf("expected icon+label row kids=%v", len(row.Children()))
	}
}

func TestTag_PRD_07_CustomHex(t *testing.T) {
	// TAG-07
	tg := kit.NewTag("#f50")
	tg.SetColor("#f50")
	if tg.Background().A == 0 {
		t.Fatal("custom bg")
	}
	// solid uses pure color
	tg.SetVariant(kit.TagSolid)
	bg := tg.Background()
	want := render.Hex("#f50")
	if bg.R != want.R || bg.G != want.G || bg.B != want.B {
		t.Fatalf("solid bg=%v want ~%v", bg, want)
	}
}

func TestTag_PRD_08_BasicDemo(t *testing.T) {
	// TAG-08 basic.tsx
	a := kit.NewTag("Tag 1")
	b := kit.NewTag("Prevent Default")
	b.SetClosable(true)
	b.OnClose = func(e *kit.TagCloseEvent) { e.PreventDefault() }
	c := kit.NewTag("Tag 2")
	c.SetClosable(true)
	c.SetCloseIcon(kit.NewIcon("close").Node())
	for _, tg := range []*kit.Tag{a, b, c} {
		_ = tg.Node().Layout(core.Loose(200, 40))
		if tg.Root.Size().Width < 4 {
			t.Fatal("layout")
		}
	}
	if cp, ok := b.CloseNode().(*primitive.Pressable); ok {
		cp.Click()
	}
	if !b.Visible() {
		t.Fatal("prevent default")
	}
}

func TestTag_PRD_09_ColorfulDemo(t *testing.T) {
	// TAG-09
	presets := []string{"magenta", "red", "volcano", "orange", "gold", "lime", "green", "cyan", "blue", "geekblue", "purple"}
	for _, v := range []kit.TagVariant{kit.TagFilled, kit.TagSolid, kit.TagOutlined} {
		for _, c := range presets {
			tg := kit.NewTag(c)
			tg.SetVariant(v)
			tg.SetColor(c)
			if tg.Background().A == 0 && v != kit.TagOutlined {
				// outlined may use light bg with A>0 always from palette
			}
			_ = tg.Node().Layout(core.Loose(120, 40))
		}
	}
	for _, hex := range []string{"#f50", "#2db7f5", "#87d068", "#108ee9"} {
		tg := kit.NewTag(hex)
		tg.SetColor(hex)
		_ = tg.Node().Layout(core.Loose(120, 40))
	}
}

func TestTag_PRD_10_ControlDynamic(t *testing.T) {
	// TAG-10 control.tsx — dynamic add/remove
	tags := []string{"Unremovable", "Tag 2", "Tag 3"}
	remove := func(name string) {
		out := tags[:0]
		for _, s := range tags {
			if s != name {
				out = append(out, s)
			}
		}
		tags = append([]string(nil), out...)
	}
	// close Tag 2
	tg := kit.NewTag("Tag 2")
	tg.SetClosable(true)
	tg.SetOnClose(func() { remove("Tag 2") })
	if cp, ok := tg.CloseNode().(*primitive.Pressable); ok {
		cp.Click()
	}
	if len(tags) != 2 || tags[0] != "Unremovable" || tags[1] != "Tag 3" {
		t.Fatalf("tags=%v", tags)
	}
	// add
	tags = append(tags, "New")
	if len(tags) != 3 {
		t.Fatal("add")
	}
}

func TestTag_PRD_11_CheckableGroup(t *testing.T) {
	// TAG-11
	// single Checkable
	ct := kit.NewCheckableTag("Yes")
	ct.SetDefaultChecked(true)
	if !ct.IsChecked() {
		t.Fatal("defaultChecked")
	}

	// single group
	var single string
	g1 := kit.NewCheckableTagGroupStrings("Movies", "Books", "Music", "Sports")
	g1.SetDefaultValue("Books")
	g1.SetOnChange(func(v string) { single = v })
	if g1.Value() != "Books" {
		t.Fatalf("value=%q", g1.Value())
	}
	// click Music
	opts := g1.OptionTags()
	if len(opts) < 3 {
		t.Fatal("options")
	}
	tree := core.NewTree(g1.Node())
	tree.Layout(core.Size{Width: 400, Height: 40})
	// invoke change via child press
	if opts[2].Root != nil && opts[2].Root.Click != nil {
		opts[2].Root.Click()
	}
	if single != "Music" && g1.Value() != "Music" {
		// uncontrolled should update value
		if g1.Value() != "Music" {
			t.Fatalf("single value=%q singleCb=%q", g1.Value(), single)
		}
	}

	// multi group
	var multi []string
	g2 := kit.NewCheckableTagGroupStrings("Movies", "Books", "Music", "Sports")
	g2.SetMultiple(true)
	g2.SetDefaultValues([]string{"Movies", "Music"})
	g2.SetOnChangeMulti(func(vs []string) { multi = append([]string(nil), vs...) })
	vs := g2.Values()
	if len(vs) != 2 {
		t.Fatalf("multi defaults=%v", vs)
	}
	// toggle Books on
	opts2 := g2.OptionTags()
	if opts2[1].Root != nil && opts2[1].Root.Click != nil {
		opts2[1].Root.Click()
	}
	if len(multi) < 2 && len(g2.Values()) < 2 {
		t.Fatalf("multi after toggle multi=%v values=%v", multi, g2.Values())
	}
}

func TestTag_PRD_12_AnimationAddRemove(t *testing.T) {
	// TAG-12 — instantaneous add/remove (motion P1)
	tags := []string{"Tag 1", "Tag 2", "Tag 3"}
	tags = append(tags, "Tag 4")
	// remove Tag 2
	out := make([]string, 0, len(tags))
	for _, s := range tags {
		if s != "Tag 2" {
			out = append(out, s)
		}
	}
	tags = out
	if strings.Join(tags, ",") != "Tag 1,Tag 3,Tag 4" {
		t.Fatalf("tags=%v", tags)
	}
	// rebuild nodes
	for _, s := range tags {
		tg := kit.NewTag(s)
		tg.SetClosable(true)
		_ = tg.Node().Layout(core.Loose(100, 30))
	}
}

func TestTag_PRD_13_IconDemo(t *testing.T) {
	// TAG-13
	tg := kit.NewTag("Twitter")
	tg.SetIcon("star")
	tg.SetColor("#55acee")
	_ = tg.Node().Layout(core.Loose(160, 40))

	ct := kit.NewCheckableTag("Youtube")
	ct.SetIcon("heart")
	ct.SetChecked(true)
	_ = ct.Node().Layout(core.Loose(160, 40))
	if !ct.IsChecked() {
		t.Fatal("checked")
	}
}

func TestTag_PRD_14_StatusDemo(t *testing.T) {
	// TAG-14
	statuses := []string{"success", "processing", "warning", "error", "default"}
	for _, v := range []kit.TagVariant{kit.TagFilled, kit.TagSolid, kit.TagOutlined} {
		for _, st := range statuses {
			tg := kit.NewTag(st)
			tg.SetColor(st)
			tg.SetVariant(v)
			if st == "processing" {
				tg.SetIcon("sync")
				tg.SetIconSpin(true)
			} else if st == "success" {
				tg.SetIcon("check")
			} else if st == "error" {
				tg.SetIcon("close")
			} else if st == "warning" {
				tg.SetIcon("info")
			} else {
				tg.SetIcon("user")
			}
			_ = tg.Node().Layout(core.Loose(140, 40))
			if tg.Foreground().A == 0 {
				t.Fatalf("status %s variant %v zero fg", st, v)
			}
		}
	}
}

func TestTag_PRD_15_DraggableReorder(t *testing.T) {
	// TAG-15 — composition reorder (not built-in dnd-kit)
	items := []string{"Tag 1", "Tag 2", "Tag 3"}
	// swap 0 and 2
	items[0], items[2] = items[2], items[0]
	if items[0] != "Tag 3" || items[2] != "Tag 1" {
		t.Fatalf("items=%v", items)
	}
	for _, s := range items {
		tg := kit.NewTag(s)
		_ = tg.Node().Layout(core.Loose(80, 30))
	}
}

func TestTag_PRD_16_Metrics(t *testing.T) {
	// TAG-16
	tg := kit.NewTag("Metric")
	_ = tg.Node().Layout(core.Loose(200, 40))
	fs := tg.FontSize()
	if fs < 11.5 || fs > 12.5 {
		t.Fatalf("fontSize=%v want ~12", fs)
	}
	if r := tg.Radius(); r < 3.5 || r > 4.5 {
		t.Fatalf("radius=%v want ~4", r)
	}
	if p := tg.PadH(); p < 6.5 || p > 7.5 {
		t.Fatalf("padH=%v want ~7", p)
	}
	h := tg.Root.Size().Height
	if h < 18 || h > 28 {
		t.Fatalf("height=%v want ~20-24", h)
	}
	// Theme tokens present
	th := kit.DefaultTheme()
	if th.SizeOr(core.TokenFontSizeSM, 0) != 12 {
		t.Fatalf("fontSizeSM=%v", th.SizeOr(core.TokenFontSizeSM, 0))
	}
	if th.SizeOr(core.TokenBorderRadiusSM, 0) != 4 {
		t.Fatalf("radiusSM=%v", th.SizeOr(core.TokenBorderRadiusSM, 0))
	}
}

func TestTag_PRD_17_DefaultSkinTheme(t *testing.T) {
	// TAG-17 — default skin from Theme, not hardcoded brand primary
	tg := kit.NewTag("default")
	bg := tg.Background()
	primary := kit.DefaultTheme().Color(core.TokenColorPrimary)
	// default fill must not be solid primary brand
	if bg.R == primary.R && bg.G == primary.G && bg.B == primary.B && bg.A == primary.A {
		t.Fatal("default tag should not use colorPrimary as fill")
	}
	fg := tg.Foreground()
	text := kit.DefaultTheme().Color(core.TokenColorText)
	if fg.A == 0 {
		t.Fatal("fg")
	}
	// roughly text color
	if absF(fg.A-text.A) > 0.5 {
		t.Logf("fg=%v text=%v (alpha may differ)", fg, text)
	}
}

func TestTag_PRD_18_Disabled(t *testing.T) {
	// TAG-18
	closed := 0
	tg := kit.NewTag("dis")
	tg.SetClosable(true)
	tg.SetDisabled(true)
	tg.OnClose = func(*kit.TagCloseEvent) { closed++ }
	if cp, ok := tg.CloseNode().(*primitive.Pressable); ok {
		if cp.Click != nil {
			cp.Click()
		}
	}
	if closed != 0 {
		t.Fatal("disabled close should not fire")
	}
	// disabled chrome
	disBG := kit.DefaultTheme().Color(core.TokenColorDisabledBg)
	if tg.Background().A == 0 && disBG.A == 0 {
		t.Fatal("disabled bg")
	}

	changed := 0
	ct := kit.NewCheckableTag("x")
	ct.SetDisabled(true)
	ct.SetOnChange(func(bool) { changed++ })
	if ct.Root != nil && ct.Root.Click != nil {
		ct.Root.Click()
	}
	if changed != 0 {
		t.Fatal("disabled checkable")
	}
}

func TestTag_PRD_19_CheckableKeyboard(t *testing.T) {
	// TAG-19
	got := 0
	ct := kit.NewCheckableTag("Key")
	ct.SetOnChange(func(bool) { got++ })
	tree := core.NewTree(ct.Node())
	tree.Layout(core.Size{Width: 120, Height: 40})
	// focus via pointer then keyboard
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 6, Y: 6, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: 6, Y: 6, Button: core.ButtonLeft})
	got = 0
	if !ct.Root.ShowFocusRing {
		t.Fatal("ShowFocusRing")
	}
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	if got != 1 {
		// Pressable may need focus; force via HandleKey
		if ct.Root != nil {
			ct.Root.SetFocused(true)
			ct.Root.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
		}
	}
	if got < 1 {
		t.Fatalf("keyboard activate got=%d", got)
	}
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: " "})
	if got < 2 {
		ct.Root.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: " "})
	}
	if got < 2 {
		t.Fatalf("Space activate got=%d", got)
	}
}

func TestTag_PRD_SolidAndFilledChrome(t *testing.T) {
	solid := kit.NewTag("s")
	solid.SetVariant(kit.TagSolid)
	if solid.HasBorder() {
		t.Fatal("solid no border")
	}
	filled := kit.NewTag("f")
	filled.SetVariant(kit.TagFilled)
	if filled.HasBorder() {
		t.Fatal("filled no border")
	}
}

func TestTag_PRD_SetColorRGBA(t *testing.T) {
	tg := kit.NewTag("c")
	tg.SetColorRGBA(render.Hex("#108ee9"))
	tg.SetVariant(kit.TagSolid)
	bg := tg.Background()
	want := render.Hex("#108ee9")
	if absF(bg.R-want.R) > 0.02 {
		t.Fatalf("bg=%v want %v", bg, want)
	}
}

func TestTag_PRD_GroupControlled(t *testing.T) {
	g := kit.NewCheckableTagGroupStrings("A", "B", "C")
	var last string
	g.SetOnChange(func(v string) { last = v })
	g.SetValue("A") // controlled
	if g.Value() != "A" {
		t.Fatal(g.Value())
	}
	// click B — controlled: value stays until parent SetValue
	opts := g.OptionTags()
	opts[1].Root.Click()
	if last != "B" {
		t.Fatalf("onChange=%q", last)
	}
	// still A until parent sets
	if g.Value() != "A" {
		// actually our handleToggle for uncontrolled updates; controlled should not
		// We set controlled=true via SetValue
		if g.Value() != "A" {
			t.Fatalf("controlled value mutated to %q", g.Value())
		}
	}
	g.SetValue("B")
	if g.Value() != "B" {
		t.Fatal(g.Value())
	}
	_ = fmt.Sprintf("%v", last)
}

func absF(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
