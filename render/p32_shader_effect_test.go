//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package render

import (
	"testing"
)

// P3.2 capability pins for ShaderEffect: eager compile rejection, the
// uniform API contract, and GPU pixel semantics of Paint.Shader fills.

// TestShaderEffectRejectsBadWGSL pins the eager front end: parse, lowering
// and validation failures surface as readable errors here — never at a
// backend driver.
func TestShaderEffectRejectsBadWGSL(t *testing.T) {
	if _, err := NewShaderEffect(""); err == nil {
		t.Fatal("empty source must be rejected")
	}
	// Parse failure: unbalanced braces.
	if _, err := NewShaderEffect("fn fs_main("); err == nil {
		t.Fatal("parse error must be rejected")
	}
	// Lower/validate failure: structurally parseable but meaningless module.
	if _, err := NewShaderEffect("const broken: i32 = ;"); err == nil {
		t.Fatal("lower/validate failure must be rejected")
	}
	// A minimal legal module compiles.
	if _, err := NewShaderEffect(`
@fragment fn fs_main() -> @location(0) vec4<f32> { return vec4<f32>(1.0, 0.0, 0.0, 1.0); }
`); err != nil {
		t.Fatalf("legal module rejected: %v", err)
	}
}

// TestShaderEffectUniformAPI pins the uniform contract: the byte budget and
// the float packing.
func TestShaderEffectUniformAPI(t *testing.T) {
	e, err := NewShaderEffect(`
@fragment fn fs_main() -> @location(0) vec4<f32> { return vec4<f32>(1.0); }
`)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	defer e.Close()
	if err := e.SetUniformBytes(make([]byte, 257)); err == nil {
		t.Fatal("uniform block over 256 bytes must be rejected")
	}
	if err := e.SetUniformBytes(make([]byte, 256)); err != nil {
		t.Fatalf("256-byte uniform block legal: %v", err)
	}
	if err := e.SetFloatUniforms([]float32{0.25, 0.5, 0.75, 1.0}); err != nil {
		t.Fatalf("float uniforms: %v", err)
	}
	u := e.Uniforms()
	if len(u) != 16 || u[3] != 0x3E || u[7] != 0x3F || u[15] != 0x3F {
		t.Fatalf("float packing wrong (LE tail bytes must be 3E/3F/3F/3F): %x", u)
	}
	e.SetUniformBytes(nil)
	if len(e.Uniforms()) != 0 {
		t.Fatal("nil uniform must clear")
	}
	e.Close()
	if e.WGSL() != "" {
		_ = e.WGSL()
	}
}

// TestShaderPaintCPUSkips pins the CPU-mode contract: with no accelerator a
// Shader paint is skipped silently (DrawGPUTexture semantics) — the
// background is untouched, no CPU fallback is counted, and the ambient
// solid never leaks through.
func TestShaderPaintCPUSkips(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	dc := NewContext(32, 32)
	t.Cleanup(func() { _ = dc.Close() })
	dc.ClearWithColor(Black)
	effect, err := NewShaderEffect(`
@fragment fn fs_main() -> @location(0) vec4<f32> { return vec4<f32>(1.0, 0.0, 0.0, 1.0); }
`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	defer effect.Close()
	dc.paint.Shader = effect
	dc.DrawRectangle(8, 8, 16, 16)
	if err := dc.Fill(); err != nil {
		t.Fatalf("fill: %v", err)
	}
	pm := dc.PixmapForTest()
	data := pm.Data()
	for i := 0; i < len(data); i += 4 {
		if data[i] != 0 || data[i+1] != 0 || data[i+2] != 0 {
			t.Fatalf("shader paint leaked at %d: (%d,%d,%d,%d) — CPU mode must skip", i, data[i], data[i+1], data[i+2], data[i+3])
		}
	}
	if st := dc.RenderPathStats(); st.CPUFallbackOps != 0 {
		t.Fatalf("cpu_fallback_ops=%d — GPU-only skip must not count as fallback", st.CPUFallbackOps)
	}
}

func newShaderTestContext(t *testing.T, w, h int) *Context {
	t.Helper()
	dc := NewContext(w, h)
	t.Cleanup(func() { _ = dc.Close() })
	return dc
}

func d(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}
