package rwgpu

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
)

func TestRenderPassClearAndSubmit(t *testing.T) {
	instance, err := CreateInstance(nil)
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}
	defer instance.Release()

	adapter, err := instance.RequestAdapter(nil)
	if err != nil {
		t.Fatalf("RequestAdapter failed: %v", err)
	}
	defer adapter.Release()

	device, err := adapter.RequestDevice(nil)
	if err != nil {
		t.Fatalf("RequestDevice failed: %v", err)
	}
	defer device.Release()

	texture, err := device.CreateTexture(&TextureDescriptor{
		Usage:     types.TextureUsageRenderAttachment | types.TextureUsageCopySrc,
		Dimension: types.TextureDimension2D,
		Size: types.Extent3D{
			Width:              4,
			Height:             4,
			DepthOrArrayLayers: 1,
		},
		Format:        types.TextureFormatRGBA8Unorm,
		MipLevelCount: 1,
		SampleCount:   1,
	})
	if err != nil {
		t.Fatalf("CreateTexture failed: %v", err)
	}
	defer texture.Release()

	view, err := texture.CreateView(nil)
	if err != nil {
		t.Fatalf("CreateView failed: %v", err)
	}
	defer view.Release()

	encoder, err := device.CreateCommandEncoder(nil)
	if err != nil {
		t.Fatalf("CreateCommandEncoder failed: %v", err)
	}
	defer encoder.Release()

	pass, err := encoder.BeginRenderPass(&RenderPassDescriptor{
		ColorAttachments: []RenderPassColorAttachment{
			{
				View:    view,
				LoadOp:  types.LoadOpClear,
				StoreOp: types.StoreOpStore,
				ClearValue: Color{
					R: 0.1,
					G: 0.2,
					B: 0.3,
					A: 1.0,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("BeginRenderPass failed: %v", err)
	}
	pass.SetViewport(0, 0, 4, 4, 0, 1)
	pass.End()
	pass.Release()

	cmdBuffer, err := encoder.Finish(nil)
	if err != nil {
		t.Fatalf("Finish failed: %v", err)
	}
	defer cmdBuffer.Release()

	queue := device.Queue()
	if queue == nil {
		t.Fatal("Queue returned nil")
	}
	defer queue.Release()

	if _, err := queue.Submit(cmdBuffer); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	device.Poll(true)
}
