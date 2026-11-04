//go:build !nogpu

package gpu

// P3.7 批 3a 内部钉：quad 材质腿（D8，R1 范围=骨架 + 内部命令字段）。
// 判据源：batch3a-design.md §2/§5。像素判据全部「恒等材质 ≡ 无材质逐位」——
// A 档组合保持引擎乘链不变 ⇒ 材质腿与标准图像腿同序同值（探针 P3 §三实证）。
//
// 驱动方式照批 1a 内部钉先例：createNativeTestDevice + NewGPURenderSession +
// 直构 ImageDrawCommand → buildImageResources → 自管 pass → RecordDraws →
// 读回。零公开 API。

import (
	"fmt"
	"image"
	"testing"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
	intImage "github.com/energye/gpui/render/internal/image"
)

const p37AIdentityWGSL = `
@group(0) @binding(1) var p37a_tex: texture_2d<f32>;
@group(0) @binding(2) var p37a_samp: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSample(p37a_tex, p37a_samp, uv);
}`

func p37ACheckerImg(t *testing.T, w, h int) *intImage.ImageBuf {
	t.Helper()
	img, err := intImage.NewImageBuf(w, h, intImage.FormatRGBAPremul)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { img.Dispose() })
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			px := []byte{uint8(40 + ((x*7 + y*13) % 200)), uint8(30 + ((x*11 + y*5) % 220)), uint8(20 + ((x*3 + y*17) % 230)), 255}
			if err := img.SetPixelBytes(x, y, px); err != nil {
				t.Fatal(err)
			}
		}
	}
	img.NotifyPixelsChanged()
	return img
}

func p37AQuadCmd(img *intImage.ImageBuf, x, y float32, mat *render.ShaderEffect) *ImageDrawCommand {
	cmd := p36CmdOf(img)
	cmd.Material = mat
	cmd.TLX, cmd.TLY = x, y
	cmd.TRX, cmd.TRY = x+float32(img.Width()), y
	cmd.BRX, cmd.BRY = x+float32(img.Width()), y+float32(img.Height())
	cmd.BLX, cmd.BLY = x, y+float32(img.Height())
	cmd.DstX, cmd.DstY = x, y
	cmd.DstW, cmd.DstH = float32(img.Width()), float32(img.Height())
	return cmd
}

// p37AMaterialPass runs one buildImageResources + RecordDraws over a cleared
// target and returns the readback RGBA bytes.
func p37AMaterialPass(t *testing.T, s *GPURenderSession, device hal.Device, queue hal.Queue, cmds []ImageDrawCommand, w, h uint32) []byte {
	t.Helper()
	res, err := s.buildImageResources(cmds, w, h, make([]bool, len(cmds)))
	if err != nil {
		t.Fatalf("buildImageResources: %v", err)
	}
	if res == nil {
		t.Fatalf("no image resources")
	}
	tex, err := device.CreateTexture(&hal.TextureDescriptor{
		Label: "p37a_mat_target", Size: hal.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
		MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
		Format: types.TextureFormatBGRA8Unorm,
		Usage:  types.TextureUsageRenderAttachment | types.TextureUsageCopySrc,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer tex.Destroy()
	view, err := device.CreateTextureView(tex, &hal.TextureViewDescriptor{
		Format: types.TextureFormatBGRA8Unorm, Dimension: types.TextureViewDimension2D,
		Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer view.Destroy()
	dsTex, err := device.CreateTexture(&hal.TextureDescriptor{
		Label: "p37a_mat_ds", Size: hal.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
		MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
		Format: types.TextureFormatDepth24PlusStencil8,
		Usage:  types.TextureUsageRenderAttachment,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer dsTex.Destroy()
	dsView, err := device.CreateTextureView(dsTex, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer dsView.Destroy()

	enc, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "p37a_mat_enc"})
	if err != nil {
		t.Fatal(err)
	}
	rp, err := enc.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "p37a_mat_pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View: view, LoadOp: types.LoadOpClear, StoreOp: types.StoreOpStore,
			ClearValue: types.Color{R: 1, G: 1, B: 1, A: 1},
		}},
		DepthStencilAttachment: &hal.RenderPassDepthStencilAttachment{
			View: dsView, DepthLoadOp: types.LoadOpClear, DepthStoreOp: types.StoreOpDiscard,
			DepthClearValue: 1, StencilLoadOp: types.LoadOpClear, StencilStoreOp: types.StoreOpDiscard,
			StencilClearValue: 0,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	s.imagePipeline.RecordDraws(rp, res, s.noClipBindGroup, false)
	rp.End()
	cmd, err := enc.Finish()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := queue.Submit(cmd); err != nil {
		t.Fatal(err)
	}
	_ = device.WaitIdle()
	pix, err := readTextureViewRegionRGBA(device, queue, packView(view), image.Rect(0, 0, int(w), int(h)), int(w), int(h))
	if err != nil {
		t.Fatal(err)
	}
	return pix
}

// TestP37AMaterialLeg 父入口：一台真设备一个会话跑全部材质腿钉（本机 wgpu
// 设备数有限，逐测试建设备会撞 RequestDevice 上限——A3 同族）。
func TestP37AMaterialLeg(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	defer cleanup()
	s := NewGPURenderSession(device, queue, 1)
	defer s.Destroy()

	t.Run("IdentityEqualsStandard", func(t *testing.T) {
		img := p37ACheckerImg(t, 16, 16)
		effect, err := render.NewShaderEffect(p37AIdentityWGSL)
		if err != nil {
			t.Fatalf("NewShaderEffect: %v", err)
		}
		defer effect.Close()

		const W, H = 64, 32
		cmds := []ImageDrawCommand{
			*p37AQuadCmd(img, 4, 8, nil),     // 标准腿 @ (4,8)
			*p37AQuadCmd(img, 36, 8, effect), // 材质腿 @ (36,8)
		}
		pix := p37AMaterialPass(t, s, device, queue, cmds, W, H)

		diff, worst, at := 0, 0, ""
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				ao := ((8+y)*W + 4 + x) * 4
				bo := ((8+y)*W + 36 + x) * 4
				for c := 0; c < 4; c++ {
					d := int(pix[ao+c]) - int(pix[bo+c])
					if d < 0 {
						d = -d
					}
					if d > 0 {
						diff++
						if d > worst {
							worst = d
							at = fmt.Sprintf("(%d,%d)c%d", 4+x, 8+y, c)
						}
					}
				}
			}
		}
		if diff != 0 {
			t.Fatalf("identity material vs standard: %d differing channels, worst %d at %s — 组合腿乘链偏离标准 FS", diff, worst, at)
		}
	})

	t.Run("TintAndOpacityRideEngineBlock", func(t *testing.T) {
		img := p37ACheckerImg(t, 16, 16)
		effect, err := render.NewShaderEffect(p37AIdentityWGSL)
		if err != nil {
			t.Fatalf("NewShaderEffect: %v", err)
		}
		defer effect.Close()

		const W, H = 64, 32
		std := p37AQuadCmd(img, 4, 8, nil)
		std.TintR, std.TintG, std.TintB, std.TintA = 1.0, 0.5, 0.25, 1.0
		std.Opacity = 0.5
		mat := p37AQuadCmd(img, 36, 8, effect)
		mat.TintR, mat.TintG, mat.TintB, mat.TintA = 1.0, 0.5, 0.25, 1.0
		mat.Opacity = 0.5
		pix := p37AMaterialPass(t, s, device, queue, []ImageDrawCommand{*std, *mat}, W, H)

		diff := 0
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				ao := ((8+y)*W + 4 + x) * 4
				bo := ((8+y)*W + 36 + x) * 4
				for c := 0; c < 4; c++ {
					if pix[ao+c] != pix[bo+c] {
						diff++
					}
				}
			}
		}
		if diff != 0 {
			t.Fatalf("identity material with tint+opacity vs standard: %d differing channels — 引擎块数值与标准 uniform 不同序", diff)
		}
	})

	t.Run("BatchShape", func(t *testing.T) {
		img := p37ACheckerImg(t, 8, 8)
		effect, err := render.NewShaderEffect(p37AIdentityWGSL)
		if err != nil {
			t.Fatal(err)
		}
		defer effect.Close()
		effect2, err := render.NewShaderEffect(p37AIdentityWGSL)
		if err != nil {
			t.Fatal(err)
		}
		defer effect2.Close()

		res, err := s.buildImageResources([]ImageDrawCommand{
			*p37AQuadCmd(img, 0, 0, effect),
			*p37AQuadCmd(img, 16, 0, effect),
		}, 32, 16, make([]bool, 2))
		if err != nil {
			t.Fatal(err)
		}
		if len(res.drawCalls) != 1 || res.drawCalls[0].vertexCount != 12 {
			t.Fatalf("same-effect material neighbours must coalesce into one Draw (got %d calls, vertCnt=%d)", len(res.drawCalls), res.drawCalls[0].vertexCount)
		}

		res2, err := s.buildImageResources([]ImageDrawCommand{
			*p37AQuadCmd(img, 0, 0, effect),
			*p37AQuadCmd(img, 16, 0, nil),
			*p37AQuadCmd(img, 24, 0, effect2),
		}, 48, 16, make([]bool, 3))
		if err != nil {
			t.Fatal(err)
		}
		if len(res2.drawCalls) != 3 {
			t.Fatalf("material/nil/other-effect must not share slots (got %d calls)", len(res2.drawCalls))
		}
	})

	t.Run("Guards", func(t *testing.T) {
		img := p37ACheckerImg(t, 8, 8)
		effect, err := render.NewShaderEffect(p37AIdentityWGSL)
		if err != nil {
			t.Fatal(err)
		}

		persp := p37AQuadCmd(img, 0, 0, effect)
		persp.Perspective = true
		persp.Weights = [4]float32{1, 1, 1, 1}
		if _, err := s.buildImageResources([]ImageDrawCommand{*persp}, 16, 16, []bool{false}); err == nil {
			t.Fatal("material + perspective must be a named rejection in batch 3a")
		}
		blended := p37AQuadCmd(img, 0, 0, effect)
		blended.BlendMode = render.BlendMultiply
		if _, err := s.buildImageResources([]ImageDrawCommand{*blended}, 16, 16, []bool{false}); err == nil {
			t.Fatal("material + non-Normal blend must be a named rejection in batch 3a")
		}
		if err := effect.Close(); err != nil {
			t.Fatal(err)
		}
		closed := p37AQuadCmd(img, 0, 0, effect)
		if _, err := s.buildImageResources([]ImageDrawCommand{*closed}, 16, 16, []bool{false}); err == nil {
			t.Fatal("closed effect must be rejected")
		}
	})

	t.Run("Eviction", func(t *testing.T) {
		// 直调 ensure 前必须先走 ensureImagePipeline（clip 布局在那里就绪），
		// 否则红的是环境不是判据（3e-2 首轮无效变异同族）。
		if err := s.ensureImagePipeline(); err != nil {
			t.Fatal(err)
		}
		effect, err := render.NewShaderEffect(p37AIdentityWGSL)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.ensureMaterialPipeline(effect, false); err != nil {
			t.Fatal(err)
		}
		if _, err := s.ensureMaterialPipeline(effect, true); err != nil {
			t.Fatal(err)
		}
		s.userMaterial.mu.Lock()
		_, alive := s.userMaterial.entries[effect]
		s.userMaterial.mu.Unlock()
		if !alive {
			t.Fatal("effect entry missing after ensure")
		}
		if err := effect.Close(); err != nil {
			t.Fatal(err)
		}
		s.userMaterial.mu.Lock()
		_, alive = s.userMaterial.entries[effect]
		s.userMaterial.mu.Unlock()
		if alive {
			t.Fatal("closed effect still cached — 驱逐钩子失效（L2-P0-1 同族）")
		}
		if _, err := s.ensureMaterialPipeline(effect, false); err == nil {
			t.Fatal("ensure after Close must refuse")
		}
	})
}
