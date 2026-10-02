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
//
// dedup cache: identical repeats are skipped instead of forwarded (GL
// glExecState parity — the GL backend already gates redundant sets at
// Execute; native had no equivalent and paid full FFI per repeat). Only
// EXACT duplicates are skipped, so pixels can't change: same pipeline
// pointer, same group/buffer handles with same offsets, same viewport/
// scissor/stencil/blend values. Explicit unsets (nil group) always forward.
// Fixed small arrays, no per-call alloc; out-of-range indices forward.
type RenderPassEncoder struct {
	r        *rwgpu.RenderPassEncoder
	released bool

	curPipeline *RenderPipeline
	boundGroups [4]boundGroupEntry
	boundVerts  [8]boundVertEntry
	viewport    [6]float32
	viewportSet bool
	scissor     [4]uint32
	scissorSet  bool
	stencilRef  uint32
	stencilSet  bool
	hasBlend    bool
	blend       Color
}

// boundGroupEntry remembers one group slot.
type boundGroupEntry struct {
	set     bool
	group   *BindGroup
	offsets []uint32
}

// boundVertEntry remembers one vertex slot.
type boundVertEntry struct {
	set    bool
	buf    *Buffer
	offset uint64
}

// offsetsEqual compares without allocating (offsets are almost always empty;
// non-empty rare path compares element-wise).
func offsetsEqual(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// SetPipeline sets the active render pipeline.
// Implements hal.RenderPassEncoder (takes hal.RenderPipeline interface, internal unpack).
func (p *RenderPassEncoder) SetPipeline(pipeline hal.RenderPipeline) {
	wp, ok := pipeline.(*RenderPipeline)
	if !ok || wp == nil || wp.r == nil {
		return
	}
	if wp == p.curPipeline {
		return
	}
	p.curPipeline = wp
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
		if index < uint32(len(p.boundGroups)) {
			p.boundGroups[index] = boundGroupEntry{}
		}
		return
	}
	if index < uint32(len(p.boundGroups)) {
		if prev := &p.boundGroups[index]; prev.set && prev.group == wg && offsetsEqual(prev.offsets, offsets) {
			return
		}
		p.r.SetBindGroup(index, wg.r, offsets)
		e := &p.boundGroups[index]
		e.set = true
		e.group = wg
		if len(offsets) == 0 {
			e.offsets = nil
		} else {
			e.offsets = append(e.offsets[:0], offsets...)
		}
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
	if slot < uint32(len(p.boundVerts)) {
		if prev := &p.boundVerts[slot]; prev.set && prev.buf == wb && prev.offset == offset {
			return
		}
	}
	// rwgpu takes (slot, buffer, offset, size). Pass MaxUint64 for "rest of buffer".
	p.r.SetVertexBuffer(slot, wb.r, offset, math.MaxUint64)
	if slot < uint32(len(p.boundVerts)) {
		p.boundVerts[slot] = boundVertEntry{set: true, buf: wb, offset: offset}
	}
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
	v := [6]float32{x, y, width, height, minDepth, maxDepth}
	if p.viewportSet && p.viewport == v {
		return
	}
	p.viewport, p.viewportSet = v, true
	p.r.SetViewport(x, y, width, height, minDepth, maxDepth)
}

// SetScissorRect sets the scissor rectangle for clipping.
func (p *RenderPassEncoder) SetScissorRect(x, y, width, height uint32) {
	v := [4]uint32{x, y, width, height}
	if p.scissorSet && p.scissor == v {
		return
	}
	p.scissor, p.scissorSet = v, true
	p.r.SetScissorRect(x, y, width, height)
}

// SetBlendConstant sets the blend constant color.
func (p *RenderPassEncoder) SetBlendConstant(color *Color) {
	if color == nil {
		return
	}
	if p.hasBlend && p.blend == *color {
		return
	}
	p.blend, p.hasBlend = *color, true
	p.r.SetBlendConstant(&rwgpu.Color{
		R: color.R,
		G: color.G,
		B: color.B,
		A: color.A,
	})
}

// SetStencilReference sets the stencil reference value.
func (p *RenderPassEncoder) SetStencilReference(reference uint32) {
	if p.stencilSet && p.stencilRef == reference {
		return
	}
	p.stencilRef, p.stencilSet = reference, true
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
