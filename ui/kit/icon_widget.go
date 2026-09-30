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

// BuildIcon is the sole icon construction entry (WIDGET_MODEL Build).
func BuildIcon(ctx scope.Ctx, props IconProps) *IconInstance {
	in := newIconInstance(ctx, props)
	in.Mount(ctx)
	return in
}

// NewIcon builds with default context (convenience for tests/demos).
func NewIcon(name string) *IconInstance {
	return BuildIcon(scope.DefaultCtx(), DefaultIconProps(name))
}
