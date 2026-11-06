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

// P3.7 材质×透视专项（批 3b「组合顶点机已就绪，只差权重透传与钉」的兑现；
// 2026-11-07 立项，拍板口径 evidence/P3.7/batch6/mat-persp-assessment.md：
// 材质+透视与材质+bicubic 两半都做，真窗=新建专项窗）。
//
// 红测 A——权重透传腿（bicubic 纹素契约归 B 批红测）：
//   - 材质+透视（自持权重 / 画布投影层）真画出来，不再 ErrQuadMaterialUnsupported；
//   - identity 材质+透视 ≡ 无材质透视**逐位**（组合腿乘链不偏离标准透视腿）；
//   - 透视正确插值解析钉：uv.y 带状材质在梯形上的色带分界 = hom_w 公式反推的
//     y=70/3≈23.33（梯形 TL/TR w=2、BR/BL w=1；若权重又被拍平成 1，仿射插值
//     分界错位到 y=30，(40,26) 探针即红）；
//   - 精灵路 material+persp 不再 skip+计数，画面 ≡ 无材质透视精灵逐位；
//   - 拒绝面收缩有界：behind-eye（投影层）仍 ErrQuadBehindEye、材质+blend≠Normal
//     仍在 blend 校验拒绝、DrawImageEx 材质+blend 仍 skip+计数。

import (
	"errors"
	"testing"

	"github.com/energye/gpui/render"
)

// p37MPBandWGSL uv.y 带状材质：uv.y<0.5 红、否则蓝（纯 uv 函数，不采样纹理）。
const p37MPBandWGSL = `@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    if (uv.y < 0.5) {
        return vec4<f32>(0.86, 0.20, 0.22, 1.0);
    }
    return vec4<f32>(0.20, 0.42, 0.92, 1.0);
}`

func p37MPixel(t *testing.T, dc *render.Context, x, y int) (uint8, uint8, uint8) {
	t.Helper()
	r, g, b, _ := dc.Image().At(x, y).RGBA()
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
}

// TestP37MatPerspQuadIdentityBitwise：quad 口自持权重——identity 材质+透视
// ≡ 无材质透视逐位，且不再具名拒。
func TestP37MatPerspQuadIdentityBitwise(t *testing.T) {
	requireFilterOpsGPU(t)
	img := p37BChecker(16, 16)
	e, err := render.NewShaderEffect(p37BIdentityWGSL)
	if err != nil {
		t.Fatalf("NewShaderEffect: %v", err)
	}
	defer e.Close()

	corners := [4]render.Point{{X: 10, Y: 8}, {X: 54, Y: 6}, {X: 50, Y: 54}, {X: 8, Y: 50}}
	weights := [4]float64{1.5, 1.2, 1.0, 2.0}
	opts := render.QuadDrawOptions{Perspective: true, Weights: weights}

	dcPlain := render.NewContext(64, 64)
	defer func() { _ = dcPlain.Close() }()
	dcPlain.ClearWithColor(render.White)
	if err := dcPlain.DrawImageQuad(img, corners, opts); err != nil {
		t.Fatalf("plain persp quad: %v", err)
	}

	dcMat := render.NewContext(64, 64)
	defer func() { _ = dcMat.Close() }()
	dcMat.ClearWithColor(render.White)
	errMat := dcMat.DrawImageQuad(img, corners, render.QuadDrawOptions{
		Perspective: true, Weights: weights, Material: e,
	})
	if errMat != nil {
		t.Fatalf("material+persp err=%v, want nil（组合放行）", errMat)
	}
	for _, dc := range []*render.Context{dcPlain, dcMat} {
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("FlushGPU: %v", err)
		}
	}
	a := p37BRegion(t, dcPlain, 8, 6, 47, 49)
	b := p37BRegion(t, dcMat, 8, 6, 47, 49)
	if n, worst := p37BDiff(a, b); n != 0 {
		t.Fatalf("identity material+persp vs plain persp: %d differing channels, worst %d — 组合腿乘链偏离标准透视腿", n, worst)
	}
}

// TestP37MatPerspGradientBandAnalytic：透视正确插值的解析钉——带状分界在
// y=70/3≈23.33（仿射插值 30）。(40,18) 红带锚；(40,26) 蓝带反拍平锚
// （权重若被拍平，uv.y(26)=0.4<0.5 该点会错画成红）。
func TestP37MatPerspGradientBandAnalytic(t *testing.T) {
	requireFilterOpsGPU(t)
	e, err := render.NewShaderEffect(p37MPBandWGSL)
	if err != nil {
		t.Fatalf("NewShaderEffect: %v", err)
	}
	defer e.Close()

	dc := render.NewContext(96, 64)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.White)
	// 梯形：上边 y=10（x 20..60）、下边 y=50（x 10..70）；TL/TR w=2（远）、
	// BR/BL w=1（近）。中心线 x=40 全程在四边形内。
	err = dc.DrawImageQuad(p37BChecker(8, 8), [4]render.Point{
		{X: 20, Y: 10}, {X: 60, Y: 10}, {X: 70, Y: 50}, {X: 10, Y: 50},
	}, render.QuadDrawOptions{Perspective: true, Weights: [4]float64{2, 2, 1, 1}, Material: e})
	if err != nil {
		t.Fatalf("material+persp band quad: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	r, g, b := p37MPixel(t, dc, 40, 18)
	if p37MPxA(r, 219) > 2 || p37MPxA(g, 51) > 2 || p37MPxA(b, 56) > 2 {
		t.Fatalf("(40,18) = (%d,%d,%d), want ≈(219,51,56) 红带（uv.y=%.3f<0.5）",
			r, g, b, 0.333)
	}
	r, g, b = p37MPixel(t, dc, 40, 26)
	if p37MPxA(r, 51) > 2 || p37MPxA(g, 107) > 2 || p37MPxA(b, 235) > 2 {
		t.Fatalf("(40,26) = (%d,%d,%d), want ≈(51,107,235) 蓝带——透视正确分界 23.33；"+
			"若此处偏红=权重被拍平成仿射（分界 30）", r, g, b)
	}
}

func p37MPxA(a uint8, want int) int {
	d := int(a) - want
	if d < 0 {
		return -d
	}
	return d
}

// TestP37MatPerspProjectionLayerIdentityBitwise：材质+画布投影层组合
// （perspLeg 权重来自投影映射）——identity ≡ 无材质逐位，不再具名拒。
func TestP37MatPerspProjectionLayerIdentityBitwise(t *testing.T) {
	requireFilterOpsGPU(t)
	img := p37BChecker(16, 16)
	e, err := render.NewShaderEffect(p37BIdentityWGSL)
	if err != nil {
		t.Fatalf("NewShaderEffect: %v", err)
	}
	defer e.Close()

	projM := render.Matrix4FromElements([16]float64{
		1, 0, 0.0625, 0,
		0, 1, 0.125, 0,
		0, 0, 1, 0,
		0, 0.00390625, 0, 1,
	})
	corners := [4]render.Point{{X: 12, Y: 12}, {X: 60, Y: 12}, {X: 60, Y: 52}, {X: 12, Y: 52}}
	opts := render.QuadDrawOptions{Interpolation: render.InterpNearest, Opacity: 1}

	dcPlain := render.NewContext(96, 72)
	defer func() { _ = dcPlain.Close() }()
	dcPlain.ClearWithColor(render.White)
	dcPlain.SetProjection(projM)
	if err := dcPlain.DrawImageQuad(img, corners, opts); err != nil {
		t.Fatalf("plain projected quad: %v", err)
	}

	dcMat := render.NewContext(96, 72)
	defer func() { _ = dcMat.Close() }()
	dcMat.ClearWithColor(render.White)
	dcMat.SetProjection(projM)
	if err := dcMat.DrawImageQuad(img, corners, render.QuadDrawOptions{
		Interpolation: render.InterpNearest, Opacity: 1, Material: e,
	}); err != nil {
		t.Fatalf("material+projection-layer err=%v, want nil（组合放行）", err)
	}
	for _, dc := range []*render.Context{dcPlain, dcMat} {
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("FlushGPU: %v", err)
		}
	}
	a := p37BRegion(t, dcPlain, 8, 8, 72, 56)
	b := p37BRegion(t, dcMat, 8, 8, 72, 56)
	if n, worst := p37BDiff(a, b); n != 0 {
		t.Fatalf("identity material+projection vs plain projected: %d differing channels, worst %d", n, worst)
	}
}

// TestP37MatPerspSpriteRoadDrawn：批绘口材质精灵+自带透视——不再跳过+计数，
// 画面 ≡ 无材质透视精灵逐位。
func TestP37MatPerspSpriteRoadDrawn(t *testing.T) {
	requireFilterOpsGPU(t)
	img := p37BChecker(16, 16)
	e, err := render.NewShaderEffect(p37BIdentityWGSL)
	if err != nil {
		t.Fatalf("NewShaderEffect: %v", err)
	}
	defer e.Close()

	plain := render.AtlasSprite{
		SrcX: 0, SrcY: 0, SrcW: 16, SrcH: 16,
		DstX: 0, DstY: 0, DstW: 40, DstH: 40,
		Perspective: true,
		Corners:     [4]render.Point{{X: 4, Y: 8}, {X: 56, Y: 4}, {X: 52, Y: 56}, {X: 6, Y: 52}},
		Weights:     [4]float64{1.4, 1.1, 1.0, 1.8},
	}
	withMat := plain
	dcPlain := render.NewContext(64, 64)
	defer func() { _ = dcPlain.Close() }()
	dcPlain.ClearWithColor(render.White)
	if _, err := dcPlain.DrawAtlasEx(img, []render.AtlasSprite{plain}, render.AtlasDrawOptions{}); err != nil {
		t.Fatalf("plain persp sprite: %v", err)
	}
	dcMat := render.NewContext(64, 64)
	defer func() { _ = dcMat.Close() }()
	dcMat.ClearWithColor(render.White)
	before := dcMat.RenderPathStats().MaterialSkips
	if _, err := dcMat.DrawAtlasEx(img, []render.AtlasSprite{func() render.AtlasSprite {
		s := withMat
		s.Material = e
		return s
	}()}, render.AtlasDrawOptions{}); err != nil {
		t.Fatalf("material persp sprite: %v", err)
	}
	if got := dcMat.RenderPathStats().MaterialSkips - before; got != 0 {
		t.Fatalf("material+persp sprite MaterialSkips +%d, want 0（不再跳过）", got)
	}
	for _, dc := range []*render.Context{dcPlain, dcMat} {
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("FlushGPU: %v", err)
		}
	}
	a := p37BRegion(t, dcPlain, 4, 4, 53, 53)
	b := p37BRegion(t, dcMat, 4, 4, 53, 53)
	if n, worst := p37BDiff(a, b); n != 0 {
		t.Fatalf("identity material persp sprite vs plain: %d differing channels, worst %d", n, worst)
	}
}

// TestP37MatPerspRefusalsRemain：拒绝面收缩有界——behind-eye（投影层，任一角
// w≤0）仍 ErrQuadBehindEye（带不带材质同错）；材质+blend≠Normal 仍在 blend
// 校验拒绝；DrawImageEx 材质+blend 仍 skip+计数。
func TestP37MatPerspRefusalsRemain(t *testing.T) {
	requireFilterOpsGPU(t)
	img := p37BChecker(8, 8)
	e, err := render.NewShaderEffect(p37BIdentityWGSL)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	// behind-eye：projHomography 的 w(y)=1-0.003y 在 y≈333 过零，四角跨眼平面。
	dc := render.NewContext(320, 220)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.White)
	dc.SetProjection(projHomography())
	errPlain := dc.DrawImageQuad(img, [4]render.Point{
		{X: 0, Y: 0}, {X: 120, Y: 0}, {X: 120, Y: 400}, {X: 0, Y: 400},
	}, render.QuadDrawOptions{Interpolation: render.InterpNearest, Opacity: 1})
	if !errors.Is(errPlain, render.ErrQuadBehindEye) {
		t.Fatalf("behind-eye plain err=%v want ErrQuadBehindEye", errPlain)
	}
	errMat := dc.DrawImageQuad(img, [4]render.Point{
		{X: 0, Y: 0}, {X: 120, Y: 0}, {X: 120, Y: 400}, {X: 0, Y: 400},
	}, render.QuadDrawOptions{Interpolation: render.InterpNearest, Opacity: 1, Material: e})
	if !errors.Is(errMat, render.ErrQuadBehindEye) {
		t.Fatalf("behind-eye material err=%v want ErrQuadBehindEye（几何拒绝与材质无关）", errMat)
	}

	// blend≠Normal：quad 口 blend 校验先行拒绝（既有口径，材质不改变它）。
	dc.SetProjection(render.Matrix4Identity())
	errBl := dc.DrawImageQuad(img, [4]render.Point{{X: 4, Y: 4}, {X: 40, Y: 4}, {X: 40, Y: 40}, {X: 4, Y: 40}},
		render.QuadDrawOptions{Material: e, BlendMode: render.BlendMultiply})
	if errBl == nil {
		t.Fatal("material+non-Normal blend must be rejected at the blend check")
	}
	// 图像口材质+blend：无错误通道 ⇒ CPU skip + 计数（既有口径不变）。
	before := dc.RenderPathStats().MaterialSkips
	dc.DrawImageEx(img, render.DrawImageOptions{X: 4, Y: 4, BlendMode: render.BlendMultiply, Material: e})
	if got := dc.RenderPathStats().MaterialSkips - before; got != 1 {
		t.Fatalf("DrawImageEx material+blend must skip+count (got %d)", got)
	}
}
