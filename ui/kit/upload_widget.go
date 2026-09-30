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

// BuildUpload is the sole upload construction entry.
func BuildUpload(ctx scope.Ctx, props UploadProps) *UploadInstance {
	in := newUploadInstance(ctx, props)
	in.Mount(ctx)
	return in
}

// BuildUploadDragger is the drag-type sugar entry.
func BuildUploadDragger(ctx scope.Ctx, props UploadProps) *UploadInstance {
	props.Type = UploadTypeDrag
	return BuildUpload(ctx, props)
}

// UploadSubscribedAspects reports Ctx aspects the upload reads.
func UploadSubscribedAspects() []scope.Aspect {
	return []scope.Aspect{
		scope.AspectTheme,
		scope.AspectLocale,
		scope.AspectDisabled,
	}
}
