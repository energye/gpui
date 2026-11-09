//go:build !nogpu

package render_test

//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

import (
	"testing"

	"github.com/energye/gpui/render"
)

// P4.1 批 3（子项④，D2a 口径）：DrawShadow 非仿射投影下的设备空间语义。
//
// Impeller 模型（dl_dispatcher.cc:906-924）：occluder 抬升是**屏幕空间**量——
// Translate(0, -occluder_z * light.y) 作用在 CTM 之后，blur sigma 也是设备空间
// 量（kLightRadius * occluder_z / CTM.y-scale）。同 elevation 的两个 occluder
// 在同一投影下：剪影相对本体的下移应**相等且等于 elevation 设备像素**（方向光
// (0,-1,1) 只做屏幕 y 平移，模糊尾两方相同）。
//
// 现状把抬升平移放在用户空间（投影之前）：透视（W←y 矩阵）下不同高度的局部
// 缩放不同 → 剪影位移 = elevation × 局部缩放，两方块位移不等且偏离 elevation。
//
// 红测用**相对断言**：measure(方块) = shadow 外沿 − occluder 底边 = 位移 + 模糊
// 尾；两方 elevation 相同 → 模糊尾相消 → |measure_A − measure_B| 应 ≈ 0；现状
// = elevation × |s_A − s_B| > 0（可测）。

func TestDrawShadowProjectionLiftIsDeviceSpace(t *testing.T) {
	// 锁 CPU 栅格：本测断言 DrawShadow 的抬升语义（逻辑）。GPU 层链上
	// 「投影×隔离层×blur」的组合存在 readback 时序不稳（场景一墨水在
	// 183k~125k 间摆动、race 下恒 51.8%）——独立引擎洞（#4，P4.1 台账
	// 登记），与批 1 #3 同族（GPU 层内容在合成/回读链上不同步），不在
	// 本批语义收口范围内。
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	const (
		W, H      = 240, 420
		elevation = 20.0
	)
	// 真透视（W←y，P3.7 真窗同款形式）：W = 1 + 0.002·y。occluder 用户
	// y∈[100,400] → 投影后 y_dev∈[83.3,222.2]。Impeller 语义：剪影=投影形状
	// 整体下移 elevation（设备空间、形状不变）；现状=用户空间平移后投影
	// （y_dev∈[96.8,228.3]，形状被压缩 ~7px、位置偏移）。
	proj := render.Matrix4FromElements([16]float64{
		1, 0, 0.0625, 0,
		0, 1, 0.125, 0,
		0, 0, 1, 0,
		0, 0.002, 0, 1,
	})
	if _, aff := proj.Affine(); aff {
		t.Skip("projection degenerated to affine; nothing to assert")
	}
	occluder := func() *render.Path {
		r := render.NewPath()
		r.MoveTo(60, 100)
		r.LineTo(180, 100)
		r.LineTo(180, 400)
		r.LineTo(60, 400)
		r.Close()
		return r
	}
	drawOccluder := func(dc *render.Context, dev bool) {
		// dev=false: path 是用户空间（场景一，经投影链绘制）；dev=true: path
		// 已是设备坐标（场景二，恒等链）。本体画在 shadow 之上（shadow 只画
		// 剪影，本体由调用方补）。
		dc.SetRGBA(0.05, 0.05, 0.08, 1)
		_ = dc.Fill()
	}

	// 场景一：现状 —— 透视投影下 DrawShadow（用户空间抬升）。
	dcA := render.NewContext(W, H)
	defer dcA.Close()
	dcA.ClearWithColor(render.White)
	dcA.SetProjection(proj)
	dcA.DrawShadow(occluder(), render.RGBA{R: 0, G: 0, B: 0, A: 1}, elevation, false)
	dcA.SetPath(occluder())
	drawOccluder(dcA, false)
	if err := dcA.FlushGPU(); err != nil {
		t.Fatalf("flush A: %v", err)
	}

	// 场景二：期望语义的真值 —— path 手动投影成设备坐标，恒等链 DrawShadow
	//（无投影干扰时 DrawShadow 的抬升/模糊就是纯设备空间，与 Impeller 同构）。
	devPath := render.NewPath()
	for _, pt := range [][2]float64{{60, 100}, {180, 100}, {180, 400}, {60, 400}} {
		d, _, okP := proj.ProjectPoint(pt[0], pt[1], 0)
		if !okP {
			t.Fatalf("project point %v", pt)
		}
		if pt == [2]float64{60, 100} {
			devPath.MoveTo(d.X, d.Y)
		} else {
			devPath.LineTo(d.X, d.Y)
		}
	}
	devPath.Close()
	dcB := render.NewContext(W, H)
	defer dcB.Close()
	dcB.ClearWithColor(render.White)
	dcB.DrawShadow(devPath, render.RGBA{R: 0, G: 0, B: 0, A: 1}, elevation, false)
	dcB.SetPath(devPath)
	drawOccluder(dcB, true)
	if err := dcB.FlushGPU(); err != nil {
		t.Fatalf("flush B: %v", err)
	}

	// 区域墨水：只积分 occluder 投影底边（y_dev=222.2）以下的 shadow 带——
	// blur 对称且两场景 σ 相同，全画面积分会把剪影带差稀释到 <2%；带内积分
	// 直接量化「剪影可见带长度差」（现状 ~6px vs 期望 ~20px）。
	const bandTop = 215.0
	ink := func(dc *render.Context) float64 {
		img := dc.Image()
		total := 0.0
		for y := int(bandTop); y < H; y++ {
			for x := 0; x < W; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				total += 255 - (float64(r>>8)+float64(g>>8)+float64(b>>8))/3
			}
		}
		return total
	}
	inkA, inkB := ink(dcA), ink(dcB)
	if inkB <= 0 {
		t.Fatal("no ink in reference band")
	}
	rel := (inkA - inkB) / inkB
	// 现状（用户空间抬升）：剪影可见带比期望语义短 ~20% → 红；修好后（设备
	// 空间抬升）两场景同剪影 → rel≈0 → 绿。
	if rel < -0.10 {
		t.Fatalf("shadow band under perspective holds %.1f%% of the device-space semantics ink (want >=90%%): inkA=%.0f inkB=%.0f — user-space lift draws a shorter, distorted silhouette instead of the Impeller screen-translate lift",
			(1+rel)*100, inkA, inkB)
	}
}
