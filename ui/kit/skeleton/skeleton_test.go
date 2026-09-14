package skeleton_test

import (
	"encoding/json"
	"image"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/skeleton"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type skelFile struct {
	TitleHeight        float64 `json:"titleHeight"`
	ParagraphLiHeight  float64 `json:"paragraphLiHeight"`
	BlockRadius        float64 `json:"blockRadius"`
	RoundRadius        float64 `json:"roundRadius"`
	AvatarSmall        float64 `json:"avatarSmall"`
	AvatarMiddle       float64 `json:"avatarMiddle"`
	AvatarLarge        float64 `json:"avatarLarge"`
	ButtonH            float64 `json:"buttonH"`
	ButtonW            float64 `json:"buttonW"`
	InputH             float64 `json:"inputH"`
	InputW             float64 `json:"inputW"`
	ImageSize          float64 `json:"imageSize"`
	NodeSize           float64 `json:"nodeSize"`
	LastRowRatio       float64 `json:"lastRowRatio"`
	TitleNoAvatarRatio float64 `json:"titleNoAvatarRatio"`
	TitleAvatarRatio   float64 `json:"titleAvatarRatio"`
	ShimmerSec         float64 `json:"shimmerSec"`
	TitleGap           float64 `json:"titleGap"`
	RowGap             float64 `json:"rowGap"`
	AvatarGap          float64 `json:"avatarGap"`
	DefaultWidth       float64 `json:"defaultWidth"`
	Tolerance          float64 `json:"tolerance"`
}

func loadSkel(t *testing.T) skelFile {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "skeleton.json"))
	if err != nil {
		t.Fatalf("read skeleton.json: %v", err)
	}
	var f skelFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("parse skeleton.json: %v", err)
	}
	if f.Tolerance <= 0 {
		t.Fatal("tolerance must be positive")
	}
	return f
}

func paintGray(t *testing.T, node rendering.RenderObject, w, h int) image.Image {
	t.Helper()
	dc := render.NewContext(w, h)
	t.Cleanup(func() { _ = dc.Close() })
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	pc := rendering.NewPaintContext(dc, 1)
	node.Paint(pc)
	return dc.Image()
}

func TestSkeleton_PRD_SKL01_Defaults(t *testing.T) {
	s := skeleton.NewSkeleton()
	if s == nil || s.Node() == nil || s.ChromeNode() == nil {
		t.Fatal("NewSkeleton must not crash")
	}
	if !s.Loading() {
		t.Fatal("default loading must be true")
	}
	if s.Active() || s.HasAvatar() || s.Round() {
		t.Fatal("default active/avatar/round must be false")
	}
	if !s.HasTitle() || !s.HasParagraph() {
		t.Fatal("default title/paragraph must be true")
	}
	if s.ParagraphRows() != 0 || s.EffectiveRows() != 3 {
		t.Fatalf("default rows raw=%d effective=%d want 0/3", s.ParagraphRows(), s.EffectiveRows())
	}
	if s.AvatarShape() != skeleton.AvatarCircle || s.AvatarSize() != skeleton.SizeLarge {
		t.Fatalf("default avatar %q/%q want circle/large", s.AvatarShape(), s.AvatarSize())
	}
	if s.Focusable() || s.Role() != "" || s.AriaLabel() != "" {
		t.Fatal("skeleton decorative: no focus, no role, no label")
	}
	if len(s.SemanticParts()) != 6 {
		t.Fatalf("semantic parts=%v want 6", s.SemanticParts())
	}
	sz := s.Layout(rendering.Loose(400, 400))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("default layout=%v want positive", sz)
	}
	_ = theme.Default.Current()
}

func TestSkeleton_PRD_SKL02_LoadingSkeleton(t *testing.T) {
	fx := loadSkel(t)
	s := skeleton.NewSkeleton()
	sz := s.Layout(rendering.Loose(200, 200))
	if math.Abs(sz.Width-200) > fx.Tolerance {
		t.Fatalf("width=%v want 200", sz.Width)
	}
	img := paintGray(t, s.Node(), 200, 130)
	// Title block inside must be gray (FillSecondary over white).
	if r, g, b, _ := img.At(10, 8).RGBA(); r > 0xF800 || g > 0xF800 || b > 0xF800 {
		t.Fatalf("title #%04x%04x%04x want gray block", r, g, b)
	}
	// Gap between title and rows stays white.
	if r, g, b, _ := img.At(10, 24).RGBA(); r < 0xE000 || g < 0xE000 || b < 0xE000 {
		t.Fatalf("gap #%04x%04x%04x want white", r, g, b)
	}
	// First paragraph row gray.
	if r, g, b, _ := img.At(100, 40).RGBA(); r > 0xF800 || g > 0xF800 || b > 0xF800 {
		t.Fatalf("row0 #%04x%04x%04x want gray", r, g, b)
	}
}

func TestSkeleton_PRD_SKL03_LoadingFalseChildren(t *testing.T) {
	s := skeleton.NewSkeleton()
	body := rendering.NewRenderColorBox(60, 20, 0.2, 0.5, 0.9, 1)
	s.SetContent(body)
	if s.IsShowingContent() {
		t.Fatal("loading=true must not show content yet")
	}
	s.SetLoading(false)
	if !s.IsShowingContent() {
		t.Fatal("loading=false with content must show children")
	}
	sz := s.Layout(rendering.Loose(400, 200))
	if math.Abs(sz.Width-60) > 0.5 || math.Abs(sz.Height-20) > 0.5 {
		t.Fatalf("content layout=%v want 60x20", sz)
	}
	img := paintGray(t, s.Node(), 80, 40)
	if r, g, b, _ := img.At(30, 10).RGBA(); b < 0x8000 || r > 0x8000 {
		t.Fatalf("children #%04x%04x%04x want blue box", r, g, b)
	}
	// Back to skeleton: content leaves the tree.
	s.SetLoading(true)
	if s.IsShowingContent() {
		t.Fatal("loading=true must hide children")
	}
}

func TestSkeleton_PRD_SKL04_ActiveShimmer(t *testing.T) {
	s := skeleton.NewSkeleton()
	s.SetActive(true)
	if !s.Active() || !s.WantsFrame() {
		t.Fatal("active skeleton must want frames")
	}
	if s.Phase() != 0 {
		t.Fatalf("initial phase=%v want 0", s.Phase())
	}
	s.Tick(0.35)
	if s.Phase() <= 0 {
		t.Fatalf("phase=%v must advance", s.Phase())
	}
	s.Layout(rendering.Loose(200, 200))
	img := paintGray(t, s.Node(), 200, 130)
	_ = img
}

func TestSkeleton_PRD_SKL05_AvatarParagraph(t *testing.T) {
	fx := loadSkel(t)
	s := skeleton.NewSkeleton()
	s.SetAvatar(true)
	if !s.HasAvatar() || s.EffectiveRows() != 2 {
		t.Fatalf("avatar rows=%d want 2", s.EffectiveRows())
	}
	if math.Abs(s.EffectiveAvatarSize()-fx.AvatarLarge) > fx.Tolerance {
		t.Fatalf("avatar=%v want %v", s.EffectiveAvatarSize(), fx.AvatarLarge)
	}
	s.SetAvatarShape(skeleton.AvatarSquare)
	if s.AvatarShape() != skeleton.AvatarSquare {
		t.Fatalf("shape=%q want square", s.AvatarShape())
	}
	s.SetAvatarSize(skeleton.SizeSmall)
	if math.Abs(s.EffectiveAvatarSize()-fx.AvatarSmall) > fx.Tolerance {
		t.Fatalf("small avatar=%v want %v", s.EffectiveAvatarSize(), fx.AvatarSmall)
	}
	s.SetAvatarSize(skeleton.SizeLarge)
	sz := s.Layout(rendering.Loose(200, 200))
	if math.Abs(sz.Width-200) > fx.Tolerance {
		t.Fatalf("width=%v want 200", sz.Width)
	}
	img := paintGray(t, s.Node(), 200, 100)
	if r, g, b, _ := img.At(20, 20).RGBA(); r > 0xF800 || g > 0xF800 || b > 0xF800 {
		t.Fatalf("avatar #%04x%04x%04x want gray", r, g, b)
	}
}

func TestSkeleton_PRD_SKL06_ParagraphRows4(t *testing.T) {
	s := skeleton.NewSkeleton()
	s.SetParagraphRows(4)
	if s.EffectiveRows() != 4 {
		t.Fatalf("rows=%d want 4", s.EffectiveRows())
	}
	sz := s.Layout(rendering.Loose(300, 400))
	ws := s.EffectiveParagraphWidths(300)
	if len(ws) != 4 {
		t.Fatalf("widths=%v want 4 rows", ws)
	}
	for i := 0; i < 3; i++ {
		if math.Abs(ws[i]-300) > 0.5 {
			t.Fatalf("row%d=%v want full 300", i, ws[i])
		}
	}
	if math.Abs(ws[3]-300*0.61) > 1.0 {
		t.Fatalf("last=%v want 61%% of 300", ws[3])
	}
	if sz.Height <= 0 {
		t.Fatalf("layout=%v want positive height", sz)
	}
	s.Layout(rendering.Loose(300, 400))
	paintGray(t, s.Node(), 300, int(sz.Height)+4)
}

func TestSkeleton_PRD_SKL07_ReducedMotion(t *testing.T) {
	s := skeleton.NewSkeleton()
	s.SetActive(true)
	s.SetReduceMotion(true)
	if !s.ReduceMotion() {
		t.Fatal("reduce flag must stick")
	}
	s.Tick(0.7)
	if s.Phase() != 0 {
		t.Fatalf("reduced phase=%v want 0", s.Phase())
	}
	if s.WantsFrame() {
		t.Fatal("reduced-motion must not want frames")
	}
}

func TestSkeleton_PRD_SKL08_BasicExample(t *testing.T) {
	fx := loadSkel(t)
	s := skeleton.NewSkeleton()
	// Layout matrix: Exact / Loose / Min-Max each run.
	exact := s.Layout(rendering.Tight(300, 120))
	if math.Abs(exact.Width-300) > fx.Tolerance || math.Abs(exact.Height-120) > fx.Tolerance {
		t.Fatalf("exact=%v want 300x120", exact)
	}
	loose := s.Layout(rendering.Loose(400, 400))
	if math.Abs(loose.Width-400) > fx.Tolerance {
		t.Fatalf("loose width=%v want 400", loose.Width)
	}
	mm := s.Layout(rendering.Constraints{MinWidth: 200, MaxWidth: 400, MinHeight: 0, MaxHeight: 400})
	if mm.Width < 200-fx.Tolerance || mm.Width > 400+fx.Tolerance {
		t.Fatalf("minmax width=%v want 200..400", mm.Width)
	}
	// Basic shape: title 38% + 3 rows, last 61%.
	s.Layout(rendering.Loose(400, 400))
	if got := s.EffectiveTitleWidth(400); math.Abs(got-400*fx.TitleNoAvatarRatio) > 1.0 {
		t.Fatalf("basic title=%v want 38%% of 400", got)
	}
	ws := s.EffectiveParagraphWidths(400)
	if len(ws) != 3 || math.Abs(ws[2]-400*fx.LastRowRatio) > 1.0 {
		t.Fatalf("basic rows=%v want last 61%%", ws)
	}
}

func TestSkeleton_PRD_SKL09_ComplexExample(t *testing.T) {
	fx := loadSkel(t)
	s := skeleton.NewSkeleton()
	s.SetAvatar(true)
	s.Layout(rendering.Loose(400, 400))
	if s.EffectiveRows() != 2 {
		t.Fatalf("complex rows=%d want 2", s.EffectiveRows())
	}
	avail := 400 - fx.AvatarLarge - fx.AvatarGap
	if got := s.EffectiveTitleWidth(400); math.Abs(got-avail*fx.TitleAvatarRatio) > 1.0 {
		t.Fatalf("complex title=%v want 50%% of %v", got, avail)
	}
	img := paintGray(t, s.Node(), 400, 100)
	if r, g, b, _ := img.At(20, 20).RGBA(); r > 0xF800 || g > 0xF800 || b > 0xF800 {
		t.Fatalf("complex avatar #%04x%04x%04x want gray", r, g, b)
	}
}

func TestSkeleton_PRD_SKL10_ActiveExample(t *testing.T) {
	s := skeleton.NewSkeleton()
	s.SetActive(true)
	p0 := s.Phase()
	s.Tick(skeleton.ShimmerPeriodSec / 4)
	if s.Phase() == p0 {
		t.Fatal("active example must advance shimmer")
	}
	if !s.WantsFrame() {
		t.Fatal("active example must want frames")
	}
	s.Layout(rendering.Loose(300, 200))
	paintGray(t, s.Node(), 300, 130)
}

func TestSkeleton_PRD_SKL11_ElementExample(t *testing.T) {
	fx := loadSkel(t)
	av := skeleton.NewSkeletonAvatar()
	if math.Abs(av.EffectiveSize()-fx.AvatarMiddle) > fx.Tolerance {
		t.Fatalf("avatar element=%v want middle %v", av.EffectiveSize(), fx.AvatarMiddle)
	}
	av.Layout(rendering.Loose(100, 100))
	paintGray(t, av.Node(), 48, 48)

	bt := skeleton.NewSkeletonButton()
	if math.Abs(bt.EffectiveHeight()-fx.ButtonH) > fx.Tolerance || math.Abs(bt.EffectiveWidth()-fx.ButtonW) > fx.Tolerance {
		t.Fatalf("button=%vx%v want %vx%v", bt.EffectiveWidth(), bt.EffectiveHeight(), fx.ButtonW, fx.ButtonH)
	}
	bt.Layout(rendering.Loose(200, 100))
	paintGray(t, bt.Node(), 80, 48)
	bt.SetBlock(true)
	bsz := bt.Layout(rendering.Loose(200, 100))
	if math.Abs(bsz.Width-200) > fx.Tolerance {
		t.Fatalf("block button width=%v want 200", bsz.Width)
	}
	bt.SetShape(skeleton.ButtonRound)
	bt.Layout(rendering.Loose(200, 100))

	in := skeleton.NewSkeletonInput()
	if math.Abs(in.EffectiveHeight()-fx.InputH) > fx.Tolerance || math.Abs(in.EffectiveWidth()-fx.InputW) > fx.Tolerance {
		t.Fatalf("input=%vx%v want %vx%v", in.EffectiveWidth(), in.EffectiveHeight(), fx.InputW, fx.InputH)
	}
	in.Layout(rendering.Loose(300, 100))
	paintGray(t, in.Node(), 170, 48)

	im := skeleton.NewSkeletonImage()
	if math.Abs(im.EffectiveSize()-fx.ImageSize) > fx.Tolerance {
		t.Fatalf("image=%v want %v", im.EffectiveSize(), fx.ImageSize)
	}
	im.Layout(rendering.Loose(200, 200))
	paintGray(t, im.Node(), 110, 110)

	nd := skeleton.NewSkeletonNode(nil)
	if math.Abs(nd.EffectiveSize()-fx.NodeSize) > fx.Tolerance {
		t.Fatalf("node=%v want %v", nd.EffectiveSize(), fx.NodeSize)
	}
	nd.Layout(rendering.Loose(200, 200))
	paintGray(t, nd.Node(), 110, 110)
	child := rendering.NewRenderColorBox(30, 30, 0.2, 0.5, 0.9, 1)
	nd2 := skeleton.NewSkeletonNode(child)
	nsz := nd2.Layout(rendering.Loose(200, 200))
	if math.Abs(nsz.Width-30) > fx.Tolerance || math.Abs(nsz.Height-30) > fx.Tolerance {
		t.Fatalf("node child layout=%v want 30x30", nsz)
	}
	if av.Focusable() || bt.Focusable() || in.Focusable() || im.Focusable() || nd.Focusable() {
		t.Fatal("element placeholders must not be Focusable")
	}
	if av.Role() != "" || bt.Role() != "" {
		t.Fatal("element placeholders must have empty Role")
	}
	_ = av.AriaLabel()
}

func TestSkeleton_PRD_SKL12_ChildrenExample(t *testing.T) {
	s := skeleton.NewSkeleton()
	body := rendering.NewRenderColorBox(80, 24, 0.2, 0.6, 0.3, 1)
	s.SetContent(body)
	s.SetLoading(false)
	sz := s.Layout(rendering.Loose(400, 200))
	if math.Abs(sz.Width-80) > 0.5 || math.Abs(sz.Height-24) > 0.5 {
		t.Fatalf("children layout=%v want 80x24", sz)
	}
	img := paintGray(t, s.Node(), 100, 40)
	if r, g, b, _ := img.At(40, 12).RGBA(); g < 0x8000 {
		t.Fatalf("children #%04x%04x%04x want green box", r, g, b)
	}
}

func TestSkeleton_PRD_SKL13_ListExample(t *testing.T) {
	items := []*skeleton.Skeleton{}
	for i := 0; i < 3; i++ {
		s := skeleton.NewSkeleton()
		s.SetActive(true)
		s.SetAvatar(true)
		s.SetParagraphRows(4)
		items = append(items, s)
	}
	for _, s := range items {
		if s.EffectiveRows() != 4 || !s.HasAvatar() || !s.Active() {
			t.Fatal("list item must be active avatar rows=4")
		}
		sz := s.Layout(rendering.Loose(400, 300))
		if sz.Height <= 0 {
			t.Fatalf("list item layout=%v", sz)
		}
		paintGray(t, s.Node(), 400, 130)
	}
}

func TestSkeleton_PRD_SKL14_StyleClassExample(t *testing.T) {
	s := skeleton.NewSkeleton()
	s.SetClassNames(skeleton.SkeletonClassNames{Root: "sk-root", Header: "sk-header", Section: "sk-section", Avatar: "sk-avatar", Title: "sk-title", Paragraph: "sk-para"})
	got := s.ClassNames()
	if got.Root != "sk-root" || got.Title != "sk-title" || got.Paragraph != "sk-para" {
		t.Fatalf("classNames=%+v want shallow hooks", got)
	}
	s.SetStyles(skeleton.SkeletonStyles{Title: skeleton.Style{"width": "200px"}})
	if s.Styles().Title["width"] != "200px" {
		t.Fatalf("styles=%+v want title width hook", s.Styles())
	}
	s.SetStyle(skeleton.Style{"background": "#f5f5f5"})
	if s.Style()["background"] != "#f5f5f5" {
		t.Fatalf("style=%v want background hook", s.Style())
	}
	before := s.Layout(rendering.Loose(300, 200))
	after := s.Layout(rendering.Loose(300, 200))
	if before != after {
		t.Fatalf("hooks must not change layout %v vs %v", before, after)
	}
}

func TestSkeleton_PRD_SKL15_SemanticExample(t *testing.T) {
	s := skeleton.NewSkeleton()
	parts := map[string]bool{}
	for _, p := range s.SemanticParts() {
		parts[p] = true
	}
	for _, want := range []string{"root", "header", "section", "avatar", "title", "paragraph"} {
		if !parts[want] {
			t.Fatalf("semantic missing %s in %v", want, s.SemanticParts())
		}
	}
	s.SetClassNames(skeleton.SkeletonClassNames{Root: "r", Header: "h", Section: "s", Avatar: "a", Title: "t", Paragraph: "p"})
	got := s.ClassNames()
	if got.Header != "h" || got.Section != "s" || got.Avatar != "a" {
		t.Fatalf("semantic hooks=%+v", got)
	}
	s.Layout(rendering.Loose(300, 200))
	paintGray(t, s.Node(), 300, 130)
}

func TestSkeleton_PRD_SKL16_TokenMetrics(t *testing.T) {
	fx := loadSkel(t)
	tok := theme.Default.Current()
	eq := func(name string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > fx.Tolerance {
			t.Fatalf("%s=%v want %v", name, got, want)
		}
	}
	eq("controlHeight", tok.ControlHeight, 32)
	eq("controlHeightSM", tok.ControlHeightSM, 24)
	eq("controlHeightLG", tok.ControlHeightLG, 40)
	eq("radiusSM", tok.RadiusSM, fx.BlockRadius)
	s := skeleton.NewSkeleton()
	eq("titleHeight", s.EffectiveTitleHeight(), fx.TitleHeight)
	eq("rowHeight", s.EffectiveRowHeight(), fx.ParagraphLiHeight)
	eq("blockRadius", s.EffectiveRadius(), fx.BlockRadius)
	eq("avatarSmall", func() float64 { s.SetAvatarSize(skeleton.SizeSmall); return s.EffectiveAvatarSize() }(), fx.AvatarSmall)
	eq("avatarMiddle", func() float64 { s.SetAvatarSize(skeleton.SizeMiddle); return s.EffectiveAvatarSize() }(), fx.AvatarMiddle)
	eq("avatarLarge", func() float64 { s.SetAvatarSize(skeleton.SizeLarge); return s.EffectiveAvatarSize() }(), fx.AvatarLarge)
	eq("shimmer", skeleton.ShimmerPeriodSec, fx.ShimmerSec)
	eq("lastRow", fx.LastRowRatio, 0.61)
	eq("titleNoAvatar", fx.TitleNoAvatarRatio, 0.38)
	eq("titleAvatar", fx.TitleAvatarRatio, 0.5)
}

func TestSkeleton_PRD_SKL17_ThemeSkin(t *testing.T) {
	s := skeleton.NewSkeleton()
	tok := theme.Default.Current()
	want := render.RGBA{R: tok.ColorFillSecondary.R, G: tok.ColorFillSecondary.G, B: tok.ColorFillSecondary.B, A: tok.ColorFillSecondary.A}
	if got := s.EffectiveFillColor(); got != want {
		t.Fatalf("fill=%+v want ColorFillSecondary %+v", got, want)
	}
	alt := tok
	alt.ColorFillSecondary = theme.RGBA(255, 0, 0, 0.5)
	s.SetTheme(&alt)
	if got := s.EffectiveFillColor(); got.R != 1 || got.G != 0 {
		t.Fatalf("override fill=%+v want red", got)
	}
	s.SetTheme(nil)
	if got := s.EffectiveFillColor(); got != want {
		t.Fatalf("cleared fill=%+v want %+v", got, want)
	}
	p := theme.NewProvider(theme.DefaultTokens())
	s.SetProvider(p)
	if got := s.EffectiveFillColor(); got != want {
		t.Fatalf("provider fill=%+v want %+v", got, want)
	}
	s.SetProvider(nil)
	_ = theme.Default.Current()
}

func TestSkeleton_PRD_SKL18_RoundCapsule(t *testing.T) {
	fx := loadSkel(t)
	s := skeleton.NewSkeleton()
	if math.Abs(s.EffectiveRadius()-fx.BlockRadius) > fx.Tolerance {
		t.Fatalf("default radius=%v want %v", s.EffectiveRadius(), fx.BlockRadius)
	}
	plain := s.Layout(rendering.Loose(300, 200))
	s.SetRound(true)
	if !s.Round() {
		t.Fatal("round flag must stick")
	}
	if s.EffectiveRadius() < 100 {
		t.Fatalf("round radius=%v want capsule", s.EffectiveRadius())
	}
	rounded := s.Layout(rendering.Loose(300, 200))
	if plain != rounded {
		t.Fatalf("round must not change layout %v vs %v", plain, rounded)
	}
	paintGray(t, s.Node(), 300, int(rounded.Height)+4)
}
