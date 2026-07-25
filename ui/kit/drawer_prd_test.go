package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/drawer.md §6.9 — P0 PRD cases.
// L3/L4 (DRW-22/23) and P1 (DRW-15/24+) are deferred outside this P0 test set.

func TestDrawer_PRD_DRW_01_Defaults(t *testing.T) {
	d := kit.NewDrawer("Basic Drawer")
	if d.Open {
		t.Fatal("Open default should be false")
	}
	if d.Title != "Basic Drawer" {
		t.Fatalf("Title=%q", d.Title)
	}
	if d.Placement != kit.DrawerPlacementRight {
		t.Fatalf("Placement=%q want right", d.Placement)
	}
	if d.Size != kit.DrawerSizeDefault {
		t.Fatalf("Size=%q want default", d.Size)
	}
	if d.Width != kit.DefaultDrawerWidth || d.Height != kit.DefaultDrawerHeight {
		t.Fatalf("size aliases width=%v height=%v want 378", d.Width, d.Height)
	}
	if !d.Closable || !d.Mask || !d.MaskClosable || !d.Keyboard {
		t.Fatalf("default bools closable=%v mask=%v maskClosable=%v keyboard=%v",
			d.Closable, d.Mask, d.MaskClosable, d.Keyboard)
	}
	if d.Loading || d.DestroyOnHidden || d.Resizable {
		t.Fatalf("default flags loading=%v destroy=%v resizable=%v", d.Loading, d.DestroyOnHidden, d.Resizable)
	}
	if d.Node() == nil {
		t.Fatal("Node nil")
	}
}

func TestDrawer_PRD_DRW_02_OpenVisible(t *testing.T) {
	d, tree := mountedDrawer("D")
	d.SetOpen(true)
	tree.Layout(core.Size{Width: 800, Height: 600})
	if tree.Overlays().Len() != 1 {
		t.Fatalf("overlay len=%d want 1", tree.Overlays().Len())
	}
	if !d.Portal.Open {
		t.Fatal("portal should be open")
	}
}

func TestDrawer_PRD_DRW_03_OnClosePath(t *testing.T) {
	d, tree := mountedDrawer("D")
	closed := 0
	d.OnClose = func() { closed++ }
	d.SetOpen(true)
	tree.Layout(core.Size{Width: 800, Height: 600})

	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 20, Y: 300, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: 20, Y: 300, Button: core.ButtonLeft})
	if d.Open {
		t.Fatal("mask close should close drawer")
	}
	if closed != 1 {
		t.Fatalf("OnClose calls=%d want 1", closed)
	}
}

func TestDrawer_PRD_DRW_04_Placements(t *testing.T) {
	for _, tc := range []struct {
		p      kit.DrawerPlacement
		x, y   float64
		w, h   float64
		sizePx float64
	}{
		{kit.DrawerPlacementRight, 800 - 256, 0, 256, 600, 256},
		{kit.DrawerPlacementLeft, 0, 0, 256, 600, 256},
		{kit.DrawerPlacementTop, 0, 0, 800, 256, 256},
		{kit.DrawerPlacementBottom, 0, 600 - 256, 800, 256, 256},
	} {
		d, tree := mountedDrawer("D")
		d.SetPlacement(tc.p)
		d.SetSizePx(tc.sizePx)
		d.SetOpen(true)
		tree.Layout(core.Size{Width: 800, Height: 600})
		panel := largestDecorated(d.Scope)
		if panel == nil {
			t.Fatalf("%s panel not found", tc.p)
		}
		b := core.AbsoluteBounds(panel)
		assertNear(t, string(tc.p)+" x", b.Min.X, tc.x, 0.5)
		assertNear(t, string(tc.p)+" y", b.Min.Y, tc.y, 0.5)
		assertNear(t, string(tc.p)+" w", b.Width(), tc.w, 0.5)
		assertNear(t, string(tc.p)+" h", b.Height(), tc.h, 0.5)
	}
}

func TestDrawer_PRD_DRW_05_MaskClosableFalse(t *testing.T) {
	d, tree := mountedDrawer("D")
	d.SetMaskClosable(false)
	d.SetOpen(true)
	tree.Layout(core.Size{Width: 800, Height: 600})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 20, Y: 300, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: 20, Y: 300, Button: core.ButtonLeft})
	if !d.Open {
		t.Fatal("maskClosable=false should keep drawer open")
	}
}

func TestDrawer_PRD_DRW_06_EscapeKeyboard(t *testing.T) {
	d, tree := mountedDrawer("D")
	d.SetOpen(true)
	tree.Layout(core.Size{Width: 800, Height: 600})
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Escape"})
	if d.Open {
		t.Fatal("Escape should close when keyboard=true")
	}

	d.SetKeyboard(false)
	d.SetOpen(true)
	tree.Layout(core.Size{Width: 800, Height: 600})
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Escape"})
	if !d.Open {
		t.Fatal("Escape should not close when keyboard=false")
	}
}

func TestDrawer_PRD_DRW_07_DefaultWidth(t *testing.T) {
	d, tree := mountedDrawer("D")
	d.SetOpen(true)
	tree.Layout(core.Size{Width: 800, Height: 600})
	panel := largestDecorated(d.Scope)
	if panel == nil {
		t.Fatal("panel not found")
	}
	assertNear(t, "default panel width", panel.Size().Width, kit.DefaultDrawerWidth, 0.5)
}

func TestDrawer_PRD_DRW_08_FooterVisible(t *testing.T) {
	d := kit.NewDrawer("D")
	d.SetFooter(kit.NewButton("Submit").Node())
	if slot := primitive.FindSlot(d.Scope, "footer"); slot == nil || slot.Child() == nil {
		t.Fatal("footer slot should be mounted")
	}
}

func TestDrawer_PRD_DRW_09_DestroyOnHidden(t *testing.T) {
	d := kit.NewDrawer("D")
	d.SetContent(kit.NewText("body").Node())
	d.SetDestroyOnHidden(true)
	d.SetOpen(true)
	body := primitive.FindSlot(d.Scope, "body")
	if body == nil || body.Child() == nil {
		t.Fatal("body should mount while open")
	}
	d.SetOpen(false)
	if body.Child() != nil {
		t.Fatal("body should unmount when hidden and destroyOnHidden=true")
	}
}

func TestDrawer_PRD_DRW_10_BasicOfficialDemo(t *testing.T) {
	d, tree := mountedDrawer("Basic Drawer")
	d.SetContent(simpleDrawerParagraphs("Some contents...", 3))
	d.SetOpen(true)
	tree.Layout(core.Size{Width: 800, Height: 600})
	if tree.Overlays().Len() != 1 || primitive.FindSlot(d.Scope, "body").Child() == nil {
		t.Fatal("basic drawer demo should render open body")
	}
}

func TestDrawer_PRD_DRW_11_PlacementOfficialDemo(t *testing.T) {
	d, tree := mountedDrawer("Basic Drawer")
	for _, p := range []kit.DrawerPlacement{
		kit.DrawerPlacementTop,
		kit.DrawerPlacementRight,
		kit.DrawerPlacementBottom,
		kit.DrawerPlacementLeft,
	} {
		d.SetPlacement(p)
		d.SetClosable(false)
		d.SetOpen(true)
		tree.Layout(core.Size{Width: 800, Height: 600})
		if d.Placement != p || !d.Open {
			t.Fatalf("placement demo p=%s open=%v", d.Placement, d.Open)
		}
	}
}

func TestDrawer_PRD_DRW_12_ResizableOfficialDemo(t *testing.T) {
	d, tree := mountedDrawer("Resizable Drawer")
	d.SetSizePx(256)
	d.SetResizable(true)
	d.SetResizeBounds(200, 400)
	var started, resized, ended bool
	d.OnResizeStart = func() { started = true }
	d.OnResize = func(size float64) { resized = size >= 296-0.5 && size <= 296+0.5 }
	d.OnResizeEnd = func() { ended = true }
	d.SetOpen(true)
	tree.Layout(core.Size{Width: 800, Height: 600})
	handle := firstDraggable(d.Scope)
	if handle == nil {
		t.Fatal("resize handle not found")
	}
	handle.HandlePointer(&core.PointerEvent{Type: core.PointerDown, X: 400, Y: 300, Button: core.ButtonLeft})
	handle.HandlePointer(&core.PointerEvent{Type: core.PointerMove, X: 360, Y: 300, Button: core.ButtonLeft})
	handle.HandlePointer(&core.PointerEvent{Type: core.PointerUp, X: 360, Y: 300, Button: core.ButtonLeft})
	if !started || !resized || !ended {
		t.Fatalf("resize callbacks started=%v resized=%v ended=%v size=%v", started, resized, ended, d.Width)
	}
	assertNear(t, "resized width", d.Width, 296, 0.5)
}

func TestDrawer_PRD_DRW_13_LoadingOfficialDemo(t *testing.T) {
	d, tree := mountedDrawer("Loading Drawer")
	d.SetDestroyOnHidden(true)
	d.SetLoading(true)
	d.SetOpen(true)
	tree.Layout(core.Size{Width: 800, Height: 600})
	body := primitive.FindSlot(d.Scope, "body")
	if body == nil || body.Child() == nil || body.Child().TypeID() != "kit.Skeleton" {
		t.Fatalf("loading body=%v", body)
	}
	d.AttachTicker(tree)
	if !d.Tick(0.016) {
		t.Fatal("loading drawer should tick while open")
	}
	d.SetLoading(false)
	if d.Tick(0.016) {
		t.Fatal("non-loading drawer should not tick")
	}
}

func TestDrawer_PRD_DRW_14_ExtraOfficialDemo(t *testing.T) {
	d, _ := mountedDrawer("Drawer with extra actions")
	row := primitive.Row(kit.NewButton("Cancel").Node(), kit.NewButton("OK").Node())
	row.Gap = 8
	d.SetExtra(row)
	d.SetSizePx(500)
	if slot := primitive.FindSlot(d.Scope, "extra"); slot == nil || slot.Child() == nil {
		t.Fatal("extra actions should mount in header")
	}
	if d.Width != 500 {
		t.Fatalf("size=%v want 500", d.Width)
	}
}

func TestDrawer_PRD_DRW_16_FormOfficialDemo(t *testing.T) {
	d, tree := mountedDrawer("Create a new account")
	d.SetSizePx(720)
	d.SetExtra(primitive.Row(kit.NewButton("Cancel").Node(), kit.NewButton("Submit").Node()))
	form := kit.NewForm(nil)
	form.BindInput("name", kit.NewInput("Please enter user name"), true, "Name")
	form.BindInput("url", kit.NewInput("Please enter url"), true, "Url")
	form.BindInput("description", kit.NewTextArea("please enter url description", 4).Input, true, "Description")
	d.SetContent(form.Node())
	d.SetOpen(true)
	tree.Layout(core.Size{Width: 900, Height: 700})
	if d.Width != 720 || primitive.FindSlot(d.Scope, "body").Child() == nil {
		t.Fatal("form drawer should render content at size 720")
	}
}

func TestDrawer_PRD_DRW_17_ProfileOfficialDemo(t *testing.T) {
	d, tree := mountedDrawer("User Profile")
	d.SetSizePx(640)
	d.SetClosable(false)
	d.SetContent(profileContent())
	d.SetOpen(true)
	tree.Layout(core.Size{Width: 900, Height: 700})
	if d.Closable {
		t.Fatal("profile demo should support closable=false")
	}
	if d.Width != 640 || primitive.FindSlot(d.Scope, "body").Child() == nil {
		t.Fatal("profile drawer should render content at size 640")
	}
}

func TestDrawer_PRD_DRW_18_19_L2MetricsAndTheme(t *testing.T) {
	d, tree := mountedDrawer("D")
	d.SetOpen(true)
	tree.Layout(core.Size{Width: 800, Height: 600})
	panel := largestDecorated(d.Scope)
	if panel == nil {
		t.Fatal("panel not found")
	}
	assertNear(t, "padding left", panel.Padding.Left, kit.DefaultDrawerPadding, 0.5)
	assertNear(t, "padding top", panel.Padding.Top, kit.DefaultDrawerPadding, 0.5)
	assertNear(t, "radius", panel.Radius, 0, 0.01)
	th := kit.DefaultTheme()
	if !drawerApproxColor(panel.Background, th.Color(core.TokenColorBgContainer), 0.02) {
		t.Fatalf("panel bg=%v want token bgContainer=%v", panel.Background, th.Color(core.TokenColorBgContainer))
	}
	d.SetSize(kit.DrawerSizeLarge)
	if d.Width != kit.DefaultDrawerLargeSize {
		t.Fatalf("large size=%v want %v", d.Width, kit.DefaultDrawerLargeSize)
	}
}

func TestDrawer_PRD_DRW_20_LoadingTickerToken(t *testing.T) {
	d, tree := mountedDrawer("D")
	d.SetLoading(true)
	d.SetOpen(true)
	tree.Layout(core.Size{Width: 800, Height: 600})
	d.AttachTicker(tree)
	if !d.Tick(0.05) {
		t.Fatal("loading should request continued ticking")
	}
	body := primitive.FindSlot(d.Scope, "body")
	if body == nil || body.Child() == nil || body.Child().TypeID() != "kit.Skeleton" {
		t.Fatal("loading should use Skeleton body")
	}
}

func TestDrawer_PRD_DRW_21_A11yFocus(t *testing.T) {
	d := kit.NewDrawer("Accessible Drawer")
	d.SetContent(kit.NewText("body").Node())
	d.Viewport = core.Size{Width: 800, Height: 600}
	bg := primitive.NewPressable(primitive.NewText("bg"))
	root := primitive.Column(bg, d.Node())
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 800, Height: 600})
	tree.SetFocus(bg)
	d.SetOpen(true)
	tree.Layout(core.Size{Width: 800, Height: 600})
	if tree.Focus() == bg {
		t.Fatal("focus should enter drawer")
	}
	dialog := firstRole(d.Scope, "dialog")
	if dialog == nil || dialog.Base().Label != "Accessible Drawer" {
		t.Fatalf("dialog role/label missing: %v", dialog)
	}
}

func mountedDrawer(title string) (*kit.Drawer, *core.Tree) {
	d := kit.NewDrawer(title)
	d.SetContent(kit.NewText("body").Node())
	d.Viewport = core.Size{Width: 800, Height: 600}
	tree := core.NewTree(d.Node())
	tree.Layout(core.Size{Width: 800, Height: 600})
	return d, tree
}

func simpleDrawerParagraphs(value string, count int) core.Node {
	col := primitive.Column()
	col.Gap = 8
	for i := 0; i < count; i++ {
		col.AddChild(kit.NewText(value).Node())
	}
	return col
}

func profileContent() core.Node {
	col := primitive.Column()
	col.Gap = 12
	for _, s := range []string{
		"Personal",
		"Full Name: Lily",
		"Account: AntDesign@example.com",
		"Company",
		"Position: Programmer",
		"Contacts",
		"GitHub: github.com/ant-design/ant-design",
	} {
		col.AddChild(kit.NewText(s).Node())
	}
	return col
}

func largestDecorated(root core.Node) *primitive.Decorated {
	var best *primitive.Decorated
	var area float64
	walkNodes(root, func(n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			sz := d.Size()
			a := sz.Width * sz.Height
			if a > area {
				best, area = d, a
			}
		}
	})
	return best
}

func firstDraggable(root core.Node) *primitive.Draggable {
	var out *primitive.Draggable
	walkNodes(root, func(n core.Node) {
		if out != nil {
			return
		}
		if d, ok := n.(*primitive.Draggable); ok {
			out = d
		}
	})
	return out
}

func firstRole(root core.Node, role string) core.Node {
	var out core.Node
	walkNodes(root, func(n core.Node) {
		if out == nil && n.Base().Role == role {
			out = n
		}
	})
	return out
}

func walkNodes(n core.Node, fn func(core.Node)) {
	if n == nil {
		return
	}
	fn(n)
	for _, c := range n.Children() {
		walkNodes(c, fn)
	}
}

func assertNear(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if got < want-tol || got > want+tol {
		t.Fatalf("%s=%v want %v±%v", name, got, want, tol)
	}
}

func drawerApproxColor(a, b render.RGBA, tol float64) bool {
	return drawerAbs(a.R-b.R) <= tol && drawerAbs(a.G-b.G) <= tol && drawerAbs(a.B-b.B) <= tol && drawerAbs(a.A-b.A) <= tol
}

func drawerAbs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
