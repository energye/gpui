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
	"strings"
	"testing"

	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
)

func TestVramHandleTagDisjoint(t *testing.T) {
	handles := map[string]func(uint32) uintptr{
		"texture": vramTextureHandle,
		"buffer":  vramBufferHandle,
		"sampler": vramSamplerHandle,
		"program": vramProgramHandle,
		"shader":  vramShaderHandle,
	}
	for _, id := range []uint32{1, 2, 100, 0xffffff} {
		seen := map[uintptr]string{}
		for name, fn := range handles {
			h := fn(id)
			if h == 0 {
				t.Fatalf("%s handle %d must be non-zero", name, id)
			}
			if prev, ok := seen[h]; ok {
				t.Fatalf("handle collision for id=%d: %s == %s (%d)", id, name, prev, h)
			}
			seen[h] = name
		}
	}
	if vramBufferHandle(0) != 0 {
		t.Fatalf("buffer handle 0 must stay 0 for VramForget early-out, got %d", vramBufferHandle(0))
	}
	if vramTextureHandle(1) == 0 {
		t.Fatal("texture handle 1 must be non-zero")
	}
	// FBO handles fold id+height into one tagged slot: distinct heights
	// must not collide, and must not hit the id-only slots above.
	if vramHandleForFBO(7, 700) == vramHandleForFBO(7, 1016) {
		t.Fatal("FBO handles with different heights must not collide")
	}
	for _, h := range []uintptr{vramHandleForFBO(1, 700), vramHandleForFBO(2, 500)} {
		for name, fn := range map[string]func(uint32) uintptr{
			"texture": vramTextureHandle, "buffer": vramBufferHandle,
			"sampler": vramSamplerHandle, "program": vramProgramHandle,
		} {
			if h == fn(1) || h == fn(2) {
				t.Fatalf("FBO handle %d collides with %s slot", h, name)
			}
		}
	}
}

func TestGlAllocOOMShape(t *testing.T) {
	if !glAllocOOM(gl.OUT_OF_MEMORY) {
		t.Fatal("OUT_OF_MEMORY must be OOM")
	}
	if !glAllocOOM(gl.INVALID_OPERATION) {
		t.Fatal("INVALID_OPERATION from alloc must be OOM")
	}
	if glAllocOOM(gl.NO_ERROR) {
		t.Fatal("NO_ERROR must not be OOM")
	}
	if glAllocOOM(gl.INVALID_ENUM) {
		t.Fatal("INVALID_ENUM must not be OOM")
	}
	err := glAllocErr("TexImage2D", gl.INVALID_OPERATION, "format=0x88f0, level=0, 1351x783")
	if !strings.Contains(strings.ToLower(err.Error()), "out of memory") {
		t.Fatalf("0x502 alloc error must shape as OOM, got %q", err)
	}
	err = glAllocErr("TexImage2D", gl.INVALID_ENUM, "bad enum")
	if strings.Contains(strings.ToLower(err.Error()), "out of memory") {
		t.Fatalf("non-OOM error must not claim OOM, got %q", err)
	}
}
