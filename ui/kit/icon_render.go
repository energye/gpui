package kit

import (
	"github.com/energye/gpui/ui/kit/internal/prim"
	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/theme"
)

// IconLayout is the square layout box (WIDGET_MODEL Render segment).
type IconLayout struct {
	Edge float64
}

// ComputeIconLayout returns the size x size box; hit == layout == paint.
func ComputeIconLayout(edge float64) IconLayout {
	if edge <= 0 {
		edge = DefaultIconSize
	}
	return IconLayout{Edge: edge}
}

// IconHit reports square containment.
func (l IconLayout) Hit(x, y float64) bool {
	return x >= 0 && y >= 0 && x < l.Edge && y < l.Edge
}

// IconPaintSpec binds resolved colors to a glyph name for paint.
type IconPaintSpec struct {
	Name      string
	Variant   string
	Size      float64
	AngleDeg  float64
	TwoTone   bool
	HasSecond bool
	Main      theme.Color
	Secondary theme.Color
	CustomKey string
}

// ResolveIconPaintSpec snapshots instance paint inputs.
func (in *IconInstance) ResolveIconPaintSpec() IconPaintSpec {
	if in == nil {
		return IconPaintSpec{Size: DefaultIconSize}
	}
	in.mu.Lock()
	props := in.props
	phase := in.spinPhase
	custom := in.customKey
	ctx := in.ctx
	in.mu.Unlock()
	res := ResolveIconCtx(ctx, props)
	name := props.Name
	if custom != "" {
		name = "__custom__" + custom
	}
	variant := props.Variant
	if variant == "" {
		variant = "outlined"
	}
	// Two-tone variant implies two-tone paint even without explicit set.
	twoTone := res.TwoTone || variant == "twotone"
	return IconPaintSpec{
		Name:      name,
		Variant:   variant,
		Size:      res.Size,
		AngleDeg:  props.Rotate + phase*360,
		TwoTone:   twoTone,
		HasSecond: res.HasSecond,
		Main:      res.Main,
		Secondary: res.Secondary,
		CustomKey: custom,
	}
}

// IconPainterForSpec returns the L1 painter for a spec. Custom keys
// resolve as glyph names when known (offline iconfont maps type to a
// glyph key); unknown custom keys draw a heart stand-in, never blank.
func IconPainterForSpec(spec IconPaintSpec) prim.Painter {
	name := spec.Name
	variant := spec.Variant
	if variant == "" {
		variant = "outlined"
	}
	if spec.CustomKey != "" {
		if prim.IsKnownAntdIcon(spec.CustomKey, "outlined") || prim.IsKnownIconGlyph(spec.CustomKey) {
			name = spec.CustomKey
			variant = "outlined"
		} else {
			name = "heart"
			variant = "filled"
		}
	} else if len(name) >= 10 && name[:10] == "__custom__" {
		name = "heart"
		variant = "filled"
	}
	return prim.IconPainterFor(spec.Size, prim.IconPaintSpec{
		Name:      name,
		Theme:     variant,
		Main:      spec.Main,
		Secondary: spec.Secondary,
		HasSecond: spec.HasSecond,
		TwoTone:   spec.TwoTone,
		AngleDeg:  spec.AngleDeg,
	})
}

// IconSubscribedAspects reports Ctx aspects icons read.
func IconSubscribedAspects() []scope.Aspect {
	return []scope.Aspect{scope.AspectTheme, scope.AspectMotion}
}
