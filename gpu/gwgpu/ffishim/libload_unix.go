//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build linux || darwin

package ffishim

import (
	"unsafe"

	purego "github.com/ebitengine/purego"
)

// LoadLibrary opens a shared library (RTLD_NOW|RTLD_GLOBAL, same as rwgpu).
func LoadLibrary(name string) (unsafe.Pointer, error) {
	h, err := purego.Dlopen(name, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return nil, err
	}
	return ptrOf(h), nil
}

// GetSymbol resolves a symbol from an opened library.
func GetSymbol(handle unsafe.Pointer, name string) (unsafe.Pointer, error) {
	ptr, err := purego.Dlsym(uintptr(handle), name)
	if err != nil {
		return nil, err
	}
	return ptrOf(ptr), nil
}

// FreeLibrary closes an opened library.
func FreeLibrary(handle unsafe.Pointer) error {
	return purego.Dlclose(uintptr(handle))
}
