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

package text

import (
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// mmapFontFile maps path read-only via CreateFileMapping/MapViewOfFile —
// the Windows counterpart of SkData::MakeFromFileName. The returned release
// unmaps the view and closes both handles.
func mmapFontFile(path string) ([]byte, func(), error) {
	wpath, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, nil, err
	}
	handle, err := windows.CreateFile(wpath, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil,
		windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, nil, err
	}
	st, err := os.Stat(path)
	if err != nil {
		_ = windows.CloseHandle(handle)
		return nil, nil, err
	}
	size := st.Size()
	if size <= 0 {
		_ = windows.CloseHandle(handle)
		return nil, nil, syscall.EINVAL
	}
	mapping, err := windows.CreateFileMapping(handle, nil,
		windows.PAGE_READONLY, uint32(size>>32), uint32(size), nil)
	if err != nil {
		_ = windows.CloseHandle(handle)
		return nil, nil, err
	}
	addr, err := windows.MapViewOfFile(mapping, windows.FILE_MAP_READ, 0, 0, 0)
	if err != nil {
		_ = windows.CloseHandle(mapping)
		_ = windows.CloseHandle(handle)
		return nil, nil, err
	}
	data := unsafe.Slice((*byte)(unsafe.Pointer(addr)), size)
	return data, func() {
		_ = windows.UnmapViewOfFile(addr)
		_ = windows.CloseHandle(mapping)
		_ = windows.CloseHandle(handle)
	}, nil
}
