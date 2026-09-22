package kit

import (
	"sync"

	"github.com/energye/gpui/ui/kit/internal/prim"
	"github.com/energye/gpui/ui/kit/internal/scope"
)

// IconInstance owns one mounted icon (WIDGET_MODEL State segment).
type IconInstance struct {
	props IconProps
	ctx   scope.Ctx

	mu        sync.Mutex
	spinPhase float64
	mounted   bool
	customKey string
}

// newIconInstance builds the live host.
func newIconInstance(ctx scope.Ctx, props IconProps) *IconInstance {
	if !props.DecorSet {
		props.Decorative = true
	}
	in := &IconInstance{props: props, ctx: ctx.Normalize()}
	in.customKey = props.CustomPainter
	if in.customKey == "" {
		in.customKey = props.CustomPainterID
	}
	return in
}

// Mount marks the host live.
func (in *IconInstance) Mount(ctx scope.Ctx) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.ctx = ctx.Normalize()
	in.mounted = true
}

// Update swaps props/ctx snapshots.
func (in *IconInstance) Update(ctx scope.Ctx, next IconProps) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if !next.DecorSet {
		next.Decorative = true
	}
	in.props = next
	in.ctx = ctx.Normalize()
	in.customKey = next.CustomPainter
	if in.customKey == "" {
		in.customKey = next.CustomPainterID
	}
}

// Unmount marks the host dead.
func (in *IconInstance) Unmount() {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.mounted = false
}

// SetState runs f under the lock (sole state mutation gate).
func (in *IconInstance) SetState(f func(*IconInstance)) {
	if in == nil || f == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	f(in)
}

// Known reports registry hit (official 848, offline source, or custom).
func (in *IconInstance) Known() bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.customKey != "" {
		return true
	}
	variant := in.props.Variant
	if variant == "" {
		variant = "outlined"
	}
	if prim.IsKnownAntdIcon(in.props.Name, variant) {
		return true
	}
	if prim.IsKnownIconGlyph(in.props.Name) {
		return true
	}
	_, ok := lookupIconSource(in.props.Name)
	return ok
}

// EffectiveSize reports layout edge (default 16).
func (in *IconInstance) EffectiveSize() float64 {
	if in == nil {
		return DefaultIconSize
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return ResolveIconSize(in.props)
}

// EffectiveAngle reports rotate + spinPhase*360 in degrees.
func (in *IconInstance) EffectiveAngle() float64 {
	if in == nil {
		return 0
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.props.Rotate + in.spinPhase*360
}

// SpinPhase reports 0..1 ticker progress.
func (in *IconInstance) SpinPhase() float64 {
	if in == nil {
		return 0
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.spinPhase
}

// Tick advances spin; returns false when idle so hosts drop the ticker
// (matches Button idle convention, fixes icon E5 class bug).
func (in *IconInstance) Tick(dt float64) bool {
	if in == nil || dt <= 0 {
		return false
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if !in.props.Spin {
		return false
	}
	if in.ctx.Motion.ReducedMotion || !in.ctx.Motion.Enabled {
		return false
	}
	in.spinPhase += dt / 0.9
	for in.spinPhase >= 1 {
		in.spinPhase -= 1
	}
	return true
}

// IsSpinning reports active ticker need.
func (in *IconInstance) IsSpinning() bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.props.Spin && in.mounted && in.ctx.Motion.MotionEnabled()
}

// Role reports img for labeled icons, empty for decorative.
func (in *IconInstance) Role() string {
	if in == nil {
		return ""
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.props.AriaLabel != "" {
		return "img"
	}
	return ""
}

// TabStop reports false always: pure icons never take Tab (hosts do).
func (in *IconInstance) TabStop() bool { return false }

// HitDefer reports true for decorative icons.
func (in *IconInstance) HitDefer() bool {
	if in == nil {
		return true
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.props.Decorative && in.props.AriaLabel == ""
}
