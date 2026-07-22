package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
)

func TestTabs_GroupHeaderAndDivider(t *testing.T) {
	tabs := kit.NewTabs(
		kit.MenuItem{Key: "cat:g", Label: "General", Disabled: true},
		kit.MenuItem{Divider: true},
		kit.MenuItem{Key: "btn", Label: "Button"},
		kit.MenuItem{Key: "icon", Label: "Icon"},
	)
	if tabs.Active != "btn" {
		t.Fatalf("Active=%q want first selectable btn", tabs.Active)
	}
	// Cannot activate header
	tabs.SetActive("cat:g")
	if tabs.Active != "btn" {
		t.Fatalf("header activated Active=%q", tabs.Active)
	}
	tabs.SetContent("btn", kit.NewText("B").Node())
	tabs.SetContent("icon", kit.NewText("I").Node())
	tabs.SetActive("icon")
	if tabs.Active != "icon" {
		t.Fatal(tabs.Active)
	}
	_ = tabs.Node().Layout(core.Loose(400, 300))
	tabs.SetPosition(kit.TabLeft)
	_ = tabs.Node().Layout(core.Loose(400, 300))
}
