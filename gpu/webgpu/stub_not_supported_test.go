//go:build !(js && wasm)

package webgpu

import (
	"errors"
	"testing"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

// H4-b2 桩定案（webgpu 对照侧）：QuerySet 恒返 ErrTimestampsNotSupported，
// Bundle/加速结构返错不支持，Destroy/空输入 nil-safe。零 native 依赖，
// &Device{} 零值即可（对照 H2B 口径）。

func TestH4B2_WebGPUCreateStubsNotSupported(t *testing.T) {
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
	if _, err := d.CreateAccelerationStructure(&hal.AccelerationStructureDescriptor{}); err == nil {
		t.Fatal("CreateAccelerationStructure must error (no ray tracing)")
	}
}

func TestH4B2_WebGPUDestroyStubsNilSafe(t *testing.T) {
	d := &Device{}

	d.DestroyQuerySet(nil)
	d.DestroyRenderBundle(nil)
	d.DestroyAccelerationStructure(nil)
	d.FreeCommandBuffer(nil)
	if got := d.GetAccelerationStructureBuildSizes(nil); got != (hal.AccelerationStructureBuildSizes{}) {
		t.Fatalf("GetAccelerationStructureBuildSizes nil = %+v, want zero", got)
	}
	if got := d.GetAccelerationStructureDeviceAddress(nil); got != 0 {
		t.Fatalf("GetAccelerationStructureDeviceAddress nil = %d, want 0", got)
	}
	if got := d.TlasInstanceToBytes(hal.TlasInstance{}); got != nil {
		t.Fatalf("TlasInstanceToBytes = %v, want nil", got)
	}
}

func TestH4B2_WebGPUEncoderStubsNilSafe(t *testing.T) {
	enc := &CommandEncoder{}
	enc.ResolveQuerySet(nil, 0, 0, nil, 0)
	enc.BuildAccelerationStructures(nil)
	enc.BuildAccelerationStructures([]hal.BuildAccelerationStructureDescriptor{})
	enc.PlaceAccelerationStructureBarrier(hal.AccelerationStructureBarrier{})
	enc.CopyAccelerationStructure(nil, nil, types.AccelerationStructureCopyModeClone)
	enc.ReadAccelerationStructureCompactSize(nil, nil, 0)

	rp := &RenderPassEncoder{}
	rp.ExecuteBundle(nil)
}
