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
	"math"

	"github.com/energye/gpui/ui/rendering"
)

// IconBox is the Render-side icon host (WIDGET_MODEL Render segment).
// Static glyph content lives in the inner repaint-boundary box; spin
// rotation lives on the outer transform layer only (Flutter
// RotationTransition parity): per-frame ticks never re-resolve the paint
// spec, never swap OnPaint, never re-record the glyph.
type IconBox struct {
	Inner *rendering.RenderBox
	Outer *rendering.RenderTransform
	Inst  *IconInstance
}

// IconTransformAngleRad reports the outer layer angle for in (radians).
func IconTransformAngleRad(in *IconInstance) float64 {
	if in == nil {
		return 0
	}
	return in.EffectiveAngle() * math.Pi / 180
}

// BuildIconBox builds one icon render box: static paint inside, transform
// outside. Size<=0 means the kit default edge.
func BuildIconBox(ctx ScopeCtx, props IconProps, size float64, debug string) *IconBox {
	edge := size
	if edge <= 0 {
		edge = ResolveIconSize(props)
	}
	in := BuildIcon(ctx, props)
	spec := ResolveStaticIconPaintSpec(in)
	painter := IconPainterForSpec(spec)
	inner := PrimNewCustomPaint(edge, edge, painter, true, nil)
	if debug != "" {
		inner.SetDebugName(debug)
	}
	outer := PrimNewTransformed(IconTransformAngleRad(in), 1, 1, inner)
	if debug != "" {
		outer.SetDebugName(debug + "-rot")
	}
	outer.FixedWidth, outer.FixedHeight = edge, edge
	outer.SetRepaintBoundary(true)
	return &IconBox{Inner: inner, Outer: outer, Inst: in}
}

// SyncIconTransform pushes the instance angle to the outer layer.
// Call after Tick; paint-only, never layout.
func (b *IconBox) SyncIconTransform() {
	if b == nil || b.Outer == nil || b.Inst == nil {
		return
	}
	b.Outer.SetRotation(IconTransformAngleRad(b.Inst))
}

// ResolveStaticIconPaintSpec snapshots instance paint inputs with the
// static Rotate only: spin phase never enters the painter (it rides the
// transform layer via SyncIconTransform).
func ResolveStaticIconPaintSpec(in *IconInstance) IconPaintSpec {
	if in == nil {
		return IconPaintSpec{Size: DefaultIconSize}
	}
	spec := in.ResolveIconPaintSpec()
	spec.AngleDeg = in.staticRotate()
	return spec
}

// staticRotate reports props.Rotate without spin phase.
func (in *IconInstance) staticRotate() float64 {
	if in == nil {
		return 0
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.props.Rotate
}
