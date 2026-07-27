package rendering_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

func mustRGBA(t *testing.T, w, h int) *render.ImageBuf {
	t.Helper()
	img, err := render.NewImageBuf(w, h, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	// Touch pixels so storage is non-trivial.
	_ = img.SetRGBA(0, 0, 255, 0, 0, 255)
	return img
}

func TestRenderImage_SetImage_SamePointer_StaysLive(t *testing.T) {
	im := rendering.NewRenderImage(16, 16)
	a := mustRGBA(t, 4, 4)
	im.SetImage(a)
	im.SetImage(a) // re-set same owned buffer must NOT Dispose
	if a.Disposed() {
		t.Fatal("SetImage(a); SetImage(a) must keep buffer live")
	}
	if im.Img != a || !im.OwnsImage() || im.State != rendering.ImageReady {
		t.Fatalf("want Ready+owned same ptr; state=%v owns=%v img=%v", im.State, im.OwnsImage(), im.Img == a)
	}
	// Demote to shared then re-claim ownership without dispose.
	im.SetImageShared(a)
	if a.Disposed() || im.OwnsImage() {
		t.Fatal("SetImageShared(same) must drop owns only")
	}
	im.SetImage(a)
	if a.Disposed() || !im.OwnsImage() || im.State != rendering.ImageReady {
		t.Fatal("SetImage(same) after shared must re-own live buffer")
	}
	dc := render.NewContext(16, 16)
	defer dc.Close()
	dc.BeginFrame()
	pc := rendering.NewPaintContext(dc, 1)
	im.Paint(pc) // Ready + live must paint path, not error chrome only by dispose
	im.Clear()
	if !a.Disposed() {
		t.Fatal("Clear still Disposes owned")
	}
}

func TestRenderImage_SetImage_ReplacesAndDisposesPrior(t *testing.T) {
	im := rendering.NewRenderImage(32, 32)
	a := mustRGBA(t, 8, 8)
	b := mustRGBA(t, 8, 8)

	im.SetImage(a)
	if !im.OwnsImage() {
		t.Fatal("SetImage must take ownership")
	}
	if im.State != rendering.ImageReady || im.Img != a {
		t.Fatalf("state/img after first set")
	}
	if a.Disposed() {
		t.Fatal("owned live buffer must not be disposed yet")
	}

	im.SetImage(b)
	if !a.Disposed() {
		t.Fatal("prior owned buffer must be Dispose'd on replace")
	}
	if b.Disposed() {
		t.Fatal("new buffer must remain live")
	}
	if im.Img != b || !im.OwnsImage() {
		t.Fatal("node must hold new owned buffer")
	}

	// Second dispose via Clear.
	im.Clear()
	if !b.Disposed() {
		t.Fatal("Clear must Dispose owned buffer")
	}
	if im.Img != nil || im.OwnsImage() {
		t.Fatal("after Clear, no image / no ownership")
	}
	if im.State != rendering.ImageIdle {
		t.Fatalf("state=%v want Idle", im.State)
	}

	// Idempotent clear / double path.
	im.Clear()
	im.SetError()
}

func TestRenderImage_SetImageShared_DoesNotDispose(t *testing.T) {
	im := rendering.NewRenderImage(16, 16)
	shared := mustRGBA(t, 4, 4)
	im.SetImageShared(shared)
	if im.OwnsImage() {
		t.Fatal("shared must not set owns")
	}
	im.Clear()
	if shared.Disposed() {
		t.Fatal("Clear must not Dispose shared (non-owned) buffer")
	}
	// Caller still owns it.
	shared.Dispose()
	if !shared.Disposed() {
		t.Fatal("caller Dispose")
	}
}

func TestRenderImage_SetError_DisposesOwned(t *testing.T) {
	im := rendering.NewRenderImage(10, 10)
	img := mustRGBA(t, 2, 2)
	im.SetImage(img)
	im.SetError()
	if !img.Disposed() {
		t.Fatal("SetError must release owned image")
	}
	if im.State != rendering.ImageError || im.Img != nil {
		t.Fatal("error state clears Img")
	}
}

func TestRenderImage_Paint_AfterDisposeSafe(t *testing.T) {
	im := rendering.NewRenderImage(20, 20)
	img := mustRGBA(t, 4, 4)
	im.SetImage(img)
	// External dispose while node still points at buffer (shared-style leak path):
	// paint must not crash; Ready + disposed → error chrome path.
	im.SetImageShared(img) // reinstall without own so we can Dispose externally
	// Wait - SetImageShared releaseOwned first which doesn't dispose previous if we
	// just set via SetImage then SetImageShared: first releaseOwned disposes owned.
	// Rebuild scenario:
	im2 := rendering.NewRenderImage(20, 20)
	ext := mustRGBA(t, 4, 4)
	im2.SetImageShared(ext)
	ext.Dispose()
	im2.State = rendering.ImageReady // force ready with disposed ptr
	im2.Img = ext

	dc := render.NewContext(20, 20)
	defer dc.Close()
	dc.BeginFrame()
	pc := rendering.NewPaintContext(dc, 1)
	// Must not panic.
	im2.Paint(pc)
	im2.Clear() // shared: no double-free panic
	ext.Dispose()
}

func TestRenderImage_RejectsAlreadyDisposedSetImage(t *testing.T) {
	im := rendering.NewRenderImage(8, 8)
	dead := mustRGBA(t, 2, 2)
	dead.Dispose()
	im.SetImage(dead)
	if im.State != rendering.ImageError {
		t.Fatalf("state=%v want Error", im.State)
	}
	if im.Img != nil || im.OwnsImage() {
		t.Fatal("must not adopt disposed buffer")
	}
}

func TestImageBuf_Dispose_ViaRenderAlias(t *testing.T) {
	// Prove public render.ImageBuf surface (type alias) exposes Dispose.
	img, err := render.NewImageBuf(3, 3, render.FormatRGBA8)
	if err != nil {
		t.Fatal(err)
	}
	img.Dispose()
	img.Dispose()
	if !img.Disposed() {
		t.Fatal("alias Dispose")
	}
	dc := render.NewContext(8, 8)
	defer dc.Close()
	dc.BeginFrame()
	// Draw after dispose is a no-op (must not panic).
	dc.DrawImage(img, 0, 0)
}
