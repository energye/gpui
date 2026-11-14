//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !unix && !windows

package text

import "syscall"

// mmapFontFile is not implemented on this platform; fontFileAcquire falls
// back to os.ReadFile with the same per-path sharing semantics.
func mmapFontFile(path string) ([]byte, func(), error) {
	return nil, nil, syscall.ENOSYS
}
