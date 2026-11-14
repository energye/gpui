//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build unix

package text

import (
	"os"
	"syscall"
)

// mmapFontFile maps path read-only into the address space (Skia
// SkData::MakeFromFileName / SkStream::MakeFromFile semantics: mmap first,
// plain read only as fallback). The returned release unmaps the pages.
func mmapFontFile(path string) ([]byte, func(), error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	size := st.Size()
	if size <= 0 {
		return nil, nil, syscall.EINVAL
	}
	data, err := syscall.Mmap(int(f.Fd()), 0, int(size), syscall.PROT_READ, syscall.MAP_PRIVATE)
	if err != nil {
		return nil, nil, err
	}
	return data, func() { _ = syscall.Munmap(data) }, nil
}
