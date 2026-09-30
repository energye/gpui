//go:build (windows || linux) && !(js && wasm)

package gles

import (
	"testing"

	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

func TestH4B2_GlesCreateStubsNotSupported(t *testing.T) {
	d := &Device{}

	if _, err := d.CreateQuerySet(nil); err == nil {
		t.Fatal("CreateQuerySet nil desc must error, not panic")
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
		t.Fatal("CreateAccelerationStructure must error (no ray tracing on GL)")
	}
}

func TestH4B2_GlesDestroyStubsNilSafe(t *testing.T) {
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

func TestH4B2_GlesEncoderStubsNilSafe(t *testing.T) {
	enc := &CommandEncoder{}
	enc.ResolveQuerySet(nil, 0, 0, nil, 0)
	enc.ResolveQuerySet(nil, 0, 1, nil, 0)
	enc.BuildAccelerationStructures(nil)
	enc.BuildAccelerationStructures([]hal.BuildAccelerationStructureDescriptor{})
	enc.PlaceAccelerationStructureBarrier(hal.AccelerationStructureBarrier{})
	enc.CopyAccelerationStructure(nil, nil, gputypes.AccelerationStructureCopyModeClone)
	enc.ReadAccelerationStructureCompactSize(nil, nil, 0)

	rp := &RenderPassEncoder{}
	rp.ExecuteBundle(nil)
}
