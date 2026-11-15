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

import "sync"

// Registry is the InheritedWidget dependency registry: mounted nodes
// subscribe the aspects their build reads; a SetCtx change notifies
// only real dependents (framework.dart dependOnInheritedWidgetOfExactType
// semantics — a change to an aspect nobody reads rebuilds nobody).
type Registry struct {
	mu  sync.Mutex
	ctx Ctx
	// subs keeps subscription order stable so notification order is
	// deterministic (map iteration would randomize it).
	subs []*RegistrySub
}

// RegistrySub is one subscription: a node plus the aspects it reads.
type RegistrySub struct {
	Node    *WidgetNode
	Aspects []Aspect
}

// NewRegistry starts a scope holding ctx.
func NewRegistry(ctx Ctx) *Registry {
	return &Registry{ctx: ctx.Normalize()}
}

// Ctx returns the current scope context.
func (r *Registry) Ctx() Ctx {
	if r == nil {
		return Ctx{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ctx
}

// Subscribe registers node for the given aspects. Resubscribing the
// same node replaces its aspect list (the build's reads changed).
func (r *Registry) Subscribe(node *WidgetNode, aspects ...Aspect) {
	if r == nil || node == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.subs {
		if r.subs[i].Node == node {
			r.subs[i].Aspects = append([]Aspect(nil), aspects...)
			return
		}
	}
	r.subs = append(r.subs, &RegistrySub{
		Node:    node,
		Aspects: append([]Aspect(nil), aspects...),
	})
}

// Unsubscribe removes node; unsubscribing an unknown node is a no-op.
func (r *Registry) Unsubscribe(node *WidgetNode) {
	if r == nil || node == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.subs {
		if r.subs[i].Node == node {
			r.subs = append(r.subs[:i], r.subs[i+1:]...)
			return
		}
	}
}

// SetCtx installs next and notifies only nodes subscribed to a changed
// aspect. Notification order follows subscription order.
func (r *Registry) SetCtx(next Ctx) {
	if r == nil {
		return
	}
	next = next.Normalize()
	r.mu.Lock()
	changed := ChangedAspects(r.ctx, next)
	if len(changed) == 0 {
		r.mu.Unlock()
		return
	}
	// Snapshot the matching subs under the lock; NotifyCtx calls into
	// instance code which may reenter the registry.
	var hits []*WidgetNode
	for _, s := range r.subs {
		if Subscribed(changed, s.Aspects...) {
			hits = append(hits, s.Node)
		}
	}
	r.ctx = next
	r.mu.Unlock()
	for _, n := range hits {
		n.NotifyCtx(next)
	}
}

// NotifyCtx delivers a context change to the bound instance and marks
// the root for repaint (theme/locale/etc. surface through Resolve;
// geometry-only effects ride the instance's own setters). Registry
// membership implies a live binding, so this forwards unconditionally.
func (n *WidgetNode) NotifyCtx(next Ctx) {
	if n == nil || n.Inst == nil {
		return
	}
	n.Inst.UpdateCtx(next)
	n.Root.MarkNeedsPaint()
}
