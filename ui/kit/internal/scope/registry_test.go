//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// notifyCtxInstance records UpdateCtx/Unmount calls and hands back a
// settled RenderSpinner root (same baseline trick as bind_test).
//----------------------------------------

package scope_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// notifyInstance extends fakeInstance with UpdateCtx call accounting.
type notifyInstance struct {
	fakeInstance
	updateCalls int
	lastUpdate  scope.Ctx
}

func newNotifyInstance() *notifyInstance {
	in := &notifyInstance{}
	in.table = paintTable
	in.node = rendering.NewRenderSpinner(24)
	in.node.Layout(rendering.Tight(24, 24))
	in.node.Paint(&rendering.PaintContext{})
	return in
}

func (f *notifyInstance) UpdateCtx(ctx scope.Ctx) {
	f.updateCalls++
	f.lastUpdate = ctx
}

// TestRegistry_NotifyOnlyDependents: the plan's core case — a node
// subscribed to theme stays silent when only locale changes, and is
// notified when theme changes.
func TestRegistry_NotifyOnlyDependents(t *testing.T) {
	reg := scope.NewRegistry(scope.DefaultCtx())
	inst := newNotifyInstance()
	node := scope.Attach(inst, scope.AspectTheme)
	node.Mount(reg.Ctx())
	reg.Subscribe(node, scope.AspectTheme)

	// Locale-only change: no notification.
	reg.SetCtx(reg.Ctx().WithLocale("en-US"))
	if inst.updateCalls != 0 {
		t.Fatalf("locale-only change notified theme subscriber %d times", inst.updateCalls)
	}
	if inst.node.NeedsPaint() {
		t.Fatal("locale-only change dirtied theme-only subscriber")
	}

	// Theme change: notified with the new ctx and marked dirty.
	skin := reg.Ctx().Theme
	skin.ColorPrimary = theme.Hex("#722ed1")
	reg.SetCtx(reg.Ctx().WithTheme(skin))
	if inst.updateCalls != 1 {
		t.Fatalf("theme change notified %d times, want 1", inst.updateCalls)
	}
	if inst.lastUpdate.Theme.ColorPrimary != theme.Hex("#722ed1") {
		t.Fatal("instance did not receive the new theme")
	}
	if !inst.node.NeedsPaint() {
		t.Fatal("theme change did not mark subscriber paint dirty")
	}
}

// TestRegistry_UnsubscribeStopsNotify: after Unsubscribe the node is
// invisible to later changes.
func TestRegistry_UnsubscribeStopsNotify(t *testing.T) {
	reg := scope.NewRegistry(scope.DefaultCtx())
	inst := newNotifyInstance()
	node := scope.Attach(inst, scope.AspectLocale)
	node.Mount(reg.Ctx())
	reg.Subscribe(node, scope.AspectLocale)

	reg.SetCtx(reg.Ctx().WithLocale("en-US"))
	if inst.updateCalls != 1 {
		t.Fatalf("first locale change notified %d times, want 1", inst.updateCalls)
	}

	reg.Unsubscribe(node)
	reg.SetCtx(reg.Ctx().WithLocale("ja-JP"))
	if inst.updateCalls != 1 {
		t.Fatalf("change after Unsubscribe notified %d extra times", inst.updateCalls-1)
	}
}

// TestRegistry_NoAspectChangeSilent: a SetCtx that changes nothing is
// a no-op for everyone.
func TestRegistry_NoAspectChangeSilent(t *testing.T) {
	reg := scope.NewRegistry(scope.DefaultCtx())
	inst := newNotifyInstance()
	node := scope.Attach(inst, scope.AspectTheme)
	node.Mount(reg.Ctx())
	reg.Subscribe(node, scope.AspectTheme)

	reg.SetCtx(reg.Ctx())
	if inst.updateCalls != 0 {
		t.Fatal("no-change SetCtx notified subscribers")
	}
}

// TestRegistry_MultiAspect: a node reading theme+locale is notified by
// either change; size-only changes stay silent.
func TestRegistry_MultiAspect(t *testing.T) {
	reg := scope.NewRegistry(scope.DefaultCtx())
	inst := newNotifyInstance()
	node := scope.Attach(inst, scope.AspectTheme, scope.AspectLocale)
	node.Mount(reg.Ctx())
	reg.Subscribe(node, scope.AspectTheme, scope.AspectLocale)

	reg.SetCtx(reg.Ctx().WithSize(scope.SizeSmall))
	if inst.updateCalls != 0 {
		t.Fatal("size-only change notified theme+locale subscriber")
	}
	reg.SetCtx(reg.Ctx().WithDisabled(true))
	if inst.updateCalls != 0 {
		t.Fatal("disabled-only change notified theme+locale subscriber")
	}
	reg.SetCtx(reg.Ctx().WithLocale("en-US"))
	if inst.updateCalls != 1 {
		t.Fatalf("locale change notified %d times, want 1", inst.updateCalls)
	}
}

// TestRegistry_ResubscribeReplaces: resubscribing swaps the aspect
// list, so a build that stopped reading an aspect stops being notified.
func TestRegistry_ResubscribeReplaces(t *testing.T) {
	reg := scope.NewRegistry(scope.DefaultCtx())
	inst := newNotifyInstance()
	node := scope.Attach(inst, scope.AspectLocale)
	node.Mount(reg.Ctx())
	reg.Subscribe(node, scope.AspectLocale)
	reg.Subscribe(node, scope.AspectSize) // build no longer reads locale

	reg.SetCtx(reg.Ctx().WithLocale("en-US"))
	if inst.updateCalls != 0 {
		t.Fatal("locale change notified after resubscribe dropped locale")
	}
	reg.SetCtx(reg.Ctx().WithSize(scope.SizeLarge))
	if inst.updateCalls != 1 {
		t.Fatalf("size change notified %d times, want 1", inst.updateCalls)
	}
}

// TestRegistry_NotifyOrderStable: both locale subscribers hear the
// change exactly once (subscription-order list, not a randomized map).
func TestRegistry_NotifyOrderStable(t *testing.T) {
	reg := scope.NewRegistry(scope.DefaultCtx())
	instA := newNotifyInstance()
	instB := newNotifyInstance()
	nodeA := scope.Attach(instA, scope.AspectLocale)
	nodeB := scope.Attach(instB, scope.AspectLocale)
	nodeA.Mount(reg.Ctx())
	nodeB.Mount(reg.Ctx())
	reg.Subscribe(nodeA, scope.AspectLocale)
	reg.Subscribe(nodeB, scope.AspectLocale)

	reg.SetCtx(reg.Ctx().WithLocale("en-US"))
	if instA.updateCalls != 1 || instB.updateCalls != 1 {
		t.Fatalf("both subscribers must be notified once, got A=%d B=%d", instA.updateCalls, instB.updateCalls)
	}
}

// TestRegistry_NilNodeSubscribe: nil node subscribe/unsubscribe are
// safe no-ops (defensive at the boundary).
func TestRegistry_NilNodeSubscribe(t *testing.T) {
	reg := scope.NewRegistry(scope.DefaultCtx())
	reg.Subscribe(nil, scope.AspectTheme)
	reg.Unsubscribe(nil)
	reg.SetCtx(reg.Ctx().WithLocale("en-US")) // must not panic
}
