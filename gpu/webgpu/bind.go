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

// BindGroupLayout defines the structure of resource bindings for shaders.
// On the wgpu-native backend, this wraps rwgpu BindGroupLayout.
type BindGroupLayout struct {
	r        *rwgpu.BindGroupLayout
	device   *Device
	released bool
}

// Release destroys the bind group layout.
func (l *BindGroupLayout) Release() {
	if l.released {
		return
	}
	l.released = true
	if l.r != nil {
		l.r.Release()
	}
}

// Destroy implements hal.BindGroupLayout: same as Release.
func (l *BindGroupLayout) Destroy() { l.Release() }

// PipelineLayout defines the bind group layout arrangement for a pipeline.
// On the wgpu-native backend, this wraps rwgpu PipelineLayout.
type PipelineLayout struct {
	r        *rwgpu.PipelineLayout
	device   *Device
	released bool
}

// Release destroys the pipeline layout.
func (l *PipelineLayout) Release() {
	if l.released {
		return
	}
	l.released = true
	if l.r != nil {
		l.r.Release()
	}
}

// Destroy implements hal.PipelineLayout: same as Release.
func (l *PipelineLayout) Destroy() { l.Release() }

// LateBufferBindingInfo records the actual buffer binding size for a layout entry
// with MinBindingSize == 0.
type LateBufferBindingInfo struct {
	BindingIndex uint32
	Size         uint64
}

// BindGroup represents bound GPU resources for shader access.
// On the wgpu-native backend, this wraps rwgpu BindGroup.
type BindGroup struct {
	r        *rwgpu.BindGroup
	device   *Device
	released bool
}

// Release marks the bind group for destruction.
func (g *BindGroup) Release() {
	if g.released {
		return
	}
	g.released = true
	if g.r != nil {
		g.r.Release()
	}
}

// Destroy implements hal.BindGroup: same as Release.
func (g *BindGroup) Destroy() { g.Release() }

var (
	_ hal.BindGroupLayout = (*BindGroupLayout)(nil)
	_ hal.PipelineLayout  = (*PipelineLayout)(nil)
	_ hal.BindGroup       = (*BindGroup)(nil)
)
