package kit_test

import (
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/message.md §6.9 — P0 PRD cases (MSG-01 … MSG-20).
// L3/L4 (MSG-21/22) and P1 (MSG-23+) are intentionally not covered here.

func TestMessage_PRD_01_Defaults(t *testing.T) {
	msg := kit.NewMessage()
	if msg.Duration != kit.DefaultMessageDuration {
		t.Fatalf("Duration=%v want %v", msg.Duration, kit.DefaultMessageDuration)
	}
	if msg.Top != kit.DefaultMessageTop {
		t.Fatalf("Top=%v want %v", msg.Top, kit.DefaultMessageTop)
	}
	if msg.Stack {
		t.Fatal("Stack default should be false")
	}
	if msg.StackThreshold != kit.DefaultMessageStackThreshold {
		t.Fatalf("StackThreshold=%v want %v", msg.StackThreshold, kit.DefaultMessageStackThreshold)
	}
	if msg.Node() == nil {
		t.Fatal("nil node")
	}
}

func TestMessage_PRD_02_SuccessVisible(t *testing.T) {
	msg := kit.NewMessage()
	msg.Success("This is a success message", 0)
	if msg.Count() != 1 {
		t.Fatalf("count=%d want 1", msg.Count())
	}
	it := msg.Items()[0]
	if it.Type != kit.MessageSuccess || it.Content != "This is a success message" {
		t.Fatalf("item=%+v", it)
	}
	if msg.Portal == nil || !msg.Portal.Open {
		t.Fatal("success should open portal")
	}
}

func TestMessage_PRD_03_DurationExpires(t *testing.T) {
	msg := kit.NewMessage()
	closed := 0
	msg.Open(kit.MessageConfig{
		Type:        kit.MessageSuccess,
		Content:     "short",
		Duration:    0.1,
		DurationSet: true,
		OnClose:     func() { closed++ },
	})
	msg.Tick(0.11)
	if msg.Count() != 0 || closed != 1 {
		t.Fatalf("count=%d closed=%d want 0/1", msg.Count(), closed)
	}
}

func TestMessage_PRD_04_DurationZeroSticky(t *testing.T) {
	msg := kit.NewMessage()
	msg.Info("sticky", 0)
	msg.Tick(60)
	if msg.Count() != 1 {
		t.Fatalf("count=%d want sticky message", msg.Count())
	}
}

func TestMessage_PRD_05_KeyUpdate(t *testing.T) {
	msg := kit.NewMessage()
	msg.Open(kit.MessageConfig{Key: "same", Content: "Loading...", Type: kit.MessageLoading})
	msg.Open(kit.MessageConfig{Key: "same", Content: "Loaded!", Type: kit.MessageSuccess, Duration: 2, DurationSet: true})
	if msg.Count() != 1 {
		t.Fatalf("count=%d want 1", msg.Count())
	}
	it := msg.Items()[0]
	if it.Content != "Loaded!" || it.Type != kit.MessageSuccess || it.Duration != 2 {
		t.Fatalf("updated item=%+v", it)
	}
}

func TestMessage_PRD_06_MultipleStacked(t *testing.T) {
	msg := kit.NewMessage()
	msg.Info("one", 0)
	msg.Success("two", 0)
	msg.Error("three", 0)
	if msg.Count() != 3 {
		t.Fatalf("count=%d want 3", msg.Count())
	}
	if got := len(msg.VisibleItems()); got != 3 {
		t.Fatalf("visible=%d want 3 without stack collapse", got)
	}
}

func TestMessage_PRD_07_Destroy(t *testing.T) {
	msg := kit.NewMessage()
	msg.Info("one", 0)
	msg.Success("two", 0)
	msg.Destroy()
	if msg.Count() != 0 {
		t.Fatalf("count=%d want 0", msg.Count())
	}
	if msg.Portal != nil && msg.Portal.Open {
		t.Fatal("portal should close after destroy")
	}
}

func TestMessage_PRD_08_TypesAndIcons(t *testing.T) {
	msg := kit.NewMessage()
	msg.Info("info", 0)
	msg.Success("success", 0)
	msg.Error("error", 0)
	msg.Warning("warning", 0)
	msg.Loading("loading", 0)
	got := msg.Items()
	want := []struct {
		typ  kit.MessageType
		icon string
	}{
		{kit.MessageInfo, "info"},
		{kit.MessageSuccess, "check"},
		{kit.MessageError, "close"},
		{kit.MessageWarning, "info"},
		{kit.MessageLoading, "loading"},
	}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Type != want[i].typ || got[i].IconName != want[i].icon {
			t.Fatalf("item[%d]=%+v want %v/%s", i, got[i], want[i].typ, want[i].icon)
		}
	}
}

func TestMessage_PRD_09_OfficialHooksExample(t *testing.T) {
	msg := kit.NewMessage()
	tree := layoutMessage(msg)
	msg.Info("Hello, Ant Design!", 0)
	tree.Layout(core.Size{Width: 800, Height: 600})
	if !overlayHasText(tree, "Hello, Ant Design!") {
		t.Fatal("hooks example text not rendered")
	}
}

func TestMessage_PRD_10_OfficialOtherTypesExample(t *testing.T) {
	msg := kit.NewMessage()
	msg.Success("This is a success message", 0)
	msg.Error("This is an error message", 0)
	msg.Warning("This is a warning message", 0)
	if msg.Count() != 3 {
		t.Fatalf("count=%d want 3", msg.Count())
	}
	for _, s := range []string{"success", "error", "warning"} {
		if !itemsContain(msg.Items(), s) {
			t.Fatalf("missing %q in items", s)
		}
	}
}

func TestMessage_PRD_11_OfficialDurationExample(t *testing.T) {
	msg := kit.NewMessage()
	msg.Open(kit.MessageConfig{
		Type:        kit.MessageSuccess,
		Content:     "This is a prompt message for success, and it will disappear in 10 seconds",
		Duration:    10,
		DurationSet: true,
	})
	it := msg.Items()[0]
	if it.Duration != 10 {
		t.Fatalf("duration=%v want 10", it.Duration)
	}
}

func TestMessage_PRD_12_OfficialStackExample(t *testing.T) {
	msg := kit.NewMessage()
	msg.SetStack(true)
	msg.SetStackThreshold(3)
	for i := 0; i < 4; i++ {
		msg.Open(kit.MessageConfig{Type: kit.MessageInfo, Content: "Message", Duration: 0, DurationSet: true})
	}
	if msg.Count() != 4 {
		t.Fatalf("count=%d want 4", msg.Count())
	}
	vis := msg.VisibleItems()
	if len(vis) != 1 || vis[0].Content != "Message" {
		t.Fatalf("visible=%+v want latest collapsed", vis)
	}
}

func TestMessage_PRD_13_OfficialLoadingExample(t *testing.T) {
	msg := kit.NewMessage()
	msg.Loading("Action in progress..", 0)
	before := msg.SpinPhase()
	if !msg.Tick(0.5) {
		t.Fatal("loading should keep ticker active")
	}
	if msg.SpinPhase() == before {
		t.Fatalf("spin phase did not advance: %v", before)
	}
	if msg.Items()[0].Type != kit.MessageLoading {
		t.Fatalf("type=%v want loading", msg.Items()[0].Type)
	}
}

func TestMessage_PRD_14_OfficialThenableExample(t *testing.T) {
	msg := kit.NewMessage()
	msg.Loading("Action in progress..", 0.1).Then(func() {
		msg.Success("Loading finished", 0)
	})
	msg.Tick(0.11)
	if msg.Count() != 1 {
		t.Fatalf("count=%d want chained success", msg.Count())
	}
	it := msg.Items()[0]
	if it.Type != kit.MessageSuccess || it.Content != "Loading finished" {
		t.Fatalf("item=%+v", it)
	}
}

func TestMessage_PRD_15_OfficialStyleClassExample(t *testing.T) {
	msg := kit.NewMessage()
	msg.Open(kit.MessageConfig{
		Type:        kit.MessageSuccess,
		Content:     "This is a message with object styles",
		Duration:    0,
		DurationSet: true,
		Style: kit.Style{
			Background: render.Hex("#f6ffed"),
			Border:     render.Hex("#95de64"),
			Text:       render.Hex("#237804"),
			Radius:     16,
		},
	})
	tree := layoutMessage(msg)
	dec := firstDecorated(tree)
	if dec == nil {
		t.Fatal("decorated message card not found")
	}
	if dec.Radius != 16 || !approxColor(dec.Background, render.Hex("#f6ffed"), 0.01) {
		t.Fatalf("style not applied: radius=%v bg=%v", dec.Radius, dec.Background)
	}
}

func TestMessage_PRD_16_OfficialUpdateExample(t *testing.T) {
	msg := kit.NewMessage()
	msg.Open(kit.MessageConfig{Key: "updatable", Type: kit.MessageLoading, Content: "Loading..."})
	msg.Open(kit.MessageConfig{Key: "updatable", Type: kit.MessageSuccess, Content: "Loaded!", Duration: 2, DurationSet: true})
	if msg.Count() != 1 {
		t.Fatalf("count=%d want 1", msg.Count())
	}
	it := msg.Items()[0]
	if it.Type != kit.MessageSuccess || it.Content != "Loaded!" {
		t.Fatalf("item=%+v", it)
	}
}

func TestMessage_PRD_17_TokenGeometry(t *testing.T) {
	msg := kit.NewMessage()
	msg.Info("geometry", 0)
	tree := layoutMessage(msg)
	dec := firstDecorated(tree)
	if dec == nil {
		t.Fatal("decorated card not found")
	}
	th := kit.DefaultTheme()
	if dec.Radius != th.SizeOr(core.TokenBorderRadius, 6) {
		t.Fatalf("radius=%v want token", dec.Radius)
	}
	if dec.BorderWidth != th.SizeOr(core.TokenLineWidth, 1) {
		t.Fatalf("border=%v want token", dec.BorderWidth)
	}
	if dec.Size().Height < th.SizeOr(core.TokenControlHeight, 32) {
		t.Fatalf("height=%v below controlHeight", dec.Size().Height)
	}
}

func TestMessage_PRD_18_DefaultSkinUsesThemeToken(t *testing.T) {
	msg := kit.NewMessage()
	th := kit.DefaultTheme()
	th.Tokens.Colors[core.TokenColorPrimary] = render.Hex("#00BFFF")
	msg.SetTheme(th)
	msg.Info("token", 0)
	tree := layoutMessage(msg)
	ic := firstIcon(tree)
	if ic == nil {
		t.Fatal("icon not found")
	}
	if !approxColor(ic.Color, render.Hex("#00BFFF"), 0.01) {
		t.Fatalf("icon color=%v want theme primary", ic.Color)
	}
}

func TestMessage_PRD_19_ShallowStyleOverride(t *testing.T) {
	msg := kit.NewMessage()
	msg.SetStyle(kit.Style{
		Background: render.Hex("#fff2f0"),
		Border:     render.Hex("#ffccc7"),
		Text:       render.Hex("#cf1322"),
		Radius:     12,
	})
	msg.Error("styled", 0)
	tree := layoutMessage(msg)
	dec := firstDecorated(tree)
	if dec == nil {
		t.Fatal("decorated card not found")
	}
	if dec.Radius != 12 || !approxColor(dec.BorderColor, render.Hex("#ffccc7"), 0.01) {
		t.Fatalf("style not applied: radius=%v border=%v", dec.Radius, dec.BorderColor)
	}
}

func TestMessage_PRD_20_OnClickNoFocusSteal(t *testing.T) {
	msg := kit.NewMessage()
	clicks := 0
	msg.Open(kit.MessageConfig{
		Content:     "clickable",
		Duration:    0,
		DurationSet: true,
		OnClick:     func() { clicks++ },
	})
	tree := layoutMessage(msg)
	p := firstPressable(tree)
	if p == nil {
		t.Fatal("pressable message not found")
	}
	if p.Focusable {
		t.Fatal("message should not steal focus by default")
	}
	p.OnClick(nil)
	if clicks != 1 {
		t.Fatalf("clicks=%d want 1", clicks)
	}
}

func layoutMessage(msg *kit.Message) *core.Tree {
	msg.Viewport = core.Size{Width: 800, Height: 600}
	tree := core.NewTree(msg.Node())
	tree.Layout(core.Size{Width: 800, Height: 600})
	return tree
}

func overlayHasText(tree *core.Tree, sub string) bool {
	return walkOverlay(tree, func(n core.Node) bool {
		tx, ok := n.(*primitive.Text)
		return ok && strings.Contains(tx.Value, sub)
	})
}

func firstDecorated(tree *core.Tree) *primitive.Decorated {
	var out *primitive.Decorated
	walkOverlay(tree, func(n core.Node) bool {
		if d, ok := n.(*primitive.Decorated); ok {
			out = d
			return true
		}
		return false
	})
	return out
}

func firstIcon(tree *core.Tree) *primitive.Icon {
	var out *primitive.Icon
	walkOverlay(tree, func(n core.Node) bool {
		if ic, ok := n.(*primitive.Icon); ok {
			out = ic
			return true
		}
		return false
	})
	return out
}

func firstPressable(tree *core.Tree) *primitive.Pressable {
	var out *primitive.Pressable
	walkOverlay(tree, func(n core.Node) bool {
		if p, ok := n.(*primitive.Pressable); ok {
			out = p
			return true
		}
		return false
	})
	return out
}

func walkOverlay(tree *core.Tree, visit func(core.Node) bool) bool {
	if tree == nil || visit == nil {
		return false
	}
	var walk func(core.Node) bool
	walk = func(n core.Node) bool {
		if n == nil {
			return false
		}
		if visit(n) {
			return true
		}
		for _, c := range n.Children() {
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

func itemsContain(items []kit.MessageSnapshot, sub string) bool {
	for _, it := range items {
		if strings.Contains(it.Content, sub) {
			return true
		}
	}
	return false
}
