//go:build !(js && wasm)

package noop

import (
	"errors"
	"testing"
	"time"

	"github.com/energye/gpui/gpu/hal"
)

// H4-b3 行为差异单测（noop 同步参照）：内存后端，所有提交立即可见，
// 无需 GPU。锁住与 webgpu 对照的四项——提交索引递增、围栏记数、映射
// 越界报错、Unmap 幂等。差异点：noop Poll 追平提交值（对照 webgpu
// 恒 0），Unmap 传错类型返回 nil（对照 webgpu 报错）。

func TestH4B3_NoopSubmitIndexIncrements(t *testing.T) {
	q := &Queue{}

	i1, err := q.Submit()
	if err != nil || i1 != 1 {
		t.Fatalf("Submit() = (%d, %v), want (1, nil)", i1, err)
	}
	i2, err := q.Submit(nil)
	if err != nil || i2 != 2 {
		t.Fatalf("Submit(nil) = (%d, %v), want (2, nil) (noop ignores elements)", i2, err)
	}
	i3, err := q.Submit(nil, nil)
	if err != nil || i3 != 3 {
		t.Fatalf("Submit(nil, nil) = (%d, %v), want (3, nil)", i3, err)
	}
	if got := q.LastSubmissionIndex(); got != 3 {
		t.Fatalf("LastSubmissionIndex() = %d, want 3", got)
	}
	// 同步后端：提交即完成，Poll 追平（对照 webgpu 恒 0）。
	if got := q.Poll(); got != 3 {
		t.Fatalf("Poll() = %d, want 3 (synchronous backend)", got)
	}
}

func TestH4B3_NoopFenceSignalWait(t *testing.T) {
	d := &Device{}

	f, err := d.CreateFence()
	if err != nil {
		t.Fatalf("CreateFence() = %v, want nil", err)
	}
	nf, ok := f.(*Fence)
	if !ok {
		t.Fatalf("CreateFence type = %T, want *noop.Fence", f)
	}
	nf.Signal(2)
	if got := nf.GetValue(); got != 2 {
		t.Fatalf("GetValue() = %d, want 2", got)
	}
	if !nf.Wait(2, time.Second) {
		t.Fatal("Wait(2) = false, want true")
	}
	if nf.Wait(3, 50*time.Millisecond) {
		t.Fatal("Wait(3) = true, want false")
	}
	ok, err = d.WaitForFence(f, 2, time.Second)
	if err != nil || !ok {
		t.Fatalf("WaitForFence(2) = (%v, %v), want (true, nil)", ok, err)
	}
	signaled, err := d.GetFenceStatus(f)
	if err != nil || !signaled {
		t.Fatalf("GetFenceStatus() = (%v, %v), want (true, nil)", signaled, err)
	}
	if err := d.ResetFence(f); err != nil {
		t.Fatalf("ResetFence() = %v, want nil", err)
	}
	if signaled, _ := d.GetFenceStatus(f); signaled {
		t.Fatal("GetFenceStatus() after Reset = true, want false")
	}
	d.DestroyFence(f)
	d.DestroyFence(nil) // 空操作，不 panic。
}

func TestH4B3_NoopMapInvalidRangeAndUnmapIdempotent(t *testing.T) {
	d := &Device{}

	buf, err := d.CreateBuffer(&hal.BufferDescriptor{Size: 16})
	if err != nil {
		t.Fatalf("CreateBuffer(16) = %v, want nil", err)
	}
	if _, err := d.MapBuffer(buf, 0, 16); err != nil {
		t.Fatalf("MapBuffer(0,16) = %v, want nil", err)
	}
	if _, err := d.MapBuffer(buf, 12, 8); !errors.Is(err, hal.ErrInvalidMapRange) {
		t.Fatalf("MapBuffer(12,8 on size 16) = %v, want ErrInvalidMapRange", err)
	}
	if _, err := d.MapBuffer(nil, 0, 1); !errors.Is(err, hal.ErrInvalidMapRange) {
		t.Fatalf("MapBuffer(nil) = %v, want ErrInvalidMapRange", err)
	}
	// Unmap 幂等：错类型/重复调用都返回 nil（对照 webgpu 错类型报错）。
	if err := d.UnmapBuffer(nil); err != nil {
		t.Fatalf("UnmapBuffer(nil) = %v, want nil", err)
	}
	if err := d.UnmapBuffer(buf); err != nil {
		t.Fatalf("UnmapBuffer = %v, want nil", err)
	}
	if err := d.UnmapBuffer(buf); err != nil {
		t.Fatalf("UnmapBuffer twice = %v, want nil (idempotent)", err)
	}
}

func TestH4B3_NoopDeviceLostErrorScopeStubs(t *testing.T) {
	d := &Device{}

	if d.IsLost() {
		t.Fatal("IsLost() = true, want false (noop never lost)")
	}
	if got := d.Poll(hal.PollPoll); !got {
		t.Fatal("Poll(PollPoll) = false, want true (synchronous backend)")
	}
	if err := d.WaitIdle(); err != nil {
		t.Fatalf("WaitIdle() = %v, want nil", err)
	}
	d.FlushCallbacks() // 空操作，不 panic。
	d.PushErrorScope(hal.ErrorFilterValidation)
	if got := d.PopErrorScope(); got != nil {
		t.Fatalf("PopErrorScope() = %v, want nil (never captures)", got)
	}
}
