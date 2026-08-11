package res

import (
	"testing"
	"unsafe"
)

// View three-state semantics: deferred key, direct ref, queue-time raw
// pointer. These pin the P3 queue-time fallback used when a session does not
// exist yet at command-queue time (first frame / after device rebuild).

func TestView_RawFallbackSemantics(t *testing.T) {
	p1 := unsafe.Pointer(uintptr(0x1000))
	p2 := unsafe.Pointer(uintptr(0x2000))

	raw1 := ViewFromRaw(p1)
	if raw1.IsNil() {
		t.Fatal("raw view must not be nil")
	}
	if !raw1.Equals(ViewFromRaw(p1)) {
		t.Fatal("same raw pointer must compare equal")
	}
	if raw1.Equals(ViewFromRaw(p2)) {
		t.Fatal("different raw pointers must differ")
	}
	if raw1.RefID() != 0 {
		t.Fatalf("raw view RefID must be 0, got %d", raw1.RefID())
	}

	// Raw vs deferred / direct views never compare equal.
	keyView := ViewFromKey(SourceKey{Kind: KindTextureView, Role: RoleLayerRT, Index: 1})
	if raw1.Equals(keyView) {
		t.Fatal("raw must not equal a deferred key view")
	}
	if !(View{}).IsNil() {
		t.Fatal("zero view must be nil")
	}
	_ = p2
}

func TestView_DeferredAndDirectEquality(t *testing.T) {
	reg := NewRegistry()
	refA := reg.Register(&fakeNative{tag: 1})
	refB := reg.Register(&fakeNative{tag: 2})

	a1 := ViewFromRef(refA)
	a2 := ViewFromRef(refA)
	b := ViewFromRef(refB)
	if !a1.Equals(a2) {
		t.Fatal("same ref must compare equal")
	}
	if a1.Equals(b) {
		t.Fatal("different refs must differ")
	}
	k1 := ViewFromKey(SourceKey{Kind: KindTextureView, Role: RoleLayerRT, Index: 1})
	k2 := ViewFromKey(SourceKey{Kind: KindTextureView, Role: RoleLayerRT, Index: 2})
	if !k1.Equals(ViewFromKey(SourceKey{Kind: KindTextureView, Role: RoleLayerRT, Index: 1})) {
		t.Fatal("same deferred key must compare equal")
	}
	if k1.Equals(k2) {
		t.Fatal("different deferred keys must differ")
	}
	if k1.Equals(a1) {
		t.Fatal("deferred and direct views must differ")
	}
	reg.Release(refA)
	reg.Release(refB)
}