package kit_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/result.md §6.9 — P0 L1/L2 PRD cases (RES-01 … RES-18).

func TestResult_PRD_01_Defaults(t *testing.T) {
	r := kit.NewResult()
	if r.Status != kit.ResultInfo {
		t.Fatalf("Status=%q want info", r.Status)
	}
	if r.Title != "" || r.SubTitle != "" || r.Loading {
		t.Fatalf("unexpected defaults title=%q sub=%q loading=%v", r.Title, r.SubTitle, r.Loading)
	}
	root, ok := r.Node().(*primitive.Flex)
	if !ok {
		t.Fatalf("root=%T want *primitive.Flex", r.Node())
	}
	if root.Base().Role != "status" {
		t.Fatalf("role=%q want status", root.Base().Role)
	}
	if r.IconNode() == nil {
		t.Fatal("default info icon is nil")
	}
}

func TestResult_PRD_02_StatusSuccess(t *testing.T) {
	r := kit.NewResult()
	r.SetStatus(kit.ResultSuccess)
	r.SetTitle("Done")
	ic, ok := r.IconNode().(*primitive.Icon)
	if !ok {
		t.Fatalf("icon=%T want *primitive.Icon", r.IconNode())
	}
	if ic.Name != "check" {
		t.Fatalf("icon=%q want check", ic.Name)
	}
	if !approxResultColor(ic.Color, kit.DefaultTheme().Color(core.TokenColorSuccess), 0.02) {
		t.Fatalf("success color=%v", ic.Color)
	}
}

func TestResult_PRD_03_Status404ExceptionSkin(t *testing.T) {
	r := kit.NewResult()
	r.SetStatus(kit.Result404)
	r.SetTitle("404")
	r.SetSubTitle("Sorry, the page you visited does not exist.")
	img, ok := r.IconNode().(*primitive.Canvas)
	if !ok {
		t.Fatalf("404 icon=%T want exception canvas", r.IconNode())
	}
	if img.Width != 250 || img.Height != 295 {
		t.Fatalf("exception image=%vx%v want 250x295", img.Width, img.Height)
	}
}

func TestResult_PRD_04_ExtraButtonClickable(t *testing.T) {
	clicks := 0
	btn := kit.NewButton("Go Console")
	btn.SetType(kit.ButtonPrimary)
	btn.SetOnClick(func() { clicks++ })
	r := kit.NewResult()
	r.SetStatus(kit.ResultSuccess)
	r.SetTitle("Successfully Purchased Cloud Server ECS!")
	r.SetExtra(btn.Node())
	tree := core.NewTree(r.Node())
	tree.Layout(core.Size{Width: 600, Height: 420})
	x, y := nodeCenter(btn.Node())
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
	if clicks != 1 {
		t.Fatalf("clicks=%d want 1", clicks)
	}
}

func TestResult_PRD_05_CustomIcon(t *testing.T) {
	custom := primitive.NewIcon("star")
	r := kit.NewResult()
	r.SetIcon(custom)
	if r.IconNode() != custom {
		t.Fatalf("custom icon not used: %T", r.IconNode())
	}
	r.SetIconName("heart")
	ic, ok := r.IconNode().(*primitive.Icon)
	if !ok || ic.Name != "heart" {
		t.Fatalf("named icon=%T/%v want heart", r.IconNode(), ic)
	}
}

func TestResult_PRD_06_SubTitleVisible(t *testing.T) {
	r := kit.NewResult()
	r.SetSubTitle("Order number: 2017182818828182881")
	if r.SubTitleNode() == nil || r.SubTitleNode().Value != r.SubTitle {
		t.Fatalf("subtitle node=%v value=%q", r.SubTitleNode(), r.SubTitle)
	}
}

func TestResult_PRD_07_14_OfficialMainExamples(t *testing.T) {
	for _, tc := range []struct {
		id     string
		build  func() *kit.Result
		status kit.ResultStatus
		title  string
	}{
		{"RES-07 Success", buildResultExampleSuccess, kit.ResultSuccess, "Successfully Purchased Cloud Server ECS!"},
		{"RES-08 Info", buildResultExampleInfo, kit.ResultInfo, "Your operation has been executed"},
		{"RES-09 Warning", buildResultExampleWarning, kit.ResultWarning, "There are some problems with your operation."},
		{"RES-10 403", buildResultExample403, kit.Result403, "403"},
		{"RES-11 404", buildResultExample404, kit.Result404, "404"},
		{"RES-12 500", buildResultExample500, kit.Result500, "500"},
		{"RES-13 Error", buildResultExampleError, kit.ResultError, "Submission Failed"},
		{"RES-14 CustomIcon", buildResultExampleCustomIcon, kit.ResultInfo, "Great, we have done all the operations!"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			r := tc.build()
			if r.Status != tc.status {
				t.Fatalf("status=%q want %q", r.Status, tc.status)
			}
			if r.Title != tc.title {
				t.Fatalf("title=%q want %q", r.Title, tc.title)
			}
			if r.IconNode() == nil {
				t.Fatal("missing icon")
			}
			if len(r.Extra) == 0 {
				t.Fatal("official main example should include extra action")
			}
			if tc.status == kit.ResultError && r.BodyNode() == nil {
				t.Fatal("error example should include body")
			}
			if sz := r.Node().Layout(core.Loose(760, 520)); sz.Width <= 0 || sz.Height <= 0 {
				t.Fatalf("bad layout size=%v", sz)
			}
		})
	}
}

func TestResult_PRD_15_TokenMetrics(t *testing.T) {
	r := buildResultExampleError()
	root := r.Node().(*primitive.Flex)
	if !approxResultFloat(root.Padding.Top, 48, 0.5) || !approxResultFloat(root.Padding.Left, 32, 0.5) {
		t.Fatalf("root padding=%+v want block48 inline32", root.Padding)
	}
	ic := r.IconNode().(*primitive.Icon)
	if !approxResultFloat(ic.Size, 72, 0.5) {
		t.Fatalf("icon size=%v want 72", ic.Size)
	}
	if !approxResultFloat(r.TitleNode().FontSize, 24, 0.5) {
		t.Fatalf("title font=%v want 24", r.TitleNode().FontSize)
	}
	if !approxResultFloat(r.SubTitleNode().FontSize, 14, 0.5) {
		t.Fatalf("subtitle font=%v want 14", r.SubTitleNode().FontSize)
	}
	if !approxResultFloat(r.ExtraNode().Gap, 8, 0.5) || !approxResultFloat(r.ExtraNode().Padding.Top, 24, 0.5) {
		t.Fatalf("extra gap/padding=%v/%+v want gap8 top24", r.ExtraNode().Gap, r.ExtraNode().Padding)
	}
	body := r.BodyNode()
	if !approxResultFloat(body.Padding.Top, 24, 0.5) || !approxResultFloat(body.Padding.Left, 40, 0.5) {
		t.Fatalf("body padding=%+v want block24 inline40", body.Padding)
	}
}

func TestResult_PRD_16_TokenColors(t *testing.T) {
	th := kit.DefaultTheme()
	th.Tokens = th.Tokens.Clone()
	want := render.Hex("#00aa55")
	th.Tokens.Colors[core.TokenColorSuccess] = want
	r := kit.NewResult()
	r.SetTheme(th)
	r.SetStatus(kit.ResultSuccess)
	ic := r.IconNode().(*primitive.Icon)
	if !approxResultColor(ic.Color, want, 0.02) {
		t.Fatalf("icon color=%v want theme success %v", ic.Color, want)
	}
	if r.TitleNode() != nil && r.TitleNode().Color == want {
		t.Fatal("title should use text token, not status token")
	}
}

func TestResult_PRD_17_DisabledNotApplicable(t *testing.T) {
	r := kit.NewResult()
	r.SetTitle("Static result")
	if r.Root.Base().Role != "status" {
		t.Fatalf("role=%q want status", r.Root.Base().Role)
	}
	// Result has no disabled state in antd; interactive disabled behavior belongs to extra controls.
	btn := kit.NewButton("Disabled")
	btn.SetDisabled(true)
	r.SetExtra(btn.Node())
	if !btn.Disabled {
		t.Fatal("extra control disabled state should remain owned by Button")
	}
}

func TestResult_PRD_18_KeyboardFocusMainPath(t *testing.T) {
	clicks := 0
	btn := kit.NewButton("Next")
	btn.SetOnClick(func() { clicks++ })
	r := kit.NewResult()
	r.SetTitle("Great, we have done all the operations!")
	r.SetExtra(btn.Node())
	tree := core.NewTree(r.Node())
	tree.Layout(core.Size{Width: 520, Height: 360})
	tree.SetFocus(btn.Node())
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: " "})
	if clicks != 2 {
		t.Fatalf("keyboard clicks=%d want 2", clicks)
	}
	if !btn.Root.ShowFocusRing {
		t.Fatal("extra button focus ring should remain visible")
	}
}

func TestResult_PRD_LoadingTicker(t *testing.T) {
	r := kit.NewResult()
	r.SetStatus(kit.ResultSuccess)
	tree := core.NewTree(r.Node())
	r.AttachTicker(tree)
	r.SetLoading(true)
	if !tree.HasActiveTickers() {
		t.Fatal("loading should register ticker")
	}
	ic := r.IconNode().(*primitive.Icon)
	before := ic.SpinPhase
	tree.TickActive(0.2)
	if ic.SpinPhase == before {
		t.Fatalf("spin phase did not advance: %v", ic.SpinPhase)
	}
	r.SetLoading(false)
	if tree.HasActiveTickers() {
		t.Fatal("loading=false should remove ticker")
	}
}

func buildResultExampleSuccess() *kit.Result {
	r := kit.NewResult()
	r.SetStatus(kit.ResultSuccess)
	r.SetTitle("Successfully Purchased Cloud Server ECS!")
	r.SetSubTitle("Order number: 2017182818828182881 Cloud server configuration takes 1-5 minutes, please wait.")
	primary := kit.NewButton("Go Console")
	primary.SetType(kit.ButtonPrimary)
	r.SetExtra(primary.Node(), kit.NewButton("Buy Again").Node())
	return r
}

func buildResultExampleInfo() *kit.Result {
	r := kit.NewResult()
	r.SetTitle("Your operation has been executed")
	primary := kit.NewButton("Go Console")
	primary.SetType(kit.ButtonPrimary)
	r.SetExtra(primary.Node())
	return r
}

func buildResultExampleWarning() *kit.Result {
	r := kit.NewResult()
	r.SetStatus(kit.ResultWarning)
	r.SetTitle("There are some problems with your operation.")
	primary := kit.NewButton("Go Console")
	primary.SetType(kit.ButtonPrimary)
	r.SetExtra(primary.Node())
	return r
}

func buildResultExample403() *kit.Result {
	r := kit.NewResult()
	r.SetStatus(kit.Result403)
	r.SetTitle("403")
	r.SetSubTitle("Sorry, you are not authorized to access this page.")
	primary := kit.NewButton("Back Home")
	primary.SetType(kit.ButtonPrimary)
	r.SetExtra(primary.Node())
	return r
}

func buildResultExample404() *kit.Result {
	r := kit.NewResult()
	r.SetStatus(kit.Result404)
	r.SetTitle("404")
	r.SetSubTitle("Sorry, the page you visited does not exist.")
	primary := kit.NewButton("Back Home")
	primary.SetType(kit.ButtonPrimary)
	r.SetExtra(primary.Node())
	return r
}

func buildResultExample500() *kit.Result {
	r := kit.NewResult()
	r.SetStatus(kit.Result500)
	r.SetTitle("500")
	r.SetSubTitle("Sorry, something went wrong.")
	primary := kit.NewButton("Back Home")
	primary.SetType(kit.ButtonPrimary)
	r.SetExtra(primary.Node())
	return r
}

func buildResultExampleError() *kit.Result {
	r := kit.NewResult()
	r.SetStatus(kit.ResultError)
	r.SetTitle("Submission Failed")
	r.SetSubTitle("Please check and modify the following information before resubmitting.")
	primary := kit.NewButton("Go Console")
	primary.SetType(kit.ButtonPrimary)
	r.SetExtra(primary.Node(), kit.NewButton("Buy Again").Node())
	r.SetBody(
		primitive.NewText("The content you submitted has the following error:"),
		primitive.NewText("x Your account has been frozen. Thaw immediately >"),
		primitive.NewText("x Your account is not yet eligible to apply. Apply Unlock >"),
	)
	return r
}

func buildResultExampleCustomIcon() *kit.Result {
	r := kit.NewResult()
	r.SetIconName("star")
	r.SetTitle("Great, we have done all the operations!")
	primary := kit.NewButton("Next")
	primary.SetType(kit.ButtonPrimary)
	r.SetExtra(primary.Node())
	return r
}

func nodeCenter(n core.Node) (float64, float64) {
	var x, y float64
	for cur := n; cur != nil; cur = cur.Parent() {
		off := cur.Base().Offset()
		x += off.X
		y += off.Y
	}
	sz := n.Base().Size()
	return x + sz.Width/2, y + sz.Height/2
}

func approxResultFloat(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func approxResultColor(a, b render.RGBA, tol float64) bool {
	return math.Abs(a.R-b.R) <= tol &&
		math.Abs(a.G-b.G) <= tol &&
		math.Abs(a.B-b.B) <= tol &&
		math.Abs(a.A-b.A) <= tol
}
