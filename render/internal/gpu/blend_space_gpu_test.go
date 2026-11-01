//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

//go:build !nogpu

package gpu

import (
	"encoding/binary"
	"testing"

	"github.com/energye/gpui/render"
)

// P3.1 GPU pins: the dual-tex uniform carries the linear flag at offset 28
// (the shader's params.linear slot), and fixed-function linear requests are
// counted as explicit sRGB-domain degradations until the P3.1b format switch.

// TestPackDualTexParamsLinearFlag pins the uniform wire position of the
// linear bit — the WGSL Params struct reads it at offset 28.
func TestPackDualTexParamsLinearFlag(t *testing.T) {
	buf := make([]byte, dualTexUniformPayloadSize)
	packDualTexParams(buf, 1, 0, 0, 1, 1, 1, false, true)
	if got := binary.LittleEndian.Uint32(buf[28:32]); got != 1 {
		t.Fatalf("linear=true flag at [28:32] = %d, want 1", got)
	}
	if got := binary.LittleEndian.Uint32(buf[0:4]); got != 1 {
		t.Fatalf("modeU at [0:4] = %d, want 1 (multiply)", got)
	}

	buf2 := make([]byte, dualTexUniformPayloadSize)
	packDualTexParams(buf2, 2, 0, 0, 1, 1, 1, false, false)
	if got := binary.LittleEndian.Uint32(buf2[28:32]); got != 0 {
		t.Fatalf("linear=false flag at [28:32] = %d, want 0", got)
	}
	if got := binary.LittleEndian.Uint32(buf2[4:8]); got != 0 {
		t.Fatalf("dst_tight at [4:8] = %d, want 0", got)
	}
}

// TestNoteLinearFixedFunctionFallback pins the degradation counter: advanced
// modes never hit it (dual-tex owns them), fixed-function modes count once per
// draw, and a nil paint is a no-op.
func TestNoteLinearFixedFunctionFallback(t *testing.T) {
	rc := &GPURenderContext{shared: &GPUShared{}}

	rc.noteLinearFixedFunctionFallback(&render.Paint{BlendMode: render.BlendMultiply, BlendSpace: render.BlendSpaceLinear})
	if got := rc.shared.linearFixedFunctionFallbacks.Load(); got != 0 {
		t.Fatalf("advanced+linear must not count (dual-tex path), got %d", got)
	}

	rc.noteLinearFixedFunctionFallback(&render.Paint{BlendMode: render.BlendPlus, BlendSpace: render.BlendSpaceLinear})
	if got := rc.shared.linearFixedFunctionFallbacks.Load(); got != 1 {
		t.Fatalf("fixed-function+linear counted %d, want 1", got)
	}

	rc.noteLinearFixedFunctionFallback(&render.Paint{BlendMode: render.BlendPlus})
	if got := rc.shared.linearFixedFunctionFallbacks.Load(); got != 1 {
		t.Fatalf("sRGB-space paint counted, total %d, want still 1", got)
	}

	rc.noteLinearFixedFunctionFallback(nil)
	if got := rc.shared.linearFixedFunctionFallbacks.Load(); got != 1 {
		t.Fatalf("nil paint counted, total %d, want still 1", got)
	}
}

// TestFixedFunctionTableForLinearRouting pins the routing premise: Plus is
// fixed-function (degrades) and Multiply is advanced (dual-tex linear owns
// it). If the tables shift, the degradation contract in the P3.1 docs must be
// revisited alongside.
func TestFixedFunctionTableForLinearRouting(t *testing.T) {
	if !paintSupportsGPUFixedBlend(&render.Paint{BlendMode: render.BlendPlus}) {
		t.Fatal("Plus must stay in the fixed-function table (P3.1 degradation set)")
	}
	if paintSupportsGPUFixedBlend(&render.Paint{BlendMode: render.BlendMultiply}) {
		t.Fatal("Multiply must stay in the advanced (dual-tex) set")
	}
	if !paintSupportsGPUAdvancedBlend(&render.Paint{BlendMode: render.BlendMultiply}) {
		t.Fatal("Multiply must keep dual-tex support")
	}
}
