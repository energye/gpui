package kit

import (
	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/theme"
)

// Public scope re-exports for examples/kit_* windows. Components in
// this package import internal/scope directly; examples import kit.

// Size tiers.
type (
	// ScopeSizeType selects the global control-size tier.
	ScopeSizeType = scope.SizeType
	// ScopeDirection selects the inline base direction.
	ScopeDirection = scope.Direction
	// ScopeOverflowMode selects popup overflow behavior.
	ScopeOverflowMode = scope.OverflowMode
	// ScopeVariantType selects the global input look.
	ScopeVariantType = scope.VariantType
	// ScopeTargetRef names a floating-layer mount point.
	ScopeTargetRef = scope.TargetRef
	// ScopeEmptyFn renders the empty placeholder.
	ScopeEmptyFn = scope.EmptyFn
	// ScopeHolderFn renders static-call holder content.
	ScopeHolderFn = scope.HolderFn
	// ScopeMotionConfig carries the motion switch.
	ScopeMotionConfig = scope.MotionConfig
	// ScopeCtx is the value threaded through Build/Layout/Paint.
	ScopeCtx = scope.Ctx
	// ScopeAspect names one subscribable Ctx field group.
	ScopeAspect = scope.Aspect
	// ScopeFocusRing is the library-wide keyboard focus outline.
	ScopeFocusRing = scope.FocusRing
)

const (
	// ScopeSizeSmall is the compact tier.
	ScopeSizeSmall = scope.SizeSmall
	// ScopeSizeMedium is the default tier.
	ScopeSizeMedium = scope.SizeMedium
	// ScopeSizeLarge is the roomy tier.
	ScopeSizeLarge = scope.SizeLarge
	// ScopeDirLTR lays out left to right.
	ScopeDirLTR = scope.DirLTR
	// ScopeDirRTL lays out right to left.
	ScopeDirRTL = scope.DirRTL
	// ScopeOverflowViewport clips inside the viewport.
	ScopeOverflowViewport = scope.OverflowViewport
	// ScopeOverflowScroll scrolls with the container.
	ScopeOverflowScroll = scope.OverflowScroll
	// ScopeVariantOutlined draws bordered inputs.
	ScopeVariantOutlined = scope.VariantOutlined
	// ScopeVariantFilled draws filled inputs.
	ScopeVariantFilled = scope.VariantFilled
	// ScopeVariantBorderless draws chromeless inputs.
	ScopeVariantBorderless = scope.VariantBorderless

	// ScopeAspectTheme subscribes to token tables.
	ScopeAspectTheme = scope.AspectTheme
	// ScopeAspectSize subscribes to the size tier.
	ScopeAspectSize = scope.AspectSize
	// ScopeAspectDisabled subscribes to subtree disable.
	ScopeAspectDisabled = scope.AspectDisabled
	// ScopeAspectDir subscribes to base direction.
	ScopeAspectDir = scope.AspectDir
	// ScopeAspectMotion subscribes to motion config.
	ScopeAspectMotion = scope.AspectMotion
	// ScopeAspectLocale subscribes to locale.
	ScopeAspectLocale = scope.AspectLocale
	// ScopeAspectPopup subscribes to popup behavior.
	ScopeAspectPopup = scope.AspectPopup
	// ScopeAspectEmpty subscribes to empty/holder renders.
	ScopeAspectEmpty = scope.AspectEmpty
	// ScopeAspectVariant subscribes to the input look.
	ScopeAspectVariant = scope.AspectVariant
)

// Scope widget states.
type (
	// ScopeState names the current interaction situation.
	ScopeState = scope.WidgetState
)

const (
	// ScopeStateHover reports pointer over.
	ScopeStateHover = scope.StateHover
	// ScopeStatePressed reports held down.
	ScopeStatePressed = scope.StatePressed
	// ScopeStateFocused reports keyboard focus.
	ScopeStateFocused = scope.StateFocused
	// ScopeStateDisabled reports disabled.
	ScopeStateDisabled = scope.StateDisabled
	// ScopeStateLoading reports pending action.
	ScopeStateLoading = scope.StateLoading
	// ScopeStateSelected reports toggled-on.
	ScopeStateSelected = scope.StateSelected
	// ScopeStateError reports validation error.
	ScopeStateError = scope.StateError
)

// DefaultScopeCtx returns the F0-1 default context.
func DefaultScopeCtx() ScopeCtx {
	return scope.DefaultCtx()
}

// ScopeChangedAspects reports which aspects differ.
func ScopeChangedAspects(before, after ScopeCtx) []ScopeAspect {
	return scope.ChangedAspects(before, after)
}

// ScopeSubscribed reports whether changed contains a wanted aspect.
func ScopeSubscribed(changed []ScopeAspect, want ...ScopeAspect) bool {
	return scope.Subscribed(changed, want...)
}

// ScopeDisabledOr combines subtree and widget disable with OR.
func ScopeDisabledOr(subtree, widget bool) bool {
	return scope.DisabledOr(subtree, widget)
}

// ScopeResolveFocusRing derives the keyboard focus ring from seed.
func ScopeResolveFocusRing(seed theme.Tokens) ScopeFocusRing {
	return scope.ResolveFocusRing(seed)
}

// ScopeShowFocusRing reports whether the ring paints for a state.
func ScopeShowFocusRing(s ScopeState) bool {
	return scope.ShowFocusRing(s)
}

// ScopeResolveColor resolves one color through props > theme > seed.
func ScopeResolveColor(props *theme.Color, ctheme *theme.Color, seed theme.Color) theme.Color {
	return scope.ResolveColor(props, ctheme, seed)
}

// ScopeResolveFloat resolves one number through props > theme > seed.
func ScopeResolveFloat(props *float64, ctheme *float64, seed float64) float64 {
	return scope.ResolveFloat(props, ctheme, seed)
}
