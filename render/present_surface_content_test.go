//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"testing"
	"unsafe"

	gpucontext "github.com/energye/gpui/gpu/context"
)

// The P1 gate: only a persistent surface with a known-complete back buffer
// may register the frame view as content-bearing. Resize/recovery/force
// clears validity so the session composites full.
func TestSurfaceContentFlags(t *testing.T) {
	c := &Context{}
	if c.surfacePersistentEnabled() || c.surfaceContentValidFlag() {
		t.Fatal("zero Context must default to non-persistent/invalid")
	}
	c.setSurfacePersistent(true)
	c.setSurfaceContentValid(true)
	if !c.surfacePersistentEnabled() || !c.surfaceContentValidFlag() {
		t.Fatal("setters must be observable")
	}
	c.setSurfaceContentValid(false)
	if c.surfaceContentValidFlag() {
		t.Fatal("validity must clear")
	}
	var nilCtx *Context
	nilCtx.setSurfacePersistent(true)
	nilCtx.setSurfaceContentValid(true)
}

// markSurfaceContentForPresent is the ONLY registration site. The gate must
// refuse everything that is not a persistent surface with a known-complete
// back buffer — layer/effect flushes (recordLocal → FlushGPUWithView) share
// the flush core and must never inherit the surface-carry mark (marking them
// made steady-state record passes LoadOpLoad pooled RTs' previous tenant's
// pixels — the hover solid-icon regression).
func TestMarkSurfaceContentForPresentGate(t *testing.T) {
	nonNil := gpucontext.NewTextureView(unsafe.Pointer(new(int)))

	// Nil receiver and nil view must be safe no-ops.
	var nilCtx *Context
	nilCtx.markSurfaceContentForPresent(nonNil)

	c := &Context{}
	c.markSurfaceContentForPresent(gpucontext.TextureView{}) // nil view refused first
	c.markSurfaceContentForPresent(nonNil)                   // not persistent → no-op

	c.setSurfacePersistent(true)
	c.markSurfaceContentForPresent(nonNil) // persistent but invalid → no-op
	c.setSurfaceContentValid(true)
	// persistent+valid reaches gpuCtxOps; this test context has no GPU ops
	// (no accelerator in unit tests), so the mark is still a safe no-op here.
	c.markSurfaceContentForPresent(nonNil)
}
