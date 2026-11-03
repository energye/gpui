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
	"github.com/energye/gpui/gpu/shader/ir"
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
//     PREMULTIPLIED-alpha RGBA color (docs.flutter.dev fragment-shader
//     contract: "output must be premultiplied alpha") — rgb must already be
//     scaled by a. The engine un-premultiplies pass-1 back into the
//     straight-alpha image composite chain, so a premul result composites
//     exactly as written (L2-P1-4: feeding premul bytes straight into that
//     chain squared the alpha — vec4(0.5,0,0,0.5) grayed to (126,127,127)).
//   - Optional bindings, both at @group(0): binding(0) a uniform block
//     (≤256 bytes, 16-byte field alignment) fed by SetUniformBytes/
//     SetFloatUniforms, and binding(1)+binding(2) one texture_2d<f32> +
//     sampler pair fed by SetImage. The GPU bind layout is derived from the
//     module's declared bindings, never from whichever image happens to be
//     bound at one draw (L2-P1-3: first-draw runtime values used to freeze
//     the cached bind group layout and poison every later state change).
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

	usesImage bool // declared texture_2d at @group(0) binding(1) (immutable)
	closed    bool
}

// ShaderInterface reports the immutable bind requirements derived from the
// module source at NewShaderEffect time: binding(0) uniform is always laid
// out at the full 256-byte budget, and the binding(1)/(2) texture+sampler
// pair is laid out iff the module declares it (UsesImage).
type ShaderInterface struct {
	UsesImage bool
}

// ShaderBinding returns the derived bind interface (L2-P1-3: pipeline caches
// must key their layouts off the shader module, not off per-draw state).
func (e *ShaderEffect) ShaderBinding() ShaderInterface {
	if e == nil {
		return ShaderInterface{}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return ShaderInterface{UsesImage: e.usesImage}
}

// shaderUniformMax is the uniform budget for one effect (one 256B slot minus
// the engine's own stride overhead stays well inside every backend limit).
// GPU bind layouts use this contract value, never the live uniform length.
const shaderUniformMax = 256

// ShaderUniformBudget is the public handle on the uniform-block budget in
// bytes (the @group(0) binding(0) layout size used by GPU pipeline caches).
const ShaderUniformBudget = shaderUniformMax

// NewShaderEffect compiles and validates a user WGSL fragment module.
// It returns a readable error for parse, lowering, or validation failures —
// rejection happens here, before any backend sees the source. Contract
// violations (missing fs_main, non-vec4<f32> return, out-of-slot bindings,
// over-budget uniform, f64 scalars) are rejected with the same eagerness
// (L2-P1-5: they used to reach backends and diverge per driver).
func NewShaderEffect(wgsl string) (*ShaderEffect, error) {
	if wgsl == "" {
		return nil, fmt.Errorf("shader: empty WGSL source")
	}
	mod, err := validateUserShader(wgsl)
	if err != nil {
		return nil, err
	}
	if err := validateShaderContract(mod); err != nil {
		return nil, err
	}
	return &ShaderEffect{wgsl: wgsl, usesImage: declaresImageBinding(mod)}, nil
}

// Shader bind-slot contract (fixed by capability definition):
// binding(0)=uniform, binding(1)=sampled texture_2d<f32>, binding(2)=sampler.
const (
	shaderSlotUniform = 0
	shaderSlotTexture = 1
	shaderSlotSampler = 2
)

// validateShaderContract enforces the ShaderEffect contract on the lowered
// module so every accepted effect is renderable on all three backends.
func validateShaderContract(mod *ir.Module) error {
	fs := -1
	for i := range mod.EntryPoints {
		if mod.EntryPoints[i].Stage == ir.StageFragment && mod.EntryPoints[i].Name == "fs_main" {
			fs = i
			break
		}
	}
	if fs < 0 {
		return fmt.Errorf("shader: contract: missing @fragment entry point fs_main")
	}
	if err := checkFsReturn(mod, &mod.EntryPoints[fs].Function); err != nil {
		return err
	}
	slots := map[uint32]string{}
	for i := range mod.GlobalVariables {
		gv := &mod.GlobalVariables[i]
		if gv.Binding == nil || gv.Binding.Group != 0 {
			continue
		}
		b := gv.Binding.Binding
		inner := typeInnerOf(mod, gv.Type)
		switch b {
		case shaderSlotUniform:
			if _, ok := inner.(ir.StructType); !ok {
				return fmt.Errorf("shader: contract: binding(0) must be a uniform struct, %q is %T", gv.Name, inner)
			}
			if sz := ir.TypeSize(mod, gv.Type); sz > shaderUniformMax {
				return fmt.Errorf("shader: contract: uniform block %d bytes exceeds %d", sz, shaderUniformMax)
			}
			slots[b] = "uniform"
		case shaderSlotTexture:
			img, ok := inner.(ir.ImageType)
			if !ok {
				return fmt.Errorf("shader: contract: binding(1) must be texture_2d<f32>, %q is %T", gv.Name, inner)
			}
			if img.Class != ir.ImageClassSampled || img.SampledKind != ir.ScalarFloat {
				return fmt.Errorf("shader: contract: binding(1) must be a sampled float texture, %q is not", gv.Name)
			}
			slots[b] = "texture"
		case shaderSlotSampler:
			if _, ok := inner.(ir.SamplerType); !ok {
				return fmt.Errorf("shader: contract: binding(2) must be a sampler, %q is %T", gv.Name, inner)
			}
			slots[b] = "sampler"
		default:
			return fmt.Errorf("shader: contract: @group(0) binding(%d) (%q) is outside the contract slots (0/1/2)", b, gv.Name)
		}
	}
	if slots[shaderSlotSampler] == "sampler" && slots[shaderSlotTexture] != "texture" {
		return fmt.Errorf("shader: contract: sampler at binding(2) requires a texture at binding(1)")
	}
	for h := range mod.Types {
		if bad, why := hasF64(mod, mod.Types[h].Inner, map[int]bool{}); bad {
			return fmt.Errorf("shader: contract: f64 scalars are not supported (%s)", why)
		}
	}
	return nil
}

func checkFsReturn(mod *ir.Module, fn *ir.Function) error {
	if fn.Result == nil {
		return fmt.Errorf("shader: contract: fs_main must return vec4<f32>")
	}
	vec, ok := typeInnerOf(mod, fn.Result.Type).(ir.VectorType)
	if !ok || vec.Size != ir.Vec4 || vec.Scalar.Kind != ir.ScalarFloat {
		return fmt.Errorf("shader: contract: fs_main must return vec4<f32>, got %s", fsResultName(mod, fn.Result.Type))
	}
	return nil
}

func fsResultName(mod *ir.Module, h ir.TypeHandle) string {
	if int(h) < len(mod.Types) && mod.Types[h].Name != "" {
		return mod.Types[h].Name
	}
	switch typeInnerOf(mod, h).(type) {
	case ir.VectorType:
		return "a vector"
	default:
		return "a non-vector type"
	}
}

func typeInnerOf(mod *ir.Module, h ir.TypeHandle) ir.TypeInner {
	if int(h) >= len(mod.Types) {
		return nil
	}
	return mod.Types[h].Inner
}

// hasF64 walks a type (structs/vectors/arrays/pointers/matrix scalars)
// looking for 64-bit floats; the visited set guards recursive layouts.
func hasF64(mod *ir.Module, inner ir.TypeInner, seen map[int]bool) (bool, string) {
	switch t := inner.(type) {
	case ir.ScalarType:
		if t.Kind == ir.ScalarFloat && t.Width == 8 {
			return true, "f64 scalar"
		}
	case ir.VectorType:
		return hasF64(mod, t.Scalar, seen)
	case ir.MatrixType:
		return hasF64(mod, t.Scalar, seen)
	case ir.ArrayType:
		if h := t.Base; int(h) < len(mod.Types) && !seen[int(h)] {
			seen[int(h)] = true
			return hasF64(mod, mod.Types[h].Inner, seen)
		}
	case ir.PointerType:
		if h := t.Base; int(h) < len(mod.Types) && !seen[int(h)] {
			seen[int(h)] = true
			return hasF64(mod, mod.Types[h].Inner, seen)
		}
	case ir.StructType:
		for _, m := range t.Members {
			if int(m.Type) < len(mod.Types) && !seen[int(m.Type)] {
				seen[int(m.Type)] = true
				if bad, why := hasF64(mod, mod.Types[m.Type].Inner, seen); bad {
					return true, m.Name + ": " + why
				}
			}
		}
	}
	return false, ""
}

// declaresImageBinding reports whether the lowered module declares a
// sampled texture at @group(0) @binding(1) — the contract's image slot.
func declaresImageBinding(mod *ir.Module) bool {
	for i := range mod.GlobalVariables {
		gv := &mod.GlobalVariables[i]
		if gv.Binding == nil || gv.Binding.Group != 0 || gv.Binding.Binding != 1 {
			continue
		}
		if int(gv.Type) < len(mod.Types) {
			if _, ok := mod.Types[gv.Type].Inner.(ir.ImageType); ok {
				return true
			}
		}
	}
	return false
}

// validateUserShader runs the shared translator pipeline (Parse → Lower →
// Validate) — the same front end the gles backend applies at pipeline build
// time, so rejection semantics are identical across backends. The lowered
// module is returned for interface derivation.
func validateUserShader(wgsl string) (*ir.Module, error) {
	ast, err := shader.Parse(wgsl)
	if err != nil {
		return nil, fmt.Errorf("shader: parse: %w", err)
	}
	mod, err := shader.Lower(ast)
	if err != nil {
		return nil, fmt.Errorf("shader: lower: %w", err)
	}
	if errs, err := shader.Validate(mod); err != nil {
		return nil, fmt.Errorf("shader: validate: %w", err)
	} else if len(errs) > 0 {
		return nil, fmt.Errorf("shader: validate: %d error(s), first: %v", len(errs), errs[0])
	}
	return mod, nil
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
	if e.closed {
		return fmt.Errorf("shader: effect closed")
	}
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
	if e.closed {
		return
	}
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

// shaderEffectEvicts mirrors the RegisterPurgeEvictable pattern: every live
// GPU holder registers an evict hook so ShaderEffect.Close releases the
// GPU-side pipeline caches keyed by the effect (L2-P0-1: entries used to be
// insert-only, leaking modules/uniform buffers/pipelines per effect).
var (
	shaderEvictMu      sync.Mutex
	shaderEffectEvicts = map[string]func(*ShaderEffect){}
)

// RegisterShaderEffectEvictor installs a hook called on every
// ShaderEffect.Close (id names the holder; reuse it to unregister).
func RegisterShaderEffectEvictor(id string, evict func(*ShaderEffect)) {
	shaderEvictMu.Lock()
	shaderEffectEvicts[id] = evict
	shaderEvictMu.Unlock()
}

// UnregisterShaderEffectEvictor removes a holder's hook.
func UnregisterShaderEffectEvictor(id string) {
	shaderEvictMu.Lock()
	delete(shaderEffectEvicts, id)
	shaderEvictMu.Unlock()
}

// Closed reports whether Close has been called (consumer refusal, same
// discriminator shape as ImageBuf.Disposed / AnimatedImage.Disposed).
func (e *ShaderEffect) Closed() bool {
	if e == nil {
		return true
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.closed
}

func notifyShaderEffectClosed(e *ShaderEffect) {
	shaderEvictMu.Lock()
	hooks := make([]func(*ShaderEffect), 0, len(shaderEffectEvicts))
	for _, h := range shaderEffectEvicts {
		hooks = append(hooks, h)
	}
	shaderEvictMu.Unlock()
	for _, h := range hooks {
		h(e)
	}
}

// Close releases the effect: GPU-side pipeline caches keyed by it are
// evicted through the registered holders, and further use is refused.
// In-flight draws keep their uploaded GPU copies alive for the frame.
// Idempotent.
func (e *ShaderEffect) Close() error {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil
	}
	e.closed = true
	e.image = nil
	e.uniform = nil
	e.mu.Unlock()
	notifyShaderEffectClosed(e)
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
