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
	"errors"
	"testing"

	"github.com/energye/gpui/gpu/hal"
)

// Acquire 未配置/上下文丢失一律 hal.ErrSurfaceLost（errors.Is 可判）；
// Discard 全路径 nil-safe；外来纹理忽略。

func TestH4D_SurfaceAcquireStrictCodes(t *testing.T) {
	// nil 接收者。
	var nilSurf *Surface
	if _, err := nilSurf.AcquireTexture(nil); !errors.Is(err, hal.ErrSurfaceLost) {
		t.Fatalf("nil surface Acquire = %v, want ErrSurfaceLost", err)
	}
	nilSurf.DiscardTexture(nil) // must not panic

	// 零值（未配置）。
	zero := &Surface{}
	if _, err := zero.AcquireTexture(nil); !errors.Is(err, hal.ErrSurfaceLost) {
		t.Fatalf("unconfigured Acquire = %v, want ErrSurfaceLost", err)
	}
	if w, h := zero.ActualExtent(); w != 0 || h != 0 {
		t.Fatalf("unconfigured ActualExtent = %dx%d, want 0x0", w, h)
	}
	zero.DiscardTexture(nil) // must not panic

	// 配了 config 但上下文丢了（ scavenged ctx）。
	lost := &Surface{
		configured: true,
		config:     &hal.SurfaceConfiguration{Width: 64, Height: 64},
		ctx:        nil,
	}
	if _, err := lost.AcquireTexture(nil); !errors.Is(err, hal.ErrSurfaceLost) {
		t.Fatalf("lost-context Acquire = %v, want ErrSurfaceLost", err)
	}
}

func TestH4D_SurfaceDiscardTracksInflight(t *testing.T) {
	s := &Surface{}
	mine := &SurfaceTexture{surface: s}
	foreign := &SurfaceTexture{}

	// 空盘 Discard 全是空操作。
	s.DiscardTexture(nil)
	s.DiscardTexture(mine)
	s.DiscardTexture(foreign)

	// 装一帧在途：nil 清掉，命中清掉，外来忽略。
	s.current = mine
	s.DiscardTexture(foreign)
	if s.current != mine {
		t.Fatal("Discard(foreign) dropped the in-flight frame, want keep")
	}
	s.DiscardTexture(mine)
	if s.current != nil {
		t.Fatal("Discard(matching) kept the in-flight frame, want drop")
	}
	s.current = mine
	s.DiscardTexture(nil)
	if s.current != nil {
		t.Fatal("Discard(nil) kept the in-flight frame, want drop")
	}
}
