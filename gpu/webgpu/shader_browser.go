//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build js && wasm

package webgpu

import "github.com/energye/gpui/gpu/webgpu/internal/browser"

// ShaderModule represents a compiled shader module.
type ShaderModule struct {
	browser  *browser.ShaderModule
	released bool
}

// Release destroys the shader module.
func (m *ShaderModule) Release() {
	if m.released {
		return
	}
	m.released = true
}
