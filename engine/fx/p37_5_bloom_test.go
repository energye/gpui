//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// P3.7 批 5 · 步骤 1：fx 的 CPU 泛光改成薄转调渲染层核。
//
// 这几枚钉全是手算期望（金图 JSON 是回归基线，不背算法正确性）：剖面比例
// 钉住「用的是可分离高斯、不是盒」，1×1 的斜坡值钉住「软斜坡 + LDR 出口钳」，
// 恒等钉 Intensity==0 逐位不变，域钉要求引擎的夹不许宽过渲染层。
// 摘掉任何一处（换回盒、提前钳、自定域、绕过核）必红。
package fx

import (
	"math"
	"strings"
	"testing"

	"github.com/energye/gpui/engine/core"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/hdr"
)

// fx 侧的容差：颜色要过渲染层的 float32 存储，相对误差 1e-7 量级；
// 这不是 8-bit 量化（1/255≈4e-3），但也不再是纯 float64 逐位回放。
const bloomSlack = 1e-6

func imageFromColors(t *testing.T, w, h int, colors ...core.Color) *Image {
	t.Helper()
	img, err := NewImageFromColors(w, h, colors)
	if err != nil {
		t.Fatalf("NewImageFromColors: %v", err)
	}
	return img
}

// 钉 1：剖面必须是高斯的指数衰减，不是盒的窗口内等值。
// 15×1 单亮心（σ=1 ⇒ 核半径 ceil(3σ)=3），心距 d 的辉按 exp(-d²/2) 衰减。
func TestP375BloomProfileIsGaussianDecay(t *testing.T) {
	colors := make([]core.Color, 15)
	for i := range colors {
		colors[i] = core.RGBA(0, 0, 0, 1)
	}
	colors[7] = core.RGBA(1, 1, 1, 1)
	img := imageFromColors(t, 15, 1, colors...)

	b, err := NewBloom(0.9, 1, MaxBloomIntensity)
	if err != nil {
		t.Fatalf("NewBloom: %v", err)
	}
	got, err := b.Apply(img)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got.Pix[7].R != 1 {
		t.Fatalf("亮心 = %v，want 1（LDR 出口必须钳住）", got.Pix[7].R)
	}
	n1 := got.Pix[8].R
	n2 := got.Pix[9].R
	n3 := got.Pix[10].R
	if !(n1 > n2 && n2 > n3 && n3 > 0) {
		t.Fatalf("辉光剖面 %v/%v/%v 不是单调衰减 —— 窗口内等值是盒模糊", n1, n2, n3)
	}
	// G(1)/G(2) = exp(1.5)，与核归一化无关。
	if r := n1 / n2; math.Abs(r-math.Exp(1.5)) > 0.05 {
		t.Errorf("n1/n2=%v，want exp(1.5)=%v（高斯 σ=1 的相邻比）", r, math.Exp(1.5))
	}
	if got.Pix[11].R != 0 {
		t.Errorf("核半径外（d=4）还有辉 %v，want 0", got.Pix[11].R)
	}
}

// 钉 2：软斜坡逐值 + LDR 出口钳。1×1 时高斯所有抽头折到同一点。
// v=0.75、thr=0.5 ⇒ m=0.5 ⇒ 辉=0.375 ⇒ 1.125 ⇒ 出口钳到 1；alpha 不进核。
func TestP375BloomSoftKneeAndLDRExitClamp(t *testing.T) {
	img := imageFromColors(t, 1, 1, core.RGBA(0.75, 0.75, 0.75, 0.3))
	b, err := NewBloom(0.5, 1, 1)
	if err != nil {
		t.Fatalf("NewBloom: %v", err)
	}
	got, err := b.Apply(img)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got.Pix[0].R != 1 {
		t.Errorf("R=%v，want 1（0.75+0.375 超 1 后由本阶段钳住，核本身不钳）", got.Pix[0].R)
	}
	if got.Pix[0].A != 0.3 {
		t.Errorf("alpha=%v，want 逐位不变的 0.3（alpha 不许过 float32 往返）", got.Pix[0].A)
	}

	// 同一颗心，把增益收到不越界：0.75 + 0.5*0.375 = 0.9375，逐值可算。
	b2, err := NewBloom(0.5, 1, 0.5)
	if err != nil {
		t.Fatalf("NewBloom: %v", err)
	}
	got2, err := b2.Apply(img)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if d := math.Abs(got2.Pix[0].R - 0.9375); d > bloomSlack {
		t.Errorf("R=%v，want 0.9375（偏差 %v）", got2.Pix[0].R, d)
	}
}

// 钉 3：Intensity==0 走恒等早退 —— 逐位不变，连不可精确表示的数也不动。
func TestP375BloomIdentityIsBitwise(t *testing.T) {
	img := imageFromColors(t, 2, 1,
		core.RGBA(0.3, 0.7, 0.1, 0.25), core.RGBA(1, 0.2, 0.6, 1))
	b, err := NewBloom(0, 4, 0)
	if err != nil {
		t.Fatalf("NewBloom: %v", err)
	}
	if !b.IsIdentity() {
		t.Fatal("Intensity==0 必须报恒等")
	}
	got, err := b.Apply(img)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !equalExact(got.Pix, img.Pix) {
		t.Fatalf("恒等帧改了像素：%v vs %v", got.Pix, img.Pix)
	}
}

// 钉 4：Threshold==1 谁都不发亮（颜色只能差 float32 往返的量）。
func TestP375BloomThresholdOneAddsNothing(t *testing.T) {
	img := imageFromColors(t, 2, 1,
		core.RGBA(0.3, 1, 0.5, 1), core.RGBA(1, 0.9, 0.2, 0.5))
	b, err := NewBloom(1, 2, MaxBloomIntensity)
	if err != nil {
		t.Fatalf("NewBloom: %v", err)
	}
	got, err := b.Apply(img)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for i, c := range got.Pix {
		want := img.Pix[i]
		if math.Abs(c.R-want.R) > bloomSlack || math.Abs(c.G-want.G) > bloomSlack ||
			math.Abs(c.B-want.B) > bloomSlack || c.A != want.A {
			t.Errorf("像素 %d = %v，want 不变的 %v", i, c, want)
		}
	}
}

// 钉 5：域委托——引擎的 σ 上限就是渲染层那枚，亮度夹必须更严，越界具名报错。
func TestP375BloomDomainDelegatesToRenderLayer(t *testing.T) {
	// MaxBloomSigma 就是渲染层那枚常量（委托，不自立一套），
	// 因此引擎允许的最大值必须还能被渲染层自己的域接受。
	if err := (render.BloomParams{Threshold: 1, Sigma: MaxBloomSigma,
		Intensity: MaxBloomIntensity}).Validate(); err != nil {
		t.Fatalf("引擎上限被渲染层拒了：%v（域委托反了）", err)
	}
	if MaxBloomIntensity >= render.MaxBloomIntensity {
		t.Fatalf("引擎夹 %v 必须严于渲染层 %v（可更严不可更宽）",
			MaxBloomIntensity, render.MaxBloomIntensity)
	}
	_, err := NewBloom(0.5, MaxBloomSigma+1, 1)
	if core.CodeOf(err) != core.CodeInvalidArg {
		t.Fatalf("σ 越界应 InvalidArg，实得 %v", err)
	}
	if !strings.Contains(err.Error(), "sigma") {
		t.Errorf("错误没点名参数：\"%v\"", err)
	}
}

// 钉 6：单边超过渲染层 float 目标上限的图，具名拒绝而不是偷偷截断。
func TestP375BloomRefusesOverCapImage(t *testing.T) {
	w := hdr.MaxFloatDimension + 1
	img, err := NewImage(w, 1)
	if err != nil {
		t.Fatalf("NewImage(%d,1): %v", w, err)
	}
	b, err := NewBloom(0.5, 1, 1)
	if err != nil {
		t.Fatalf("NewBloom: %v", err)
	}
	_, err = b.Apply(img)
	if core.CodeOf(err) != core.CodeInvalidArg {
		t.Fatalf("超上限应 InvalidArg，实得 %v", err)
	}
	if !strings.Contains(err.Error(), "float target") {
		t.Errorf("错误没说明是渲染层 float 上限：\"%v\"", err)
	}
}

// 钉 7：Grade 整链里泛光仍在第一段，参数换了词汇不改链序。
func TestP375GradeStillRunsBloomFirst(t *testing.T) {
	b, err := NewBloom(0.6, 1, 0.8)
	if err != nil {
		t.Fatalf("NewBloom: %v", err)
	}
	vig, err := NewVignette(core.V2(0.5, 0.5), 0.25, 0.85, 0.45)
	if err != nil {
		t.Fatalf("NewVignette: %v", err)
	}
	g, err := NewGrade(b, vig, nil, TonemapNone, 1)
	if err != nil {
		t.Fatalf("NewGrade: %v", err)
	}
	stages := g.Stages()
	if len(stages) != 4 || stages[0] != "bloom" {
		t.Fatalf("Stages=%v，want bloom 打头", stages)
	}
	img := imageFromColors(t, 3, 3,
		core.RGBA(0.95, 0.95, 0.95, 1), core.RGBA(0.1, 0.1, 0.1, 1), core.RGBA(0.5, 0.5, 0.5, 1),
		core.RGBA(0.2, 0.2, 0.2, 1), core.RGBA(1, 1, 1, 1), core.RGBA(0.3, 0.3, 0.3, 1),
		core.RGBA(0.7, 0.7, 0.7, 1), core.RGBA(0.4, 0.4, 0.4, 1), core.RGBA(0.6, 0.6, 0.6, 1))
	got, err := g.Apply(img)
	if err != nil {
		t.Fatalf("Grade.Apply: %v", err)
	}
	for _, c := range got.Pix {
		if c.R < 0 || c.R > 1 || c.G < 0 || c.G > 1 || c.B < 0 || c.B > 1 {
			t.Fatalf("整链出口出现越界色：%v", c)
		}
	}
}
