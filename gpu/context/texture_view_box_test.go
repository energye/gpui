// Copyright 2026 The gogpu Authors
// SPDX-License-Identifier: MIT

//go:build !(js && wasm)

package context

import (
	"testing"
	"unsafe"

	"github.com/energye/gpui/gpu/hal"
)

// stubTextureView is a minimal hal.TextureView for handle roundtrip tests.
type stubTextureView struct{ id int }

func (s *stubTextureView) Destroy()              {}
func (s *stubTextureView) NativeHandle() uintptr { return uintptr(s.id) }
func (s *stubTextureView) Texture() hal.Texture  { return nil }

func TestPackUnpackTextureViewRoundtrip(t *testing.T) {
	v := &stubTextureView{id: 7}
	h := PackTextureView(v)
	if h.IsNil() {
		t.Fatal("PackTextureView returned nil handle for non-nil view")
	}
	if got := UnpackTextureView(h); got != hal.TextureView(v) {
		t.Fatalf("UnpackTextureView = %v, want %v", got, v)
	}
}

func TestPackNilTextureView(t *testing.T) {
	if h := PackTextureView(nil); !h.IsNil() {
		t.Fatalf("PackTextureView(nil) = non-nil handle")
	}
	if got := UnpackTextureView(TextureView{}); got != nil {
		t.Fatalf("UnpackTextureView(zero) = %v, want nil", got)
	}
	if got := UnpackTextureViewRaw(nil); got != nil {
		t.Fatalf("UnpackTextureViewRaw(nil) = %v, want nil", got)
	}
}

func TestUnpackRawPointerRoundtrip(t *testing.T) {
	v := &stubTextureView{id: 3}
	h := PackTextureView(v)
	if got := UnpackTextureViewRaw(h.Pointer()); got != hal.TextureView(v) {
		t.Fatalf("UnpackTextureViewRaw = %v, want %v", got, v)
	}
}

// TestUnpackForeignPointerFailsClosed guards the H3片7d black-screen class:
// a handle carrying a raw concrete pointer (not a packed box) must decode to
// nil — never to a garbage non-nil view that draws nowhere while presents
// still count.
func TestUnpackForeignPointerFailsClosed(t *testing.T) {
	foreign := 42
	h := NewTextureView(unsafe.Pointer(&foreign))
	if got := UnpackTextureView(h); got != nil {
		t.Fatalf("UnpackTextureView(foreign) = %v, want nil", got)
	}
	if got := UnpackTextureViewRaw(unsafe.Pointer(&foreign)); got != nil {
		t.Fatalf("UnpackTextureViewRaw(foreign) = %v, want nil", got)
	}
}
