//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
)

// TestMSAAColorAttachmentStoresForLaterLoad pins the WebGPU contract that a
// 4x MSAA resolve attachment must be stored, not discarded, because a later
// mid-frame pass may load it. The final pass is not knowable at encode time,
// so every MSAA pass stores (see colorAttachment).
func TestMSAAColorAttachmentStoresForLaterLoad(t *testing.T) {
	device, _, cleanup := createNativeDevice(t)
	defer cleanup()

	makeView := func(label string, samples uint32) (hal.Texture, hal.TextureView) {
		tex, err := device.CreateTexture(&hal.TextureDescriptor{
			Label:         label,
			Size:          hal.Extent3D{Width: 32, Height: 32, DepthOrArrayLayers: 1},
			MipLevelCount: 1,
			SampleCount:   samples,
			Dimension:     types.TextureDimension2D,
			Format:        types.TextureFormatBGRA8Unorm,
			Usage:         types.TextureUsageRenderAttachment,
		})
		if err != nil {
			t.Fatalf("CreateTexture(%s): %v", label, err)
		}
		view, err := device.CreateTextureView(tex, nil)
		if err != nil {
			tex.Destroy()
			t.Fatalf("CreateTextureView(%s): %v", label, err)
		}
		return tex, view
	}

	msaaTex, msaaView := makeView("msaa-repro", 4)
	defer msaaView.Destroy()
	defer msaaTex.Destroy()
	targetTex, targetView := makeView("resolve-repro", 1)
	defer targetView.Destroy()
	defer targetTex.Destroy()

	s := &GPURenderSession{sampleCount: 4}
	s.textures.msaaView = msaaView

	for _, load := range []types.LoadOp{types.LoadOpClear, types.LoadOpLoad} {
		att := s.colorAttachment(targetView, load)
		t.Logf("loadOp=%v storeOp=%v", load, att.StoreOp)
		if att.StoreOp != types.StoreOpStore {
			t.Fatalf("sampleCount=4 loadOp=%v returns StoreOp=%v; a later LoadOpLoad would read undefined MSAA memory", load, att.StoreOp)
		}
	}
}

// TestRenderSessionMSAATwoPassPreservesContent verifies the actual mid-frame
// path: the first 4x MSAA pass clears/stores, the second pass loads the same
// resolve target and must keep the first pass's pixels.
func TestRenderSessionMSAATwoPassPreservesContent(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()

	const W, H = 64, 64
	tex, err := device.CreateTexture(&hal.TextureDescriptor{
		Label:         "msaa-two-pass-target",
		Size:          hal.Extent3D{Width: W, Height: H, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatBGRA8Unorm,
		Usage:         types.TextureUsageRenderAttachment | types.TextureUsageCopySrc,
	})
	if err != nil {
		t.Fatalf("CreateTexture: %v", err)
	}
	defer tex.Destroy()
	view, err := device.CreateTextureView(tex, nil)
	if err != nil {
		t.Fatalf("CreateTextureView: %v", err)
	}
	defer view.Destroy()

	s := NewGPURenderSession(device, queue, 4)
	defer s.Destroy()
	s.BeginFrame()
	target := render.GPURenderTarget{
		View:       packView(view),
		ViewWidth:  W,
		ViewHeight: H,
		Width:      W,
		Height:     H,
	}
	group := func(x, y float64, color [4]float32) []ScissorGroup {
		return []ScissorGroup{{ConvexCommands: []ConvexDrawCommand{{
			Points: []render.Point{{X: x, Y: y}, {X: x + 20, Y: y}, {X: x + 10, Y: y + 20}},
			Color:  color,
		}}}}
	}
	if err := s.RenderFrameGrouped(target, group(4, 4, [4]float32{1, 0, 0, 1}), nil, nil); err != nil {
		t.Fatalf("first MSAA pass: %v", err)
	}
	if err := s.RenderFrameGrouped(target, group(40, 40, [4]float32{0, 1, 0, 1}), nil, nil); err != nil {
		t.Fatalf("second MSAA pass: %v", err)
	}
	data := readbackTexture(t, device, queue, tex, W, H)
	if data == nil {
		t.Skip("readback not available")
	}
	// BGRA8 target: red = (0,0,255), green = (0,255,0).
	pixel := func(x, y int) (byte, byte, byte) {
		i := (y*W + x) * 4
		return data[i+2], data[i+1], data[i]
	}
	if r, g, b := pixel(10, 10); r < 240 || g > 20 || b > 20 {
		t.Fatalf("first pass pixel (10,10) = (%d,%d,%d), want red", r, g, b)
	}
	if r, g, b := pixel(50, 50); r > 20 || g < 240 || b > 20 {
		t.Fatalf("second pass pixel (50,50) = (%d,%d,%d), want green", r, g, b)
	}
}
