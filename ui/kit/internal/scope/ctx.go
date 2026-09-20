// Package scope carries the F0-1 theme-and-range context.
//
// Ctx is the explicit value threaded through Build/Layout/Paint,
// following Flutter's InheritedWidget data flow. A zero Ctx reads as
// the documented defaults; providers build a derived copy instead of
// mutating the parent. Consumers subscribe by aspect so only the
// fields they read can mark them dirty.
package scope

import (
	"github.com/energye/gpui/ui/theme"
)

// SizeType selects the global control-size tier.
type SizeType string

const (
	// SizeSmall is the compact tier.
	SizeSmall SizeType = "small"
	// SizeMedium is the default tier.
	SizeMedium SizeType = "medium"
	// SizeLarge is the roomy tier.
	SizeLarge SizeType = "large"
)

// Direction selects the inline base direction.
type Direction string

const (
	// DirLTR lays out left to right.
	DirLTR Direction = "ltr"
	// DirRTL lays out right to left.
	DirRTL Direction = "rtl"
)

// OverflowMode selects how a popup layer handles viewport overflow.
type OverflowMode string

const (
	// OverflowViewport clips and flips inside the viewport.
	OverflowViewport OverflowMode = "viewport"
	// OverflowScroll lets the layer scroll with its container.
	OverflowScroll OverflowMode = "scroll"
)

// VariantType selects the global input look.
type VariantType string

const (
	// VariantOutlined draws a border around inputs.
	VariantOutlined VariantType = "outlined"
	// VariantFilled draws inputs on a filled background.
	VariantFilled VariantType = "filled"
	// VariantBorderless draws inputs without chrome.
	VariantBorderless VariantType = "borderless"
)

// TargetRef names the container a floating layer mounts into.
// Empty means the current position (the antd getPopupContainer default
// of mounting where the trigger sits).
type TargetRef string

// EmptyFn renders the empty placeholder for a named component.
type EmptyFn func(componentName string) string

// HolderFn renders static-call content (message/modal/notice holders).
type HolderFn func(kind string) string

// MotionConfig carries the motion master switch plus the
// system reduced-motion preference. MotionEnabled reports
// whether animated effects may run.
type MotionConfig struct {
	// Enabled is the explicit motion switch (ConfigProvider motion).
	Enabled bool
	// ReducedMotion mirrors the OS reduced-motion preference.
	ReducedMotion bool
}

// MotionEnabled reports whether animated effects may run.
func (m MotionConfig) MotionEnabled() bool {
	return m.Enabled && !m.ReducedMotion
}

// DefaultMotionConfig is the F0-1 default: motion on, no
// reduced-motion preference asserted.
func DefaultMotionConfig() MotionConfig {
	return MotionConfig{Enabled: true}
}

// Ctx is the value threaded through Build/Layout/Paint. Every field
// has a default (see DefaultCtx); consumers subscribe by aspect and
// only re-resolve when a subscribed aspect changes.
type Ctx struct {
	// Theme carries seed plus alias plus per-component token tables.
	Theme theme.Tokens
	// Size is the global control-size tier.
	Size SizeType
	// Disabled disables the whole subtree (OR with per-widget disabled).
	Disabled bool
	// Dir is the inline base direction.
	Dir Direction
	// Motion carries the motion switch plus reduced-motion preference.
	Motion MotionConfig
	// Locale selects language-sensitive copy such as empty text.
	Locale string
	// PopupContainer names the floating-layer mount point.
	PopupContainer TargetRef
	// TargetContainer names the scroll-listener container.
	TargetContainer TargetRef
	// PopupMatchWidth forces dropdown layers to match trigger width.
	// Nil means follow the component default.
	PopupMatchWidth *bool
	// PopupOverflow selects the popup overflow behavior.
	PopupOverflow OverflowMode
	// RenderEmpty renders the empty placeholder for a component.
	RenderEmpty EmptyFn
	// HolderRender renders static-call holder content.
	HolderRender HolderFn
	// Variant selects the global input look.
	Variant VariantType
}

// DefaultCtx returns the F0-1 default context: default light seed,
// medium size, LTR, motion on, zh-CN locale, viewport overflow,
// outlined inputs, and the built-in empty/holder renders.
func DefaultCtx() Ctx {
	return Ctx{
		Theme:           theme.DefaultTokens(),
		Size:            SizeMedium,
		Disabled:        false,
		Dir:             DirLTR,
		Motion:          DefaultMotionConfig(),
		Locale:          "zh-CN",
		PopupContainer:  "",
		TargetContainer: "",
		PopupMatchWidth: nil,
		PopupOverflow:   OverflowViewport,
		RenderEmpty:     DefaultRenderEmpty,
		HolderRender:    DefaultHolderRender,
		Variant:         VariantOutlined,
	}
}

// DefaultRenderEmpty is the built-in empty placeholder.
func DefaultRenderEmpty(componentName string) string {
	if componentName == "" {
		return "No data"
	}
	return "No " + componentName + " data"
}

// DefaultHolderRender is the built-in static-call holder content.
func DefaultHolderRender(kind string) string {
	if kind == "" {
		return "holder"
	}
	return kind + " holder"
}

// Aspect names one subscribable Ctx field group.
type Aspect string

const (
	// AspectTheme subscribes to the token tables.
	AspectTheme Aspect = "theme"
	// AspectSize subscribes to the control-size tier.
	AspectSize Aspect = "size"
	// AspectDisabled subscribes to the subtree disabled flag.
	AspectDisabled Aspect = "disabled"
	// AspectDir subscribes to the base direction.
	AspectDir Aspect = "dir"
	// AspectMotion subscribes to the motion config.
	AspectMotion Aspect = "motion"
	// AspectLocale subscribes to the locale.
	AspectLocale Aspect = "locale"
	// AspectPopup subscribes to popup mount/overflow behavior.
	AspectPopup Aspect = "popup"
	// AspectEmpty subscribes to the empty/holder renders.
	AspectEmpty Aspect = "empty"
	// AspectVariant subscribes to the global input look.
	AspectVariant Aspect = "variant"
)

// WithTheme returns a copy with the token tables replaced.
func (c Ctx) WithTheme(t theme.Tokens) Ctx {
	c.Theme = t
	return c
}

// WithSize returns a copy with the size tier replaced.
func (c Ctx) WithSize(s SizeType) Ctx {
	c.Size = s
	return c
}

// WithDisabled returns a copy with the subtree disabled flag replaced.
func (c Ctx) WithDisabled(disabled bool) Ctx {
	c.Disabled = disabled
	return c
}

// WithDir returns a copy with the base direction replaced.
func (c Ctx) WithDir(d Direction) Ctx {
	c.Dir = d
	return c
}

// WithMotion returns a copy with the motion config replaced.
func (c Ctx) WithMotion(m MotionConfig) Ctx {
	c.Motion = m
	return c
}

// WithLocale returns a copy with the locale replaced.
func (c Ctx) WithLocale(locale string) Ctx {
	c.Locale = locale
	return c
}

// WithPopupContainer returns a copy with the mount point replaced.
func (c Ctx) WithPopupContainer(ref TargetRef) Ctx {
	c.PopupContainer = ref
	return c
}

// WithTargetContainer returns a copy with the scroll container replaced.
func (c Ctx) WithTargetContainer(ref TargetRef) Ctx {
	c.TargetContainer = ref
	return c
}

// WithPopupMatchWidth returns a copy with the match-width flag replaced.
func (c Ctx) WithPopupMatchWidth(match *bool) Ctx {
	c.PopupMatchWidth = match
	return c
}

// WithPopupOverflow returns a copy with the overflow mode replaced.
func (c Ctx) WithPopupOverflow(mode OverflowMode) Ctx {
	c.PopupOverflow = mode
	return c
}

// WithRenderEmpty returns a copy with the empty render replaced.
func (c Ctx) WithRenderEmpty(fn EmptyFn) Ctx {
	c.RenderEmpty = fn
	return c
}

// WithHolderRender returns a copy with the holder render replaced.
func (c Ctx) WithHolderRender(fn HolderFn) Ctx {
	c.HolderRender = fn
	return c
}

// WithVariant returns a copy with the global input look replaced.
func (c Ctx) WithVariant(v VariantType) Ctx {
	c.Variant = v
	return c
}

// ChangedAspects reports which aspects differ between two contexts.
// RenderEmpty and HolderRender compare by presence only because
// function values cannot be compared deeply; swapping either counts
// as an empty-aspect change.
func ChangedAspects(before, after Ctx) []Aspect {
	var out []Aspect
	if before.Theme != after.Theme {
		out = append(out, AspectTheme)
	}
	if before.Size != after.Size {
		out = append(out, AspectSize)
	}
	if before.Disabled != after.Disabled {
		out = append(out, AspectDisabled)
	}
	if before.Dir != after.Dir {
		out = append(out, AspectDir)
	}
	if before.Motion != after.Motion {
		out = append(out, AspectMotion)
	}
	if before.Locale != after.Locale {
		out = append(out, AspectLocale)
	}
	if before.PopupContainer != after.PopupContainer ||
		before.TargetContainer != after.TargetContainer ||
		!equalMatchWidth(before.PopupMatchWidth, after.PopupMatchWidth) ||
		before.PopupOverflow != after.PopupOverflow {
		out = append(out, AspectPopup)
	}
	if (before.RenderEmpty == nil) != (after.RenderEmpty == nil) ||
		(before.HolderRender == nil) != (after.HolderRender == nil) {
		out = append(out, AspectEmpty)
	}
	if before.Variant != after.Variant {
		out = append(out, AspectVariant)
	}
	return out
}

func equalMatchWidth(a, b *bool) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// Subscribed reports whether the aspect list contains at least one
// of the given aspects. Components call it with the aspects their
// current build actually read.
func Subscribed(changed []Aspect, want ...Aspect) bool {
	for _, c := range changed {
		for _, w := range want {
			if c == w {
				return true
			}
		}
	}
	return false
}

// Normalize fills missing values with the F0-1 defaults so a
// partially built Ctx still behaves like DefaultCtx.
func (c Ctx) Normalize() Ctx {
	if c.Size == "" {
		c.Size = SizeMedium
	}
	if c.Dir == "" {
		c.Dir = DirLTR
	}
	if c.Locale == "" {
		c.Locale = "zh-CN"
	}
	if c.PopupOverflow == "" {
		c.PopupOverflow = OverflowViewport
	}
	if c.Variant == "" {
		c.Variant = VariantOutlined
	}
	if c.RenderEmpty == nil {
		c.RenderEmpty = DefaultRenderEmpty
	}
	if c.HolderRender == nil {
		c.HolderRender = DefaultHolderRender
	}
	if c.Theme.FontSize <= 0 {
		c.Theme = theme.DefaultTokens()
	}
	return c
}
