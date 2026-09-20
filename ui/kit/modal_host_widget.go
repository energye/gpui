package kit

import (
	"github.com/energye/gpui/ui/kit/internal/scope"
)

// BuildModalHost is the sole host construction entry (Props/State/Render/Theme glue).
func BuildModalHost(ctx scope.Ctx, props ModalHostProps) *ModalHostInstance {
	in := newModalHostInstance(ctx, props)
	in.Mount(ctx)
	return in
}

// ModalHostSubscribedAspects reports Ctx aspects the host reads.
func ModalHostSubscribedAspects() []scope.Aspect {
	return []scope.Aspect{
		scope.AspectTheme,
		scope.AspectLocale,
		scope.AspectPopup,
		scope.AspectEmpty,
	}
}
