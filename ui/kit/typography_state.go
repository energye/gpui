//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package kit

import (
	"sync"

	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/rendering"
)

// TypographyInstance is one mounted Typography (State facet). It owns the
// operator state machines (editing / copied / expanded) and implements the
// framework face (KIT_FRAME_BINDING_PLAN 件 1): Node/Mount/UpdateCtx/
// Unmount/DirtyClassOf, with the props-carrying Update on the concrete type.
type TypographyInstance struct {
	props TypographyProps
	ctx   scope.Ctx
	mu    sync.Mutex

	// node is the render root produced by the build (the content label;
	// operators attach beside it, the binding dirties through here).
	node *rendering.RenderText
	// relay is the WidgetNode dirty tail; nil until Mount.
	relay scope.DirtyRelay

	// editing: uncontrolled flip only when Editable.EditingCtl is nil.
	editing      bool
	editingCtl   bool
	editingValue string

	// copied is the transient success state (≈3s feedback window).
	copied bool

	// expanded: uncontrolled flip only when Ellipsis.ExpandedCtl is nil.
	expanded      bool
	expandedCtl   bool
	defaultExpand bool

	mounted bool
}

// dirtyBits declares which state flips need layout vs paint (件 1
// DirtyTable): expansion changes line count → layout; editing/copy
// feedback swap appearance only → paint.
var typographyDirty = scope.DirtyTable{
	LayoutBits: typographyStateExpanded,
	PaintBits:  typographyStateEditing | typographyStateCopied,
}

// instance-local state bits for the DirtyTable declaration.
const (
	typographyStateEditing  scope.WidgetState = 1 << 16
	typographyStateCopied   scope.WidgetState = 1 << 17
	typographyStateExpanded scope.WidgetState = 1 << 18
)

// newTypographyInstance builds the state machine. The render root arrives
// with the build (the label already carries resolved look + text).
func newTypographyInstance(ctx scope.Ctx, p TypographyProps, node *rendering.RenderText) *TypographyInstance {
	in := &TypographyInstance{
		props: p,
		ctx:   ctx.Normalize(),
		node:  node,
	}
	if p.Editable.EditingCtl != nil {
		in.editingCtl = true
		in.editing = *p.Editable.EditingCtl
	}
	if p.Ellipsis.ExpandedCtl != nil {
		in.expandedCtl = true
		in.expanded = *p.Ellipsis.ExpandedCtl
	} else {
		in.expanded = p.Ellipsis.DefaultExpanded
	}
	return in
}

// ---- framework face (scope.Instance) ----

// Node returns the render root (binding contract).
func (in *TypographyInstance) Node() rendering.RenderObject {
	if in == nil {
		return nil
	}
	return in.node
}

// Mount snapshots ctx and takes the dirty relay (binding contract).
func (in *TypographyInstance) Mount(ctx scope.Ctx, relay scope.DirtyRelay) {
	if in == nil {
		return
	}
	in.mu.Lock()
	in.ctx = ctx.Normalize()
	in.relay = relay
	in.mounted = true
	in.mu.Unlock()
}

// UpdateCtx absorbs a context change (InheritedWidget path, 件 2): theme
// and locale changes re-resolve appearance; the node properties mutate
// in place, the tree never rebuilds.
func (in *TypographyInstance) UpdateCtx(ctx scope.Ctx) {
	if in == nil {
		return
	}
	in.mu.Lock()
	in.ctx = ctx.Normalize()
	props := in.props
	relay := in.relay
	in.mu.Unlock()
	in.applyResolved(relay, props, in.ctx)
}

// Unmount ends the lifecycle (idempotent).
func (in *TypographyInstance) Unmount() {
	if in == nil {
		return
	}
	in.mu.Lock()
	in.mounted = false
	in.relay = nil
	in.mu.Unlock()
}

// DirtyClassOf declares the dirty class per state bit (binding contract).
func (in *TypographyInstance) DirtyClassOf(changed scope.WidgetState) scope.DirtyClass {
	return typographyDirty.Classify(changed)
}

// SetState is the sole state-mutation gate: run f under the lock, then
// hand the changed bits to the relay (silent before Mount / after Unmount).
func (in *TypographyInstance) SetState(f func(*TypographyInstance), changed scope.WidgetState) {
	if in == nil || f == nil {
		return
	}
	in.mu.Lock()
	f(in)
	relay := in.relay
	in.mu.Unlock()
	if relay != nil {
		relay.MarkStateDirty(changed)
	}
}

// Update absorbs new props: swap the snapshot, re-resolve, mutate node
// properties in place (never rebuild the tree). Controlled editing /
// expansion mirrors the new ctl values.
func (in *TypographyInstance) Update(ctx scope.Ctx, next TypographyProps) {
	if in == nil {
		return
	}
	in.mu.Lock()
	old := in.props
	in.props = next
	in.ctx = ctx.Normalize()
	if next.Editable.EditingCtl != nil {
		in.editingCtl = true
		in.editing = *next.Editable.EditingCtl
	}
	if next.Ellipsis.ExpandedCtl != nil {
		in.expandedCtl = true
		in.expanded = *next.Ellipsis.ExpandedCtl
	}
	relay := in.relay
	in.mu.Unlock()

	// Content change: mutate the text node (render-level SetText marks).
	if old.Value != next.Value {
		in.node.SetText(next.Value)
	}
	in.applyResolved(relay, next, in.ctx)
}

// applyResolved pushes the resolved look onto the node (Update/UpdateCtx
// shared tail).
func (in *TypographyInstance) applyResolved(relay scope.DirtyRelay, p TypographyProps, ctx scope.Ctx) {
	if in.node == nil {
		return
	}
	disabled := scope.DisabledOr(ctx.Disabled, p.Disabled)
	res := ResolveTypography(ctx.Theme, p, disabled)
	st := res.TypographyLabelStyle()
	if st.FontSize > 0 {
		in.node.SetFontSize(st.FontSize)
	}
	in.node.SetColor(st.Color.R, st.Color.G, st.Color.B, st.Color.A)
	in.node.SetDecoration(res.Decoration)
	if relay != nil {
		relay.MarkStateDirty(typographyStateCopied) // paint-class look sync
	}
}

// ---- editing machine ----

// StartEditing enters edit mode (operator trigger); fires OnStart.
func (in *TypographyInstance) StartEditing() {
	if in == nil {
		return
	}
	in.mu.Lock()
	if in.props.Disabled || in.props.Editable.EditingCtl != nil {
		relay := in.relay
		in.mu.Unlock()
		if relay != nil {
			relay.MarkStateDirty(0)
		}
		return
	}
	in.editing = true
	in.editingValue = in.props.Value
	onStart := in.props.Editable.OnStart
	in.mu.Unlock()
	if onStart != nil {
		onStart()
	}
	in.SetState(func(*TypographyInstance) {}, typographyStateEditing)
}

// CommitEditing validates and applies the pending value (Enter): fires
// OnChange + OnEnd, leaves edit mode.
func (in *TypographyInstance) CommitEditing() {
	if in == nil {
		return
	}
	in.mu.Lock()
	if !in.editing {
		in.mu.Unlock()
		return
	}
	value := in.editingValue
	if in.props.Editable.MaxLength > 0 && len([]rune(value)) > in.props.Editable.MaxLength {
		value = string([]rune(value)[:in.props.Editable.MaxLength])
	}
	in.editing = false
	onChange, onEnd := in.props.Editable.OnChange, in.props.Editable.OnEnd
	in.mu.Unlock()

	in.node.SetText(value)
	if onChange != nil {
		onChange(value)
	}
	if onEnd != nil {
		onEnd()
	}
	in.SetState(func(*TypographyInstance) {}, typographyStateEditing)
}

// CancelEditing aborts editing (Esc): fires OnCancel, value untouched.
func (in *TypographyInstance) CancelEditing() {
	if in == nil {
		return
	}
	in.mu.Lock()
	was := in.editing
	in.editing = false
	onCancel := in.props.Editable.OnCancel
	in.mu.Unlock()
	if was && onCancel != nil {
		onCancel()
	}
	if was {
		in.SetState(func(*TypographyInstance) {}, typographyStateEditing)
	}
}

// SetEditingValue stages the in-editor text (before commit).
func (in *TypographyInstance) SetEditingValue(v string) {
	if in == nil {
		return
	}
	in.mu.Lock()
	in.editingValue = v
	in.mu.Unlock()
}

// Editing reports the current edit mode.
func (in *TypographyInstance) Editing() bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.editing
}

// ---- copy machine ----

// Copy performs the operator action: payload = Copyable.Text or Value;
// OnCopy fires once. The clipboard write is host-injected (P0 映射宿主,
// PROGRESS 记剪贴板宿主后补) — the state machine side is complete here.
func (in *TypographyInstance) Copy() string {
	if in == nil {
		return ""
	}
	in.mu.Lock()
	if in.props.Disabled {
		in.mu.Unlock()
		return ""
	}
	payload := in.props.Copyable.Text
	if payload == "" {
		payload = in.props.Value
	}
	onCopy := in.props.Copyable.OnCopy
	in.copied = true
	in.mu.Unlock()
	if onCopy != nil {
		onCopy(payload)
	}
	in.SetState(func(*TypographyInstance) {}, typographyStateCopied)
	return payload
}

// Copied reports the transient success feedback state.
func (in *TypographyInstance) Copied() bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.copied
}

// ---- expansion machine ----

// ToggleExpand flips expansion (expandable/collapsible operator); the
// controlled form mirrors the ctl value and reports it (antd fires
// onExpand with the current controlled value on click).
func (in *TypographyInstance) ToggleExpand() {
	if in == nil {
		return
	}
	in.mu.Lock()
	p := in.props
	relay := in.relay
	if p.Disabled {
		in.mu.Unlock()
		return
	}
	if p.Ellipsis.ExpandedCtl != nil {
		final := in.expanded
		in.mu.Unlock()
		if onExpand := p.Ellipsis.OnExpand; onExpand != nil {
			onExpand(final)
		}
		if relay != nil {
			relay.MarkStateDirty(typographyStateExpanded)
		}
		return
	}
	in.expanded = !in.expanded
	final := in.expanded
	onExpand := p.Ellipsis.OnExpand
	in.mu.Unlock()
	if onExpand != nil {
		onExpand(final)
	}
	in.SetState(func(*TypographyInstance) {}, typographyStateExpanded)
}

// Expanded reports the current expansion state (受控优先).
func (in *TypographyInstance) Expanded() bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.expanded
}

// SetExpanded writes the controlled expansion (外部优先).
func (in *TypographyInstance) SetExpanded(v bool) {
	if in == nil {
		return
	}
	in.mu.Lock()
	in.expanded = v
	onExpand := in.props.Ellipsis.OnExpand
	in.mu.Unlock()
	if onExpand != nil {
		onExpand(v)
	}
	in.SetState(func(*TypographyInstance) {}, typographyStateExpanded)
}
