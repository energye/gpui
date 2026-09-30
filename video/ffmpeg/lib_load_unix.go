//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !windows

package ffmpeg

import (
	"github.com/ebitengine/purego"
)

// openLib 打开动态库（Unix：dlopen，RTLD_NOW|RTLD_GLOBAL）。
func openLib(path string) (uintptr, error) {
	return purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
}

// symAddr 取数据符号地址（Unix：dlsym）。
func symAddr(handle uintptr, name string) (uintptr, error) {
	return purego.Dlsym(handle, name)
}
