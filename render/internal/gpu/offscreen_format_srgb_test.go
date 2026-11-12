//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

// P3.1b-1 能力钉：opt-in 线性目标允许创建 sRGB 离屏面。先写会红（白名单
// 还没有 sRGB），再把 sRGB 加进 offscreenEngineFormats。
func TestP31bOffscreenSrgbWhitelist(t *testing.T) {
	for _, f := range []types.TextureFormat{
		types.TextureFormatRGBA8UnormSrgb,
		types.TextureFormatBGRA8UnormSrgb,
	} {
		if !offscreenEngineFormats(f) {
			t.Fatalf("offscreenEngineFormats(%v) = false, want true for opt-in linear target", f)
		}
	}
	// Default formats must remain accepted; unrelated formats must stay closed.
	for _, f := range []types.TextureFormat{
		types.TextureFormatBGRA8Unorm,
		types.TextureFormatRGBA8Unorm,
		types.TextureFormatRGBA16Float,
	} {
		if !offscreenEngineFormats(f) {
			t.Fatalf("offscreenEngineFormats(%v) = false, want true", f)
		}
	}
	if offscreenEngineFormats(types.TextureFormatDepth32Float) {
		t.Fatal("depth format must stay rejected")
	}
}

// 真 GPU 探针：1×1 clear pass 是否接受 sRGB 目标（M5 同款判据）。
func TestP31bOffscreenSrgbRenderable(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()
	for _, f := range []types.TextureFormat{
		types.TextureFormatRGBA8UnormSrgb,
		types.TextureFormatBGRA8UnormSrgb,
	} {
		if !probeOffscreenFormatRenderable(device, queue, f) {
			t.Fatalf("probeOffscreenFormatRenderable(%v) = false", f)
		}
	}
}

// 引擎级：CreateOffscreenTextureFormat 接受 sRGB、按 4 B/px 记账、可作 clear
// 目标、释放后账本归零；sRGB 与 Unorm 的池键必须分开。
func TestP31bOffscreenSrgbTargetAccounting(t *testing.T) {
	t.Setenv("GPUI_NO_OFFSCREEN_POOL", "1")
	shared, rc := newP2Shared(t)

	if offscreenKey(64, 64, types.TextureFormatRGBA8UnormSrgb) == offscreenKey(64, 64, types.TextureFormatRGBA8Unorm) {
		t.Fatal("sRGB and Unorm offscreen keys must differ")
	}
	before := hal.VramLiveBytes()
	view, release, err := rc.CreateOffscreenTextureFormat(64, 64, types.TextureFormatRGBA8UnormSrgb)
	if err != nil {
		t.Fatalf("create RGBA8UnormSrgb offscreen: %v", err)
	}
	if view.IsNil() {
		t.Fatal("sRGB offscreen view is nil")
	}
	if got := unpackView(view).Texture().Format(); got != types.TextureFormatRGBA8UnormSrgb {
		t.Fatalf("view texture format = %v, want RGBA8UnormSrgb", got)
	}
	if got, want := hal.VramLiveBytes()-before, uint64(64*64*4); got != want {
		t.Fatalf("VRAM delta = %d, want %d (8-bit sRGB is 4 B/px)", got, want)
	}
	device, queue := shared.Device(), shared.Queue()
	enc, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "p31b_srgb_clear"})
	if err != nil {
		t.Fatalf("encoder: %v", err)
	}
	rp, err := enc.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "p31b_srgb_clear_pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View: unpackView(view), LoadOp: types.LoadOpClear, StoreOp: types.StoreOpStore,
			ClearValue: types.Color{R: 0.5, G: 0.25, B: 0.1, A: 1},
		}},
	})
	if err != nil {
		t.Fatalf("begin sRGB clear pass: %v", err)
	}
	if err := rp.End(); err != nil {
		t.Fatalf("end sRGB clear pass: %v", err)
	}
	cmd, err := enc.Finish()
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	if _, err := queue.Submit(cmd); err != nil {
		t.Fatalf("submit: %v", err)
	}
	device.FreeCommandBuffer(cmd)
	release()
	if got := hal.VramLiveBytes(); got != before {
		t.Fatalf("VRAM after release = %d, want baseline %d", got, before)
	}
}
