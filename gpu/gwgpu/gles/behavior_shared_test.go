// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

//go:build (windows || linux) && !(js && wasm)

package gles

import (
	"errors"
	"testing"
	"time"

	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// h4b3BadBuf 充当错类型缓冲：满足 hal.Buffer 接口但不是 *Buffer，
// MapBuffer 必须拒收（ErrInvalidMapRange），UnmapBuffer 必须返回 nil。
type h4b3BadBuf struct{}

func (h4b3BadBuf) Destroy()                    {}
func (h4b3BadBuf) NativeHandle() uintptr       { return 0 }
func (h4b3BadBuf) Size() uint64                { return 0 }
func (h4b3BadBuf) Usage() gputypes.BufferUsage { return 0 }
func (h4b3BadBuf) Label() string               { return "" }

// H4-b3 行为差异单测（GL 侧共享部分，win/linux 通用）：
// 只用零值 Device 与无驱动能力的围栏（NewFence(nil)），不碰 GL 上下文。
// 锁住三项：围栏退化记数语义、映射越界报错、Unmap 幂等与设备丢失/错误
// 作用域桩语义。对照 webgpu：webgpu 围栏是空壳（无 Signal/GetLatest，
// Device 层恒报已信号）、Unmap 传错类型报 ErrInvalidMapRange（GL 返回
// nil，见 TestH4B3_GlesUnmapIdempotent 注释）。

func TestH4B3_GlesFenceNilContextFallbackCounter(t *testing.T) {
	f := NewFence(nil)

	if got := f.GetLatest(); got != 0 {
		t.Fatalf("GetLatest initial = %d, want 0", got)
	}
	if got := f.GetValue(); got != 0 {
		t.Fatalf("GetValue initial = %d, want 0", got)
	}
	// 无驱动围栏能力时 Signal 只记数（resource.go:301）。
	if err := f.Signal(2); err != nil {
		t.Fatalf("Signal(2) = %v, want nil", err)
	}
	if got := f.GetLatest(); got != 2 {
		t.Fatalf("GetLatest after Signal(2) = %d, want 2", got)
	}
	if !f.Wait(2, time.Second) {
		t.Fatal("Wait(2) = false, want true (already signaled)")
	}
	if f.Wait(3, 50*time.Millisecond) {
		t.Fatal("Wait(3) = true, want false (not yet signaled)")
	}
	f.Maintain() // 无同步对象时必须是空操作，不 panic。
	f.Reset()
	if got := f.GetValue(); got != 0 {
		t.Fatalf("GetValue after Reset = %d, want 0", got)
	}
	f.Destroy() // 无同步对象时必须是空操作，不 panic。
	f.Destroy()
}

func TestH4B3_GlesMapBufferInvalidRange(t *testing.T) {
	d := &Device{}

	if _, err := d.MapBuffer(nil, 0, 8); !errors.Is(err, hal.ErrInvalidMapRange) {
		t.Fatalf("MapBuffer(nil) = %v, want ErrInvalidMapRange", err)
	}
	if _, err := d.MapBuffer(h4b3BadBuf{}, 0, 1); !errors.Is(err, hal.ErrInvalidMapRange) {
		t.Fatalf("MapBuffer(wrong type) = %v, want ErrInvalidMapRange", err)
	}
	if _, err := d.MapBuffer(&Buffer{size: 8}, 4, 8); !errors.Is(err, hal.ErrInvalidMapRange) {
		t.Fatalf("MapBuffer(offset+size overflow) = %v, want ErrInvalidMapRange", err)
	}
	if _, err := d.MapBuffer(&Buffer{size: 8}, 9, 0); !errors.Is(err, hal.ErrInvalidMapRange) {
		t.Fatalf("MapBuffer(offset past end) = %v, want ErrInvalidMapRange", err)
	}
}

func TestH4B3_GlesUnmapIdempotent(t *testing.T) {
	d := &Device{}

	// GL 侧 Unmap 对未映射/错类型一律返回 nil（幂等）。
	// 差异点：webgpu 的 UnmapBuffer 传错类型返回 ErrInvalidMapRange
	//（device.go:652），GL/noop/metal 返回 nil。调用方按 hal 注释
	//“未映射调 Unmap 未定义”约束只传合法缓冲，本单测只锁住 GL 自身语义。
	if err := d.UnmapBuffer(nil); err != nil {
		t.Fatalf("UnmapBuffer(nil) = %v, want nil", err)
	}
	if err := d.UnmapBuffer(h4b3BadBuf{}); err != nil {
		t.Fatalf("UnmapBuffer(wrong type) = %v, want nil", err)
	}
	if err := d.UnmapBuffer(&Buffer{size: 8}); err != nil {
		t.Fatalf("UnmapBuffer(unmapped) = %v, want nil", err)
	}
}

func TestH4B3_GlesDeviceLostErrorScopeStubs(t *testing.T) {
	d := &Device{}

	// GL 暂无设备丢失跟踪与错误作用域捕获：恒 false/nil。
	// 对照 webgpu：IsLost 走底层库状态，Push/Pop 进出真实错误作用域。
	if d.IsLost() {
		t.Fatal("IsLost() = true, want false (no device-lost tracking on GL)")
	}
	if got := d.Poll(hal.PollPoll); !got {
		t.Fatal("Poll(PollPoll) = false, want true (synchronous backend)")
	}
	if got := d.Poll(hal.PollWait); !got {
		t.Fatal("Poll(PollWait) = false, want true (synchronous backend)")
	}
	d.FlushCallbacks() // 空操作，不 panic。
	d.PushErrorScope(hal.ErrorFilterValidation)
	d.PushErrorScope(hal.ErrorFilterOutOfMemory)
	if got := d.PopErrorScope(); got != nil {
		t.Fatalf("PopErrorScope() = %v, want nil (never captures)", got)
	}
}
