//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !(js && wasm)

package webgpu

import (
	"github.com/energye/gpui/gpu/hal"
	rwgpu "github.com/energye/gpui/gpu/rwgpu"
)

// ShaderModule represents a compiled shader module.
// On the wgpu-native backend, this wraps rwgpu ShaderModule.
type ShaderModule struct {
	r        *rwgpu.ShaderModule
	device   *Device
	released bool
}

// Release destroys the shader module.
func (m *ShaderModule) Release() {
	if m.released {
		return
	}
	m.released = true
	if m.r != nil {
		m.r.Release()
	}
}

// Destroy implements hal.ShaderModule: same as Release.
func (m *ShaderModule) Destroy() { m.Release() }

var _ hal.ShaderModule = (*ShaderModule)(nil)
