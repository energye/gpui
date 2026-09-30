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

// ComputePassEncoder records compute dispatch commands.
// On the wgpu-native backend, this wraps rwgpu ComputePassEncoder.
type ComputePassEncoder struct {
	r        *rwgpu.ComputePassEncoder
	released bool
}

// SetPipeline sets the active compute pipeline.
// Implements hal.ComputePassEncoder (takes hal.ComputePipeline interface, internal unpack).
func (p *ComputePassEncoder) SetPipeline(pipeline hal.ComputePipeline) {
	wp, ok := pipeline.(*ComputePipeline)
	if !ok || wp == nil || wp.r == nil {
		return
	}
	p.r.SetPipeline(wp.r)
}

// SetBindGroup sets a bind group for the given index.
// Implements hal.ComputePassEncoder (takes hal.BindGroup interface, internal unpack).
func (p *ComputePassEncoder) SetBindGroup(index uint32, group hal.BindGroup, offsets []uint32) {
	wg, ok := group.(*BindGroup)
	if !ok || wg == nil || wg.r == nil {
		return
	}
	p.r.SetBindGroup(index, wg.r, offsets)
}

// Dispatch dispatches compute work.
func (p *ComputePassEncoder) Dispatch(x, y, z uint32) {
	// rwgpu uses DispatchWorkgroups instead of Dispatch.
	p.r.DispatchWorkgroups(x, y, z)
}

// DispatchIndirect dispatches compute work with GPU-generated parameters.
// Implements hal.ComputePassEncoder (takes hal.Buffer interface, internal unpack).
func (p *ComputePassEncoder) DispatchIndirect(buffer hal.Buffer, offset uint64) {
	wb, ok := buffer.(*Buffer)
	if !ok || wb == nil || wb.r == nil {
		return
	}
	p.r.DispatchWorkgroupsIndirect(wb.r, offset)
}

// End ends the compute pass.
func (p *ComputePassEncoder) End() error {
	if p.released {
		return ErrReleased
	}
	p.released = true
	if p.r != nil {
		p.r.End()
		p.r.Release()
		p.r = nil
	}
	return nil
}

var _ hal.ComputePassEncoder = (*ComputePassEncoder)(nil)
