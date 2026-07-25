package kit_test

import (
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/notification.md §6.9 — P0 PRD cases (NTF-01 … NTF-19).
// L3/L4 (NTF-20/21) and P1 (NTF-22) are intentionally not covered here.

func TestNotification_PRD_01_Defaults(t *testing.T) {
	// NTF-01
	n := kit.NewNotification()
	if n.Duration != kit.DefaultNotificationDuration {
		t.Fatalf("Duration=%v want %v", n.Duration, kit.DefaultNotificationDuration)
	}
	if n.Placement != kit.NotificationTopRight {
		t.Fatalf("Placement=%v want topRight", n.Placement)
	}
	if n.Top != kit.DefaultNotificationTop || n.Bottom != kit.DefaultNotificationBottom {
		t.Fatalf("Top/Bottom=%v/%v", n.Top, n.Bottom)
	}
	if !n.Closable {
		t.Fatal("Closable default should be true")
	}
	if n.Stack {
		t.Fatal("Stack default should be false")
	}
	if n.StackThreshold != kit.DefaultNotificationStackThreshold {
		t.Fatalf("StackThreshold=%v", n.StackThreshold)
	}
	if n.Node() == nil {
		t.Fatal("nil node")
	}
}

func TestNotification_PRD_02_OpenVisible(t *testing.T) {
	// NTF-02
	n := kit.NewNotification()
	n.Open(kit.NotificationConfig{
		Title:       "Notification Title",
		Description: "body",
		Duration:    0,
		DurationSet: true,
	})
	if n.Count() != 1 {
		t.Fatalf("count=%d want 1", n.Count())
	}
	if n.Portal == nil || !n.Portal.Open {
		t.Fatal("open should open portal")
	}
	it := n.Items()[0]
	if it.Title != "Notification Title" || it.Description != "body" {
		t.Fatalf("item=%+v", it)
	}
}

func TestNotification_PRD_03_PlacementBottomLeft(t *testing.T) {
	// NTF-03 / NTF-S2
	n := kit.NewNotification()
	n.Open(kit.NotificationConfig{
		Key:         "bl",
		Title:       "Notification bottomLeft",
		Description: "This is the content of the notification.",
		Placement:   kit.NotificationBottomLeft,
		Duration:    0,
		DurationSet: true,
	})
	if n.PlacementOf("bl") != kit.NotificationBottomLeft {
		t.Fatalf("placement=%v", n.PlacementOf("bl"))
	}
	tree := layoutNotification(n)
	off, ok := n.ItemOffset("bl")
	if !ok {
		t.Fatal("item offset missing")
	}
	// bottom-left: near left edge and bottom half of 800x600
	if off.X > 40 {
		t.Fatalf("bottomLeft X=%v want near left edge", off.X)
	}
	if off.Y < 200 {
		t.Fatalf("bottomLeft Y=%v want lower half", off.Y)
	}
	_ = tree
}

func TestNotification_PRD_04_DurationExpires(t *testing.T) {
	// NTF-04 / NTF-S3
	n := kit.NewNotification()
	closed := 0
	n.Open(kit.NotificationConfig{
		Title:       "short",
		Duration:    0.1,
		DurationSet: true,
		OnClose:     func() { closed++ },
	})
	n.Tick(0.11)
	if n.Count() != 0 || closed != 1 {
		t.Fatalf("count=%d closed=%d want 0/1", n.Count(), closed)
	}
}

func TestNotification_PRD_05_KeyUpdate(t *testing.T) {
	// NTF-05 / NTF-S4
	n := kit.NewNotification()
	n.Open(kit.NotificationConfig{
		Key:         "updatable",
		Title:       "Notification Title",
		Description: "description.",
		Duration:    0,
		DurationSet: true,
	})
	n.Open(kit.NotificationConfig{
		Key:         "updatable",
		Title:       "New Title",
		Description: "New description.",
		Duration:    0,
		DurationSet: true,
	})
	if n.Count() != 1 {
		t.Fatalf("count=%d want 1", n.Count())
	}
	it := n.Items()[0]
	if it.Title != "New Title" || it.Description != "New description." {
		t.Fatalf("updated item=%+v", it)
	}
}

func TestNotification_PRD_06_ManualClose(t *testing.T) {
	// NTF-06 / NTF-S5
	n := kit.NewNotification()
	closed := 0
	n.Open(kit.NotificationConfig{
		Key:         "c1",
		Title:       "closable",
		Duration:    0,
		DurationSet: true,
		OnClose:     func() { closed++ },
	})
	n.Destroy("c1")
	if n.Count() != 0 || closed != 1 {
		t.Fatalf("count=%d closed=%d", n.Count(), closed)
	}
}

func TestNotification_PRD_07_ActionsClickable(t *testing.T) {
	// NTF-07 / NTF-S6
	n := kit.NewNotification()
	clicks := 0
	n.Open(kit.NotificationConfig{
		Title:       "Notification Title",
		Description: "A function will be called after the notification is closed.",
		Duration:    0,
		DurationSet: true,
		Actions: []kit.NotificationAction{
			{Label: "Confirm", Primary: true, OnClick: func() { clicks++ }},
		},
	})
	if n.Items()[0].ActionCount != 1 {
		t.Fatalf("actions=%d", n.Items()[0].ActionCount)
	}
	tree := layoutNotification(n)
	btn := firstNotificationActionPressable(tree)
	if btn == nil {
		t.Fatal("action button not found")
	}
	btn.OnClick(nil)
	if clicks != 1 {
		t.Fatalf("clicks=%d want 1", clicks)
	}
}

func TestNotification_PRD_08_OfficialHooksExample(t *testing.T) {
	// NTF-08
	n := kit.NewNotification()
	tree := layoutNotification(n)
	n.Info(kit.NotificationConfig{
		Title:       "Notification topLeft",
		Description: "Hello, Ant Design!",
		Placement:   kit.NotificationTopLeft,
		Duration:    0,
		DurationSet: true,
	})
	tree.Layout(core.Size{Width: 800, Height: 600})
	if !notificationHasText(tree, "Hello, Ant Design!") {
		t.Fatal("hooks example text not rendered")
	}
	if !notificationHasText(tree, "Notification topLeft") {
		t.Fatal("hooks title not rendered")
	}
}

func TestNotification_PRD_09_OfficialDurationExample(t *testing.T) {
	// NTF-09
	n := kit.NewNotification()
	n.Open(kit.NotificationConfig{
		Title:       "Notification Title",
		Description: "I will never close automatically. This is a purposely very very long description that has many many characters and words.",
		Duration:    0,
		DurationSet: true,
	})
	n.Tick(60)
	if n.Count() != 1 {
		t.Fatalf("duration=0 should stay open, count=%d", n.Count())
	}
}

func TestNotification_PRD_10_OfficialWithIconExample(t *testing.T) {
	// NTF-10
	n := kit.NewNotification()
	n.Success(kit.NotificationConfig{
		Title:       "Notification Title",
		Description: "This is the content of the notification.",
		Duration:    0,
		DurationSet: true,
	})
	n.Info(kit.NotificationConfig{
		Title:       "Notification Title",
		Description: "This is the content of the notification.",
		Duration:    0,
		DurationSet: true,
	})
	n.Warning(kit.NotificationConfig{
		Title:       "Notification Title",
		Description: "This is the content of the notification.",
		Duration:    0,
		DurationSet: true,
	})
	n.Error(kit.NotificationConfig{
		Title:       "Notification Title",
		Description: "This is the content of the notification.",
		Duration:    0,
		DurationSet: true,
	})
	if n.Count() != 4 {
		t.Fatalf("count=%d want 4", n.Count())
	}
	got := n.Items()
	wantIcons := []string{"check", "info", "info", "close"}
	for i, w := range wantIcons {
		if got[i].IconName != w {
			t.Fatalf("item[%d] icon=%q want %q type=%v", i, got[i].IconName, w, got[i].Type)
		}
	}
}

func TestNotification_PRD_11_OfficialWithBtnExample(t *testing.T) {
	// NTF-11
	n := kit.NewNotification()
	destroyed := 0
	key := "open1"
	n.Open(kit.NotificationConfig{
		Key:         key,
		Title:       "Notification Title",
		Description: "A function will be be called after the notification is closed.",
		Duration:    0,
		DurationSet: true,
		Actions: []kit.NotificationAction{
			{Label: "Destroy All", OnClick: func() { n.Destroy(); destroyed++ }},
			{Label: "Confirm", Primary: true, OnClick: func() { n.Destroy(key) }},
		},
		OnClose: func() {},
	})
	tree := layoutNotification(n)
	if !notificationHasText(tree, "Destroy All") || !notificationHasText(tree, "Confirm") {
		t.Fatal("action labels not rendered")
	}
	// Invoke Destroy All path via API parity
	n.Destroy()
	if n.Count() != 0 {
		t.Fatalf("count=%d after destroy all", n.Count())
	}
	_ = destroyed
}

func TestNotification_PRD_12_OfficialCustomIconExample(t *testing.T) {
	// NTF-12
	n := kit.NewNotification()
	n.Open(kit.NotificationConfig{
		Title:       "Notification Title",
		Description: "This is the content of the notification.",
		IconName:    "star",
		Duration:    0,
		DurationSet: true,
		Style:       kit.Style{Text: render.Hex("#108ee9")},
	})
	if n.Items()[0].IconName != "star" {
		t.Fatalf("icon=%q want star", n.Items()[0].IconName)
	}
	tree := layoutNotification(n)
	ic := firstNotificationIcon(tree)
	if ic == nil {
		t.Fatal("custom icon not found")
	}
	if ic.Name != "star" {
		t.Fatalf("icon name=%q", ic.Name)
	}
}

func TestNotification_PRD_13_OfficialPlacementExample(t *testing.T) {
	// NTF-13
	n := kit.NewNotification()
	placements := []kit.NotificationPlacement{
		kit.NotificationTop, kit.NotificationBottom,
		kit.NotificationTopLeft, kit.NotificationTopRight,
		kit.NotificationBottomLeft, kit.NotificationBottomRight,
	}
	for _, p := range placements {
		n.Open(kit.NotificationConfig{
			Key:         string(p),
			Title:       "Notification " + string(p),
			Description: "This is the content of the notification.",
			Placement:   p,
			Duration:    0,
			DurationSet: true,
		})
	}
	if n.Count() != 6 {
		t.Fatalf("count=%d want 6", n.Count())
	}
	tree := layoutNotification(n)
	tr, okTR := n.ItemOffset(string(kit.NotificationTopRight))
	tl, okTL := n.ItemOffset(string(kit.NotificationTopLeft))
	br, okBR := n.ItemOffset(string(kit.NotificationBottomRight))
	if !okTR || !okTL || !okBR {
		t.Fatalf("offsets missing tr=%v tl=%v br=%v", okTR, okTL, okBR)
	}
	if tl.X >= tr.X {
		t.Fatalf("topLeft X=%v should be left of topRight X=%v", tl.X, tr.X)
	}
	if tr.Y >= br.Y {
		t.Fatalf("topRight Y=%v should be above bottomRight Y=%v", tr.Y, br.Y)
	}
	_ = tree
}

func TestNotification_PRD_14_OfficialUpdateExample(t *testing.T) {
	// NTF-14
	n := kit.NewNotification()
	const key = "updatable"
	n.Open(kit.NotificationConfig{
		Key: key, Title: "Notification Title", Description: "description.",
		Duration: 0, DurationSet: true,
	})
	n.Open(kit.NotificationConfig{
		Key: key, Title: "New Title", Description: "New description.",
		Duration: 0, DurationSet: true,
	})
	if n.Count() != 1 {
		t.Fatalf("count=%d", n.Count())
	}
	it := n.Items()[0]
	if it.Title != "New Title" || it.Description != "New description." {
		t.Fatalf("item=%+v", it)
	}
}

func TestNotification_PRD_15_OfficialStackExample(t *testing.T) {
	// NTF-15
	n := kit.NewNotification()
	n.SetStack(true)
	n.SetStackThreshold(3)
	for i := 0; i < 4; i++ {
		n.Open(kit.NotificationConfig{
			Title:       "Notification Title",
			Description: "This is the content of the notification.",
			Duration:    0,
			DurationSet: true,
		})
	}
	if n.Count() != 4 {
		t.Fatalf("count=%d want 4", n.Count())
	}
	vis := n.VisibleItems()
	if len(vis) != 1 {
		t.Fatalf("visible=%d want 1 collapsed", len(vis))
	}
}

func TestNotification_PRD_16_TokenGeometry(t *testing.T) {
	// NTF-16
	n := kit.NewNotification()
	n.Open(kit.NotificationConfig{
		Title: "geometry", Description: "desc",
		Duration: 0, DurationSet: true,
	})
	tree := layoutNotification(n)
	dec := firstNotificationDecorated(tree)
	if dec == nil {
		t.Fatal("decorated card not found")
	}
	th := kit.DefaultTheme()
	wantRadius := th.SizeOr(core.TokenBorderRadiusLG, kit.DefaultNotificationRadius)
	if !approxNtf(dec.Radius, wantRadius, 0.5) {
		t.Fatalf("radius=%v want %v", dec.Radius, wantRadius)
	}
	wantLine := th.SizeOr(core.TokenLineWidth, 1)
	if !approxNtf(dec.BorderWidth, wantLine, 0.5) {
		t.Fatalf("border=%v want %v", dec.BorderWidth, wantLine)
	}
	if !approxNtf(dec.Width, kit.DefaultNotificationWidth, 0.5) {
		t.Fatalf("width=%v want %v", dec.Width, kit.DefaultNotificationWidth)
	}
	pad := dec.Padding
	wantPadV := th.SizeOr(core.TokenPadding, kit.DefaultNotificationPadV)
	wantPadH := th.SizeOr(core.TokenPaddingLG, kit.DefaultNotificationPadH)
	if !approxNtf(pad.Top, wantPadV, 0.5) || !approxNtf(pad.Left, wantPadH, 0.5) {
		t.Fatalf("padding=%+v want V=%v H=%v", pad, wantPadV, wantPadH)
	}
}

func TestNotification_PRD_17_DefaultSkinUsesThemeToken(t *testing.T) {
	// NTF-17
	n := kit.NewNotification()
	th := kit.DefaultTheme()
	th.Tokens.Colors[core.TokenColorSuccess] = render.Hex("#00AA55")
	n.SetTheme(th)
	n.Success(kit.NotificationConfig{
		Title: "token", Description: "skin",
		Duration: 0, DurationSet: true,
	})
	tree := layoutNotification(n)
	ic := firstNotificationIcon(tree)
	if ic == nil {
		t.Fatal("icon not found")
	}
	if !approxColorNtf(ic.Color, render.Hex("#00AA55"), 0.01) {
		t.Fatalf("icon color=%v want theme success", ic.Color)
	}
	dec := firstNotificationDecorated(tree)
	if dec == nil {
		t.Fatal("card not found")
	}
	bg := th.Color(core.TokenColorBgContainer)
	if !approxColorNtf(dec.Background, bg, 0.01) {
		t.Fatalf("bg=%v want container token %v", dec.Background, bg)
	}
}

func TestNotification_PRD_18_DisabledNotApplicable(t *testing.T) {
	// NTF-18 — Notification has no disabled state; document N/A for 适用者.
	n := kit.NewNotification()
	if n.Node() == nil {
		t.Fatal("nil")
	}
}

func TestNotification_PRD_19_CloseFocusableNoBodyFocusSteal(t *testing.T) {
	// NTF-19
	n := kit.NewNotification()
	clicks := 0
	n.Open(kit.NotificationConfig{
		Title:       "clickable",
		Description: "body",
		Duration:    0,
		DurationSet: true,
		OnClick:     func() { clicks++ },
	})
	tree := layoutNotification(n)
	body := firstNotificationBodyPressable(tree)
	if body == nil {
		t.Fatal("body pressable not found")
	}
	if body.Focusable {
		t.Fatal("notice body should not steal focus by default")
	}
	body.OnClick(nil)
	if clicks != 1 {
		t.Fatalf("clicks=%d want 1", clicks)
	}
	closeBtn := firstNotificationClosePressable(tree)
	if closeBtn == nil {
		t.Fatal("close button not found")
	}
	if !closeBtn.Focusable || !closeBtn.ShowFocusRing {
		t.Fatalf("close focusable=%v ring=%v", closeBtn.Focusable, closeBtn.ShowFocusRing)
	}
	if closeBtn.Base().Role != "button" {
		t.Fatalf("close role=%q", closeBtn.Base().Role)
	}
}

func layoutNotification(n *kit.Notification) *core.Tree {
	n.Viewport = core.Size{Width: 800, Height: 600}
	tree := core.NewTree(n.Node())
	tree.Layout(core.Size{Width: 800, Height: 600})
	return tree
}

func notificationHasText(tree *core.Tree, sub string) bool {
	return walkNotificationOverlay(tree, func(node core.Node) bool {
		tx, ok := node.(*primitive.Text)
		return ok && strings.Contains(tx.Value, sub)
	})
}

func firstNotificationDecorated(tree *core.Tree) *primitive.Decorated {
	var out *primitive.Decorated
	walkNotificationOverlay(tree, func(node core.Node) bool {
		if d, ok := node.(*primitive.Decorated); ok {
			out = d
			return true
		}
		return false
	})
	return out
}

func firstNotificationIcon(tree *core.Tree) *primitive.Icon {
	var out *primitive.Icon
	walkNotificationOverlay(tree, func(node core.Node) bool {
		if ic, ok := node.(*primitive.Icon); ok {
			out = ic
			return true
		}
		return false
	})
	return out
}

func firstNotificationActionPressable(tree *core.Tree) *primitive.Pressable {
	var out *primitive.Pressable
	walkNotificationOverlay(tree, func(node core.Node) bool {
		p, ok := node.(*primitive.Pressable)
		if !ok {
			return false
		}
		// kit.Button pressable typically carries the label as Base.Label
		if p.Base().Label == "Confirm" || (p.Base().Role == "button" && p.Base().Label != kit.DefaultNotificationCloseAria && p.Click != nil) {
			// Prefer non-close buttons; Confirm may be nested.
			if p.Base().Label == kit.DefaultNotificationCloseAria {
				return false
			}
			out = p
			// keep searching for exact Confirm label
			if p.Base().Label == "Confirm" {
				return true
			}
		}
		return false
	})
	return out
}

func firstNotificationBodyPressable(tree *core.Tree) *primitive.Pressable {
	var out *primitive.Pressable
	walkNotificationOverlay(tree, func(node core.Node) bool {
		p, ok := node.(*primitive.Pressable)
		if !ok {
			return false
		}
		if p.Base().Role == "button" {
			return false
		}
		if p.Base().Role == "alert" || p.Base().Role == "status" || strings.Contains(p.Base().Label, "clickable") {
			out = p
			return true
		}
		return false
	})
	return out
}

func firstNotificationClosePressable(tree *core.Tree) *primitive.Pressable {
	var out *primitive.Pressable
	walkNotificationOverlay(tree, func(node core.Node) bool {
		p, ok := node.(*primitive.Pressable)
		if !ok {
			return false
		}
		if p.Base().Role == "button" && p.Base().Label == kit.DefaultNotificationCloseAria {
			out = p
			return true
		}
		return false
	})
	return out
}

func walkNotificationOverlay(tree *core.Tree, visit func(core.Node) bool) bool {
	if tree == nil || visit == nil {
		return false
	}
	var walk func(core.Node) bool
	walk = func(node core.Node) bool {
		if node == nil {
			return false
		}
		if visit(node) {
			return true
		}
		for _, c := range node.Children() {
			if walk(c) {
				return true
			}
		}
		return false
	}
	for _, e := range tree.Overlays().Entries() {
		if walk(e.Node) {
			return true
		}
	}
	return walk(tree.Root())
}

func approxNtf(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

func approxColorNtf(a, b render.RGBA, tol float64) bool {
	return approxNtf(a.R, b.R, tol) && approxNtf(a.G, b.G, tol) && approxNtf(a.B, b.B, tol)
}
