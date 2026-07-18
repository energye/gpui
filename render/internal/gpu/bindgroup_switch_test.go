//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
)

// TestBindGroupLayoutSwitch_StencilThenSDF reproduces the mem_anim crash class:
// stencil_fill_bind left bound when switching to sdf_render pipeline in one pass.
func TestBindGroupLayoutSwitch_StencilThenSDF(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()

	const w, h uint32 = 64, 64
	sampleCount := uint32(1)

	sdf := NewSDFRenderPipeline(device, queue, sampleCount)
	defer sdf.Destroy()
	if err := sdf.ensurePipelineWithStencil(); err != nil {
		t.Fatalf("sdf pipeline: %v", err)
	}

	stencil := NewStencilRenderer(device, queue, sampleCount)
	defer stencil.Destroy()
	if err := stencil.createPipelines(); err != nil {
		t.Fatalf("stencil pipelines: %v", err)
	}

	colorTex, err := device.CreateTexture(&webgpu.TextureDescriptor{
		Label:  "bg_switch_color",
		Size:   webgpu.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
		Format: types.TextureFormatBGRA8Unorm,
		Usage:  types.TextureUsageRenderAttachment,
	})
	if err != nil {
		t.Fatalf("color tex: %v", err)
	}
	defer colorTex.Release()
	colorView, err := device.CreateTextureView(colorTex, nil)
	if err != nil {
		t.Fatalf("color view: %v", err)
	}
	defer colorView.Release()

	dsTex, err := device.CreateTexture(&webgpu.TextureDescriptor{
		Label:  "bg_switch_ds",
		Size:   webgpu.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
		Format: types.TextureFormatDepth24PlusStencil8,
		Usage:  types.TextureUsageRenderAttachment,
	})
	if err != nil {
		t.Fatalf("ds tex: %v", err)
	}
	defer dsTex.Release()
	dsView, err := device.CreateTextureView(dsTex, nil)
	if err != nil {
		t.Fatalf("ds view: %v", err)
	}
	defer dsView.Release()

	fan := []float32{10, 10, 50, 10, 50, 50, 10, 50}
	coverQuad := [12]float32{0, 0, float32(w), 0, float32(w), float32(h), 0, 0, float32(w), float32(h), 0, float32(h)}
	bufs, err := stencil.createRenderBuffers(w, h, fan, coverQuad, render.RGBA{R: 1, G: 0, B: 0, A: 1})
	if err != nil {
		t.Fatalf("stencil bufs: %v", err)
	}
	defer bufs.destroy()

	session := NewGPURenderSession(device, queue, sampleCount)
	defer session.Destroy()
	if err := session.ensurePipelines(); err != nil {
		t.Fatalf("ensure pipelines: %v", err)
	}
	// Use session's no-clip / mask bind groups (created by ensurePipelines).
	shapes := []SDFRenderShape{{
		Kind: 0, CenterX: 32, CenterY: 32, Param1: 10, Param2: 10,
		ColorR: 0, ColorG: 1, ColorB: 0, ColorA: 1,
	}}
	sdfRes, err := session.buildSDFResources(shapes, w, h)
	if err != nil {
		t.Fatalf("sdf resources: %v", err)
	}

	enc, err := device.CreateCommandEncoder(&webgpu.CommandEncoderDescriptor{Label: "bg_switch"})
	if err != nil {
		t.Fatalf("encoder: %v", err)
	}
	rp, err := enc.BeginRenderPass(&webgpu.RenderPassDescriptor{
		Label: "bg_switch_pass",
		ColorAttachments: []webgpu.RenderPassColorAttachment{{
			View: colorView, LoadOp: types.LoadOpClear, StoreOp: types.StoreOpStore,
			ClearValue: types.Color{R: 0, G: 0, B: 0, A: 1},
		}},
		DepthStencilAttachment: &webgpu.RenderPassDepthStencilAttachment{
			View:              dsView,
			DepthLoadOp:       types.LoadOpClear,
			DepthStoreOp:      types.StoreOpDiscard,
			DepthClearValue:   1,
			StencilLoadOp:     types.LoadOpClear,
			StencilStoreOp:    types.StoreOpDiscard,
			StencilClearValue: 0,
		},
	})
	if err != nil {
		t.Fatalf("begin pass: %v", err)
	}

	// Order that crashes mem_anim: stencil first (leaves stencil_fill_bind), then SDF.
	stencil.RecordPath(rp, bufs, render.FillRuleNonZero, session.noClipBindGroup, session.frameMaskBindGroup(), render.BlendNormal)
	sdf.RecordDraws(rp, sdfRes, session.noClipBindGroup, session.frameMaskBindGroup())

	if err := rp.End(); err != nil {
		t.Fatalf("end pass: %v", err)
	}
	cmd, err := enc.Finish()
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	// Submit is where wgpu validates; panic/abort means the bug is present.
	queue.Submit(cmd)
}
