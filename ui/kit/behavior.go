package kit

import (
	"github.com/energye/gpui/ui/animation"
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/kit/internal/behavior"
	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/overlay"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/semantics"
	"github.com/energye/gpui/ui/theme"
)

// Public behavior re-exports for examples/kit_f0_* windows. Product code
// in this package imports internal/behavior directly; examples import kit.

type (
	// BehaviorInteractiveConfig describes one pressable control.
	BehaviorInteractiveConfig = behavior.InteractiveConfig
	// BehaviorInteractive owns the five-state machine.
	BehaviorInteractive = behavior.Interactive
	// BehaviorFieldConfig describes one form value.
	BehaviorFieldConfig = behavior.FieldConfig
	// BehaviorField owns the committed value plus IME composition.
	BehaviorField = behavior.Field
	// BehaviorTriggerConfig describes one floating layer trigger.
	BehaviorTriggerConfig = behavior.TriggerConfig
	// BehaviorTrigger owns open state, placement and focus lock.
	BehaviorTrigger = behavior.OverlayTrigger
	// BehaviorPlacement is the twelve-direction vocabulary.
	BehaviorPlacement = overlay.Placement
	// BehaviorResolveOptions tunes placement.
	BehaviorResolveOptions = overlay.ResolveOptions
	// BehaviorResolved is the placement outcome.
	BehaviorResolved = overlay.Resolved
)

// NewBehaviorInteractive builds the five-state machine.
func NewBehaviorInteractive(cfg BehaviorInteractiveConfig) *BehaviorInteractive {
	return behavior.NewInteractive(cfg)
}

// NewBehaviorField builds the form controller.
func NewBehaviorField(cfg BehaviorFieldConfig) *BehaviorField {
	return behavior.NewField(cfg)
}

// NewBehaviorTrigger builds the floating trigger.
func NewBehaviorTrigger(cfg BehaviorTriggerConfig, mgr *focus.FocusManager, st *overlay.State) *BehaviorTrigger {
	return behavior.NewTrigger(cfg, mgr, st)
}

// BehaviorResolveFollower positions a follower via overlay only.
func BehaviorResolveFollower(anchor rendering.Rect, ow, oh float64, want BehaviorPlacement, opt *BehaviorResolveOptions) BehaviorResolved {
	return behavior.ResolveFollower(anchor, ow, oh, want, opt)
}

// BehaviorContains reports rect hit for window routing.
func BehaviorContains(x, y, w, h, px, py float64) bool {
	return behavior.Contains(x, y, w, h, px, py)
}

type (
	// BehaviorMotionDurations carries resolved fast/mid/slow seconds.
	BehaviorMotionDurations = behavior.MotionDurations
	// BehaviorSpinner drives true rotation via a repeating Controller.
	BehaviorSpinner = behavior.Spinner
	// BehaviorWaveConfig describes one ripple.
	BehaviorWaveConfig = behavior.WaveConfig
	// BehaviorWave is the one-shot ripple.
	BehaviorWave = behavior.Wave
	// BehaviorBudgets carries the steady-window fps budget.
	BehaviorBudgets = behavior.Budgets
	// BehaviorCacheBudget is the tiny LRU eviction rule.
	BehaviorCacheBudget = behavior.CacheBudget
)

// BehaviorParseDuration parses "0.1s"/"200ms" into seconds.
func BehaviorParseDuration(s string) float64 { return behavior.ParseDuration(s) }

// BehaviorResolveDurations reads the three duration tokens.
func BehaviorResolveDurations(tok theme.Tokens) BehaviorMotionDurations {
	return behavior.ResolveDurations(tok)
}

// BehaviorResolveCurve maps a token-style name to an engine curve.
func BehaviorResolveCurve(name string) animation.Curve { return behavior.ResolveCurve(name) }

// NewBehaviorSpinner builds a looping spinner.
func NewBehaviorSpinner(durSec float64, curve animation.Curve) *BehaviorSpinner {
	return behavior.NewSpinner(durSec, curve)
}

// NewBehaviorWave builds an idle ripple.
func NewBehaviorWave() *BehaviorWave { return behavior.NewWave() }

// BehaviorCanWave reports whether a ripple may start.
func BehaviorCanWave(motion scope.MotionConfig, cfg BehaviorWaveConfig) bool {
	return behavior.CanWave(motion, cfg)
}

// BehaviorWaveColor picks border color when bordered, else background.
func BehaviorWaveColor(border, bg theme.Color, hasBorder bool) theme.Color {
	return behavior.WaveColor(border, bg, hasBorder)
}

// BehaviorFadeAt returns implicit fade opacity for t in [0,1].
func BehaviorFadeAt(t float64) float64 { return behavior.FadeAt(t) }

// BehaviorAuditOne checks a single semantics node for name plus role.
func BehaviorAuditOne(n *semantics.Node) (bool, string) { return behavior.AuditOne(n) }

// BehaviorAuditTree counts named-and-roled nodes.
func BehaviorAuditTree(root *semantics.Node) (int, int) { return behavior.AuditTree(root) }

// BehaviorMinTouchOK is the advisory 44px touch target check.
func BehaviorMinTouchOK(w, h float64) bool { return behavior.MinTouchOK(w, h) }

// BehaviorContrastRatio returns the WCAG ratio of fg over bg.
func BehaviorContrastRatio(fg, bg theme.Color) float64 { return behavior.ContrastRatio(fg, bg) }

// BehaviorPassContrast reports ratio >= 4.5 (advisory only).
func BehaviorPassContrast(ratio float64) bool { return behavior.PassContrast(ratio) }

// BehaviorDefaultBudgets is the motion-window gate.
func BehaviorDefaultBudgets() BehaviorBudgets { return behavior.DefaultBudgets() }

// BehaviorCheckIsolation reports the repaint-boundary contract.
func BehaviorCheckIsolation(animatedDirty, siblingDirty bool) bool {
	return behavior.CheckIsolation(animatedDirty, siblingDirty)
}

// NewBehaviorCacheBudget builds a budget holding at most cap entries.
func NewBehaviorCacheBudget(cap int) *BehaviorCacheBudget { return behavior.NewCacheBudget(cap) }

// BehaviorVirtualBoundOK checks bind_count far below item_count.
func BehaviorVirtualBoundOK(total, bound int) bool { return behavior.VirtualBoundOK(total, bound) }

// BehaviorIsRTL reports right-to-left layout.
func BehaviorIsRTL(ctx scope.Ctx) bool { return behavior.IsRTL(ctx) }

// BehaviorMirrorX mirrors an offset inside width for RTL.
func BehaviorMirrorX(x, width float64, dir scope.Direction) float64 {
	return behavior.MirrorX(x, width, dir)
}

// BehaviorArrowPrevNext returns (prev,next) arrows for dir.
func BehaviorArrowPrevNext(dir scope.Direction, prev, next string) (string, string) {
	return behavior.ArrowPrevNext(dir, prev, next)
}

// BehaviorEmptyText resolves empty copy via Ctx.
func BehaviorEmptyText(ctx scope.Ctx, component string) string {
	return behavior.EmptyText(ctx, component)
}
