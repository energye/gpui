//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2024
//
//----------------------------------------

package render

import (
	"encoding/binary"
	"fmt"
	"math"
	"sync"

	"github.com/energye/gpui/gpu/shader"
)

// ShaderEffect is a compiled user shader (P3.2): an application-supplied WGSL
// fragment module that the GPU evaluates per pixel over the painted region —
// the Skia SkRuntimeEffect / Flutter FragmentProgram slot in this engine.
//
// # Contract (capability definition, evidence/P3.2)
//
//   - The module must define `fn fs_main(uv: vec2<f32>) -> vec4<f32>`:
//     uv is the local pixel position over the painted bounds normalized to
//     [0,1] (Flutter FragmentProgram uv semantics); the return is a
//     straight-alpha RGBA color the engine premultiplies.
//   - Optional bindings, both at @group(0): binding(0) a uniform block
//     (≤256 bytes, 16-byte field alignment) fed by SetUniformBytes/
//     SetFloatUniforms, and binding(1)+binding(2) one texture_2d<f32> +
//     sampler pair fed by SetImage.
//   - Compilation happens here, eagerly: Parse → Lower → Validate; a module
//     that fails any step is rejected with a readable error and never
//     reaches a backend driver.
//   - Execution is GPU-only (WGSL has no CPU evaluator): draws with a
//     Shader paint are skipped without counting as a CPU fallback when no
//     accelerator is available.
type ShaderEffect struct {
	mu      sync.Mutex
	wgsl    string
	uniform []byte
	image   *ImageBuf

	closed bool
}

// shaderUniformMax is the uniform budget for one effect (one 256B slot minus
// the engine's own stride overhead stays well inside every backend limit).
const shaderUniformMax = 256

// NewShaderEffect compiles and validates a user WGSL fragment module.
// It returns a readable error for parse, lowering, or validation failures —
// rejection happens here, before any backend sees the source.
func NewShaderEffect(wgsl string) (*ShaderEffect, error) {
	if wgsl == "" {
		return nil, fmt.Errorf("shader: empty WGSL source")
	}
	if err := validateUserShader(wgsl); err != nil {
		return nil, err
	}
	return &ShaderEffect{wgsl: wgsl}, nil
}

// validateUserShader runs the shared translator pipeline (Parse → Lower →
// Validate) — the same front end the gles backend applies at pipeline build
// time, so rejection semantics are identical across backends.
func validateUserShader(wgsl string) error {
	ast, err := shader.Parse(wgsl)
	if err != nil {
		return fmt.Errorf("shader: parse: %w", err)
	}
	mod, err := shader.Lower(ast)
	if err != nil {
		return fmt.Errorf("shader: lower: %w", err)
	}
	if errs, err := shader.Validate(mod); err != nil {
		return fmt.Errorf("shader: validate: %w", err)
	} else if len(errs) > 0 {
		return fmt.Errorf("shader: validate: %d error(s), first: %v", len(errs), errs[0])
	}
	return nil
}

// WGSL returns the module source (diagnostics and recording support).
func (e *ShaderEffect) WGSL() string {
	if e == nil {
		return ""
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.wgsl
}

// SetUniformBytes installs the uniform block contents (≤256 bytes; WGSL
// uniform layout rules apply — 16-byte field alignment). Passing nil clears.
func (e *ShaderEffect) SetUniformBytes(data []byte) error {
	if e == nil {
		return fmt.Errorf("shader: nil effect")
	}
	if len(data) > shaderUniformMax {
		return fmt.Errorf("shader: uniform block %d bytes exceeds %d", len(data), shaderUniformMax)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.uniform = append([]byte(nil), data...)
	return nil
}

// SetFloatUniforms packs floats little-endian into the uniform block — the
// convenient form for f32/vec2/vec3/vec4 arrays (16-byte field alignment is
// still the caller's duty, matching the WGSL uniform layout rules).
func (e *ShaderEffect) SetFloatUniforms(f []float32) error {
	data := make([]byte, 4*len(f))
	for i, v := range f {
		binary.LittleEndian.PutUint32(data[4*i:], math.Float32bits(v))
	}
	return e.SetUniformBytes(data)
}

// SetImage binds one sampled image for the @group(0) binding(1)/(2)
// texture+sampler pair. Passing nil unbinds. The engine owns the GPU
// texture lifetime for the bound copy.
func (e *ShaderEffect) SetImage(img *ImageBuf) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.image = img
}

// Image returns the bound image (GPU upload keying / recording).
func (e *ShaderEffect) Image() *ImageBuf {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.image
}

// Uniforms returns a copy of the uniform block (per-draw upload source).
func (e *ShaderEffect) Uniforms() []byte {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]byte(nil), e.uniform...)
}

// Close releases the effect. In-flight draws keep their uploaded GPU copies
// alive for the frame; the effect itself must not be used afterwards.
func (e *ShaderEffect) Close() error {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.closed = true
	e.image = nil
	e.uniform = nil
	return nil
}

// SetShader installs (or with nil clears) the user shader for subsequent
// fills and strokes. Shader reports the ambient effect. A shader paint is a
// GPU-only draw: with no accelerator it is skipped (see Paint.Shader).
func (c *Context) SetShader(e *ShaderEffect) {
	if c == nil || c.paint == nil {
		return
	}
	c.paint.Shader = e
}

// Shader reports the ambient user shader, if any.
func (c *Context) Shader() *ShaderEffect {
	if c == nil || c.paint == nil {
		return nil
	}
	return c.paint.Shader
}
