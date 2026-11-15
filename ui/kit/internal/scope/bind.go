//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package scope

import (
	"sync"

	"github.com/energye/gpui/ui/rendering"
)

// DirtyClass names what a state change needs from the render tree:
// the declared answer to "did this SetState change geometry or just
// looks", mirroring Flutter's markNeedsLayout vs markNeedsPaint split.
type DirtyClass int

const (
	// DirtyNone leaves rendering alone (pure data state).
	DirtyNone DirtyClass = iota
	// DirtyPaint repaints the subtree; geometry is unchanged.
	DirtyPaint
	// DirtyLayout relayouts then repaints (layout implies paint).
	DirtyLayout
)

// DirtyTable declares the per-bit dirty classes for one instance kind:
// bits whose change affects geometry go in LayoutBits, appearance-only
// bits in PaintBits, the rest classify as DirtyNone. Instances wire
// DirtyClassOf to one table instead of branching per state bit.
type DirtyTable struct {
	// LayoutBits whose change affects geometry.
	LayoutBits WidgetState
	// PaintBits whose change is appearance-only.
	PaintBits WidgetState
}

// Classify reports the strongest class among the changed bits.
func (t DirtyTable) Classify(changed WidgetState) DirtyClass {
	if changed&t.LayoutBits != 0 {
		return DirtyLayout
	}
	if changed&t.PaintBits != 0 {
		return DirtyPaint
	}
	return DirtyNone
}

// Diff classifies the transition before→after: the XOR of the two
// states carries exactly the bits that flipped, so a bit that was
// already set and stays set never dirties.
func (t DirtyTable) Diff(before, after WidgetState) DirtyClass {
	return t.Classify(before ^ after)
}

// DirtyRelay is the SetState tail: an instance calls it once after
// mutating state bits, passing exactly the bits that changed. Before
// Mount the relay is nil and SetState stays silent (binding not live).
type DirtyRelay interface {
	MarkStateDirty(changed WidgetState)
}

// Instance is the framework face every bound component instance
// implements. Go interfaces have no covariance, so the props-carrying
// Update stays on each concrete type (Update(ctx, next XProps)); this
// interface carries only what the binding and the aspect registry need.
type Instance interface {
	// Mount starts the lifecycle: snapshot ctx and take the dirty
	// relay for the SetState tail.
	Mount(ctx Ctx, relay DirtyRelay)
	// UpdateCtx absorbs a context change (the didChangeDependencies
	// path): swap the snapshot, re-resolve, and mark via the relay
	// per DirtyClassOf.
	UpdateCtx(ctx Ctx)
	// Unmount ends the lifecycle (idempotent).
	Unmount()
	// Node returns the render root the build produced.
	Node() rendering.RenderObject
	// DirtyClassOf reports the dirty class a state-bit change needs.
	DirtyClassOf(changed WidgetState) DirtyClass
}

// WidgetNode is the Go Element: the live binding between one mounted
// instance and its render root. It owns the three Element duties
// (framework.dart semantics): hold the binding (Inst + Root), drive
// the lifecycle (Mount/Unmount), and translate declared dirty classes
// into node marks (MarkStateDirty). The aspect registry (InheritedWidget
// semantics, KIT_FRAME_BINDING_PLAN 件 2) reads Aspects to notify only
// real dependents.
type WidgetNode struct {
	// Inst is the bound instance.
	Inst Instance
	// Root is the render root the instance's build produced.
	Root rendering.RenderObject
	// Aspects lists the Ctx aspects the build reads.
	Aspects []Aspect

	mu      sync.Mutex
	mounted bool
}

// Attach binds a built instance to its render root. The root must
// exist at attach time (the build produced it); a nil root is a build
// bug and panics at the weld point.
func Attach(inst Instance, aspects ...Aspect) *WidgetNode {
	var root rendering.RenderObject
	if inst != nil {
		root = inst.Node()
	}
	if root == nil {
		panic("scope.Attach: instance must expose a render root")
	}
	return &WidgetNode{Inst: inst, Root: root, Aspects: aspects}
}

// Mount starts the lifecycle (idempotent): hands the instance its ctx
// snapshot and this node as the dirty relay. Lock order: instance locks
// are taken before node.mu on the SetState path; this node never calls
// into the instance while holding node.mu.
func (n *WidgetNode) Mount(ctx Ctx) {
	if n == nil || n.Inst == nil {
		return
	}
	n.mu.Lock()
	if n.mounted {
		n.mu.Unlock()
		return
	}
	n.mounted = true
	n.mu.Unlock()
	n.Inst.Mount(ctx, n)
}

// MarkStateDirty translates changed state bits into the declared node
// mark. Silent no-op while not mounted: SetState before Mount (or after
// Unmount) mutates data but never touches the tree.
func (n *WidgetNode) MarkStateDirty(changed WidgetState) {
	if n == nil || changed == 0 {
		return
	}
	n.mu.Lock()
	mounted := n.mounted
	n.mu.Unlock()
	if !mounted {
		return
	}
	switch n.Inst.DirtyClassOf(changed) {
	case DirtyLayout:
		n.Root.MarkNeedsLayout()
	case DirtyPaint:
		n.Root.MarkNeedsPaint()
	}
}

// Unmount ends the lifecycle (idempotent). mounted clears first so a
// reentrant SetState during instance teardown stays silent.
func (n *WidgetNode) Unmount() {
	if n == nil || n.Inst == nil {
		return
	}
	n.mu.Lock()
	if !n.mounted {
		n.mu.Unlock()
		return
	}
	n.mounted = false
	n.mu.Unlock()
	n.Inst.Unmount()
}
