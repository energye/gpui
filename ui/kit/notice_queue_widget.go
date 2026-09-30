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

// BuildNoticeQueue is the sole queue construction entry.
func BuildNoticeQueue(ctx scope.Ctx, props NoticeQueueProps) *NoticeQueueInstance {
	in := newNoticeQueueInstance(ctx, props)
	in.Mount(ctx)
	return in
}

// NoticeQueueSubscribedAspects reports Ctx aspects the queue reads.
func NoticeQueueSubscribedAspects() []scope.Aspect {
	return []scope.Aspect{
		scope.AspectTheme,
		scope.AspectLocale,
		scope.AspectPopup,
		scope.AspectEmpty,
		scope.AspectMotion,
	}
}
