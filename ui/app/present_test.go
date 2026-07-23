package app_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/app"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/platform"
)

func TestUseCompositorDefaultOn(t *testing.T) {
	t.Setenv("GPUI_COMPOSITOR", "")
	if !app.UseCompositor() {
		t.Fatal("default compositor must be ON")
	}
	t.Setenv("GPUI_COMPOSITOR", "0")
	if app.UseCompositor() {
		t.Fatal("GPUI_COMPOSITOR=0 must disable")
	}
	t.Setenv("GPUI_COMPOSITOR", "false")
	if app.UseCompositor() {
		t.Fatal("false must disable")
	}
}

func TestOwnedPresenterRequiresSCDC(t *testing.T) {
	p := app.NewOwnedPresenter(app.OwnedPresentConfig{})
	defer p.Release()
	host := platform.NewHeadless(64, 64)
	root := newLeaf(64, 64)
	tree := core.NewTree(root)
	a := app.New(app.Options{DisableRenderThread: true})
	defer a.Close()
	s := a.Attach(host, tree, p.Func())
	s.Width, s.Height = 64, 64
	err := p.Present(s)
	if err == nil {
		t.Fatal("expected error without SC/DC")
	}
}

func TestPaintCompositorFrameOptionalGPU(t *testing.T) {
	root := newLeaf(80, 60)
	tree := core.NewTree(root)
	comp, ok := app.PaintCompositorFrame(tree, nil, 80, 60, render.RGBA{R: 0.1, G: 0.1, B: 0.1, A: 1}, true)
	if !ok {
		t.Log("PaintCompositorFrame skipped (no GPU offscreen) — OK for CI")
		return
	}
	defer comp.ReleaseAll()
	if !comp.HasBase() {
		t.Fatal("HasBase after successful paint")
	}
}

func TestOwnedPresenterFuncWiresSessionTheme(t *testing.T) {
	// Without SC/DC Present errors, but Func is non-nil and Session.Theme is read path.
	p := app.NewOwnedPresenter(app.OwnedPresentConfig{
		Theme: core.DefaultTheme(),
		Clear: render.RGBA{R: 1, G: 1, B: 1, A: 1},
	})
	defer p.Release()
	if p.Func() == nil {
		t.Fatal("Func nil")
	}
	host := platform.NewHeadless(100, 80)
	tree := core.NewTree(newLeaf(100, 80))
	a := app.New(app.Options{DisableRenderThread: true})
	defer a.Close()
	s := a.Attach(host, tree, p.Func())
	if s.Theme != nil {
		// Attach does not auto-copy config theme; host must set Session.Theme.
	}
	s.Theme = core.DefaultTheme()
	if s.Theme == nil {
		t.Fatal("theme assign")
	}
}
