//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"github.com/energye/gpui/gpu/hal"
)

func vramTestReset() {
	hal.VramTestReset()
}

func vramTestAdd(handle uintptr, need uint64) {
	hal.VramTestAdd(handle, need)
}
