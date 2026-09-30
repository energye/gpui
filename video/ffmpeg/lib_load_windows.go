//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build windows

package ffmpeg

import (
	"golang.org/x/sys/windows"
)

// openLib 打开动态库（Windows：LoadLibrary，purego.RegisterLibFunc
// 在 Windows 下走 GetProcAddress，所以句柄直接可用）。
func openLib(path string) (uintptr, error) {
	h, err := windows.LoadLibrary(path)
	if err != nil {
		return 0, err
	}
	return uintptr(h), nil
}

// symAddr 取数据符号地址（Windows：GetProcAddress）。
func symAddr(handle uintptr, name string) (uintptr, error) {
	return windows.GetProcAddress(windows.Handle(handle), name)
}
