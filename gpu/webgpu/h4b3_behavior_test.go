//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !(js && wasm)

package webgpu

import (
	"errors"
	"testing"

	"github.com/energye/gpui/gpu/hal"
)

// 对照方：gles Poll 走 Fence.GetLatest（无围栏时走提交最大值）、
// metal Poll 走 completedIndex（GPU 回调推进）、noop Poll 追平提交值。

func TestH4B3_WebGPUQueuePollAlwaysZero(t *testing.T) {
	q := &Queue{}

	// webgpu 不在 facade 层记数：Poll/Last 恒 0，与三家纯 Go 后端都不同。
	if got := q.Poll(); got != 0 {
		t.Fatalf("Poll() = %d, want 0 (wgpu-native exposes no queue poll)", got)
	}
	if got := q.LastSubmissionIndex(); got != 0 {
		t.Fatalf("LastSubmissionIndex() = %d, want 0 (untracked on wgpu-native)", got)
	}
}

func TestH4B3_WebGPUSubmitNeedsDevice(t *testing.T) {
	q := &Queue{}

	if _, err := q.Submit(); err == nil {
		t.Fatal("Submit() on zero queue = nil, want error (invalid handle)")
	}
	if got := q.Poll(); got != 0 {
		t.Fatalf("Poll() after failed submit = %d, want 0", got)
	}
}

func TestH4B3_WebGPUFenceNoopLifecycle(t *testing.T) {
	d := &Device{}
	f := &Fence{}

	// webgpu 围栏是空壳：恒报已信号（device.go:412 GetFenceStatus 真值）。
	signaled, err := d.GetFenceStatus(f)
	if err != nil || !signaled {
		t.Fatalf("GetFenceStatus() = (%v, %v), want (true, nil)", signaled, err)
	}
	if err := d.ResetFence(f); err != nil {
		t.Fatalf("ResetFence() = %v, want nil (no-op)", err)
	}
	if _, err := d.GetFenceStatus(nil); err == nil {
		t.Fatal("GetFenceStatus(nil) = nil, want error")
	}
	if _, err := d.WaitForFence(nil, 1, 0); err == nil {
		t.Fatal("WaitForFence(nil) = nil, want error")
	}
	// 已释放设备上等围栏直接报已释放，不碰底层（device.go:430）。
	rel := &Device{}
	rel.released = true
	if _, err := rel.WaitForFence(f, 1, 0); !errors.Is(err, ErrReleased) {
		t.Fatalf("WaitForFence on released device = %v, want ErrReleased", err)
	}
	f.Release()
	f.Destroy() // 幂等，不 panic。
}

func TestH4B3_WebGPUMapInvalidRange(t *testing.T) {
	d := &Device{}

	if _, err := d.MapBuffer(nil, 0, 8); !errors.Is(err, hal.ErrInvalidMapRange) {
		t.Fatalf("MapBuffer(nil) = %v, want ErrInvalidMapRange", err)
	}
	// 差异点：webgpu 的 Unmap 传错类型报错（GL/noop/metal 返回 nil，
	// 见 gles TestH4B3_GlesUnmapIdempotent）。
	if err := d.UnmapBuffer(nil); !errors.Is(err, hal.ErrInvalidMapRange) {
		t.Fatalf("UnmapBuffer(nil) = %v, want ErrInvalidMapRange", err)
	}
}

func TestH4B3_WebGPUDeviceLostErrorScopeStubs(t *testing.T) {
	d := &Device{}

	// 无底层设备时 Poll 报未完成（对照 gles/metal/noop 恒 true），
	// IsLost 报未丢失，回调/作用域入口空安全。
	if got := d.Poll(hal.PollPoll); got {
		t.Fatal("Poll(PollPoll) on zero device = true, want false (no native device)")
	}
	if d.IsLost() {
		t.Fatal("IsLost() on zero device = true, want false")
	}
	d.FlushCallbacks() // 空安全，不 panic。
	d.PushErrorScope(hal.ErrorFilterValidation)
	if got := d.PopErrorScope(); got != nil {
		t.Fatalf("PopErrorScope() on zero device = %v, want nil", got)
	}
}
