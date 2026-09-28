// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

//go:build darwin && !(js && wasm)

package metal

import (
	"errors"
	"testing"

	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// h4b3BadBuf 充当错类型缓冲：满足 hal.Buffer 接口但不是 *Buffer。
type h4b3BadBuf struct{}

func (h4b3BadBuf) Destroy()                    {}
func (h4b3BadBuf) NativeHandle() uintptr       { return 0 }
func (h4b3BadBuf) Size() uint64                { return 0 }
func (h4b3BadBuf) Usage() gputypes.BufferUsage { return 0 }
func (h4b3BadBuf) Label() string               { return "" }

// H4-b3 行为差异单测（Metal 侧）：提交索引递增、映射越界报错、Unmap
// 幂等、围栏错类型报错、设备丢失/错误作用域桩语义。
// 本机（Linux）只验编译；真机跑时 Submit 空包只建自动释放池＋推进索引
//（无命令缓冲不提交 MTL 命令），其余用例只用零值 Device/Queue，不建
// MTLDevice。锁住的对照关系：Poll 走 completedIndex（GPU 回调推进，
// 无 GPU 时滞后，见 TestH4B3_MetalSubmitIndexIncrements 注释），对照
// webgpu Queue.Poll 恒 0、gles 走 Fence.GetLatest。

func TestH4B3_MetalSubmitIndexIncrements(t *testing.T) {
	q := &Queue{}

	if got := q.Poll(); got != 0 {
		t.Fatalf("Poll() initial = %d, want 0", got)
	}
	if got := q.LastSubmissionIndex(); got != 0 {
		t.Fatalf("LastSubmissionIndex() initial = %d, want 0", got)
	}
	// 空包也推进索引（帧限流只在 frameSemaphore 非空时阻塞，此处为 nil）。
	i1, err := q.Submit()
	if err != nil || i1 != 1 {
		t.Fatalf("Submit() = (%d, %v), want (1, nil)", i1, err)
	}
	i2, err := q.Submit()
	if err != nil || i2 != 2 {
		t.Fatalf("Submit() again = (%d, %v), want (2, nil)", i2, err)
	}
	if got := q.LastSubmissionIndex(); got != 2 {
		t.Fatalf("LastSubmissionIndex() = %d, want 2", got)
	}
	// 无 GPU 回调时 completedIndex 滞后：Poll 只保证不超前，不保证追平。
	if got := q.Poll(); got > 2 {
		t.Fatalf("Poll() = %d, want <= 2 (must not run ahead)", got)
	}
}

func TestH4B3_MetalMapBufferInvalidRange(t *testing.T) {
	d := &Device{}

	if _, err := d.MapBuffer(nil, 0, 8); !errors.Is(err, hal.ErrInvalidMapRange) {
		t.Fatalf("MapBuffer(nil) = %v, want ErrInvalidMapRange", err)
	}
	if _, err := d.MapBuffer(h4b3BadBuf{}, 0, 1); !errors.Is(err, hal.ErrInvalidMapRange) {
		t.Fatalf("MapBuffer(wrong type) = %v, want ErrInvalidMapRange", err)
	}
	// raw==0（无 MTLBuffer）与越界都在碰 ObjC 之前返回。
	if _, err := d.MapBuffer(&Buffer{}, 0, 8); !errors.Is(err, hal.ErrInvalidMapRange) {
		t.Fatalf("MapBuffer(zero buffer) = %v, want ErrInvalidMapRange", err)
	}
	if _, err := d.MapBuffer(&Buffer{raw: 1, size: 8}, 4, 8); !errors.Is(err, hal.ErrInvalidMapRange) {
		t.Fatalf("MapBuffer(offset+size overflow) = %v, want ErrInvalidMapRange", err)
	}
}

func TestH4B3_MetalUnmapIdempotent(t *testing.T) {
	d := &Device{}

	// Shared 常驻映射：Unmap 恒为 nil（幂等），与 gles/noop 同，对照
	// webgpu 传错类型报 ErrInvalidMapRange。
	if err := d.UnmapBuffer(nil); err != nil {
		t.Fatalf("UnmapBuffer(nil) = %v, want nil", err)
	}
	if err := d.UnmapBuffer(&Buffer{}); err != nil {
		t.Fatalf("UnmapBuffer(zero buffer) = %v, want nil", err)
	}
}

func TestH4B3_MetalFenceInvalidType(t *testing.T) {
	d := &Device{}

	// 错类型围栏在碰 MTLSharedEvent 之前返回错误（与 gles 同口径）。
	if _, err := d.WaitForFence(nil, 1, 0); err == nil {
		t.Fatal("WaitForFence(nil) = nil, want error")
	}
	if err := d.ResetFence(nil); err == nil {
		t.Fatal("ResetFence(nil) = nil, want error")
	}
	if _, err := d.GetFenceStatus(nil); err == nil {
		t.Fatal("GetFenceStatus(nil) = nil, want error")
	}
	d.DestroyFence(nil) // nil-safe，不 panic。
}

func TestH4B3_MetalDeviceLostErrorScopeStubs(t *testing.T) {
	d := &Device{}

	// Metal 暂无设备丢失跟踪与错误作用域捕获：恒 false/nil/true。
	// 对照 webgpu：IsLost 走底层库状态，Push/Pop 进出真实错误作用域。
	if d.IsLost() {
		t.Fatal("IsLost() = true, want false (no device-lost tracking on Metal)")
	}
	if got := d.Poll(hal.PollPoll); !got {
		t.Fatal("Poll(PollPoll) = false, want true (synchronous stub)")
	}
	if got := d.Poll(hal.PollWait); !got {
		t.Fatal("Poll(PollWait) = false, want true (synchronous stub)")
	}
	d.FlushCallbacks() // 空操作，不 panic。
	d.PushErrorScope(hal.ErrorFilterValidation)
	if got := d.PopErrorScope(); got != nil {
		t.Fatalf("PopErrorScope() = %v, want nil (never captures)", got)
	}
}
