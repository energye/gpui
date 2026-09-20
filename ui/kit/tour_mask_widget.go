package kit

import (
	"github.com/energye/gpui/ui/kit/internal/scope"
)

// BuildTourMask is the sole tour construction entry.
func BuildTourMask(ctx scope.Ctx, props TourMaskProps, steps []TourMaskStep) *TourMaskInstance {
	in := newTourMaskInstance(ctx, props, steps)
	in.Mount(ctx)
	return in
}

// TourMaskSubscribedAspects reports Ctx aspects the tour reads.
func TourMaskSubscribedAspects() []scope.Aspect {
	return []scope.Aspect{
		scope.AspectTheme,
		scope.AspectLocale,
		scope.AspectPopup,
		scope.AspectEmpty,
		scope.AspectMotion,
	}
}
