//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/hal"
)

// surfaceContentView is a local hal.TextureView stub (per-file tests must not
// depend on a stub declared in another _test.go file).
type surfaceContentView struct{ hal.TextureView }

func (v *surfaceContentView) Destroy() {}

// MarkSurfaceContent registers the per-frame surface view as content-bearing:
// the first pass loads the persistent back buffer and the composite can
// damage-scissor. It must stay per-frame (never enter persistentContentViews,
// which would leak one view handle per frame).
func TestMarkSurfaceContentIsPerFrame(t *testing.T) {
	s := &GPURenderSession{}
	v := &surfaceContentView{}

	s.MarkSurfaceContent(v)
	if !s.viewRendered(v) {
		t.Fatal("surface view must be marked rendered")
	}
	if !s.hasColorContent(v) {
		t.Fatal("surface view must be marked color-content")
	}
	if len(s.persistentContentViews) != 0 {
		t.Fatalf("surface mark must not persist across frames, got %d entries", len(s.persistentContentViews))
	}

	if !s.isSurfaceCarry(v) {
		t.Fatal("surface view must be marked as a cross-frame color carry")
	}

	// Idempotent within the frame.
	s.MarkSurfaceContent(v)
	if len(s.renderedViews) != 1 || len(s.colorContentViews) != 1 || len(s.surfaceCarryViews) != 1 {
		t.Fatalf("repeat mark duplicated: rendered=%d color=%d carry=%d",
			len(s.renderedViews), len(s.colorContentViews), len(s.surfaceCarryViews))
	}

	// The carry mark is what keeps stencil transient: the composite may keep
	// the loaded COLOR, but the pass must clear stencil (a cross-frame stencil
	// load feeds stale winding into stencil-then-cover and fills paths to
	// their bounding box).
	if !s.isSurfaceCarry(v) {
		t.Fatal("cross-frame carry lost after repeat mark")
	}
	if s.isSurfaceCarry(&surfaceContentView{}) {
		t.Fatal("unregistered view reported as surface carry")
	}

	// Nil guards.
	s.MarkSurfaceContent(nil)
	var nilSession *GPURenderSession
	nilSession.MarkSurfaceContent(v)
}

// The P1 observable: a persistent surface view marked as content-bearing is
// damage-scissored (""), while a fresh view stays fresh_target and MSAA keeps
// dropping damage.
func TestCompositeFullReasonSurfaceContent(t *testing.T) {
	cases := []struct {
		sampleCount  int
		hasDamage    bool
		viewRendered bool
		want         string
	}{
		{1, true, true, ""},
		{1, true, false, "fresh_target"},
		{4, true, true, "msaa"},
		{1, false, true, "no_damage"},
	}
	for _, c := range cases {
		if got := compositeFullReason(c.sampleCount, c.hasDamage, c.viewRendered); got != c.want {
			t.Fatalf("compositeFullReason(%d,%v,%v)=%q want %q", c.sampleCount, c.hasDamage, c.viewRendered, got, c.want)
		}
	}
}

// TestPersistentContentOnlyPresentTarget pins the narrowed O13 第一件 contract:
// an offscreen session persists ONLY the flush's present target. Layer/effect
// RTs run in the same session (FlushOffscreen) but are re-recorded every frame;
// persisting them makes their pass LoadOpLoad the previous frame's pixels and
// blend the new draw on top (backdrop band darkening, a0ca7c83 regression).
func TestPersistentContentOnlyPresentTarget(t *testing.T) {
	s := &GPURenderSession{}
	present := &surfaceContentView{}
	layer := &surfaceContentView{}

	s.SetPresentTargetView(present)
	s.markViewRendered(present)
	s.markViewRendered(layer)

	if len(s.persistentContentViews) != 1 || s.persistentContentViews[0] != present {
		t.Fatalf("persistentContentViews=%d, want only the present target", len(s.persistentContentViews))
	}
	if !s.viewRendered(layer) {
		t.Fatal("layer view must still be rendered within this frame")
	}
	if s.hasColorContent(layer) {
		t.Fatal("layer view must not inherit color-content from persistence")
	}

	// Nested passes after the pin is cleared (e.g. layer RT self-flushes on the
	// same session) must never persist either.
	s.SetPresentTargetView(nil)
	other := &surfaceContentView{}
	s.markViewRendered(other)
	if len(s.persistentContentViews) != 1 {
		t.Fatalf("unpinned view leaked into persistence: %d", len(s.persistentContentViews))
	}

	// Surface sessions never persist: swapchain FIFO rotation makes previous
	// content undefined (O13 发现第三件 stays structural).
	surf := &GPURenderSession{surfaceView: present}
	surf.SetPresentTargetView(present)
	surf.markViewRendered(present)
	if len(surf.persistentContentViews) != 0 {
		t.Fatal("surface session must never persist a view")
	}
}
