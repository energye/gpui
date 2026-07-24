package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
)

func TestTabs_GroupHeaderAndDivider(t *testing.T) {
	tabs := kit.NewTabs(
		kit.TabItem{Key: "cat:g", Label: "General", Disabled: true},
		kit.TabItem{Divider: true},
		kit.TabItem{Key: "btn", Label: "Button"},
		kit.TabItem{Key: "icon", Label: "Icon"},
	)
	if tabs.ActiveKey != "btn" {
		t.Fatalf("Active=%q want first selectable btn", tabs.ActiveKey)
	}
	// Cannot activate header
	tabs.SetActive("cat:g")
	if tabs.ActiveKey != "btn" {
		t.Fatalf("header activated Active=%q", tabs.ActiveKey)
	}
	tabs.SetContent("btn", kit.NewText("B").Node())
	tabs.SetContent("icon", kit.NewText("I").Node())
	tabs.SetActive("icon")
	if tabs.ActiveKey != "icon" {
		t.Fatal(tabs.ActiveKey)
	}
	_ = tabs.Node().Layout(core.Loose(400, 300))
	tabs.SetPosition(kit.TabLeft)
	_ = tabs.Node().Layout(core.Loose(400, 300))
}
