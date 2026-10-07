//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package gles

import (
	"github.com/energye/gpui/gpu/hal"
)

// End finishes the compute pass.
func (e *ComputePassEncoder) End() error {
	// Emit end-of-pass timestamp if requested.
	if e.endTimestampIndex != nil {
		e.encoder.emitTimestamp(e.endTimestampQuerySet, e.endTimestampIndex)
	}
	return nil
}

// SetPipeline sets the compute pipeline.
// SetPipeline sets the compute pipeline.
func (e *ComputePassEncoder) SetPipeline(pipeline hal.ComputePipeline) {
	p, ok := pipeline.(*ComputePipeline)
	if !ok {
		return
	}
	e.pipeline = p
	e.encoder.commands = append(e.encoder.commands, acquireUseProgramCommand(
		p.programID))
}

// SetBindGroup sets a bind group.
// SetBindGroup sets a bind group.
func (e *ComputePassEncoder) SetBindGroup(index uint32, group hal.BindGroup, offsets []uint32) {
	bg, ok := group.(*BindGroup)
	if !ok {
		return
	}
	var groupInfos []BindGroupLayoutInfo
	if e.pipeline != nil && e.pipeline.layout != nil {
		groupInfos = e.pipeline.layout.groupInfos
	}
	e.encoder.commands = append(e.encoder.commands, acquireSetBindGroupCommand(
		index, bg, offsets, e.encoder.maxTextureUnits, groupInfos, nil))
}

// Dispatch dispatches compute work.
// Dispatch dispatches compute work.
func (e *ComputePassEncoder) Dispatch(x, y, z uint32) {
	e.encoder.commands = append(e.encoder.commands, &DispatchCommand{
		x: x, y: y, z: z,
	})
}

// DispatchIndirect dispatches compute work with GPU-generated parameters.
// DispatchIndirect dispatches compute work with GPU-generated parameters.
func (e *ComputePassEncoder) DispatchIndirect(buffer hal.Buffer, offset uint64) {
	buf, ok := buffer.(*Buffer)
	if !ok {
		return
	}
	e.encoder.commands = append(e.encoder.commands, &DispatchIndirectCommand{
		buffer: buf,
		offset: offset,
	})
}

// --- GL Command implementations ---

// MemoryBarrierCommand inserts a glMemoryBarrier call.
// Ensures compute shader writes are visible to subsequent draw/dispatch commands.
type MemoryBarrierCommand struct {
	barriers uint32
}
