package float_button_test

import (
	"encoding/json"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	float_button "github.com/energye/gpui/ui/kit/float-button"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type showcaseSpec struct {
	Size  float64 `json:"size"`
	Badge struct {
		Count int `json:"count"`
	} `json:"badge"`
	BackTop struct {
		ShownScroll float64 `json:"shownScroll"`
	} `json:"backTop"`
	Canvas struct {
		Width  float64 `json:"width"`
		Margin float64 `json:"margin"`
		Gap    float64 `json:"gap"`
		ColGap float64 `json:"colGap"`
	} `json:"canvas"`
	Tolerance struct {
		MaxDiff float64 `json:"maxDiff"`
		BadFrac float64 `json:"badFrac"`
		HardCap float64 `json:"hardCap"`
	} `json:"tolerance"`
}

func loadShowcaseSpec(t *testing.T) showcaseSpec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "float-button-p1.json"))
	if err != nil {
		t.Fatalf("read float-button-p1.json: %v", err)
	}
	var s showcaseSpec
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse showcase spec: %v", err)
	}
	if s.Size != 40 || s.Canvas.Width <= 0 || s.Tolerance.MaxDiff != 12 {
		t.Fatalf("bad showcase spec %+v", s)
	}
	return s
}

func loadShowcaseFace(t *testing.T) text.Face {
	t.Helper()
	text.ClearSystemFontPaths()
	face, desc, err := rendering.TryLoadDefaultFace(12)
	if err != nil || face == nil {
		t.Skipf("showcase needs a system face for real glyphs: %v", err)
	}
	t.Logf("showcase face: %s", desc)
	return face
}

// TestFloatButton_Showcase_MainPaths lays §6.8 main paths on one big canvas:
// basic / type / shape / content / tooltip / disabled / loading / plain group /
// menu / controlled / placement left+bottom / badge / BackTop. Three
// evidences: logic probe (tokens/layout/open/visible), pixel assertions
// (primary block, white block, real text ink, red badge), golden file compare
// (12/0.5%/64 from testdata). Regenerate with UPDATE_GOLDEN=1.
func TestFloatButton_Showcase_MainPaths(t *testing.T) {
	spec := loadShowcaseSpec(t)
	face := loadShowcaseFace(t)
	float_button.ResetFloatGlobalConfig()
	defer float_button.ResetFloatGlobalConfig()

	W, margin, gap, colGap := spec.Canvas.Width, spec.Canvas.Margin, spec.Canvas.Gap, spec.Canvas.ColGap
	edge := spec.Size

	mkBtn := func() *float_button.FloatButton { return float_button.NewFloatButton() }

	// R1 basic (basic.tsx): default circle, default glyph.
	basic := mkBtn()

	// R2 type (type.tsx): default + primary.
	typeDef := mkBtn()
	typePrim := mkBtn()
	typePrim.SetType(float_button.ButtonTypePrimary)
	typePrim.SetIcon("plus")

	// R3 shape (shape.tsx): circle + square.
	shapeCircle := mkBtn()
	shapeSquare := mkBtn()
	shapeSquare.SetShape(float_button.FloatButtonShapeSquare)

	// R4 content (content.tsx): text buttons carry a real face (no bars).
	contentDef := mkBtn()
	contentDef.SetContent("Help")
	contentDef.SetTextFace(face)
	contentPrim := mkBtn()
	contentPrim.SetContent("Help")
	contentPrim.SetType(float_button.ButtonTypePrimary)
	contentPrim.SetTextFace(face)

	// R5 tooltip + states (tooltip.tsx): hovered bubble, disabled, loading.
	tip := mkBtn()
	tip.SetTooltip("bubble tip")
	tip.SetHover(true)
	dis := mkBtn()
	dis.SetDisabled(true)
	load := mkBtn()
	load.SetLoading(true)
	load.Tick(0.2)

	// R6 plain group (group.tsx): three always-visible children.
	plainKids := []*float_button.FloatButton{mkBtn(), mkBtn(), mkBtn()}
	plainGroup := float_button.NewFloatButtonGroup(plainKids...)

	// R7 menu (group-menu.tsx): uncontrolled click menu, opened.
	menuKids := []*float_button.FloatButton{mkBtn(), mkBtn()}
	menuGroup := float_button.NewFloatButtonGroup(menuKids...)
	menuGroup.SetTrigger(float_button.FloatButtonTriggerClick)
	menuGroup.ClickTrigger()

	// R8 controlled (controlled.tsx): controlled open shows children.
	ctlKids := []*float_button.FloatButton{mkBtn()}
	ctlGroup := float_button.NewFloatButtonGroup(ctlKids...)
	ctlGroup.SetTrigger(float_button.FloatButtonTriggerClick)
	ctlGroup.SetOpen(true)

	// R9 placement (placement.tsx): left + bottom open menus.
	leftKids := []*float_button.FloatButton{mkBtn(), mkBtn()}
	leftGroup := float_button.NewFloatButtonGroup(leftKids...)
	leftGroup.SetTrigger(float_button.FloatButtonTriggerClick)
	leftGroup.SetPlacement(float_button.FloatButtonPlacementLeft)
	leftGroup.SetOpen(true)
	bottomKids := []*float_button.FloatButton{mkBtn(), mkBtn()}
	bottomGroup := float_button.NewFloatButtonGroup(bottomKids...)
	bottomGroup.SetTrigger(float_button.FloatButtonTriggerClick)
	bottomGroup.SetPlacement(float_button.FloatButtonPlacementBottom)
	bottomGroup.SetOpen(true)

	// R10 badge (badge.tsx, P1): count + dot.
	badgeCount := mkBtn()
	badgeCount.SetBadgeCount(spec.Badge.Count)
	badgeCount.SetTextFace(face)
	badgeDot := mkBtn()
	badgeDot.SetBadgeDot(true)

	// R11 BackTop (back-top.tsx, P1): visible at scrolled position.
	backTop := float_button.NewFloatBackTop()
	backTop.SetScrollY(spec.BackTop.ShownScroll)
	backTopHidden := float_button.NewFloatBackTop()

	// Logic probe before paint.
	tok := theme.Default.Current()
	toRGBA := func(c theme.Color) render.RGBA {
		return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
	}
	if typePrim.EffectiveBackground() != toRGBA(tok.ColorPrimary) {
		t.Fatal("primary must walk ColorPrimary token")
	}
	if basic.EffectiveBackground() != toRGBA(tok.ColorBgContainer) {
		t.Fatal("default must walk ColorBgContainer token")
	}
	if math.Abs(shapeCircle.EffectiveRadius()-edge/2) > 0.5 {
		t.Fatalf("circle r=%v want 20", shapeCircle.EffectiveRadius())
	}
	if math.Abs(shapeSquare.EffectiveRadius()-8) > 0.5 {
		t.Fatalf("square r=%v want 8", shapeSquare.EffectiveRadius())
	}
	if !tip.TooltipVisible() {
		t.Fatal("hovered tip must show the string bubble")
	}
	if !dis.Disabled() || !load.Loading() || !load.WantsFrame() {
		t.Fatal("disabled/loading probe")
	}
	if len(plainGroup.VisibleChildren()) != 3 || !plainGroup.Open() {
		t.Fatal("plain group shows all children")
	}
	if !menuGroup.Open() || len(menuGroup.VisibleChildren()) != 2 {
		t.Fatal("menu trigger must open")
	}
	if !ctlGroup.IsControlled() || !ctlGroup.Open() {
		t.Fatal("controlled open probe")
	}
	if leftGroup.Placement() != float_button.FloatButtonPlacementLeft {
		t.Fatal("left placement probe")
	}
	if !badgeCount.BadgeVisible() || badgeCount.BadgeText() != "5" || !badgeDot.BadgeVisible() {
		t.Fatal("badge probe")
	}
	if !backTop.Visible() || backTopHidden.Visible() {
		t.Fatal("backtop visibility probe")
	}

	// One canvas: rows of layout-ready nodes, left-aligned with gaps.
	type placed struct {
		node rendering.RenderObject
		x, y float64
		w, h float64
	}
	var items []placed
	y := margin
	layoutRow := func(nodes []rendering.RenderObject, sizes []rendering.Size) {
		maxH := 0.0
		for _, s := range sizes {
			if s.Height > maxH {
				maxH = s.Height
			}
		}
		x := margin
		for i, n := range nodes {
			items = append(items, placed{node: n, x: x, y: y, w: sizes[i].Width, h: sizes[i].Height})
			x += sizes[i].Width + colGap
		}
		y += maxH + gap
	}
	loose := rendering.Loose(2000, 2000)
	btnRow := func(bs ...*float_button.FloatButton) {
		nodes := make([]rendering.RenderObject, 0, len(bs))
		sizes := make([]rendering.Size, 0, len(bs))
		for _, b := range bs {
			sz := b.Layout(loose)
			nodes = append(nodes, b.Node())
			sizes = append(sizes, sz)
		}
		layoutRow(nodes, sizes)
	}
	groupRow := func(gs ...*float_button.FloatButtonGroup) {
		nodes := make([]rendering.RenderObject, 0, len(gs))
		sizes := make([]rendering.Size, 0, len(gs))
		for _, g := range gs {
			sz := g.Layout(loose)
			nodes = append(nodes, g.Node())
			sizes = append(sizes, sz)
		}
		layoutRow(nodes, sizes)
	}

	btnRow(basic)
	btnRow(typeDef, typePrim)
	btnRow(shapeCircle, shapeSquare)
	btnRow(contentDef, contentPrim)
	btnRow(tip, dis, load)
	groupRow(plainGroup)
	groupRow(menuGroup)
	groupRow(ctlGroup)
	groupRow(leftGroup)
	groupRow(bottomGroup)
	btnRow(badgeCount, badgeDot)
	{
		sz := backTop.Layout(loose)
		if sz.Width <= 0 || sz.Height <= 0 {
			t.Fatalf("backtop layout=%v want non-zero", sz)
		}
		layoutRow([]rendering.RenderObject{backTop.Node()}, []rendering.Size{sz})
	}
	H := y - gap + margin

	// Every laid-out node is non-zero (hit == layout == paint contract).
	for i, it := range items {
		if it.w <= 0 || it.h <= 0 {
			t.Fatalf("item %d size=%vx%v want >0", i, it.w, it.h)
		}
	}
	// Placement behavior: left children sit left of the trigger.
	trigOff := leftGroup.Node().Children()[0].Offset()
	if kidOff := leftKids[0].Node().Offset(); !(kidOff.X < trigOff.X) {
		t.Fatalf("left menu: child x=%v trigger x=%v want child left", kidOff.X, trigOff.X)
	}

	dc := render.NewContext(int(W), int(H))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	for _, it := range items {
		it.node.Paint(rendering.NewPaintContext(dc, 1).WithOrigin(it.x, it.y))
	}
	got := dc.Image()

	find := func(want rendering.RenderObject) placed {
		for _, it := range items {
			if it.node == want {
				return it
			}
		}
		t.Fatal("placed item missing")
		return placed{}
	}
	primBox, basicBox := find(typePrim.Node()), find(basic.Node())
	contentBox, badgeBox := find(contentDef.Node()), find(badgeCount.Node())

	// Pixel 1: primary bare fill is the primary color (off-icon sample).
	if r, g, bl, _ := got.At(int(primBox.x+8), int(primBox.y+8)).RGBA(); true {
		r8, g8, b8 := r/257, g/257, bl/257
		if !(r8 < 60 && g8 > 90 && g8 < 150 && b8 > 220) {
			t.Fatalf("primary block #%02x%02x%02x want ~#1677ff", r8, g8, b8)
		}
	}
	// Pixel 2: default bare fill is the white container.
	if r, g, bl, _ := got.At(int(basicBox.x+8), int(basicBox.y+8)).RGBA(); true {
		r8, g8, b8 := r/257, g/257, bl/257
		if !(r8 > 240 && g8 > 240 && b8 > 240) {
			t.Fatalf("default block #%02x%02x%02x want white", r8, g8, b8)
		}
	}
	// Pixel 3: content text zone carries dark glyph ink (real text, no bars).
	// Content-only buttons paint no icon, so the whole lower area is text
	// zone (border gray #d9d9d9 never counts as dark).
	dark := 0
	for yy := int(contentBox.y + 12); yy < int(contentBox.y+contentBox.h-1); yy++ {
		for xx := int(contentBox.x + 2); xx < int(contentBox.x+contentBox.w-2); xx++ {
			r, g, bl, _ := got.At(xx, yy).RGBA()
			if r/257 < 110 && g/257 < 110 && bl/257 < 110 {
				dark++
			}
		}
	}
	if dark < 30 {
		t.Fatalf("content text dark=%d want >=30 (real glyphs missing?)", dark)
	}
	// Pixel 4: badge corner carries red overlay ink.
	red := 0
	for yy := int(badgeBox.y); yy < int(badgeBox.y+16); yy++ {
		for xx := int(badgeBox.x + 16); xx < int(badgeBox.x+badgeBox.w); xx++ {
			r, g, bl, _ := got.At(xx, yy).RGBA()
			if r/257 > 200 && g/257 < 110 && bl/257 < 110 {
				red++
			}
		}
	}
	if red < 10 {
		t.Fatalf("badge red=%d want >=10", red)
	}

	path := filepath.Join("testdata", "showcase_float_button.png")
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
