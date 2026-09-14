package skeleton_test

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/skeleton"
	"github.com/energye/gpui/ui/rendering"
)

type skeletonShowcaseSpec struct {
	CanvasW int     `json:"canvasW"`
	Margin  float64 `json:"margin"`
	Gap     float64 `json:"gap"`
	Tolerance struct {
		MaxDiff float64 `json:"maxDiff"`
		BadFrac float64 `json:"badFrac"`
		HardCap float64 `json:"hardCap"`
	} `json:"tolerance"`
}

func loadSkeletonShowcaseSpec(t *testing.T) skeletonShowcaseSpec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "showcase_spec.json"))
	if err != nil {
		t.Fatalf("read showcase_spec.json: %v", err)
	}
	var s skeletonShowcaseSpec
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse showcase spec: %v", err)
	}
	if s.CanvasW <= 0 || s.Gap < 0 {
		t.Fatalf("bad showcase spec %+v", s)
	}
	if s.Tolerance.MaxDiff != 12 || s.Tolerance.BadFrac != 0.005 || s.Tolerance.HardCap != 64 {
		t.Fatalf("showcase tolerance %+v want 12/0.005/64", s.Tolerance)
	}
	return s
}

// TestSkeleton_Showcase_MainPaths lays §6.8 main paths on one canvas:
// basic / complex / active / avatar sizes+shape+P1 number / button shapes+block /
// input sizes+block / image / node placeholder+child / children loading=false /
// list rows=4 / round capsule / P1 string widths / style-class shallow / semantic.
// Three evidences: logic probe (widths/rows/sizes/round/hooks),
// pixel assertions (token gray bars, white gaps, no black bars),
// golden compare (tolerance from testdata/showcase_spec.json).
// Regenerate with UPDATE_GOLDEN=1.
func TestSkeleton_Showcase_MainPaths(t *testing.T) {
	spec := loadSkeletonShowcaseSpec(t)
	W := float64(spec.CanvasW)
	margin, gap := spec.Margin, spec.Gap
	rowW := W - 2*margin

	// R1 basic (basic.tsx): title 38% + 3 rows, last 61%.
	basic := skeleton.NewSkeleton()
	// R2 complex (complex.tsx): avatar + title 50% + 2 rows.
	complex := skeleton.NewSkeleton()
	complex.SetAvatar(true)
	// R3 active (active.tsx): basic + 1.4s shimmer, fixed phase for determinism.
	active := skeleton.NewSkeleton()
	active.SetActive(true)
	active.Tick(0.35)
	// R4-6 avatar elements (element.tsx avatar part): sizes + shape + P1 number.
	avatarSM := skeleton.NewSkeletonAvatar()
	avatarSM.SetSize(skeleton.SizeSmall)
	avatarMDSq := skeleton.NewSkeletonAvatar()
	avatarMDSq.SetSize(skeleton.SizeMiddle)
	avatarMDSq.SetShape(skeleton.AvatarSquare)
	avatarLGNum := skeleton.NewSkeletonAvatar()
	avatarLGNum.SetSizePx(48)
	// R7-9 button elements: middle default / small round / block full width.
	buttonMid := skeleton.NewSkeletonButton()
	buttonSmallRound := skeleton.NewSkeletonButton()
	buttonSmallRound.SetSize(skeleton.SizeSmall)
	buttonSmallRound.SetShape(skeleton.ButtonRound)
	buttonBlock := skeleton.NewSkeletonButton()
	buttonBlock.SetBlock(true)
	// R10-11 input elements: middle / large block.
	inputMid := skeleton.NewSkeletonInput()
	inputLargeBlock := skeleton.NewSkeletonInput()
	inputLargeBlock.SetSize(skeleton.SizeLarge)
	inputLargeBlock.SetBlock(true)
	// R12-13 image + node placeholders.
	image := skeleton.NewSkeletonImage()
	node := skeleton.NewSkeletonNode(nil)
	nodeChildBox := rendering.NewRenderColorBox(30, 30, 0.2, 0.5, 0.9, 1)
	nodeChild := skeleton.NewSkeletonNode(nodeChildBox)
	// R14 children (children.tsx): loading=false shows business content.
	childrenBox := rendering.NewRenderColorBox(120, 24, 0.2, 0.6, 0.3, 1)
	children := skeleton.NewSkeleton()
	children.SetContent(childrenBox)
	children.SetLoading(false)
	// R15 list (list.tsx): active + avatar + rows=4.
	list := skeleton.NewSkeleton()
	list.SetActive(true)
	list.SetAvatar(true)
	list.SetParagraphRows(4)
	list.Tick(0.35)
	// R16 round: capsule radius, layout unchanged.
	rounded := skeleton.NewSkeleton()
	rounded.SetRound(true)
	// R17 P1 string widths: "200px" title + "100%"/"61%" rows.
	customStr := skeleton.NewSkeleton()
	customStr.SetParagraphRows(2)
	customStr.SetTitleWidthStr("200px")
	customStr.SetParagraphWidthsStr("100%", "61%")
	// R18 style-class (style-class.tsx): shallow hooks, paint-only.
	styled := skeleton.NewSkeleton()
	styled.SetClassNames(skeleton.SkeletonClassNames{Root: "sk-root", Title: "sk-title", Paragraph: "sk-para"})
	styled.SetStyles(skeleton.SkeletonStyles{Title: skeleton.Style{"width": "200px"}})
	styled.SetStyle(skeleton.Style{"background": "#f5f5f5"})
	// R19 _semantic: full six parts.
	semantic := skeleton.NewSkeleton()
	semantic.SetClassNames(skeleton.SkeletonClassNames{Root: "r", Header: "h", Section: "s", Avatar: "a", Title: "t", Paragraph: "p"})

	// Logic probe before paint.
	if basic.EffectiveRows() != 3 || complex.EffectiveRows() != 2 || list.EffectiveRows() != 4 {
		t.Fatalf("rows basic=%d complex=%d list=%d want 3/2/4", basic.EffectiveRows(), complex.EffectiveRows(), list.EffectiveRows())
	}
	if got := basic.EffectiveTitleWidth(rowW); got <= 0 || got >= rowW {
		t.Fatalf("basic title=%v want 38%% of %v", got, rowW)
	}
	availComplex := rowW - complex.EffectiveAvatarSize() - skeleton.AvatarGap
	if got := complex.EffectiveTitleWidth(rowW); got <= 0 || got >= availComplex+1 {
		t.Fatalf("complex title=%v want 50%% of %v", got, availComplex)
	}
	if !active.Active() || !active.WantsFrame() || active.Phase() <= 0 {
		t.Fatal("active must advance phase and want frames")
	}
	if avatarSM.EffectiveSize() >= avatarMDSq.EffectiveSize() || avatarLGNum.EffectiveSize() != 48 {
		t.Fatalf("avatar sizes sm=%v md=%v num=%v", avatarSM.EffectiveSize(), avatarMDSq.EffectiveSize(), avatarLGNum.EffectiveSize())
	}
	if avatarMDSq.Shape() != skeleton.AvatarSquare || buttonSmallRound.Shape() != skeleton.ButtonRound {
		t.Fatal("shape probe")
	}
	if buttonMid.EffectiveWidth() >= inputMid.EffectiveWidth() {
		t.Fatalf("button %v should be narrower than input %v (2h vs 5h)", buttonMid.EffectiveWidth(), inputMid.EffectiveWidth())
	}
	if !buttonBlock.Block() || !inputLargeBlock.Block() {
		t.Fatal("block flags must stick")
	}
	if image.EffectiveSize() <= 0 || node.EffectiveSize() <= 0 {
		t.Fatal("image/node sizes must be positive")
	}
	if nodeChild.Child() == nil {
		t.Fatal("node child must be attached")
	}
	if !children.IsShowingContent() {
		t.Fatal("children loading=false must show content")
	}
	if !rounded.Round() || rounded.EffectiveRadius() < 100 {
		t.Fatal("round must be capsule")
	}
	if customStr.TitleWidthStr() != "200px" || len(customStr.ParagraphWidthsStr()) != 2 {
		t.Fatalf("P1 strings %+v", customStr.ParagraphWidthsStr())
	}
	if styled.ClassNames().Root != "sk-root" || styled.Styles().Title["width"] != "200px" {
		t.Fatal("style-class hooks probe")
	}
	if len(semantic.SemanticParts()) != 6 {
		t.Fatalf("semantic parts=%v want 6", semantic.SemanticParts())
	}
	if basic.Focusable() || avatarSM.Focusable() || buttonMid.Focusable() || inputMid.Focusable() || image.Focusable() || node.Focusable() {
		t.Fatal("placeholders must not be Focusable")
	}

	// Layout every row at rowW; Layout/Node must stay non-zero.
	type placed struct {
		node rendering.RenderObject
		x    float64
		y    float64
		w    float64
		h    float64
	}
	var items []placed
	y := margin
	layoutOne := func(node rendering.RenderObject, sz rendering.Size, idx int) {
		if node == nil {
			t.Fatalf("row %d node nil", idx)
		}
		if sz.Width <= 0 || sz.Height <= 0 {
			t.Fatalf("row %d layout=%v want non-zero", idx, sz)
		}
		if ns := node.Size(); ns.Width <= 0 || ns.Height <= 0 {
			t.Fatalf("row %d node size=%v want non-zero", idx, ns)
		}
		items = append(items, placed{node: node, x: margin, y: y, w: sz.Width, h: sz.Height})
		y += sz.Height + gap
	}
	idx := 0
	layoutOne(basic.Node(), basic.Layout(rendering.Loose(rowW, 800)), idx)
	idx++
	layoutOne(complex.Node(), complex.Layout(rendering.Loose(rowW, 800)), idx)
	idx++
	layoutOne(active.Node(), active.Layout(rendering.Loose(rowW, 800)), idx)
	idx++
	layoutOne(avatarSM.Node(), avatarSM.Layout(rendering.Loose(rowW, 200)), idx)
	idx++
	layoutOne(avatarMDSq.Node(), avatarMDSq.Layout(rendering.Loose(rowW, 200)), idx)
	idx++
	layoutOne(avatarLGNum.Node(), avatarLGNum.Layout(rendering.Loose(rowW, 200)), idx)
	idx++
	layoutOne(buttonMid.Node(), buttonMid.Layout(rendering.Loose(rowW, 200)), idx)
	idx++
	layoutOne(buttonSmallRound.Node(), buttonSmallRound.Layout(rendering.Loose(rowW, 200)), idx)
	idx++
	layoutOne(buttonBlock.Node(), buttonBlock.Layout(rendering.Loose(rowW, 200)), idx)
	idx++
	layoutOne(inputMid.Node(), inputMid.Layout(rendering.Loose(rowW, 200)), idx)
	idx++
	layoutOne(inputLargeBlock.Node(), inputLargeBlock.Layout(rendering.Loose(rowW, 200)), idx)
	idx++
	layoutOne(image.Node(), image.Layout(rendering.Loose(rowW, 300)), idx)
	idx++
	layoutOne(node.Node(), node.Layout(rendering.Loose(rowW, 300)), idx)
	idx++
	layoutOne(nodeChild.Node(), nodeChild.Layout(rendering.Loose(rowW, 300)), idx)
	idx++
	layoutOne(children.Node(), children.Layout(rendering.Loose(rowW, 300)), idx)
	idx++
	layoutOne(list.Node(), list.Layout(rendering.Loose(rowW, 800)), idx)
	idx++
	layoutOne(rounded.Node(), rounded.Layout(rendering.Loose(rowW, 800)), idx)
	idx++
	layoutOne(customStr.Node(), customStr.Layout(rendering.Loose(rowW, 800)), idx)
	idx++
	layoutOne(styled.Node(), styled.Layout(rendering.Loose(rowW, 800)), idx)
	idx++
	layoutOne(semantic.Node(), semantic.Layout(rendering.Loose(rowW, 800)), idx)
	H := y - gap + margin

	dc := render.NewContext(int(W), int(H))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	for _, it := range items {
		it.node.Paint(rendering.NewPaintContext(dc, 1).WithOrigin(it.x, it.y))
	}
	got := dc.Image()

	// Pixel assertion 1: basic title interior is token gray (not white/black).
	b0 := items[0]
	titleW := basic.EffectiveTitleWidth(rowW)
	tx := int(b0.x + titleW/2)
	ty := int(b0.y + 8)
	r, g, b, _ := got.At(tx, ty).RGBA()
	r8, g8, b8 := r/257, g/257, b/257
	if r8 > 250 && g8 > 250 && b8 > 250 {
		t.Fatalf("basic title #%02x%02x%02x looks white, want token gray", r8, g8, b8)
	}
	if r8 < 20 && g8 < 20 && b8 < 20 {
		t.Fatalf("basic title #%02x%02x%02x looks black, want token gray", r8, g8, b8)
	}
	// Pixel assertion 2: gap between title and rows stays white.
	gr, gg, gb, _ := got.At(int(b0.x+10), int(b0.y+24)).RGBA()
	if gr/257 < 250 || gg/257 < 250 || gb/257 < 250 {
		t.Fatalf("basic gap #%02x%02x%02x want white", gr/257, gg/257, gb/257)
	}
	// Pixel assertion 3: complex avatar center is gray.
	c1 := items[1]
	ar, ag, ab, _ := got.At(int(c1.x+20), int(c1.y+20)).RGBA()
	if ar/257 > 250 && ag/257 > 250 && ab/257 > 250 {
		t.Fatalf("avatar #%02x%02x%02x looks white, want gray", ar/257, ag/257, ab/257)
	}
	if ar/257 < 20 && ag/257 < 20 && ab/257 < 20 {
		t.Fatalf("avatar #%02x%02x%02x looks black, want gray", ar/257, ag/257, ab/257)
	}
	// Pixel assertion 4: last-row tail beyond 61% stays white.
	lr, lg, lb, _ := got.At(int(b0.x+b0.w-10), int(b0.y+b0.h-8)).RGBA()
	if lr/257 < 250 || lg/257 < 250 || lb/257 < 250 {
		t.Fatalf("last tail #%02x%02x%02x want white", lr/257, lg/257, lb/257)
	}
	// Pixel assertion 5: canvas carries real placeholder bars and no black bars.
	gray, white, black := 0, 0, 0
	for yy := 0; yy < got.Bounds().Dy(); yy++ {
		for xx := 0; xx < got.Bounds().Dx(); xx++ {
			pr, pg, pb, _ := got.At(xx, yy).RGBA()
			 r8, g8, b8 := pr/257, pg/257, pb/257
			if r8 < 20 && g8 < 20 && b8 < 20 {
				black++
				continue
			}
			if r8 > 250 && g8 > 250 && b8 > 250 {
				white++
				continue
			}
			if r8 >= 150 && r8 <= 250 && g8 >= 150 && g8 <= 250 && b8 >= 150 && b8 <= 250 {
				gray++
			}
		}
	}
	if gray < 5000 {
		t.Fatalf("placeholder gray pixels=%d want >=5000 (bars missing?)", gray)
	}
	if white < 5000 {
		t.Fatalf("white pixels=%d want >=5000 (background missing?)", white)
	}
	if black != 0 {
		t.Fatalf("near-black pixels=%d want 0 (black bar instead of token gray?)", black)
	}

	path := filepath.Join("testdata", "showcase_skeleton.png")
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
			m := max4sk(diffSk(r1, r2), diffSk(g1, g2), diffSk(b1, b2), diffSk(a1, a2))
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

func diffSk(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func max4sk(vs ...uint32) uint32 {
	m := vs[0]
	for _, v := range vs[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
