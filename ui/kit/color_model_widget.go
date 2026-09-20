package kit

import (
	"github.com/energye/gpui/ui/kit/internal/scope"
)

// BuildColorModel is the sole color construction entry.
func BuildColorModel(ctx scope.Ctx, props ColorModelProps) *ColorModelInstance {
	in := newColorModelInstance(ctx, props)
	in.Mount(ctx)
	return in
}

// ColorModelSubscribedAspects reports Ctx aspects the model reads.
func ColorModelSubscribedAspects() []scope.Aspect {
	return []scope.Aspect{
		scope.AspectTheme,
		scope.AspectLocale,
		scope.AspectDisabled,
	}
}
