//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package scope_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/rendering"
)

// fakeInstance is the minimal Instance: a spinner as the render root,
// a declared dirty table, and counters the tests read.
type fakeInstance struct {
	states scope.WidgetState
	table  scope.DirtyTable
	node   *rendering.RenderSpinner

	mountCalls   int
	unmountCalls int
	lastCtx      scope.Ctx
}

func newFakeInstance(table scope.DirtyTable) *fakeInstance {
	in := &fakeInstance{
		table: table,
		node:  rendering.NewRenderSpinner(24),
	}
	// First frame: fresh nodes start dirty (Init marks layout+paint),
	// so settle them once to get a clean baseline for dirty assertions.
	in.node.Layout(rendering.Tight(24, 24))
	in.node.Paint(&rendering.PaintContext{})
	return in
}

func (f *fakeInstance) Mount(ctx scope.Ctx, relay scope.DirtyRelay) {
	f.mountCalls++
	f.lastCtx = ctx
	_ = relay
}

func (f *fakeInstance) UpdateCtx(ctx scope.Ctx) {
	f.lastCtx = ctx
}

func (f *fakeInstance) Unmount() {
	f.unmountCalls++
}

func (f *fakeInstance) Node() rendering.RenderObject {
	return f.node
}

func (f *fakeInstance) DirtyClassOf(changed scope.WidgetState) scope.DirtyClass {
	return f.table.Classify(changed)
}

// paintTable classifies hover/pressed/focused as appearance-only.
var paintTable = scope.DirtyTable{PaintBits: scope.StateHover | scope.StatePressed | scope.StateFocused}

// layoutTable classifies loading as geometry-affecting (spinner swaps in).
var layoutTable = scope.DirtyTable{LayoutBits: scope.StateLoading, PaintBits: scope.StateHover}

// TestWidgetNode_SetStatePaint: the plan's positive case — a paint-class
// state change through the relay leaves Root.NeedsPaint()==true.
func TestWidgetNode_SetStatePaint(t *testing.T) {
	inst := newFakeInstance(paintTable)
	node := scope.Attach(inst, scope.AspectTheme)
	node.Mount(scope.DefaultCtx())

	node.MarkStateDirty(scope.StateHover)
	if !inst.node.NeedsPaint() {
		t.Fatal("paint-class SetState did not mark Root NeedsPaint")
	}
}

// TestWidgetNode_SetStateLayout: layout-class change marks layout (and
// paint rides along per the node protocol).
func TestWidgetNode_SetStateLayout(t *testing.T) {
	inst := newFakeInstance(layoutTable)
	node := scope.Attach(inst)
	node.Mount(scope.DefaultCtx())

	node.MarkStateDirty(scope.StateLoading)
	if !inst.node.NeedsLayout() {
		t.Fatal("layout-class SetState did not mark Root NeedsLayout")
	}
	if !inst.node.NeedsPaint() {
		t.Fatal("layout mark should imply paint dirt")
	}
}

// TestWidgetNode_SetStateNone: unclassified bits never dirty the tree.
func TestWidgetNode_SetStateNone(t *testing.T) {
	inst := newFakeInstance(paintTable)
	node := scope.Attach(inst)
	node.Mount(scope.DefaultCtx())

	node.MarkStateDirty(scope.StateSelected) // not in the table
	if inst.node.NeedsPaint() || inst.node.NeedsLayout() {
		t.Fatal("DirtyNone state change dirtied the tree")
	}
	node.MarkStateDirty(0)
	if inst.node.NeedsPaint() || inst.node.NeedsLayout() {
		t.Fatal("zero-bit change dirtied the tree")
	}
}

// TestWidgetNode_UnmountedSetStateSilent: the plan's negative case —
// SetState before Mount (and after Unmount) must not panic and must
// not touch the tree.
func TestWidgetNode_UnmountedSetStateSilent(t *testing.T) {
	inst := newFakeInstance(paintTable)
	node := scope.Attach(inst)

	node.MarkStateDirty(scope.StateHover) // before Mount
	if inst.node.NeedsPaint() || inst.node.NeedsLayout() {
		t.Fatal("unmounted SetState dirtied the tree")
	}

	node.Mount(scope.DefaultCtx())
	node.MarkStateDirty(scope.StateHover)
	if !inst.node.NeedsPaint() {
		t.Fatal("mounted SetState did not mark")
	}

	node.Unmount()
	node.MarkStateDirty(scope.StatePressed) // after Unmount
	if inst.unmountCalls != 1 {
		t.Fatalf("Unmount called %d times, want 1", inst.unmountCalls)
	}
}

// TestWidgetNode_MountIdempotent: double Mount hands the lifecycle to
// the instance exactly once.
func TestWidgetNode_MountIdempotent(t *testing.T) {
	inst := newFakeInstance(paintTable)
	node := scope.Attach(inst)
	node.Mount(scope.DefaultCtx())
	node.Mount(scope.DefaultCtx())
	if inst.mountCalls != 1 {
		t.Fatalf("Mount forwarded %d times, want 1", inst.mountCalls)
	}
	if inst.lastCtx.Size != scope.SizeMedium {
		t.Fatal("instance did not receive the ctx snapshot")
	}
}

// TestWidgetNode_UnmountIdempotent: double Unmount forwards once.
func TestWidgetNode_UnmountIdempotent(t *testing.T) {
	inst := newFakeInstance(paintTable)
	node := scope.Attach(inst)
	node.Mount(scope.DefaultCtx())
	node.Unmount()
	node.Unmount()
	if inst.unmountCalls != 1 {
		t.Fatalf("Unmount forwarded %d times, want 1", inst.unmountCalls)
	}
}

// TestDirtyTable_Diff: XOR semantics — a bit already set and staying
// set never dirties; a flip classifies by the strongest changed bit.
func TestDirtyTable_Diff(t *testing.T) {
	table := scope.DirtyTable{LayoutBits: scope.StateLoading, PaintBits: scope.StateHover | scope.StatePressed}

	if got := table.Diff(scope.StateHover, scope.StateHover); got != scope.DirtyNone {
		t.Fatalf("same state classified %v, want DirtyNone", got)
	}
	if got := table.Diff(0, scope.StateHover); got != scope.DirtyPaint {
		t.Fatalf("hover gain classified %v, want DirtyPaint", got)
	}
	if got := table.Diff(scope.StateHover, 0); got != scope.DirtyPaint {
		t.Fatalf("hover loss classified %v, want DirtyPaint", got)
	}
	// Layout bit wins over a paint bit flipped in the same transition.
	if got := table.Diff(scope.StateHover, scope.StateLoading); got != scope.DirtyLayout {
		t.Fatalf("hover→loading classified %v, want DirtyLayout", got)
	}
}

// TestAttach_NilRootPanics: a build that exposes no root is a weld
// bug; Attach panics at the bind point instead of failing later.
func TestAttach_NilRootPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Attach with nil root did not panic")
		}
	}()
	scope.Attach(nil)
}

// TestAttach_NilInstancePanics covers the other weld bug shape.
func TestAttach_NilInstancePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Attach with nil instance did not panic")
		}
	}()
	var nilInst *fakeInstance
	scope.Attach(nilInst)
}

// TestDirtyTable_ClassifyStrength: Classify picks the strongest class
// when several bits are set at once.
func TestDirtyTable_ClassifyStrength(t *testing.T) {
	table := scope.DirtyTable{LayoutBits: scope.StateLoading, PaintBits: scope.StateHover}
	both := scope.StateHover | scope.StateLoading
	if got := table.Classify(both); got != scope.DirtyLayout {
		t.Fatalf("combined bits classified %v, want DirtyLayout", got)
	}
}
