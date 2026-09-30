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
	"math"

	"github.com/energye/gpui/gpu/hal"
	rwgpu "github.com/energye/gpui/gpu/rwgpu"
)

// RenderPassEncoder records draw commands within a render pass.
// On the wgpu-native backend, this wraps rwgpu RenderPassEncoder.
type RenderPassEncoder struct {
	r        *rwgpu.RenderPassEncoder
	released bool
}

// SetPipeline sets the active render pipeline.
// Implements hal.RenderPassEncoder (takes hal.RenderPipeline interface, internal unpack).
func (p *RenderPassEncoder) SetPipeline(pipeline hal.RenderPipeline) {
	wp, ok := pipeline.(*RenderPipeline)
	if !ok || wp == nil || wp.r == nil {
		return
	}
	p.r.SetPipeline(wp.r)
}

// SetBindGroup sets a bind group for the given index.
// Passing group == nil unsets the bind group (required before switching to a
// pipeline with an incompatible bind-group layout in the same render pass).
// Implements hal.RenderPassEncoder (takes hal.BindGroup interface, internal unpack).
func (p *RenderPassEncoder) SetBindGroup(index uint32, group hal.BindGroup, offsets []uint32) {
	if p == nil || p.r == nil {
		return
	}
	if group == nil {
		p.r.SetBindGroup(index, nil, nil)
		return
	}
	wg, ok := group.(*BindGroup)
	if !ok {
		return
	}
	if wg == nil || wg.r == nil {
		p.r.SetBindGroup(index, nil, nil)
		return
	}
	p.r.SetBindGroup(index, wg.r, offsets)
}

// SetVertexBuffer sets a vertex buffer for the given slot.
// Offset is in bytes.
// Implements hal.RenderPassEncoder (takes hal.Buffer interface, internal unpack).
func (p *RenderPassEncoder) SetVertexBuffer(slot uint32, buffer hal.Buffer, offset uint64) {
	wb, ok := buffer.(*Buffer)
	if !ok || wb == nil || wb.r == nil {
		return
	}
	// rwgpu takes (slot, buffer, offset, size). Pass MaxUint64 for "rest of buffer".
	p.r.SetVertexBuffer(slot, wb.r, offset, math.MaxUint64)
}

// SetIndexBuffer sets the index buffer.
// Implements hal.RenderPassEncoder (takes hal.Buffer interface, internal unpack).
func (p *RenderPassEncoder) SetIndexBuffer(buffer hal.Buffer, format IndexFormat, offset uint64) {
	wb, ok := buffer.(*Buffer)
	if !ok || wb == nil || wb.r == nil {
		return
	}
	// rwgpu takes (buffer, format, offset, size). Pass MaxUint64 for "rest of buffer".
	p.r.SetIndexBuffer(wb.r, format, offset, math.MaxUint64)
}

// SetViewport sets the viewport transformation.
func (p *RenderPassEncoder) SetViewport(x, y, width, height, minDepth, maxDepth float32) {
	p.r.SetViewport(x, y, width, height, minDepth, maxDepth)
}

// SetScissorRect sets the scissor rectangle for clipping.
func (p *RenderPassEncoder) SetScissorRect(x, y, width, height uint32) {
	p.r.SetScissorRect(x, y, width, height)
}

// SetBlendConstant sets the blend constant color.
func (p *RenderPassEncoder) SetBlendConstant(color *Color) {
	if color == nil {
		return
	}
	p.r.SetBlendConstant(&rwgpu.Color{
		R: color.R,
		G: color.G,
		B: color.B,
		A: color.A,
	})
}

// SetStencilReference sets the stencil reference value.
func (p *RenderPassEncoder) SetStencilReference(reference uint32) {
	p.r.SetStencilReference(reference)
}

// Draw draws primitives.
func (p *RenderPassEncoder) Draw(vertexCount, instanceCount, firstVertex, firstInstance uint32) {
	p.r.Draw(vertexCount, instanceCount, firstVertex, firstInstance)
}

// DrawIndexed draws indexed primitives.
func (p *RenderPassEncoder) DrawIndexed(indexCount, instanceCount, firstIndex uint32, baseVertex int32, firstInstance uint32) {
	p.r.DrawIndexed(indexCount, instanceCount, firstIndex, baseVertex, firstInstance)
}

// DrawIndirect draws primitives with GPU-generated parameters.
// Implements hal.RenderPassEncoder (takes hal.Buffer interface, internal unpack).
func (p *RenderPassEncoder) DrawIndirect(buffer hal.Buffer, offset uint64) {
	wb, ok := buffer.(*Buffer)
	if !ok || wb == nil || wb.r == nil {
		return
	}
	p.r.DrawIndirect(wb.r, offset)
}

// DrawIndexedIndirect draws indexed primitives with GPU-generated parameters.
// Implements hal.RenderPassEncoder (takes hal.Buffer interface, internal unpack).
func (p *RenderPassEncoder) DrawIndexedIndirect(buffer hal.Buffer, offset uint64) {
	wb, ok := buffer.(*Buffer)
	if !ok || wb == nil || wb.r == nil {
		return
	}
	p.r.DrawIndexedIndirect(wb.r, offset)
}

// End ends the render pass and drops the native pass-encoder reference.
// wgpu requires wgpuRenderPassEncoderRelease after End; without it the
// encoder handle (and associated device memory) leaks.
func (p *RenderPassEncoder) End() error {
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

func (p *RenderPassEncoder) DrawIndirectCount(buffer hal.Buffer, offset uint64, countBuffer hal.Buffer, countOffset uint64, maxDrawCount uint32) {
	hal.RecordIndirectCountMax(p.DrawIndirect, buffer, offset, countBuffer, countOffset, maxDrawCount)
}

// DrawIndexedIndirectCount implements hal.RenderPassEncoder: lowers to max.
func (p *RenderPassEncoder) DrawIndexedIndirectCount(buffer hal.Buffer, offset uint64, countBuffer hal.Buffer, countOffset uint64, maxDrawCount uint32) {
	hal.RecordIndirectCountMax(p.DrawIndexedIndirect, buffer, offset, countBuffer, countOffset, maxDrawCount)
}

func (p *RenderPassEncoder) ExecuteBundle(_ hal.RenderBundle) {}

var _ hal.RenderPassEncoder = (*RenderPassEncoder)(nil)
