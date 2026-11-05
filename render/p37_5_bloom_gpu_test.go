//go:build !nogpu

//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// P3.7 批 5 · 步骤 2：泛光的 GPU 那条腿（四 pass：亮部提取 → 可分离高斯 H → V → 加法合成）。
//
// 这里要证的不是「能跑」，而是**两路同式**：同一张图、同一组参数，GPU 四 pass 与 CPU 8-bit
// 节点必须落在同一档像素上（容差写死成常量并说明来源），否则「渲染层只有一份泛光算法」这句
// 话就是空的。恒等与「全场低于阈值」两条要求 GPU 腿逐位不动——pass 照走、结果不许变。
package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

// bloomProbe 走 ctx.Image()：GPU 图跑完后这条口会带回落后的像素（P3.4 的
// tile/lighting 对齐钉就是这么比两路的，同一把尺子）。
func bloomProbe(ctx *render.Context, x, y int) (uint8, uint8, uint8, uint8) {
	return filterPixel(ctx.Image(), x, y)
}

// 亮心场景：暗底（alpha 0）+ 一颗不透明亮心 + 同值不同色的红绿块，两路都从这张图出发。
// 红绿块是给对齐钉加鉴别力的：Rec.709 权重写反（GPU 或 CPU 任一侧）会让红块与绿块的
// 亮部提取量差出一个量级，只有带同亮度不同色的图才量得到；纯白场景对此完全瞎。
func bloomSpotScene(ctx *render.Context, w, h, cx, cy int) {
	ctx.Clear()
	ctx.SetRGBA(1, 1, 1, 1)
	ctx.DrawRectangle(float64(cx), float64(cy), 1, 1)
	_ = ctx.Fill()
	ctx.SetRGBA(0.95, 0, 0, 1)
	ctx.DrawRectangle(float64(cx-5), float64(cy+4), 3, 3)
	_ = ctx.Fill()
	ctx.SetRGBA(0, 0.95, 0, 1)
	ctx.DrawRectangle(float64(cx+3), float64(cy+4), 3, 3)
	_ = ctx.Fill()
}

// 钉 1：ApplyBloom 真走 GPU 图（零 CPU 回落），且均匀亮场按手算顶到纯白。
func TestP375BloomGPURunsGraph(t *testing.T) {
	requireFilterOpsGPU(t)
	const w, h = 12, 12
	ctx := render.NewContext(w, h)
	defer ctx.Close()
	fillUniform(ctx, 0.8)
	ctx.ResetRenderPathStats()
	ctx.ApplyBloom(render.BloomParams{Threshold: 0, Sigma: 2, Intensity: 0.5})
	if stats := ctx.RenderPathStats(); stats.GPUOps == 0 || stats.CPUFallbackOps != 0 {
		t.Fatalf("ApplyBloom 没走 GPU 图：%s", stats.LogLine())
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := bloomProbe(ctx, x, y)
			if r != 255 || g != 255 || b != 255 || a != 255 {
				t.Fatalf("(%d,%d) = %d,%d,%d,%d，want 全 255（v=0.8、thr=0 ⇒ m=0.8 ⇒ 0.8+0.5*0.64=1.12 钳到 1）",
					x, y, r, g, b, a)
			}
		}
	}
}

// 钉 2：两路同式——GPU 四 pass 与 CPU 8-bit 节点在同一场景上的最大通道差必须在声明容差内。
// 容差来源（写死，不设暗档）：两条腿都是 uint8 量化（1/255）+ float32 累加，
// GPU 在着色器里逐抽头算 exp 并自归一（与 CPU 的共享核同一函数式、不同舍入次序）。
func TestP375BloomGPUMatchesCPULeg(t *testing.T) {
	requireFilterOpsGPU(t)
	const w, h = 24, 24
	params := render.BloomParams{Threshold: 0.4, Sigma: 1.5, Intensity: 2}

	gpu := render.NewContext(w, h)
	defer gpu.Close()
	bloomSpotScene(gpu, w, h, 12, 12)
	gpu.ResetRenderPathStats()
	gpu.ApplyBloom(params)
	if stats := gpu.RenderPathStats(); stats.GPUOps == 0 || stats.CPUFallbackOps != 0 {
		t.Fatalf("对照的 GPU 腿没走图：%s", stats.LogLine())
	}
	gpuImg := gpu.Image()

	cpu := render.NewContext(w, h)
	defer cpu.Close()
	bloomSpotScene(cpu, w, h, 12, 12)
	restore := render.DisableGPUFilterGraphForTest()
	cpu.ApplyBloom(params)
	restore()
	cpuImg := cpu.Image()

	if d := filterMaxDiff(gpuImg, cpuImg); d > bloom8BitSlack {
		t.Fatalf("两路不同式：GPU 与 CPU 8-bit 泛光最大通道差 %d，容差 %d（同参同图必须同档）", d, bloom8BitSlack)
	}
}

// 钉 3：恒等不许驱动 pass，也不许改像素（节点在进门处就被判掉）。
func TestP375BloomGPUIdentitySkipsGraph(t *testing.T) {
	requireFilterOpsGPU(t)
	const w, h = 16, 16
	ctx := render.NewContext(w, h)
	defer ctx.Close()
	bloomSpotScene(ctx, w, h, 8, 8)
	before := ctx.Image()
	ctx.ResetRenderPathStats()
	ctx.ApplyBloom(render.BloomParams{Threshold: 0.1, Sigma: 3, Intensity: 0})
	if stats := ctx.RenderPathStats(); stats.GPUOps != 0 {
		t.Fatalf("恒等参数却驱动了 GPU pass：%s", stats.LogLine())
	}
	if d := filterMaxDiff(before, ctx.Image()); d != 0 {
		t.Fatalf("恒等参数改了像素，最大差 %d（want 0）", d)
	}
}

// 钉 4：全场低于阈值 ⇒ 掩膜处处 0 ⇒ pass 照走而输出逐位等于输入（与 CPU 腿同一判据）。
func TestP375BloomGPUDarkFieldUnchanged(t *testing.T) {
	requireFilterOpsGPU(t)
	const w, h = 12, 12
	ctx := render.NewContext(w, h)
	defer ctx.Close()
	fillUniform(ctx, 0.8)
	before := ctx.Image()
	ctx.ResetRenderPathStats()
	ctx.ApplyBloom(render.BloomParams{Threshold: 0.9, Sigma: 3, Intensity: render.MaxBloomIntensity})
	if stats := ctx.RenderPathStats(); stats.GPUOps == 0 {
		t.Fatalf("非恒等参数应走 GPU 图，却没驱动任何 pass：%s", stats.LogLine())
	}
	if d := filterMaxDiff(before, ctx.Image()); d != 0 {
		t.Fatalf("全场低于阈值却改了像素，最大差 %d（want 0）", d)
	}
}

// 钉 5：亮心向外的高斯衰减在 GPU 腿同样成立（窗口内等值＝盒，全 0＝没加法）。
func TestP375BloomGPUSpreadDecays(t *testing.T) {
	requireFilterOpsGPU(t)
	const w, h = 15, 15
	ctx := render.NewContext(w, h)
	defer ctx.Close()
	bloomSpotScene(ctx, w, h, 7, 7)
	ctx.ResetRenderPathStats()
	ctx.ApplyBloom(render.BloomParams{Threshold: 0.9, Sigma: 1, Intensity: render.MaxBloomIntensity})
	if stats := ctx.RenderPathStats(); stats.GPUOps == 0 {
		t.Fatalf("泛光没走 GPU：%s", stats.LogLine())
	}
	nearR, _, _, _ := bloomProbe(ctx, 8, 7)
	farR, _, _, _ := bloomProbe(ctx, 9, 7)
	tailR, _, _, _ := bloomProbe(ctx, 10, 7)
	// 判据只压到 d=2：σ=1 的第三枚抽头是 0.0044 档，乘完掩膜与增益后落在
	// 1/255 以下，让它「必须大于 0」就是拿量化余量当断言（实测 d=3 在 0/1 之间翻）。
	// 与盒模糊的分界在 d=1 vs d=2：盒是窗口内等值，高斯必严格衰减。
	if !(nearR > farR && farR > 0) {
		t.Fatalf("GPU 辉光剖面 %d/%d 不是严格衰减（等值就是盒模糊，0 就是没加法合成）", nearR, farR)
	}
	if tailR > farR {
		t.Fatalf("GPU 辉光剖面反弹：%d/%d/%d", nearR, farR, tailR)
	}
	untouchedR, _, _, _ := bloomProbe(ctx, 2, 2)
	if untouchedR != 0 {
		t.Fatalf("远处（d=5）被点亮：%d，want 0", untouchedR)
	}
}

// 钉 6：加法只碰 RGB，alpha 保留 src（与 CPU 腿、hdr float 腿同一契约）。
func TestP375BloomGPUKeepsSourceAlpha(t *testing.T) {
	requireFilterOpsGPU(t)
	const w, h = 12, 12
	ctx := render.NewContext(w, h)
	defer ctx.Close()
	ctx.ClearWithColor(render.RGBA{R: 0.6, G: 0.6, B: 0.6, A: 0.5})
	ctx.ResetRenderPathStats()
	ctx.ApplyBloom(render.BloomParams{Threshold: 0, Sigma: 2, Intensity: render.MaxBloomIntensity})
	_, _, _, a := bloomProbe(ctx, 6, 6)
	if a < 126 || a > 129 {
		t.Fatalf("alpha = %d，want ~128（0.5 的半透明底不许被泛光改写）", a)
	}
}

// 钉 7：一枚 bloom 吃 5 个 pass（原图暂存 + 提取 + H + V + 加法）。连排三枚必须
// 仍全在 GPU 图上跑完、一枚都不回落 CPU（多节点链是分派器最容易漏的形态）。
// 注：pass 槽估算（每节点 5）实测不是行为保证——运行中有 in-place 增长兜底，
// 把估算改成固定 4 槽也不红，所以它按「省一次增长的尺寸选择」登记，不作为绿证。
func TestP375BloomGPUMultiNodeChainStaysOnGraph(t *testing.T) {
	requireFilterOpsGPU(t)
	const w, h = 16, 16
	ctx := render.NewContext(w, h)
	defer ctx.Close()
	bloomSpotScene(ctx, w, h, 8, 8)
	ctx.ResetRenderPathStats()
	ctx.ApplyImageFilterGraph(
		render.ImageFilterNode{Kind: render.ImageFilterBloom,
			Bloom: render.BloomParams{Threshold: 0.8, Sigma: 1, Intensity: 0.2}},
		render.ImageFilterNode{Kind: render.ImageFilterBloom,
			Bloom: render.BloomParams{Threshold: 0.8, Sigma: 2, Intensity: 0.2}},
		render.ImageFilterNode{Kind: render.ImageFilterBloom,
			Bloom: render.BloomParams{Threshold: 0.8, Sigma: 3, Intensity: 0.2}},
	)
	if stats := ctx.RenderPathStats(); stats.GPUOps == 0 || stats.CPUFallbackOps != 0 {
		t.Fatalf("三枚 bloom 没全走 GPU 图：%s", stats.LogLine())
	}
	nearR, _, _, _ := bloomProbe(ctx, 9, 8)
	if nearR == 0 {
		t.Fatal("三枚 bloom 叠加后亮心近旁没有辉光，链没跑起来")
	}
}

// 钉 8：恒等与非法参数都不许占 pass——直接从滤镜图入口塞进去测。
// 这条是「图级判定」的牙齿：ApplyBloom 自己在门口拦掉了恒等/非法参数，
// 但图入口（ApplyImageFilterGraph）也要拦，否则一枚恒等节点会白跑五个 pass。
func TestP375BloomGraphSkipsIdentityAndBadNodes(t *testing.T) {
	requireFilterOpsGPU(t)
	const w, h = 16, 16

	opsOf := func(nodes ...render.ImageFilterNode) (int, uint8) {
		ctx := render.NewContext(w, h)
		defer ctx.Close()
		bloomSpotScene(ctx, w, h, 8, 8)
		ctx.ResetRenderPathStats()
		ctx.ApplyImageFilterGraph(nodes...)
		return int(ctx.RenderPathStats().GPUOps), func() uint8 {
			r, _, _, _ := bloomProbe(ctx, 4, 4)
			return r
		}()
	}

	base, baseR := opsOf(render.ImageFilterNode{Kind: render.ImageFilterGrayscale})
	withIdentity, idR := opsOf(
		render.ImageFilterNode{Kind: render.ImageFilterBloom,
			Bloom: render.BloomParams{Threshold: 0.2, Sigma: 2, Intensity: 0}},
		render.ImageFilterNode{Kind: render.ImageFilterGrayscale},
	)
	withBad, badR := opsOf(
		render.ImageFilterNode{Kind: render.ImageFilterBloom,
			Bloom: render.BloomParams{Threshold: 3, Sigma: 2, Intensity: 1}},
		render.ImageFilterNode{Kind: render.ImageFilterGrayscale},
	)
	if withIdentity != base || withBad != base {
		t.Fatalf("恒等/非法 bloom 占了 pass：基线 %d、恒等 %d、非法 %d（want 后两者等于基线）",
			base, withIdentity, withBad)
	}
	if idR != baseR || badR != baseR {
		t.Fatalf("恒等/非法 bloom 改了画面：基线 %d、恒等 %d、非法 %d", baseR, idR, badR)
	}
}
