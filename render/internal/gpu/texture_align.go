//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !nogpu

package gpu

// copyPitchAlignment is the WebGPU minimum bytesPerRow alignment for texture
// copies and queue.WriteTexture when more than one row is written.
const copyPitchAlignment = 256

func alignTextureBytesPerRow(bytesPerRow uint32) uint32 {
	return (bytesPerRow + copyPitchAlignment - 1) &^ (copyPitchAlignment - 1)
}
