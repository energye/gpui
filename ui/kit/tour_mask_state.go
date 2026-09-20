package kit

import (
	"sync"

	"github.com/energye/gpui/ui/kit/internal/scope"
)

// TourMaskInstance owns one guided tour run.
type TourMaskInstance struct {
	props          TourMaskProps
	steps          []TourMaskStep
	ctx            scope.Ctx
	mu             sync.Mutex
	open           bool
	current        int
	controlled     bool
	focused        string
	prevFocused    string
	scrollNeeded   bool
	onChange       func(current int)
	onClose        func()
	onFinish       func()
	mounted        bool
	actionsWrapped bool
}

func newTourMaskInstance(ctx scope.Ctx, props TourMaskProps, steps []TourMaskStep) *TourMaskInstance {
	props = normalizeTourMaskProps(props)
	in := &TourMaskInstance{
		props: props,
		steps: append([]TourMaskStep(nil), steps...),
		ctx:   ctx.Normalize(),
	}
	if props.CurrentSet {
		in.controlled = true
		in.current = props.Current
	} else {
		in.current = props.DefaultCurrent
	}
	in.open = props.Open
	in.clampCurrentLocked()
	if in.open {
		in.focused = "next"
		in.scrollNeeded = in.effectiveScrollIntoView()
	}
	return in
}

func normalizeTourMaskProps(p TourMaskProps) TourMaskProps {
	if !p.PlacementSet || p.Placement == "" {
		// Keep explicit value; default applied by resolvers.
		if p.Placement == "" {
			p.Placement = TourMaskPlacementBottom
		}
	}
	if p.Type == "" {
		p.Type = TourMaskTypeDefault
	}
	if !p.GapSet {
		p.Gap = DefaultTourMaskGap()
	} else {
		if p.Gap.Radius < 0 {
			p.Gap.Radius = 0
		}
	}
	if !p.ZIndexSet || p.ZIndex == 0 {
		if p.ZIndex == 0 {
			p.ZIndex = 1001
		}
	}
	return p
}

func (in *TourMaskInstance) clampCurrentLocked() {
	if len(in.steps) == 0 {
		in.current = 0
		return
	}
	if in.current < 0 {
		in.current = 0
	}
	if in.current >= len(in.steps) {
		in.current = len(in.steps) - 1
	}
}

// Mount marks the host live.
func (in *TourMaskInstance) Mount(ctx scope.Ctx) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.ctx = ctx.Normalize()
	in.mounted = true
}

// Update swaps props/ctx snapshots. Controlled current wins.
func (in *TourMaskInstance) Update(ctx scope.Ctx, next TourMaskProps) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.ctx = ctx.Normalize()
	wasOpen := in.open
	next = normalizeTourMaskProps(next)
	in.props = next
	if next.CurrentSet {
		in.controlled = true
		in.current = next.Current
	} else {
		in.controlled = false
	}
	in.clampCurrentLocked()
	if next.Open != wasOpen {
		in.open = next.Open
		if in.open {
			in.focused = "next"
			in.scrollNeeded = in.effectiveScrollIntoView()
		} else {
			in.focused = ""
			in.scrollNeeded = false
		}
	}
}

// Unmount closes without callbacks.
func (in *TourMaskInstance) Unmount() {
	if in == nil {
		return
	}
	in.mu.Lock()
	in.mounted = false
	in.open = false
	in.focused = ""
	in.mu.Unlock()
}

// SetState runs f under the lock (sole state mutation gate).
func (in *TourMaskInstance) SetState(f func(*TourMaskInstance)) {
	if in == nil || f == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	f(in)
}

// SetSteps replaces the step list and clamps current.
func (in *TourMaskInstance) SetSteps(steps []TourMaskStep) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.steps = append([]TourMaskStep(nil), steps...)
	in.clampCurrentLocked()
}

// SetOpen opens or closes the tour. Closing via SetOpen(false) fires onClose.
func (in *TourMaskInstance) SetOpen(open bool) {
	if in == nil {
		return
	}
	in.mu.Lock()
	if in.open == open {
		in.mu.Unlock()
		return
	}
	in.open = open
	var cb func()
	if open {
		in.focused = "next"
		in.prevFocused = "trigger"
		in.scrollNeeded = in.effectiveScrollIntoView()
	} else {
		in.focused = ""
		in.scrollNeeded = false
		cb = in.onClose
	}
	in.mu.Unlock()
	if cb != nil {
		cb()
	}
}

// SetCurrent drives controlled current from outside.
func (in *TourMaskInstance) SetCurrent(current int) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.current = current
	in.clampCurrentLocked()
	in.scrollNeeded = in.open && in.effectiveScrollIntoView()
}

// SetTarget rewrites one step target rect after layout or scroll.
func (in *TourMaskInstance) SetTarget(index int, r TourMaskRect) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if index < 0 || index >= len(in.steps) {
		return
	}
	in.steps[index].Target = r
	in.steps[index].TargetSet = true
}

// SetOnChange registers the step-change callback.
func (in *TourMaskInstance) SetOnChange(fn func(current int)) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.onChange = fn
}

// SetOnClose registers the close callback.
func (in *TourMaskInstance) SetOnClose(fn func()) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.onClose = fn
}

// SetOnFinish registers the finish callback.
func (in *TourMaskInstance) SetOnFinish(fn func()) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.onFinish = fn
}

// IsOpen reports whether the tour shows.
func (in *TourMaskInstance) IsOpen() bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.open
}

// Current reports the visible step index.
func (in *TourMaskInstance) Current() int {
	if in == nil {
		return 0
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.current
}

// StepCount reports total steps.
func (in *TourMaskInstance) StepCount() int {
	if in == nil {
		return 0
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return len(in.steps)
}

// CurrentStep returns a copy of the visible step.
func (in *TourMaskInstance) CurrentStep() (TourMaskStep, bool) {
	if in == nil {
		return TourMaskStep{}, false
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if !in.open || len(in.steps) == 0 {
		return TourMaskStep{}, false
	}
	return in.steps[in.current], true
}

// Focused reports the trapped focus inside the panel.
func (in *TourMaskInstance) Focused() string {
	if in == nil {
		return ""
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.focused
}

// ScrollNeeded reports a pending scrollIntoView request.
func (in *TourMaskInstance) ScrollNeeded() bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.scrollNeeded
}

// ConsumeScroll clears the pending scroll request after the host scrolls.
func (in *TourMaskInstance) ConsumeScroll() {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.scrollNeeded = false
}

// Next advances or finishes on the last step.
func (in *TourMaskInstance) Next() {
	if in == nil {
		return
	}
	in.mu.Lock()
	if !in.open || len(in.steps) == 0 {
		in.mu.Unlock()
		return
	}
	last := in.current >= len(in.steps)-1
	var onChange func(current int)
	var onFinish func()
	var onClose func()
	changed := 0
	finish := false
	if last {
		finish = true
		onFinish = in.onFinish
		onClose = in.onClose
		in.open = false
		in.focused = ""
		in.scrollNeeded = false
	} else {
		if in.controlled {
			changed = in.current + 1
			onChange = in.onChange
		} else {
			in.current++
			in.clampCurrentLocked()
			changed = in.current
			onChange = in.onChange
			in.scrollNeeded = in.effectiveScrollIntoView()
		}
	}
	in.mu.Unlock()
	if finish {
		if onFinish != nil {
			onFinish()
		}
		if onClose != nil {
			onClose()
		}
		return
	}
	if onChange != nil {
		onChange(changed)
	}
}

// Prev steps back when past the first step.
func (in *TourMaskInstance) Prev() {
	if in == nil {
		return
	}
	in.mu.Lock()
	if !in.open || len(in.steps) == 0 || in.current <= 0 {
		in.mu.Unlock()
		return
	}
	var onChange func(current int)
	changed := 0
	if in.controlled {
		changed = in.current - 1
		onChange = in.onChange
	} else {
		in.current--
		changed = in.current
		onChange = in.onChange
		in.scrollNeeded = in.effectiveScrollIntoView()
	}
	in.mu.Unlock()
	if onChange != nil {
		onChange(changed)
	}
}

// Close shuts the tour via close icon or mask click.
func (in *TourMaskInstance) Close() {
	if in == nil {
		return
	}
	in.mu.Lock()
	if !in.open {
		in.mu.Unlock()
		return
	}
	in.open = false
	in.focused = ""
	in.scrollNeeded = false
	cb := in.onClose
	in.mu.Unlock()
	if cb != nil {
		cb()
	}
}

// ClickMask closes only when the merged mask is enabled.
func (in *TourMaskInstance) ClickMask() bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	if !in.open {
		in.mu.Unlock()
		return false
	}
	mask := in.effectiveMaskLocked(in.current)
	in.mu.Unlock()
	if !mask {
		return false
	}
	in.Close()
	return true
}

// PressEsc closes when keyboard is on.
func (in *TourMaskInstance) PressEsc() bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	if !in.open {
		in.mu.Unlock()
		return false
	}
	kb := true
	if in.props.KeyboardSet {
		kb = in.props.Keyboard
	}
	in.mu.Unlock()
	if !kb {
		return false
	}
	in.Close()
	return true
}

// PressArrow moves with left/right keys when keyboard is on.
func (in *TourMaskInstance) PressArrow(dir string) bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	if !in.open {
		in.mu.Unlock()
		return false
	}
	kb := true
	if in.props.KeyboardSet {
		kb = in.props.Keyboard
	}
	in.mu.Unlock()
	if !kb {
		return false
	}
	switch dir {
	case "left":
		// At first step Prev is a no-op but still handled.
		before := in.Current()
		in.Prev()
		return in.Current() != before || before == 0
	case "right":
		in.Next()
		return true
	default:
		return false
	}
}

// PressTab cycles focus inside the panel (trap).
func (in *TourMaskInstance) PressTab() string {
	if in == nil {
		return ""
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if !in.open {
		return ""
	}
	order := []string{"prev", "next", "close"}
	if in.current <= 0 {
		order = []string{"next", "close"}
	}
	idx := 0
	for i, f := range order {
		if f == in.focused {
			idx = i
			break
		}
	}
	in.focused = order[(idx+1)%len(order)]
	return in.focused
}

// HitHole reports whether a point lands inside the highlight hole.
// The hole never takes focus; it only decides click pass-through.
func (in *TourMaskInstance) HitHole(x, y float64) bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if !in.open || len(in.steps) == 0 {
		return false
	}
	hole := ComputeTourMaskHole(in.steps[in.current].Target, in.effectiveGapLocked(in.current))
	return x >= hole.X && x <= hole.X+hole.W && y >= hole.Y && y <= hole.Y+hole.H
}

// ClickHole reports pass-through: false blocks, true lets it reach target.
func (in *TourMaskInstance) ClickHole(x, y float64) bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if !in.open {
		return false
	}
	if in.props.DisabledInteraction {
		return false
	}
	if len(in.steps) == 0 {
		return false
	}
	hole := ComputeTourMaskHole(in.steps[in.current].Target, in.effectiveGapLocked(in.current))
	inside := x >= hole.X && x <= hole.X+hole.W && y >= hole.Y && y <= hole.Y+hole.H
	return inside
}

func (in *TourMaskInstance) effectiveMaskLocked(index int) bool {
	if index >= 0 && index < len(in.steps) && in.steps[index].MaskSet {
		return in.steps[index].Mask
	}
	if in.props.MaskSet {
		return in.props.Mask
	}
	return true
}

func (in *TourMaskInstance) effectiveGapLocked(index int) TourMaskGap {
	// Steps carry no gap field in P0; host gap wins.
	return in.props.Gap
}

func (in *TourMaskInstance) effectiveScrollIntoView() bool {
	if in.props.ScrollIntoViewSet {
		return in.props.ScrollIntoView
	}
	return true
}

// EffectiveMask reports the merged mask for the visible step.
func (in *TourMaskInstance) EffectiveMask() bool {
	if in == nil {
		return true
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if !in.open || len(in.steps) == 0 {
		if in.props.MaskSet {
			return in.props.Mask
		}
		return true
	}
	return in.effectiveMaskLocked(in.current)
}

// EffectiveType reports step type over host type.
func (in *TourMaskInstance) EffectiveType() TourMaskType {
	if in == nil {
		return TourMaskTypeDefault
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if len(in.steps) > 0 && in.current < len(in.steps) && in.steps[in.current].TypeSet {
		return in.steps[in.current].Type
	}
	if in.props.Type != "" {
		return in.props.Type
	}
	return TourMaskTypeDefault
}

// EffectivePlacement reports step placement over host placement.
func (in *TourMaskInstance) EffectivePlacement() TourMaskPlacement {
	if in == nil {
		return TourMaskPlacementBottom
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if len(in.steps) > 0 && in.current < len(in.steps) && in.steps[in.current].PlacementSet {
		return in.steps[in.current].Placement
	}
	if in.props.Placement != "" {
		return in.props.Placement
	}
	return TourMaskPlacementBottom
}

// HolderContent renders static-call holder copy through Ctx.
func (in *TourMaskInstance) HolderContent() string {
	if in == nil {
		return ""
	}
	in.mu.Lock()
	ctx := in.ctx
	in.mu.Unlock()
	if ctx.HolderRender == nil {
		return "tour holder"
	}
	return ctx.HolderRender("tour")
}
