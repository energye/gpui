//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build (windows || linux) && !(js && wasm)

package gles

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
	"github.com/energye/gpui/gpu/gwgpu/naga"
	"github.com/energye/gpui/gpu/gwgpu/naga/glsl"
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// shaderCache memoizes WGSL→GLSL translation.
// Keyed by source + entry point + GLSL version + binding map: the same WGSL
// under a different driver version or layout must not share output, since
// layout(binding=N) emission is version-gated (SupportsExplicitLocations).
// Entries are immutable: store and return deep copies of TranslationInfo.
var shaderCache = struct {
	sync.RWMutex
	entries map[string]shaderCacheEntry
	hits    atomic.Uint64
	misses  atomic.Uint64
}{entries: make(map[string]shaderCacheEntry)}

type shaderCacheEntry struct {
	glsl string
	info glsl.TranslationInfo
}

// shaderCacheKey builds the cache key. bindingMap is canonicalized
// (sorted group:binding=slot) so equal layouts hit regardless of map order.
func shaderCacheKey(version glsl.Version, wgsl, entryPoint string, bindingMap map[glsl.BindingMapKey]uint8) string {
	var sb strings.Builder
	sb.WriteString(version.String())
	sb.WriteByte(0)
	sb.WriteString(entryPoint)
	sb.WriteByte(0)
	if len(bindingMap) > 0 {
		keys := make([]glsl.BindingMapKey, 0, len(bindingMap))
		for k := range bindingMap {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].Group != keys[j].Group {
				return keys[i].Group < keys[j].Group
			}
			return keys[i].Binding < keys[j].Binding
		})
		for _, k := range keys {
			fmt.Fprintf(&sb, "%d:%d=%d;", k.Group, k.Binding, bindingMap[k])
		}
	}
	sb.WriteByte(0)
	sb.WriteString(wgsl)
	return sb.String()
}

// cloneTranslationInfo deep-copies TranslationInfo (maps, slices,
// and SamplerBinding pointers) so cache entries stay immutable.
func cloneTranslationInfo(src glsl.TranslationInfo) glsl.TranslationInfo {
	dst := glsl.TranslationInfo{
		RequiredVersion: src.RequiredVersion,
	}
	if src.EntryPointNames != nil {
		dst.EntryPointNames = make(map[string]string, len(src.EntryPointNames))
		for k, v := range src.EntryPointNames {
			dst.EntryPointNames[k] = v
		}
	}
	if src.UsedExtensions != nil {
		dst.UsedExtensions = append([]string(nil), src.UsedExtensions...)
	}
	if src.TextureSamplerPairs != nil {
		dst.TextureSamplerPairs = append([]string(nil), src.TextureSamplerPairs...)
	}
	if src.TextureMappings != nil {
		dst.TextureMappings = make(map[string]glsl.TextureMapping, len(src.TextureMappings))
		for k, v := range src.TextureMappings {
			cp := v
			if v.SamplerBinding != nil {
				sb := *v.SamplerBinding
				cp.SamplerBinding = &sb
			}
			dst.TextureMappings[k] = cp
		}
	}
	if src.Uniforms != nil {
		dst.Uniforms = append([]glsl.UniformInfo(nil), src.Uniforms...)
	}
	return dst
}

// shaderCacheStats reports hits, misses, and entry count (tests/observability).
func shaderCacheStats() (hits, misses uint64, size int) {
	shaderCache.RLock()
	defer shaderCache.RUnlock()
	return shaderCache.hits.Load(), shaderCache.misses.Load(), len(shaderCache.entries)
}

// clearShaderCache empties the cache (tests only).
func clearShaderCache() {
	shaderCache.Lock()
	defer shaderCache.Unlock()
	shaderCache.entries = make(map[string]shaderCacheEntry)
	shaderCache.hits.Store(0)
	shaderCache.misses.Store(0)
}

// compileWGSLToGLSL compiles a WGSL shader source to GLSL for the given entry point.
//
// The version parameter specifies the target GLSL version. On GL 4.3+ this is typically
// Version430; on older drivers (e.g., GL 4.1 / GLSL 410) it must match the driver's
// reported GLSL version.
//
// The bindingMap parameter provides the pre-computed (group, binding) -> GL slot mapping
// from PipelineLayout (computed via per-type sequential counters in CreatePipelineLayout).
// If bindingMap is nil, no binding remapping is applied.
//
// Returns the GLSL source and TranslationInfo containing TextureMappings for
// SamplerBindMap construction (which sampler goes with which texture unit).
func compileWGSLToGLSL(version glsl.Version, wgsl string, entryPoint string, bindingMap map[glsl.BindingMapKey]uint8) (string, glsl.TranslationInfo, error) {
	if wgsl == "" {
		return "", glsl.TranslationInfo{}, fmt.Errorf("gles: shader source has no WGSL code")
	}

	// Cache hit: same source + entry + version + layout returns identical output.
	key := shaderCacheKey(version, wgsl, entryPoint, bindingMap)
	shaderCache.RLock()
	if e, ok := shaderCache.entries[key]; ok {
		shaderCache.RUnlock()
		shaderCache.hits.Add(1)
		return e.glsl, cloneTranslationInfo(e.info), nil
	}
	shaderCache.RUnlock()

	// Parse WGSL to AST.
	ast, err := naga.Parse(wgsl)
	if err != nil {
		return "", glsl.TranslationInfo{}, fmt.Errorf("gles: WGSL parse error: %w", err)
	}

	// Lower AST to IR.
	module, err := naga.Lower(ast)
	if err != nil {
		return "", glsl.TranslationInfo{}, fmt.Errorf("gles: WGSL lower error: %w", err)
	}

	// Compile IR to the target GLSL version.
	// On GL 4.3+ this emits layout(binding=N) qualifiers inline. On older versions
	glslCode, translationInfo, err := glsl.Compile(module, glsl.Options{
		LangVersion:        version,
		EntryPoint:         entryPoint,
		ForceHighPrecision: true,
		BindingMap:         bindingMap,
		// This flips Y and remaps Z from [0,1] to [-1,1].
		// The scene renders upside-down inside the Surface's swapchain offscreen FBO
		// (hal/gles/surface.go). Queue.Present performs an explicit Y-flipping
		// The flip also fixes gl_FragCoord.y convention in fragment shaders: with
		// the flip, gl_FragCoord.y=0 is at the top, not bottom
		// (GL convention). Without it, rrect_clip_coverage() in fragment shaders
		// gets wrong Y values.
		WriterFlags: glsl.WriterFlagAdjustCoordinateSpace | glsl.WriterFlagForcePointSize,
	})
	if err != nil {
		return "", glsl.TranslationInfo{}, fmt.Errorf("gles: GLSL compile error for entry point %q: %w", entryPoint, err)
	}

	hal.Logger().Debug("gles: GLSL generated",
		"entryPoint", entryPoint,
		"sourceLen", len(glslCode),
	)
	if hal.Logger().Enabled(context.Background(), slog.LevelDebug) {
		preview := glslCode
		if len(preview) > 2000 {
			preview = preview[:2000] + "..."
		}
		hal.Logger().Debug("gles: GLSL source", "glsl", preview)
	}

	// Cache miss: store a deep copy for identical future compilations.
	shaderCache.Lock()
	shaderCache.entries[key] = shaderCacheEntry{glsl: glslCode, info: cloneTranslationInfo(translationInfo)}
	shaderCache.Unlock()
	shaderCache.misses.Add(1)

	return glslCode, translationInfo, nil
}

// assignBindingsAfterLink assigns uniform block and sampler bindings at runtime
// after glLinkProgram on GL < 4.2 where layout(binding=N) is unavailable.
//
// The translationInfos parameter contains reflection data from all shader stages
// (vertex + fragment, or compute). The layout provides the binding map for
// resolving (group, binding) to flat GL slot indices.
func assignBindingsAfterLink(glCtx *gl.Context, program uint32, layout *PipelineLayout, translationInfos ...glsl.TranslationInfo) error {
	glCtx.UseProgram(program)

	// Assign uniform/storage buffer block bindings by name.
	for _, info := range translationInfos {
		for _, u := range info.Uniforms {
			key := glsl.BindingMapKey{Group: u.Binding.Group, Binding: u.Binding.Binding}
			slot, ok := layout.bindingMap[key]
			if !ok {
				continue
			}
			if u.IsStorage {
				// Storage buffers cannot be remapped without layout(binding) qualifiers.
				hal.Logger().Error("gles: cannot remap storage buffer binding on GL < 4.2",
					"blockName", u.BlockName,
					"group", u.Binding.Group,
					"binding", u.Binding.Binding,
				)
				return fmt.Errorf("gles: storage buffers require GL 4.3+ (layout(binding) support)")
			}
			index := glCtx.GetUniformBlockIndex(program, u.BlockName)
			if index == 0xFFFFFFFF { // GL_INVALID_INDEX
				hal.Logger().Debug("gles: uniform block not found (may be optimized out)",
					"blockName", u.BlockName)
				continue
			}
			glCtx.UniformBlockBinding(program, index, uint32(slot))
			hal.Logger().Debug("gles: assigned uniform block binding",
				"blockName", u.BlockName,
				"blockIndex", index,
				"slot", slot,
			)
		}

		// Assign texture/image sampler bindings by combined variable name.
		for name, tm := range info.TextureMappings {
			key := glsl.BindingMapKey{Group: tm.TextureBinding.Group, Binding: tm.TextureBinding.Binding}
			slot, ok := layout.bindingMap[key]
			if !ok {
				continue
			}
			location := glCtx.GetUniformLocation(program, name)
			if location < 0 {
				hal.Logger().Debug("gles: texture uniform not found (may be optimized out)",
					"name", name)
				continue
			}
			glCtx.Uniform1i(location, int32(slot))
			hal.Logger().Debug("gles: assigned texture uniform binding",
				"name", name,
				"location", location,
				"slot", slot,
			)
		}
	}

	return nil
}

// computeBindingMap computes per-type sequential binding indices for all bind group
// layouts in a pipeline layout.
func computeBindingMap(layouts []*BindGroupLayout) (map[glsl.BindingMapKey]uint8, []BindGroupLayoutInfo) {
	var (
		numSamplers       uint8
		numTextures       uint8
		numImages         uint8
		numUniformBuffers uint8
		numStorageBuffers uint8
	)

	bindingMap := make(map[glsl.BindingMapKey]uint8)
	groupInfos := make([]BindGroupLayoutInfo, len(layouts))

	for groupIdx, bgl := range layouts {
		if bgl == nil {
			continue
		}
		entries := bgl.entries

		// Find max binding number to size the BindingToSlot table.
		maxBinding := uint32(0)
		for _, entry := range entries {
			if entry.Binding > maxBinding {
				maxBinding = entry.Binding
			}
		}

		bindingToSlot := make([]uint8, maxBinding+1)
		for i := range bindingToSlot {
			bindingToSlot[i] = 0xFF // unused
		}

		for _, entry := range entries {
			var counter *uint8
			switch classifyBindGroupEntry(entry) {
			case bindingClassSampler:
				counter = &numSamplers
			case bindingClassTexture:
				counter = &numTextures
			case bindingClassImage:
				counter = &numImages
			case bindingClassUniformBuffer:
				counter = &numUniformBuffers
			case bindingClassStorageBuffer:
				counter = &numStorageBuffers
			default:
				continue
			}

			slot := *counter
			bindingToSlot[entry.Binding] = slot
			bindingMap[glsl.BindingMapKey{
				Group:   uint32(groupIdx),
				Binding: entry.Binding,
			}] = slot
			*counter++
		}

		groupInfos[groupIdx] = BindGroupLayoutInfo{BindingToSlot: bindingToSlot}
	}

	return bindingMap, groupInfos
}

// bindingClass represents the GL resource type for a binding entry.
type bindingClass uint8

const (
	bindingClassUnknown       bindingClass = iota
	bindingClassSampler                    // GL sampler objects
	bindingClassTexture                    // GL texture units (sampled textures)
	bindingClassImage                      // GL image units (storage textures)
	bindingClassUniformBuffer              // GL uniform buffer binding points
	bindingClassStorageBuffer              // GL shader storage buffer binding points
)

// classifyBindGroupEntry determines the GL resource type for a bind group layout entry.
func classifyBindGroupEntry(entry gputypes.BindGroupLayoutEntry) bindingClass {
	switch {
	case entry.Sampler != nil:
		return bindingClassSampler
	case entry.Texture != nil:
		return bindingClassTexture
	case entry.StorageTexture != nil:
		return bindingClassImage
	case entry.Buffer != nil:
		switch entry.Buffer.Type {
		case gputypes.BufferBindingTypeUniform:
			return bindingClassUniformBuffer
		case gputypes.BufferBindingTypeStorage, gputypes.BufferBindingTypeReadOnlyStorage:
			return bindingClassStorageBuffer
		default:
			// Default buffer type treated as uniform buffer.
			return bindingClassUniformBuffer
		}
	default:
		return bindingClassUnknown
	}
}
