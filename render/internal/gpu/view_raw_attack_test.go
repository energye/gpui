package gpu

import (
	"testing"
	"unsafe"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render/internal/gpu/res"
)

// Regression-attack tests for the P3 queue-time fallback: when the session
// does not exist yet (first frame / after device rebuild), command views must
// be carried as raw pointers and NOT dropped, and stale raw pointers must
// never reach wgpu unguarded.

func unsafePointer(v uintptr) unsafe.Pointer { return unsafe.Pointer(v) }

func TestViewToResView_SessionNil_CarriesRaw(t *testing.T) {
	rc := &GPURenderContext{} // session == nil (first-frame state)
	view := gpucontext.NewTextureView(unsafePointer(0x1234))

	rv := rc.viewToResView(view)
	if rv.IsNil() {
		t.Fatal("view must NOT be dropped when session is nil (P3 bug)")
	}
	if !rv.Key.IsNil() || !rv.Ref.IsNil() {
		t.Fatal("expected queue-time raw fallback (no key, no ref)")
	}
	if rv.Raw == nil {
		t.Fatal("raw pointer must be carried")
	}
	if got := rv.RefID(); got != 0 {
		t.Fatalf("raw view RefID must be 0, got %d", got)
	}
	// Nil view stays nil.
	if !rc.viewToResView(gpucontext.TextureView{}).IsNil() {
		t.Fatal("nil view must stay nil")
	}
}

func TestViewToResView_RawEquality(t *testing.T) {
	rc := &GPURenderContext{}
	p := unsafePointer(0xABCD)
	a := rc.viewToResView(gpucontext.NewTextureView(p))
	b := rc.viewToResView(gpucontext.NewTextureView(p))
	if !a.Equals(b) {
		t.Fatal("two raw views of the same pointer must merge (Equals)")
	}
	c := rc.viewToResView(gpucontext.NewTextureView(unsafePointer(0xBEEF)))
	if a.Equals(c) {
		t.Fatal("different raw pointers must not merge")
	}
}

// TestResolveCommandView_RawStaleNeverPanics: a raw pointer whose view was
// already released must not panic. Raw pointers in the real flow are live Go
// *webgpu.TextureView objects owned by the render context; the registry keeps
// them alive, so session teardown ReleaseAll is idempotent (released-guard).
// Use a real (zero-value) object to model a released view — an arbitrary fake
// address would SIGSEGV on ReleaseAll, which is out of contract.
func TestResolveCommandView_RawStaleNeverPanics(t *testing.T) {
	s := NewGPURenderSession(nil, nil, 4) // no device; resolution path only
	t.Cleanup(func() { s.Destroy() })

	// A released view: object alive, Release already called.
	viewObj := &webgpu.TextureView{}
	viewObj.Release()
	view := res.ViewFromRaw(unsafe.Pointer(viewObj))
	tv, ok := s.ResolveCommandView(&view)
	// Resolution completes without panicking; teardown ReleaseAll is safe
	// because the registry holds a real Go object.
	_ = tv
	_ = ok
}

// TestResolveCommandView_RawNilSafe: nil raw pointer fails resolution.
func TestResolveCommandView_RawNilSafe(t *testing.T) {
	s := NewGPURenderSession(nil, nil, 4)
	t.Cleanup(func() { s.Destroy() })

	view := res.ViewFromRaw(nil)
	if _, ok := s.ResolveCommandView(&view); ok {
		t.Fatal("nil raw pointer must fail resolution")
	}
}
