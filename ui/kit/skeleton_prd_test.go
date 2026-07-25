package kit_test

import (
	"image/color"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
	"github.com/energye/gpui/ui/visualtest"
)

// docs/antd/skeleton.md §6.9 — P0 PRD cases (SKL-01 … SKL-18 L1/L2).
// L3/L4 and P1 cases are deferred to visual baselines / coverage notes.

func TestSkeleton_PRD_01_Defaults(t *testing.T) {
	s := kit.NewSkeleton()
	if s == nil || s.Node() == nil {
		t.Fatal("nil skeleton")
	}
	if !s.Loading || s.Active || s.Avatar || !s.Title || !s.Paragraph || s.Round {
		t.Fatalf("defaults loading=%v active=%v avatar=%v title=%v paragraph=%v round=%v",
			s.Loading, s.Active, s.Avatar, s.Title, s.Paragraph, s.Round)
	}
	if s.AvatarShape != kit.SkeletonAvatarCircle {
		t.Fatalf("AvatarShape=%v want circle", s.AvatarShape)
	}
	if s.AvatarSize != kit.SkeletonLarge {
		t.Fatalf("AvatarSize=%v want large", s.AvatarSize)
	}
	if s.Node().TypeID() != "kit.Skeleton" {
		t.Fatalf("type=%s", s.Node().TypeID())
	}
	if !s.Node().Base().IsRepaintBoundary() {
		t.Fatal("Skeleton root must be a RepaintBoundary")
	}
	sz := s.Node().Layout(core.Loose(320, 120))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("size=%v", sz)
	}
}

func TestSkeleton_PRD_02_LoadingSkeletonVisible(t *testing.T) {
	s := kit.NewSkeleton()
	_ = s.Node().Layout(core.Loose(320, 120))
	if got := countNodes(s.Node(), "kit.SkeletonPlate"); got < 4 {
		t.Fatalf("plates=%d want title+3 paragraph", got)
	}
}

func TestSkeleton_PRD_03_LoadingFalseShowsChildren(t *testing.T) {
	child := primitive.NewText("loaded content")
	s := kit.NewSkeleton()
	s.SetContent(child)
	s.SetLoading(false)
	_ = s.Node().Layout(core.Loose(320, 120))
	if countNodes(s.Node(), "kit.SkeletonPlate") != 0 {
		t.Fatal("loading=false should hide skeleton plates")
	}
	if !hasType(s.Node(), "primitive.Text") {
		t.Fatal("children text not found")
	}
}

func TestSkeleton_PRD_04_ActiveTicker(t *testing.T) {
	s := kit.NewSkeleton()
	if s.Tick(0.05) {
		t.Fatal("inactive skeleton should not tick")
	}
	s.SetActive(true)
	if !s.Tick(0.05) {
		t.Fatal("active skeleton should tick")
	}
	s.SetLoading(false)
	if s.Tick(0.05) {
		t.Fatal("loading=false skeleton should not tick")
	}
}

func TestSkeleton_PRD_05_AvatarParagraphStructure(t *testing.T) {
	s := kit.NewSkeleton()
	s.SetAvatar(true)
	s.SetParagraphRows(4)
	_ = s.Node().Layout(core.Loose(400, 180))
	if got := countNodes(s.Node(), "kit.SkeletonPlate"); got != 6 {
		t.Fatalf("plates=%d want avatar+title+4 rows", got)
	}
}

func TestSkeleton_PRD_06_ParagraphRows4(t *testing.T) {
	s := kit.NewSkeleton()
	s.SetTitle(false)
	s.SetParagraphRows(4)
	_ = s.Node().Layout(core.Loose(320, 160))
	if got := countNodes(s.Node(), "kit.SkeletonPlate"); got != 4 {
		t.Fatalf("plates=%d want 4 paragraph rows", got)
	}
}

func TestSkeleton_PRD_07_ReducedMotionStopsTicker(t *testing.T) {
	s := kit.NewSkeleton()
	s.SetActive(true)
	tree := core.NewTree(s.Node())
	tree.Clock().ReduceMotion = true
	tree.Layout(core.Size{Width: 320, Height: 160})
	if s.Tick(0.05) {
		t.Fatal("reduced-motion tree should stop Skeleton ticker")
	}
}

func TestSkeleton_PRD_08_BasicDemo(t *testing.T) {
	s := kit.NewSkeleton()
	_ = s.Node().Layout(core.Loose(320, 120))
	if got := countNodes(s.Node(), "kit.SkeletonPlate"); got != 4 {
		t.Fatalf("basic plates=%d want 4", got)
	}
}

func TestSkeleton_PRD_09_ComplexDemo(t *testing.T) {
	s := kit.NewSkeleton()
	s.SetAvatar(true)
	s.SetParagraphRows(4)
	_ = s.Node().Layout(core.Loose(420, 180))
	if got := countNodes(s.Node(), "kit.SkeletonPlate"); got != 6 {
		t.Fatalf("complex plates=%d want avatar+title+4", got)
	}
}

func TestSkeleton_PRD_10_ActiveDemo(t *testing.T) {
	s := kit.NewSkeleton()
	s.SetActive(true)
	tree := core.NewTree(s.Node())
	tree.Layout(core.Size{Width: 320, Height: 120})
	if !s.Tick(0.05) {
		t.Fatal("active demo should animate")
	}
}

func TestSkeleton_PRD_11_ElementDemo(t *testing.T) {
	btn := kit.NewSkeletonButton()
	btn.SetActive(true)
	btn.SetSize(kit.SkeletonLarge)
	btn.SetShape(kit.SkeletonElementCircle)
	if sz := btn.Node().Layout(core.Loose(200, 80)); sz.Width != 40 || sz.Height != 40 {
		t.Fatalf("circle button size=%v want 40x40", sz)
	}

	blockBtn := kit.NewSkeletonButton()
	blockBtn.SetBlock(true)
	if sz := blockBtn.Node().Layout(core.Tight(240, 32)); sz.Width != 240 {
		t.Fatalf("block button width=%v want 240", sz.Width)
	}

	input := kit.NewSkeletonInput()
	input.SetSize(kit.SkeletonSmall)
	if sz := input.Node().Layout(core.Loose(200, 80)); sz.Height != 24 {
		t.Fatalf("small input height=%v want 24", sz.Height)
	}

	avatar := kit.NewSkeletonAvatar()
	avatar.SetShape(kit.SkeletonAvatarSquare)
	if sz := avatar.Node().Layout(core.Loose(80, 80)); sz.Width != 32 || sz.Height != 32 {
		t.Fatalf("avatar size=%v want 32x32", sz)
	}

	img := kit.NewSkeletonImage()
	if sz := img.Node().Layout(core.Loose(200, 200)); sz.Width != kit.DefaultSkeletonImageW || sz.Height != kit.DefaultSkeletonImageH {
		t.Fatalf("image size=%v", sz)
	}

	node := kit.NewSkeletonNode(primitive.NewText("custom"))
	if node.Node().Layout(core.Loose(200, 80)).Width <= 0 {
		t.Fatal("custom SkeletonNode did not layout")
	}
}

func TestSkeleton_PRD_12_ChildrenDemo(t *testing.T) {
	s := kit.NewSkeleton()
	s.SetContent(primitive.NewText("Ant Design, a design language"))
	s.SetLoading(true)
	if countNodes(s.Node(), "kit.SkeletonPlate") == 0 {
		t.Fatal("loading children demo should show skeleton first")
	}
	s.SetLoading(false)
	_ = s.Node().Layout(core.Loose(360, 120))
	if !hasType(s.Node(), "primitive.Text") {
		t.Fatal("loading=false children demo should show content")
	}
}

func TestSkeleton_PRD_13_ListDemo(t *testing.T) {
	list := primitive.Column()
	for i := 0; i < 3; i++ {
		s := kit.NewSkeleton()
		s.SetAvatar(true)
		s.SetActive(true)
		list.AddChild(s.Node())
	}
	list.Gap = 16
	_ = list.Layout(core.Loose(480, 480))
	if got := countNodes(list, "kit.Skeleton"); got != 3 {
		t.Fatalf("list skeleton count=%d want 3", got)
	}
	if got := countNodes(list, "kit.SkeletonPlate"); got < 9 {
		t.Fatalf("list plates=%d want >=9", got)
	}
}

func TestSkeleton_PRD_14_StyleClassDemo(t *testing.T) {
	s := kit.NewSkeleton()
	s.SetAvatar(true)
	s.SetParagraphRows(4)
	s.SetClassNames(kit.SkeletonClassNames{
		Root:      "sk-root",
		Header:    "sk-header",
		Section:   "sk-section",
		Avatar:    "sk-avatar",
		Title:     "sk-title",
		Paragraph: "sk-paragraph",
	})
	s.SetStyles(kit.SkeletonStyles{
		Root:  kit.Style{Border: render.RGBA{R: 0.2, G: 0.4, B: 0.6, A: 1}, Radius: 10},
		Title: kit.Style{Background: render.RGBA{R: 0.8, G: 0.9, B: 1, A: 1}, Height: 20, Radius: 20},
	})
	_ = s.Node().Layout(core.Loose(480, 220))
	for _, key := range []string{"sk-root", "sk-header", "sk-section", "sk-avatar", "sk-title", "sk-paragraph"} {
		if !hasKey(s.Node(), key) {
			t.Fatalf("missing semantic key %q", key)
		}
	}
}

func TestSkeleton_PRD_15_SemanticDemo(t *testing.T) {
	s := kit.NewSkeleton()
	s.SetAvatar(true)
	s.SetParagraphRows(4)
	s.SetClassNames(kit.SkeletonClassNames{
		Root: "root", Header: "header", Section: "section",
		Avatar: "avatar", Title: "title", Paragraph: "paragraph",
	})
	_ = s.Node().Layout(core.Loose(480, 220))
	for _, key := range []string{"root", "header", "section", "avatar", "title", "paragraph"} {
		if !hasKey(s.Node(), key) {
			t.Fatalf("semantic key %q not found", key)
		}
	}
}

func TestSkeleton_PRD_16_TokenMetrics(t *testing.T) {
	if kit.DefaultSkeletonTitleH != 16 || kit.DefaultSkeletonParagraphH != 16 {
		t.Fatalf("title/paragraph h=%v/%v want 16/16", kit.DefaultSkeletonTitleH, kit.DefaultSkeletonParagraphH)
	}
	avatar := kit.NewSkeletonAvatar()
	avatar.SetSize(kit.SkeletonLarge)
	if sz := avatar.Node().Layout(core.Loose(100, 100)); sz.Width != 40 || sz.Height != 40 {
		t.Fatalf("large avatar=%v want 40x40", sz)
	}
	mid := kit.NewSkeletonButton()
	if sz := mid.Node().Layout(core.Loose(200, 100)); sz.Height != 32 {
		t.Fatalf("middle button h=%v want 32", sz.Height)
	}
}

func TestSkeleton_PRD_17_ThemeTokenFill(t *testing.T) {
	th := kit.DefaultTheme()
	want := render.RGBA{R: 0.21, G: 0.47, B: 0.73, A: 1}
	th.Tokens.Colors[core.TokenColorFillSecondary] = want
	s := kit.NewSkeleton()
	s.SetTheme(th)
	img := visualtest.CaptureTree(240, 80, s.Node(), th)
	if img == nil {
		t.Fatal("nil capture")
	}
	got := rgbaAt(img.At(8, 8))
	if !skeletonApproxRGBA(got, want, 0.08) {
		t.Fatalf("fill=%v want token %v", got, want)
	}
}

func TestSkeleton_PRD_18_RoundStructure(t *testing.T) {
	s := kit.NewSkeleton()
	s.SetRound(true)
	_ = s.Node().Layout(core.Loose(320, 120))
	if !s.Round {
		t.Fatal("round flag not set")
	}
	if countNodes(s.Node(), "kit.SkeletonPlate") == 0 {
		t.Fatal("round skeleton should still paint plates")
	}
}

func countNodes(n core.Node, typ string) int {
	if n == nil {
		return 0
	}
	got := 0
	if n.TypeID() == typ {
		got++
	}
	for _, c := range n.Children() {
		got += countNodes(c, typ)
	}
	return got
}

func hasType(n core.Node, typ string) bool {
	return countNodes(n, typ) > 0
}

func hasKey(n core.Node, key string) bool {
	if n == nil {
		return false
	}
	if n.Base().Key == key {
		return true
	}
	for _, c := range n.Children() {
		if hasKey(c, key) {
			return true
		}
	}
	return false
}

func rgbaAt(c color.Color) render.RGBA {
	r, g, b, a := c.RGBA()
	return render.RGBA{
		R: float64(r) / 65535,
		G: float64(g) / 65535,
		B: float64(b) / 65535,
		A: float64(a) / 65535,
	}
}

func skeletonApproxRGBA(a, b render.RGBA, eps float64) bool {
	if skeletonAbs(a.R-b.R) > eps {
		return false
	}
	if skeletonAbs(a.G-b.G) > eps {
		return false
	}
	if skeletonAbs(a.B-b.B) > eps {
		return false
	}
	if skeletonAbs(a.A-b.A) > eps {
		return false
	}
	return true
}

func skeletonAbs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
