//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

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
