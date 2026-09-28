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

// H4-b2 桩定案（Metal 侧）：QuerySet 恒返 ErrTimestampsNotSupported
// （计数器待实现），Bundle 不支持，Destroy/空输入 nil-safe。
// 只用零值 Device 与未录制编码器（cmdBuffer == 0），不碰 MTL 对象。

func TestH4B2_MetalCreateStubsNotSupported(t *testing.T) {
	d := &Device{}

	if _, err := d.CreateQuerySet(nil); !errors.Is(err, hal.ErrTimestampsNotSupported) {
		t.Fatalf("CreateQuerySet nil = %v, want ErrTimestampsNotSupported", err)
	}
	if _, err := d.CreateQuerySet(&hal.QuerySetDescriptor{Count: 1}); !errors.Is(err, hal.ErrTimestampsNotSupported) {
		t.Fatalf("CreateQuerySet = %v, want ErrTimestampsNotSupported", err)
	}
	if _, err := d.CreateRenderBundleEncoder(nil); err == nil {
		t.Fatal("CreateRenderBundleEncoder nil desc must error")
	}
	if _, err := d.CreateRenderBundleEncoder(&hal.RenderBundleEncoderDescriptor{}); err == nil {
		t.Fatal("CreateRenderBundleEncoder must error (bundles unsupported)")
	}
	if _, err := d.CreateAccelerationStructure(nil); err == nil {
		t.Fatal("CreateAccelerationStructure nil desc must error")
	}
}

func TestH4B2_MetalDestroyStubsNilSafe(t *testing.T) {
	d := &Device{}

	d.DestroyQuerySet(nil)
	d.DestroyRenderBundle(nil)
	d.DestroyAccelerationStructure(nil)
	d.FreeCommandBuffer(nil)
	if got := d.GetAccelerationStructureDeviceAddress(nil); got != 0 {
		t.Fatalf("GetAccelerationStructureDeviceAddress nil = %d, want 0", got)
	}
}

func TestH4B2_MetalEncoderStubsNilSafe(t *testing.T) {
	enc := &CommandEncoder{}
	enc.ResolveQuerySet(nil, 0, 0, nil, 0)
	enc.BuildAccelerationStructures(nil)
	enc.BuildAccelerationStructures([]hal.BuildAccelerationStructureDescriptor{})
	enc.PlaceAccelerationStructureBarrier(hal.AccelerationStructureBarrier{})
	enc.CopyAccelerationStructure(nil, nil, gputypes.AccelerationStructureCopyModeClone)
	enc.ReadAccelerationStructureCompactSize(nil, nil, 0)

	rp := &RenderPassEncoder{}
	rp.ExecuteBundle(nil)
}
