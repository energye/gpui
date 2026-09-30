//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build linux && !(js && wasm)

package gles

import (
	"errors"
	"testing"
	"time"
	"unsafe"

	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
	"github.com/energye/gpui/gpu/hal"
)

// h4b3BadCmd 充当错类型命令缓冲：满足 hal.CommandBuffer 接口但不是
// *CommandBuffer，Submit 必须拒收且不推进索引。
type h4b3BadCmd struct{}

func (h4b3BadCmd) Destroy() {}

func newH4B3LinuxDevice() (*Device, *Queue) {
	ctx := NewAdapterContext(nil, &gl.Context{}, false)
	return &Device{ctx: ctx}, &Queue{ctx: ctx}
}

func TestH4B3_GlesSubmitIndexIncrements(t *testing.T) {
	_, q := newH4B3LinuxDevice()

	// 空/单/多包各推进 1（直发语义：Lock→执行→index++→Flush，无排队）。
	i1, err := q.Submit()
	if err != nil || i1 != 1 {
		t.Fatalf("Submit() = (%d, %v), want (1, nil)", i1, err)
	}
	i2, err := q.Submit(&CommandBuffer{})
	if err != nil || i2 != 2 {
		t.Fatalf("Submit(single) = (%d, %v), want (2, nil)", i2, err)
	}
	i3, err := q.Submit(&CommandBuffer{}, &CommandBuffer{})
	if err != nil || i3 != 3 {
		t.Fatalf("Submit(multi) = (%d, %v), want (3, nil)", i3, err)
	}
	if got := q.LastSubmissionIndex(); got != 3 {
		t.Fatalf("LastSubmissionIndex() = %d, want 3", got)
	}
	// 无围栏时 Poll 返回已提交最大值。
	if got := q.Poll(); got != 3 {
		t.Fatalf("Poll() without fence = %d, want 3", got)
	}
}

func TestH4B3_GlesSubmitInvalidTypeNoIncrement(t *testing.T) {
	_, q := newH4B3LinuxDevice()

	if _, err := q.Submit(&CommandBuffer{}); err != nil {
		t.Fatalf("Submit(single) = %v, want nil", err)
	}
	// 差异点：GL 拒收错类型/nil（返回错误）；webgpu 跳过 nil 元素继续
	// 提交，metal 跳过错类型元素继续提交。GL 拒收时索引不动。
	if _, err := q.Submit(h4b3BadCmd{}); err == nil {
		t.Fatal("Submit(wrong type) = nil, want error")
	}
	if got := q.LastSubmissionIndex(); got != 1 {
		t.Fatalf("LastSubmissionIndex() after rejected submit = %d, want 1", got)
	}
	if _, err := q.Submit(nil); err == nil {
		t.Fatal("Submit(nil) = nil, want error")
	}
	if got := q.LastSubmissionIndex(); got != 1 {
		t.Fatalf("LastSubmissionIndex() after nil submit = %d, want 1", got)
	}
}

func TestH4B3_GlesQueuePollFollowsFence(t *testing.T) {
	ctx := NewAdapterContext(nil, &gl.Context{}, false)
	fence := NewFence(ctx.GL())
	q := &Queue{ctx: ctx, fence: fence}

	if _, err := q.Submit(); err != nil {
		t.Fatalf("Submit() = %v, want nil", err)
	}
	// 有围栏时 Poll 走 Fence.GetLatest（对照 webgpu 恒 0、metal 走
	// completedIndex、noop 走提交最大值）。
	if got := q.Poll(); got != 1 {
		t.Fatalf("Poll() with fence = %d, want 1", got)
	}
	if _, err := q.Submit(&CommandBuffer{}); err != nil {
		t.Fatalf("Submit(single) = %v, want nil", err)
	}
	if got := q.Poll(); got != 2 {
		t.Fatalf("Poll() with fence = %d, want 2", got)
	}
}

func TestH4B3_GlesMapRoundTrip(t *testing.T) {
	d, _ := newH4B3LinuxDevice()
	buf := &Buffer{size: 16}

	m, err := d.MapBuffer(buf, 0, 16)
	if err != nil {
		t.Fatalf("MapBuffer(0,16) = %v, want nil", err)
	}
	if m.Ptr == nil {
		t.Fatal("MapBuffer Ptr = nil, want non-nil")
	}
	if !m.IsCoherent {
		t.Fatal("MapBuffer IsCoherent = false, want true (shadow slice)")
	}
	// 同一映射内经影子切片写入后立即可见（IsCoherent）。
	dst := unsafe.Slice((*byte)(m.Ptr), 4)
	dst[0], dst[1], dst[2], dst[3] = 0x11, 0x22, 0x33, 0x44
	same := unsafe.Slice((*byte)(m.Ptr), 4)
	if same[0] != 0x11 || same[1] != 0x22 || same[2] != 0x33 || same[3] != 0x44 {
		t.Fatalf("same-mapping bytes = %02x, want 11223344", same)
	}
	if err := d.UnmapBuffer(buf); err != nil {
		t.Fatalf("UnmapBuffer = %v, want nil", err)
	}
	if err := d.UnmapBuffer(buf); err != nil {
		t.Fatalf("UnmapBuffer twice = %v, want nil (idempotent)", err)
	}
	// 无 GL 背板（id==0）时 Unmap 丢弃影子：重映射拿 fresh zeros。
	// 持久化语义需真实 GL 缓冲（Unmap 经 glBufferSubData 落盘），此处
	// 只锁住无背板行为，不断言跨映射持久。
	m2, err := d.MapBuffer(buf, 0, 16)
	if err != nil {
		t.Fatalf("MapBuffer again = %v, want nil", err)
	}
	fresh := unsafe.Slice((*byte)(m2.Ptr), 4)
	if fresh[0] != 0 || fresh[1] != 0 || fresh[2] != 0 || fresh[3] != 0 {
		t.Fatalf("remapped bytes = %02x, want 00000000 (unbacked shadow dropped)", fresh)
	}
	if err := d.UnmapBuffer(buf); err != nil {
		t.Fatalf("UnmapBuffer = %v, want nil", err)
	}
	if _, err := d.MapBuffer(buf, 12, 8); !errors.Is(err, hal.ErrInvalidMapRange) {
		t.Fatalf("MapBuffer(12,8 on size 16) = %v, want ErrInvalidMapRange", err)
	}
}

func TestH4B3_GlesDeviceFenceWrappers(t *testing.T) {
	d, _ := newH4B3LinuxDevice()

	f, err := d.CreateFence()
	if err != nil {
		t.Fatalf("CreateFence() = %v, want nil", err)
	}
	gf, ok := f.(*Fence)
	if !ok {
		t.Fatalf("CreateFence type = %T, want *gles.Fence", f)
	}
	ok, err = d.WaitForFence(f, 0, time.Second)
	if err != nil || !ok {
		t.Fatalf("WaitForFence(0) = (%v, %v), want (true, nil)", ok, err)
	}
	if err := gf.Signal(1); err != nil {
		t.Fatalf("Signal(1) = %v, want nil", err)
	}
	signaled, err := d.GetFenceStatus(f)
	if err != nil || !signaled {
		t.Fatalf("GetFenceStatus() = (%v, %v), want (true, nil)", signaled, err)
	}
	ok, err = d.WaitForFence(f, 1, time.Second)
	if err != nil || !ok {
		t.Fatalf("WaitForFence(1) = (%v, %v), want (true, nil)", ok, err)
	}
	if _, err := d.WaitForFence(h4b3BadCmd{}, 1, time.Second); err == nil {
		t.Fatal("WaitForFence(wrong type) = nil, want error")
	}
	if err := d.ResetFence(h4b3BadCmd{}); err == nil {
		t.Fatal("ResetFence(wrong type) = nil, want error")
	}
	if _, err := d.GetFenceStatus(h4b3BadCmd{}); err == nil {
		t.Fatal("GetFenceStatus(wrong type) = nil, want error")
	}
	if err := d.ResetFence(f); err != nil {
		t.Fatalf("ResetFence() = %v, want nil", err)
	}
	if signaled, _ := d.GetFenceStatus(f); signaled {
		t.Fatal("GetFenceStatus() after Reset = true, want false")
	}
	d.DestroyFence(f) // 有 ctx 时走 Lock 路径，不 panic。
	d.DestroyFence(nil)
	if err := d.WaitIdle(); err != nil {
		t.Fatalf("WaitIdle() = %v, want nil", err)
	}
}
