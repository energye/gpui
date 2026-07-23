package platform_test

import (
	"testing"

	"github.com/energye/gpui/ui/app"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/primitive"
)

func TestHeadlessClipboardRoundTrip(t *testing.T) {
	h := platform.NewHeadless(100, 80)
	if !h.Caps().Has(platform.CapClipboard) {
		t.Fatal("Headless must advertise CapClipboard")
	}
	clip := h.Clipboard()
	if clip == nil {
		t.Fatal("nil clipboard")
	}
	if err := clip.WriteText("hello-clip"); err != nil {
		t.Fatal(err)
	}
	got, ok := clip.ReadText()
	if !ok || got != "hello-clip" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestAppAttachBridgesClipboard(t *testing.T) {
	host := platform.NewHeadless(120, 80)
	ed := primitive.NewEditableText()
	ed.SetValue("abc")
	ed.SelAnchor = 0
	ed.Cursor = 3
	tree := core.NewTree(ed)
	a := app.New(app.Options{DisableRenderThread: true})
	defer a.Close()
	a.Attach(host, tree, nil)
	if tree.Clipboard() == nil {
		t.Fatal("Attach should bridge host clipboard onto tree")
	}
	tree.SetFocus(ed)
	ed.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "c", Ctrl: true})
	got, ok := host.Clipboard().ReadText()
	if !ok || got != "abc" {
		t.Fatalf("host clip after Ctrl+C: %q ok=%v", got, ok)
	}
}

func TestSystemClipboardMemoryFallback(t *testing.T) {
	// NewSystemClipboard always works: OS tools optional, memory always present.
	c := platform.NewSystemClipboard()
	if c == nil {
		t.Fatal("nil system clipboard")
	}
	if err := c.WriteText("cross-platform"); err != nil {
		t.Fatal(err)
	}
	got, ok := c.ReadText()
	if !ok {
		t.Fatal("ReadText not ok")
	}
	// May be OS clipboard content or memory fallback — must be readable after write.
	if got != "cross-platform" {
		// OS clipboard might have raced with another app; accept memory path via Fallback.
		if fb, ok2 := c.(*platform.FallbackClipboard); ok2 && fb.Fallback != nil {
			g2, _ := fb.Fallback.ReadText()
			if g2 != "cross-platform" {
				t.Fatalf("got %q fallback %q", got, g2)
			}
		} else {
			t.Fatalf("got %q want cross-platform", got)
		}
	}
}

func TestFallbackClipboardAlwaysStores(t *testing.T) {
	fb := platform.NewFallbackClipboard(nil) // primary nil → memory only
	_ = fb.WriteText("only-mem")
	got, ok := fb.ReadText()
	if !ok || got != "only-mem" {
		t.Fatalf("%q ok=%v", got, ok)
	}
}
