package platform_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/primitive"
)

func TestHeadlessInjectPreservesModifiers(t *testing.T) {
	h := platform.NewHeadless(100, 80)
	h.InjectPointerMods(platform.PointerDown, 10, 10, platform.BtnLeft, true, true, false, false)
	got := h.PumpEvents()
	if len(got) != 1 {
		t.Fatalf("events=%d", len(got))
	}
	if !got[0].Shift || !got[0].Ctrl || got[0].Alt || got[0].Meta {
		t.Fatalf("mods: %+v", got[0])
	}
	h.InjectKeyMods("c", true, false, true, false, false)
	got = h.PumpEvents()
	if len(got) != 1 || !got[0].Ctrl || got[0].Key != "c" {
		t.Fatalf("key mods: %+v", got)
	}
}

func TestDispatchMapsPointerModsToCore(t *testing.T) {
	var sawShift bool
	// Pressable records via HandlePointer... use Editable + Shift click path instead:
	// Dispatch platform event through Dispatch and verify Editable extends selection.
	ed := primitive.NewEditableText()
	ed.Width, ed.Height = 100, 32
	ed.SetValue("hello")
	root := primitive.NewBox(ed)
	root.Width, root.Height = 100, 32
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 100, Height: 32})
	tree.SetFocus(ed)
	ed.Cursor, ed.SelAnchor = 0, 0

	abs := core.AbsoluteOffset(ed)
	platform.Dispatch(tree, platform.Event{
		Type: platform.EventPointer, Pointer: platform.PointerDown,
		X: abs.X + 80, Y: abs.Y + 8,
		Button: platform.BtnLeft, Shift: true,
	})
	if ed.SelAnchor != 0 {
		t.Fatalf("anchor should stay 0, got %d", ed.SelAnchor)
	}
	if ed.Cursor <= 0 {
		t.Fatalf("shift+click should move cursor, c=%d", ed.Cursor)
	}
	if ed.SelectedText() == "" {
		t.Fatal("expected selection")
	}
	_ = sawShift
}

func TestParseModifierStateExported(t *testing.T) {
	sh, ctrl, alt, meta := platform.ParseModifierState(1 | 4 | 8 | 64)
	if !sh || !ctrl || !alt || !meta {
		t.Fatal("parse")
	}
}
