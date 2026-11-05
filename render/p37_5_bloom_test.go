//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// P3.7 批 5 · 步骤 2（GPU 四 pass）第一步的渲染层侧钉：泛光参数真源 + CPU 8-bit 那条腿。
//
// 参数为什么在 `render` 而不在 `render/hdr`（步骤 1 曾放 hdr）：步骤 2 要把泛光接进渲染层
// 自己的滤镜图，而 `render/hdr → render/internal/filter → render` 已成环——`render` 引不到
// `hdr`，真源只能在上层。这也正是 design §2 的原话与台账 M5 行「与本项批 5 的泛光参数真源
// 保持同一组数字，禁止第二套」的要求。摘掉下面任何一条必红。
package render_test

import (
	"math"
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	"github.com/energye/gpui/render/hdr"
)

// 8-bit 出口的容差：两条腿都是 uint8 量化 + float32 累加，1/255 一档，两档是实测上界。
const bloom8BitSlack = 2

func fillUniform(ctx *render.Context, v float64) {
	ctx.ClearWithColor(render.RGBA{R: v, G: v, B: v, A: 1})
}

// 钉 1：参数域是渲染层唯一真源——具名错误、零值合法且恒等、上限值本身在域内。
func TestP375BloomParamsDomainSource(t *testing.T) {
	var zero render.BloomParams
	if err := zero.Validate(); err != nil {
		t.Fatalf("零值参数必须合法，却报：%v", err)
	}
	if !zero.IsIdentity() {
		t.Fatal("零值参数必须报恒等")
	}
	top := render.BloomParams{Threshold: 1, Sigma: render.MaxBloomSigma, Intensity: render.MaxBloomIntensity}
	if err := top.Validate(); err != nil {
		t.Fatalf("两侧上限值必须都在域内，却报：%v", err)
	}
	cases := []struct {
		p    render.BloomParams
		want string
	}{
		{render.BloomParams{Threshold: math.NaN()}, "threshold"},
		{render.BloomParams{Threshold: -0.1}, "threshold"},
		{render.BloomParams{Threshold: 1.5}, "threshold"},
		{render.BloomParams{Sigma: math.Inf(1)}, "sigma"},
		{render.BloomParams{Sigma: -1}, "sigma"},
		{render.BloomParams{Sigma: render.MaxBloomSigma + 1}, "sigma"},
		{render.BloomParams{Intensity: math.NaN()}, "intensity"},
		{render.BloomParams{Intensity: -0.5}, "intensity"},
		{render.BloomParams{Intensity: render.MaxBloomIntensity + 1}, "intensity"},
	}
	for _, c := range cases {
		err := c.p.Validate()
		if err == nil {
			t.Errorf("%+v 却通过校验", c.p)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%+v 的错误没点名参数 %q：%v", c.p, c.want, err)
		}
	}
}

// 钉 2：恒等不进图、逐位不改（Intensity==0）。非法参数同样整笔不动。
func TestP375BloomIdentityAndBadParamsLeavePixels(t *testing.T) {
	const w, h = 8, 8
	for _, p := range []render.BloomParams{
		{Threshold: 0.2, Sigma: 3, Intensity: 0},          // 恒等
		{Threshold: 2, Sigma: 3, Intensity: 1},            // 非法：阈值越界
		{Threshold: 0.5, Sigma: -1, Intensity: 1},         // 非法：σ 负
		{Threshold: 0.5, Sigma: 1, Intensity: math.NaN()}, // 非法：非有限
	} {
		ctx := render.NewContext(w, h)
		fillUniform(ctx, 0.8)
		before := ctx.Image()
		ctx.ResetRenderPathStats()
		ctx.ApplyBloom(p)
		if stats := ctx.RenderPathStats(); stats.GPUOps != 0 {
			t.Errorf("%+v 恒等/非法却驱动了 GPU 滤镜 pass：%s", p, stats.LogLine())
		}
		if d := filterMaxDiff(before, ctx.Image()); d != 0 {
			t.Errorf("%+v 改了像素，最大差 %d（want 0）", p, d)
		}
		ctx.Close()
	}
}

// 钉 3：均匀亮场过完泛光仍均匀，且数值可手算——这条同时钉住核归一、边缘夹取、
// 与 8-bit 出口的 clamp01（v=0.8、thr=0 ⇒ m=0.8 ⇒ 辉=0.8*0.8=0.64 ⇒ 0.8+0.5*0.64=1.12 → 钳到 1）。
func TestP375BloomCPULegUniformFieldExact(t *testing.T) {
	const w, h = 12, 12
	ctx := render.NewContext(w, h)
	defer ctx.Close()
	fillUniform(ctx, 0.8)
	restore := render.DisableGPUFilterGraphForTest()
	ctx.ApplyBloom(render.BloomParams{Threshold: 0, Sigma: 2, Intensity: 0.5})
	restore()

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := filterRawPixel(ctx, x, y)
			if r != 255 || g != 255 || b != 255 || a != 255 {
				t.Fatalf("(%d,%d) = %d,%d,%d,%d，want 全 255（1.12 钳到 1，alpha 不动）", x, y, r, g, b, a)
			}
		}
	}
}

// 钉 4：阈值把整场挡在门外 ⇒ 逐位不变（v=0.8 < thr=0.9 ⇒ 掩膜处处 0）。
func TestP375BloomCPULegThresholdBlocksAll(t *testing.T) {
	const w, h = 12, 12
	ctx := render.NewContext(w, h)
	defer ctx.Close()
	fillUniform(ctx, 0.8)
	before := ctx.Image()
	restore := render.DisableGPUFilterGraphForTest()
	ctx.ApplyBloom(render.BloomParams{Threshold: 0.9, Sigma: 3, Intensity: render.MaxBloomIntensity})
	restore()
	if d := filterMaxDiff(before, ctx.Image()); d != 0 {
		t.Fatalf("全场低于阈值却改了像素，最大差 %d（want 0）", d)
	}
}

// 钉 5：加法饱和不回绕——纯白再灌满强度还是纯白（RGBA8 的 LDR 本质，声明就在这条上）。
func TestP375BloomCPULegClampsNoWrap(t *testing.T) {
	const w, h = 8, 8
	ctx := render.NewContext(w, h)
	defer ctx.Close()
	fillUniform(ctx, 1)
	restore := render.DisableGPUFilterGraphForTest()
	ctx.ApplyBloom(render.BloomParams{Threshold: 0.5, Sigma: 1, Intensity: render.MaxBloomIntensity})
	restore()
	r, g, b, a := filterRawPixel(ctx, 4, 4)
	if r != 255 || g != 255 || b != 255 || a != 255 {
		t.Fatalf("纯白加满泛光后 = %d,%d,%d,%d，want 255（不许回绕成暗值）", r, g, b, a)
	}
}

// 钉 6：亮心在暗场里向外衰减（高斯形态，不是盒的窗口内等值），暗处远处的像素不被改动。
func TestP375BloomCPULegSpreadDecays(t *testing.T) {
	const w, h = 15, 15
	ctx := render.NewContext(w, h)
	defer ctx.Close()
	ctx.Clear()
	ctx.SetRGBA(1, 1, 1, 1)
	ctx.DrawRectangle(7, 7, 1, 1)
	_ = ctx.Fill()
	restore := render.DisableGPUFilterGraphForTest()
	ctx.ApplyBloom(render.BloomParams{Threshold: 0.9, Sigma: 1, Intensity: render.MaxBloomIntensity})
	restore()

	nearR, _, _, _ := filterRawPixel(ctx, 8, 7)
	farR, _, _, _ := filterRawPixel(ctx, 9, 7)
	tailR, _, _, _ := filterRawPixel(ctx, 10, 7)
	untouchedR, _, _, _ := filterRawPixel(ctx, 2, 2)
	if !(nearR > farR && farR > tailR && tailR > 0) {
		t.Fatalf("辉光剖面 %d/%d/%d 不是单调衰减（窗口内等值就是盒模糊，0 就是没加光）", nearR, farR, tailR)
	}
	if untouchedR != 0 {
		t.Fatalf("远处（d=5）被点亮：%d，want 0", untouchedR)
	}
}

// 钉 7：新 kind 在滤镜图里被真实消费——灰度图先泛光再灰阶，通道值必须变；
// 图里节点的先后顺序也不许被合并改写（泛光恒等节点被跳过，非恒等节点保留）。
func TestP375BloomNodeIsRoutedInGraph(t *testing.T) {
	const w, h = 12, 12
	ctx := render.NewContext(w, h)
	defer ctx.Close()
	ctx.ClearWithColor(render.RGBA{R: 0.8, G: 0.4, B: 0.2, A: 1})
	before := ctx.Image()
	restore := render.DisableGPUFilterGraphForTest()
	ctx.ApplyImageFilterGraph(
		render.ImageFilterNode{Kind: render.ImageFilterBloom,
			Bloom: render.BloomParams{Threshold: 0, Sigma: 2, Intensity: 1}},
		render.ImageFilterNode{Kind: render.ImageFilterGrayscale},
	)
	restore()
	if d := filterMaxDiff(before, ctx.Image()); d == 0 {
		t.Fatal("泛光节点没进图：整链输出与输入逐位相同")
	}
	r, g, b, _ := filterRawPixel(ctx, 6, 6)
	if r != g || g != b {
		t.Fatalf("泛光后的灰阶通道不相等：%d/%d/%d（图里第二枚灰阶节点被吃掉了）", r, g, b)
	}
}

// 钉 8：LDR 与 float 域的分叉必须正好发生在「>1」这一档——同一颗强光，
// 8-bit 那条腿压到纯白（LDR 近似），hdr 那条腿把 4.0 的光留着等调子映射。
// 摘掉任一侧的性质都红：float 腿提前钳 = 白做；8-bit 腿不回钳 = 溢出。
func TestP375BloomLegsDivergeOnlyAboveOne(t *testing.T) {
	const w, h = 4, 4
	p := render.BloomParams{Threshold: 0.5, Sigma: 1, Intensity: 1}

	ctx := render.NewContext(w, h)
	defer ctx.Close()
	fillUniform(ctx, 1)
	restore := render.DisableGPUFilterGraphForTest()
	ctx.ApplyBloom(p)
	restore()
	r, g, b, _ := filterRawPixel(ctx, 2, 2)
	if r != 255 || g != 255 || b != 255 {
		t.Fatalf("8-bit 腿没把强光钳到白：%d,%d,%d（want 255）", r, g, b)
	}

	target, err := hdr.NewFloatTarget(w, h)
	if err != nil {
		t.Fatalf("NewFloatTarget: %v", err)
	}
	texels := target.FloatPixels()
	for i := range texels {
		texels[i] = 3 // 每通道 3.0：8-bit 表示不出来的光
	}
	glowed, err := target.Bloom(p)
	if err != nil {
		t.Fatalf("Bloom: %v", err)
	}
	got := glowed.At(2, 2)
	// 掩膜按夹到 [0,1] 的亮度算（lum=1、thr=0.5 ⇒ m=1）⇒ 增量 1 ⇒ 3+1=4，不许回钳。
	for i, name := range [3]string{"R", "G", "B"} {
		if d := math.Abs(float64(got[i]) - 4); d > 1e-5 {
			t.Errorf("float 腿的 %s=%v，want 4（>1 的光必须留到调子映射）", name, got[i])
		}
	}
}
