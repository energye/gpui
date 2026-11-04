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
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/energye/gpui/gpu/shader"
	"github.com/energye/gpui/gpu/shader/ir"
)

// Batch 3d sentinel family (design.md §4.4): every rejection the engine can
// judge is a named error judgeable via errors.Is. 同族在册 M3（WGSL 前端放行
// 非法 intrinsic）不冒充修完——本族只覆盖「我方已能判的」。
var (
	// ErrShaderBadWGSL wraps WGSL front-end failures (Parse/Lower/Validate and
	// composed-module revalidation).
	ErrShaderBadWGSL = errors.New("shader: WGSL rejected by the front end")
	// ErrShaderNoEntry wraps a missing @fragment fs_main.
	ErrShaderNoEntry = errors.New("shader: missing @fragment entry point fs_main")
	// ErrShaderBadReturn wraps a non-vec4<f32> fs_main return.
	ErrShaderBadReturn = errors.New("shader: fs_main must return vec4<f32>")
	// ErrShaderSlotConflict wraps binding/pair violations (stray or unpaired
	// bindings, over-cap pairs, bad parameter prefixes).
	ErrShaderSlotConflict = errors.New("shader: binding contract violation")
	// ErrShaderUniformTooLarge wraps uniform blocks beyond ShaderUniformBudgetMax.
	ErrShaderUniformTooLarge = errors.New("shader: uniform block exceeds the budget")
	// ErrShaderClosed wraps use-after-Close refusals.
	ErrShaderClosed = errors.New("shader: effect closed")
	// ErrShaderBackendCompile wraps backend shader/pipeline compilation
	// failures (gles first-draw 等) — always also errors.Is(ErrFallbackToCPU)
	// where the legacy fallback semantics must keep working.
	ErrShaderBackendCompile = errors.New("shader: backend compilation failed")
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
//     scaled by a. The composite chain is premul-native end to end (the image
//     leg uploads level-0 bytes as-is and blends BlendStatePremultiplied; the
//     context flush merges with the premul src-over composite), so a premul
//     result composites exactly as written. (L2-P1-4 originally inserted an
//     un-premultiply here; probe P3, 2026-11-05, proved that inverted the
//     handoff — the straight bytes re-entered a premul interpretation and
//     wrapped in the u8 layer composite. The un-premultiply is gone; the
//     chain stays premul from the shader output to the destination.)
//   - Batch 3a: fs_main's parameters are a declared PREFIX of
//     (uv: vec2<f32>, tint: vec4<f32>, hom_w: f32) — each at
//     @location(index), names free, zero parameters allowed. The quad
//     (material) vertex stage feeds all three per-vertex; the fill pass-1
//     stage feeds uv and literal defaults (tint = white, hom_w = 1) for the
//     rest. A fragment that declares the tint input owns tint application
//     (the engine multiplies only opacity × clip coverage after user code);
//     without the declaration the engine keeps the historic
//     texel * opacity * tint order (Godot COLOR-takeover semantics).
//   - Optional bindings, all at @group(0): binding(0) a uniform block
//     (up to ShaderUniformBudgetMax bytes — default 256, batch 3c raised the
//     ceiling — 16-byte field alignment) fed by SetUniformBytes/
//     SetFloatUniforms, and texture+sampler pairs at bindings (1,2), (3,4),
//     … (batch 3c, D4; pair 0 fed by SetImage, further pairs by SetImageAt).
//     The GPU bind layout is derived from the module's declared bindings,
//     never from whichever image happens to be bound at one draw (L2-P1-3:
//     first-draw runtime values used to freeze the cached bind group layout
//     and poison every later state change).
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
	images  []*ImageBuf // per declared texture pair (batch 3c); slot 0 = SetImage/fill slot

	usesImage    bool // declared texture_2d at @group(0) binding(1) (immutable)
	texturePairs int  // declared texture+sampler pairs (0..MaxShaderImagePairs, batch 3c)
	uniformBytes int  // declared uniform block size (0 = none, batch 3c)
	closed       bool

	// P3.7 batch 3a: eager engine-template composition (D-3a-1 拍板 A). Both
	// leg variants are composed and re-validated at NewShaderEffect time —
	// any surgery drift or user/engine symbol collision is a named rejection
	// before any backend sees the source.
	fragParams   int    // declared fs_main input prefix length (0..3)
	fillComposed string // fill pass-1 variant (uv-only vertex stage)
	quadComposed string // quad material variant (36B vertex stage + engine groups)
}

// ShaderInterface reports the immutable bind requirements derived from the
// module source at NewShaderEffect time: binding(0) uniform is always laid
// out (its buffer sized to max(shaderUniformMax, UniformBytes)), and the
// declared texture+sampler pairs ride bindings (1,2), (3,4), … (2k, 2k+1)
// (batch 3c, D4). Pair 0 carries the drawn image itself on the material leg
// (Godot TEXTURE/UV) and the SetImage slot on the fill leg.
type ShaderInterface struct {
	UsesImage bool
	// TexturePairs is the declared texture+sampler pair count (0..MaxShaderImagePairs).
	TexturePairs int
	// UniformBytes is the declared @group(0) binding(0) uniform block size in
	// bytes (0 = the module declares no uniform block; the layout still
	// reserves the default slot).
	UniformBytes int
}

// ShaderBinding returns the derived bind interface (L2-P1-3: pipeline caches
// must key their layouts off the shader module, not off per-draw state).
func (e *ShaderEffect) ShaderBinding() ShaderInterface {
	if e == nil {
		return ShaderInterface{}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return ShaderInterface{UsesImage: e.usesImage, TexturePairs: e.texturePairs, UniformBytes: e.uniformBytes}
}

// shaderUniformMax is the DEFAULT uniform budget for one effect (one 256B
// slot minus the engine's own stride overhead stays well inside every backend
// limit). GPU bind layouts use max(shaderUniformMax, declared size) as the
// binding size, never less.
const shaderUniformMax = 256

// ShaderUniformBudget is the public handle on the DEFAULT uniform-block
// budget in bytes (the @group(0) binding(0) layout size used by GPU pipeline
// caches for effects that declare a block up to this size). Batch 3c (D4
// 加重档) raised the ceiling: blocks up to ShaderUniformBudgetMax are
// accepted and their buffers sized accordingly.
const ShaderUniformBudget = shaderUniformMax

// ShaderUniformBudgetMax is the package ceiling on one effect's uniform
// block (batch 3c, D4). 65536 matches the measured backend floor
// (gles MaxUniformBufferBindingSize, TestP37CProbeGLESMultiTextureBindings
// 2026-11-05); the pipeline ensure clamps to the device's real limit with a
// named error beyond that.
const ShaderUniformBudgetMax = 65536

// MaxShaderImagePairs is the package ceiling on declared texture+sampler
// pairs (batch 3c, D4): pair k = texture@binding(2k+1) + sampler@binding(2k+2).
// 8 pairs = 8 textures + 8 samplers, inside the measured per-stage limits of
// both backends (gles: 16/16). Pair 0 carries the drawn image itself (Godot
// TEXTURE/UV on the material leg; the fill path's SetImage slot); pairs 1+
// carry SetImageAt-bound images.
const MaxShaderImagePairs = 8

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
	params, err := fragmentParams(mod)
	if err != nil {
		return nil, err
	}
	fillComposed, err := composeAndValidate(wgsl, params, false)
	if err != nil {
		return nil, err
	}
	quadComposed, err := composeAndValidate(wgsl, params, true)
	if err != nil {
		return nil, err
	}
	texturePairs, uniformBytes := declaredBindInterface(mod)
	eff := &ShaderEffect{
		wgsl:         wgsl,
		usesImage:    texturePairs >= 1,
		texturePairs: texturePairs,
		uniformBytes: uniformBytes,
		fragParams:   params,
		fillComposed: fillComposed,
		quadComposed: quadComposed,
	}
	if texturePairs > 0 {
		eff.images = make([]*ImageBuf, texturePairs)
	}
	return eff, nil
}

// ComposedFillWGSL returns the engine-composed fragment module for the fill
// pass-1 leg (P3.7 batch 3a). The raw WGSL() stays available for diagnostics;
// GPU pipelines must build from the composed source so the engine output
// section (opacity/clip/tint handling) is part of the module.
func (e *ShaderEffect) ComposedFillWGSL() string {
	if e == nil {
		return ""
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.fillComposed
}

// ComposedQuadWGSL returns the engine-composed fragment+vertex module for the
// quad material leg (P3.7 batch 3a): engine vertex stage, user function, and
// the engine output section in one module.
func (e *ShaderEffect) ComposedQuadWGSL() string {
	if e == nil {
		return ""
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.quadComposed
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
		return fmt.Errorf("%w: contract: missing @fragment entry point fs_main", ErrShaderNoEntry)
	}
	if err := checkFsReturn(mod, &mod.EntryPoints[fs].Function); err != nil {
		return fmt.Errorf("%w: %v", ErrShaderBadReturn, err)
	}
	// Batch 3c (D4 加重档): texture+sampler pairs at bindings (1,2), (3,4), …
	// (pair k = texture@(2k+1) + sampler@(2k+2)), cap MaxShaderImagePairs;
	// binding(0) stays the single uniform block, capped at
	// ShaderUniformBudgetMax (D4 预算放开，包级上限=实测两端设备 limits 下限，
	// 管线 ensure 再夹设备真值)。
	const (
		shaderSlotUniform = 0
		shaderSlotTexture = 1 // pair 0 texture; pair k texture at binding 2k+1
		shaderSlotSampler = 2 // pair 0 sampler; pair k sampler at binding 2k+2
	)
	uniformBytes := 0
	textures := map[uint32]bool{}
	samplers := map[uint32]bool{}
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
				return fmt.Errorf("%w: contract: binding(0) must be a uniform struct, %q is %T", ErrShaderSlotConflict, gv.Name, inner)
			}
			if sz := ir.TypeSize(mod, gv.Type); sz > ShaderUniformBudgetMax {
				return fmt.Errorf("%w: contract: uniform block %d bytes exceeds ShaderUniformBudgetMax %d", ErrShaderUniformTooLarge, sz, ShaderUniformBudgetMax)
			}
			if uniformBytes > 0 {
				return fmt.Errorf("%w: contract: only one uniform block at binding(0) is supported, %q is a second", ErrShaderSlotConflict, gv.Name)
			}
			uniformBytes = int(ir.TypeSize(mod, gv.Type))
		default:
			pair := (b - shaderSlotTexture) / 2
			if b%2 == shaderSlotTexture%2 && b >= shaderSlotTexture && pair < MaxShaderImagePairs && b == 2*pair+1 {
				img, ok := inner.(ir.ImageType)
				if !ok {
					return fmt.Errorf("%w: contract: binding(%d) must be texture_2d<f32>, %q is %T", ErrShaderSlotConflict, b, gv.Name, inner)
				}
				if img.Class != ir.ImageClassSampled || img.SampledKind != ir.ScalarFloat {
					return fmt.Errorf("%w: contract: binding(%d) must be a sampled float texture, %q is not", ErrShaderSlotConflict, b, gv.Name)
				}
				textures[b] = true
				continue
			}
			if b%2 == shaderSlotSampler%2 && b >= shaderSlotSampler && pair < MaxShaderImagePairs && b == 2*pair+2 {
				if _, ok := inner.(ir.SamplerType); !ok {
					return fmt.Errorf("%w: contract: binding(%d) must be a sampler, %q is %T", ErrShaderSlotConflict, b, gv.Name, inner)
				}
				samplers[b] = true
				continue
			}
			return fmt.Errorf("%w: contract: @group(0) binding(%d) (%q) is outside the contract slots (uniform 0, texture/sampler pairs 1..%d)", ErrShaderSlotConflict, b, gv.Name, 2*MaxShaderImagePairs)
		}
	}
	for b := range textures {
		if !samplers[b+1] {
			return fmt.Errorf("%w: contract: texture at binding(%d) requires a sampler at binding(%d)", ErrShaderSlotConflict, b, b+1)
		}
	}
	for b := range samplers {
		if !textures[b-1] {
			return fmt.Errorf("%w: contract: sampler at binding(%d) requires a texture at binding(%d)", ErrShaderSlotConflict, b, b-1)
		}
	}
	if len(textures) > MaxShaderImagePairs {
		return fmt.Errorf("%w: contract: %d texture pairs exceeds MaxShaderImagePairs %d", ErrShaderSlotConflict, len(textures), MaxShaderImagePairs)
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

// declaredBindInterface derives the texture-pair count and uniform size from
// the validated module (batch 3c): pairs at bindings (1,2), (3,4), …; the
// contract has already verified well-formedness.
func declaredBindInterface(mod *ir.Module) (texturePairs, uniformBytes int) {
	uniformBytes = 0
	pairs := 0
	for i := range mod.GlobalVariables {
		gv := &mod.GlobalVariables[i]
		if gv.Binding == nil || gv.Binding.Group != 0 {
			continue
		}
		b := gv.Binding.Binding
		if b == 0 {
			if int(gv.Type) < len(mod.Types) {
				if _, ok := mod.Types[gv.Type].Inner.(ir.StructType); ok {
					uniformBytes = int(ir.TypeSize(mod, gv.Type))
				}
			}
			continue
		}
		if b%2 == 1 && int(gv.Type) < len(mod.Types) {
			if _, ok := mod.Types[gv.Type].Inner.(ir.ImageType); ok {
				if p := int(b)/2 + 1; p > pairs {
					pairs = p
				}
			}
		}
	}
	return pairs, uniformBytes
}

// validateUserShader runs the shared translator pipeline (Parse → Lower →
// Validate) — the same front end the gles backend applies at pipeline build
// time, so rejection semantics are identical across backends. The lowered
// module is returned for interface derivation.
func validateUserShader(wgsl string) (*ir.Module, error) {
	ast, err := shader.Parse(wgsl)
	if err != nil {
		return nil, fmt.Errorf("%w: parse: %w", ErrShaderBadWGSL, err)
	}
	mod, err := shader.Lower(ast)
	if err != nil {
		return nil, fmt.Errorf("%w: lower: %w", ErrShaderBadWGSL, err)
	}
	if errs, err := shader.Validate(mod); err != nil {
		return nil, fmt.Errorf("%w: validate: %w", ErrShaderBadWGSL, err)
	} else if len(errs) > 0 {
		return nil, fmt.Errorf("%w: validate: %d error(s), first: %v", ErrShaderBadWGSL, len(errs), errs[0])
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

// SetUniformBytes installs the uniform block contents (up to
// ShaderUniformBudgetMax bytes — batch 3c, D4 raised the ceiling from the
// 256B default; WGSL uniform layout rules apply, 16-byte field alignment).
// Passing nil clears.
func (e *ShaderEffect) SetUniformBytes(data []byte) error {
	if e == nil {
		return fmt.Errorf("shader: nil effect")
	}
	if len(data) > ShaderUniformBudgetMax {
		return fmt.Errorf("%w: uniform block %d bytes exceeds ShaderUniformBudgetMax %d", ErrShaderUniformTooLarge, len(data), ShaderUniformBudgetMax)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return fmt.Errorf("%w: SetUniformBytes after Close", ErrShaderClosed)
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

// SetImage binds one sampled image for the first declared texture pair
// (pair 0: @group(0) binding(1)/(2)) — the slot the fill path samples and,
// on the material leg, the slot the drawn image itself occupies (Godot
// TEXTURE/UV). Batch 3c: additional declared pairs bind via SetImageAt.
// Passing nil unbinds. The engine owns the GPU texture lifetime for the
// bound copy.
func (e *ShaderEffect) SetImage(img *ImageBuf) {
	e.SetImageAt(0, img)
}

// Image returns the pair-0 bound image (GPU upload keying / recording).
func (e *ShaderEffect) Image() *ImageBuf {
	return e.ImageAt(0)
}

// SetImageAt binds one sampled image to declared texture pair slot (batch
// 3c, D4): slot k rides bindings (2k+1)/(2k+2). On the material leg slot 0
// is the drawn image itself, so extra textures bind from slot 1 up (slots
// must stay inside the declared pair count — a named error beyond it; the
// pipeline layout derives from the module, L2-P1-3). Passing nil unbinds.
func (e *ShaderEffect) SetImageAt(slot int, img *ImageBuf) error {
	if e == nil {
		return fmt.Errorf("shader: nil effect")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return fmt.Errorf("%w: SetImageAt after Close", ErrShaderClosed)
	}
	if slot < 0 || slot >= len(e.images) {
		return fmt.Errorf("%w: image slot %d outside the declared pairs (%d)", ErrShaderSlotConflict, slot, len(e.images))
	}
	e.images[slot] = img
	return nil
}

// ImageAt returns the bound image at declared texture pair slot (nil when
// unbound or out of range).
func (e *ShaderEffect) ImageAt(slot int) *ImageBuf {
	if e == nil || slot < 0 || slot >= len(e.images) {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.images[slot]
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
	e.images = nil
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
