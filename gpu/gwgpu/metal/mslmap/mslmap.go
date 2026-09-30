//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// attribute only if msl.Options.PerEntryPointMap[ep].SizesBuffer is set. If it
// is not set, Metal binds nothing to the argument and arrayLength() returns a
// wrong value. The backend must therefore supply the map.
//
// The map must list every resource, not only the sizes buffer.
// does not number the other resources itself. A map with only SizesBuffer would
// therefore lose every normal binding. This package copies the own numbering
// rule, so the MSL for normal resources stays the same.
//
// The package has no build tags and does not call Objective-C, so its tests run
// on any CI runner.
package mslmap

import (
	"fmt"
	"sort"

	"github.com/energye/gpui/gpu/gwgpu/naga/ir"
	"github.com/energye/gpui/gpu/gwgpu/naga/msl"
)

// msl.BindTarget stores the slot in a uint8. Real Metal devices allow only 31
// buffers, so a value above this limit means the caller made a mistake.
const maxBufferSlot = 255

// Options returns MSL options that put the `_buffer_sizes` argument at
// buffer slot sizesSlot. It does this for every entry point in module, and it
//
// sizesSlot is the number of buffer bindings that the pipeline layout declares.
// That is the first free slot, directly after the last declared buffer.
func Options(module *ir.Module, sizesSlot int) (msl.Options, error) {
	opts := msl.DefaultOptions()
	if module == nil {
		return opts, fmt.Errorf("mslmap: nil IR module")
	}
	if sizesSlot < 0 || sizesSlot > maxBufferSlot {
		return opts, fmt.Errorf("mslmap: buffer sizes slot %d out of range 0..%d", sizesSlot, maxBufferSlot)
	}

	resources, err := autoResourceMap(module)
	if err != nil {
		return opts, err
	}

	slot := uint8(sizesSlot)
	perEP := make(map[string]msl.EntryPointResources, len(module.EntryPoints))
	for i := range module.EntryPoints {
		// The key is the WGSL entry point name, which is the IR name.
		perEP[module.EntryPoints[i].Name] = msl.EntryPointResources{
			Resources:   resources,
			SizesBuffer: &slot,
		}
	}
	opts.PerEntryPointMap = perEP

	return opts, nil
}

// autoResourceMap copies the numbering rule of the computeResourceMap. It
// takes every global variable that has a binding, sorts them by group and then
// by binding, and gives each one an index. Samplers, textures and buffers are
// counted apart, and each kind starts at zero.
//
// The encoder already binds bind group entries in this same order, so the
// current code depends on this rule. Because the map comes from the IR, the MSL
// for these resources does not change.
func autoResourceMap(module *ir.Module) (map[ir.ResourceBinding]msl.BindTarget, error) {
	type entry struct {
		binding ir.ResourceBinding
		kind    int // 0=buffer, 1=texture, 2=sampler
	}

	entries := make([]entry, 0, len(module.GlobalVariables))
	for _, global := range module.GlobalVariables {
		if global.Binding == nil {
			continue
		}
		if int(global.Type) >= len(module.Types) {
			continue
		}

		kind := 0
		switch module.Types[global.Type].Inner.(type) {
		case ir.SamplerType:
			kind = 2
		case ir.ImageType:
			kind = 1
		}

		entries = append(entries, entry{binding: *global.Binding, kind: kind})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].binding.Group != entries[j].binding.Group {
			return entries[i].binding.Group < entries[j].binding.Group
		}
		return entries[i].binding.Binding < entries[j].binding.Binding
	})

	resources := make(map[ir.ResourceBinding]msl.BindTarget, len(entries))
	var nextBuffer, nextTexture, nextSampler uint8
	for _, e := range entries {
		var next *uint8
		switch e.kind {
		case 1:
			next = &nextTexture
		case 2:
			next = &nextSampler
		default:
			next = &nextBuffer
		}
		idx := *next
		if idx == maxBufferSlot {
			// The index is a uint8, so the next resource of this
			// kind would have no valid slot.
			return nil, fmt.Errorf("mslmap: more than %d resources of one kind", maxBufferSlot)
		}
		*next = idx + 1

		var target msl.BindTarget
		switch e.kind {
		case 1:
			target.Texture = &idx
		case 2:
			target.Sampler = &msl.BindSamplerTarget{Slot: idx}
		default:
			target.Buffer = &idx
		}
		resources[e.binding] = target
	}

	return resources, nil
}

// RuntimeArrayBindings returns the bindings of the global variables whose type
// holds a runtime-sized array. The order is the IR declaration order.
//
// Each field
// must hold the byte size of the buffer that is bound to it.
func RuntimeArrayBindings(module *ir.Module) ([]ir.ResourceBinding, error) {
	if module == nil {
		return nil, nil
	}
	var out []ir.ResourceBinding
	for i, global := range module.GlobalVariables {
		if !typeNeedsArrayLength(module, global.Type) {
			continue
		}
		if global.Binding == nil {
			// Such a global has a field in the sizes struct, but no
			// size that the encoder can find. Skipping it would
			// shift all later fields. WGSL cannot declare this
			// today, so report an error instead of writing the
			// sizes to the wrong fields.
			return nil, fmt.Errorf("mslmap: global %d has a runtime-sized array but no binding", i)
		}
		out = append(out, *global.Binding)
	}
	return out, nil
}

// typeNeedsArrayLength reports whether the type is a runtime-sized array, or a
// struct whose last member ends in one.
func typeNeedsArrayLength(module *ir.Module, handle ir.TypeHandle) bool {
	if int(handle) >= len(module.Types) {
		return false
	}
	switch inner := module.Types[handle].Inner.(type) {
	case ir.ArrayType:
		return inner.Size.Constant == nil
	case ir.StructType:
		if len(inner.Members) == 0 {
			return false
		}
		return typeNeedsArrayLength(module, inner.Members[len(inner.Members)-1].Type)
	default:
		return false
	}
}
