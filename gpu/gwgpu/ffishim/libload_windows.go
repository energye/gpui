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

package ffishim

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// LoadLibrary opens a DLL (kernel32 LoadLibraryW; the name gets the .dll
// suffix when it has no extension).
func LoadLibrary(name string) (unsafe.Pointer, error) {
	h, err := windows.LoadLibrary(name)
	if err != nil {
		return nil, err
	}
	return ptrOf(uintptr(h)), nil
}

// GetSymbol resolves a symbol from an opened DLL.
func GetSymbol(handle unsafe.Pointer, name string) (unsafe.Pointer, error) {
	addr, err := windows.GetProcAddress(windows.Handle(uintptr(handle)), name)
	if err != nil {
		return nil, err
	}
	return ptrOf(addr), nil
}

// FreeLibrary closes an opened DLL.
func FreeLibrary(handle unsafe.Pointer) error {
	return windows.FreeLibrary(windows.Handle(uintptr(handle)))
}
