package animation

import (
	"github.com/energye/gpui/ui/scene"
	"github.com/energye/gpui/ui/scheduler"
)

// AnimatedOpacity drives layer opacity via MutSetOpacity (compositor-only, F09).
//
// It does not re-record pictures: each tick emits a Mutation{Kind: MutSetOpacity}.
// Callers apply mutations to their FramePacket / compositor path.
//
// Default animation path = compositor-only; do not MarkNeedsPaint the whole
// subtree for opacity alone.
type AnimatedOpacity struct {
	// LayerID is the opacity/boundary layer to mutate (must be non-zero for ClassifyDirty).
	LayerID uint64
	// From/To opacity in [0,1].
	From, To float64

	ctrl *Controller

	// OnMutation is called each tick with the compositor-only mutation.
	OnMutation func(m scene.Mutation)
	// OnComplete optional when one-shot finishes.
	OnComplete func()

	// last mutations for tests
	Mutations []scene.Mutation
}

// NewAnimatedOpacity creates an implicit opacity animation.
// durationSec <= 0 defaults to 1s. curve nil → EaseInOut (pleasant default for opacity).
func NewAnimatedOpacity(layerID uint64, from, to, durationSec float64, curve Curve) *AnimatedOpacity {
	if curve == nil {
		curve = CurveEaseInOut
	}
	c := NewController(durationSec)
	c.SetCurve(curve)
	a := &AnimatedOpacity{
		LayerID: layerID,
		From:    clamp01(from),
		To:      clamp01(to),
		ctrl:    c,
	}
	c.OnValue(func(t float64) {
		op := a.From + (a.To-a.From)*t
		m := scene.Mutation{
			Kind:    scene.MutSetOpacity,
			LayerID: a.LayerID,
			Opacity: op,
		}
		a.Mutations = append(a.Mutations, m)
		if a.OnMutation != nil {
			a.OnMutation(m)
		}
	})
	c.OnStatus(func(s Status) {
		if s == StatusCompleted && a.OnComplete != nil {
			a.OnComplete()
		}
	})
	return a
}

// Controller returns the underlying controller.
func (a *AnimatedOpacity) Controller() *Controller {
	if a == nil {
		return nil
	}
	return a.ctrl
}

// Start begins the animation on reg.
func (a *AnimatedOpacity) Start(reg *scheduler.TickerRegistry) {
	if a == nil || a.ctrl == nil {
		return
	}
	a.Mutations = a.Mutations[:0]
	a.ctrl.Start(reg)
}

// Stop aborts and unregisters.
func (a *AnimatedOpacity) Stop() {
	if a == nil || a.ctrl == nil {
		return
	}
	a.ctrl.Stop()
}

// IsRunning reports whether the opacity animation is active.
func (a *AnimatedOpacity) IsRunning() bool {
	return a != nil && a.ctrl != nil && a.ctrl.IsRunning()
}

// OpacityAt returns interpolated opacity for curved t in [0,1].
func (a *AnimatedOpacity) OpacityAt(t float64) float64 {
	if a == nil {
		return 1
	}
	t = clamp01(t)
	return a.From + (a.To-a.From)*t
}
