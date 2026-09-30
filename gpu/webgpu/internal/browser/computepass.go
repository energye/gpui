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

package browser

import "syscall/js"

// ComputePassEncoder wraps a browser GPUComputePassEncoder with pre-bound methods.
//
// Pre-binding JS methods at construction time avoids repeated property lookups
// during compute dispatch.
type ComputePassEncoder struct {
	// ref_ is the GPUComputePassEncoder JavaScript object.
	ref_ js.Value

	// Pre-bound methods for compute dispatch.
	fnSetPipeline        js.Value
	fnSetBindGroup       js.Value
	fnDispatchWorkgroups js.Value
	fnDispatchIndirect   js.Value
	fnEnd                js.Value
}

// NewComputePassEncoder constructs a ComputePassEncoder from a
// GPUComputePassEncoder js.Value. Pre-binds all dispatch and state methods.
func NewComputePassEncoder(ref js.Value) *ComputePassEncoder {
	return &ComputePassEncoder{
		ref_:                 ref,
		fnSetPipeline:        bindMethod(ref, "setPipeline"),
		fnSetBindGroup:       bindMethod(ref, "setBindGroup"),
		fnDispatchWorkgroups: bindMethod(ref, "dispatchWorkgroups"),
		fnDispatchIndirect:   bindMethod(ref, "dispatchWorkgroupsIndirect"),
		fnEnd:                bindMethod(ref, "end"),
	}
}

// SetPipeline sets the active compute pipeline.
func (p *ComputePassEncoder) SetPipeline(pipeline js.Value) {
	p.fnSetPipeline.Invoke(pipeline)
}

// SetBindGroup sets a bind group at the given index.
func (p *ComputePassEncoder) SetBindGroup(index uint32, group js.Value, dynamicOffsets []uint32) {
	if len(dynamicOffsets) == 0 {
		p.fnSetBindGroup.Invoke(index, group)
		return
	}
	jsArray := js.Global().Get("Uint32Array").New(len(dynamicOffsets))
	for i, offset := range dynamicOffsets {
		jsArray.SetIndex(i, js.ValueOf(offset))
	}
	p.fnSetBindGroup.Invoke(index, group, jsArray, 0, len(dynamicOffsets))
}

// DispatchWorkgroups dispatches compute work.
func (p *ComputePassEncoder) DispatchWorkgroups(x, y, z uint32) {
	p.fnDispatchWorkgroups.Invoke(x, y, z)
}

// DispatchIndirect dispatches compute work with GPU-generated parameters
// from an indirect buffer.
func (p *ComputePassEncoder) DispatchIndirect(buffer js.Value, offset uint64) {
	p.fnDispatchIndirect.Invoke(buffer, float64(offset))
}

// End ends the compute pass.
func (p *ComputePassEncoder) End() {
	p.fnEnd.Invoke()
}

// Ref returns the underlying GPUComputePassEncoder js.Value.
func (p *ComputePassEncoder) Ref() js.Value {
	return p.ref_
}
