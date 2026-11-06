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

// P3.7 材质×bicubic 纹素契约（2026-11-07 拍板：引擎喂纹素——契约前缀第 4 参
// texel=引擎按本笔采样档采好的纹素；对齐 Godot「TEXTURE 按 item 档位采样」，
// 采样质量归引擎，用户只管上色。评估/拍板见
// evidence/P3.7/batch6/mat-persp-assessment.md 与 matpersp-fix.md §B）。
//
// 红态（A 批树 1770746c，git stash 可复现）：4 参 fs_main 被 fragmentParams
// 具名拒（前缀上限 (uv,tint,hom_w)）+ material+bicubic 被 m4_quadimage 具名拒
// （ErrQuadMaterialUnsupported）——echo 测试在红态编译期即红。
//
// 钉法：**echo 材质 ≡ 标准路逐位**——返回引擎纹素原样的材质，与无材质标准路
// （bicubic 核 / nearest / linear 采样）必须逐位同图（乘链同序：engine
// texel × opacity × [tint]，tint 白恒等）。三口各钉：quad / sprite / image。

import (
	"testing"

	"github.com/energye/gpui/render"
)

// p37MPEchoWGSL：纹素回显材质（4 参全声明；引擎注入绑定路径——用户源不声明
// 纹理对，引擎注入 gpui_eng_tex/gpui_eng_samp 并把对计入绑定面）。
const p37MPEchoWGSL = `@fragment
fn fs_main(@location(0) uv: vec2<f32>, @location(1) tint: vec4<f32>, @location(2) hom_w: f32, @location(3) texel: vec4<f32>) -> @location(0) vec4<f32> {
    return texel;
}`

// p37MPEchoOwnBindingsWGSL：同回显但用户自带纹理对声明（引擎复用用户变量名，
// 不注入）——两条绑定名解析路都钉住。
const p37MPEchoOwnBindingsWGSL = `@group(0) @binding(1) var mp_tex: texture_2d<f32>;
@group(0) @binding(2) var mp_samp: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>, @location(1) tint: vec4<f32>, @location(2) hom_w: f32, @location(3) texel: vec4<f32>) -> @location(0) vec4<f32> {
    return texel;
}`

func p37MPEchoEffect(t *testing.T, wgsl string) *render.ShaderEffect {
	t.Helper()
	e, err := render.NewShaderEffect(wgsl)
	if err != nil {
		t.Fatalf("NewShaderEffect(echo): %v（4 参契约 + 纹素馈送必须被接受）", err)
	}
	return e
}

// TestP37MatTexelEchoQuadBitwise：quad 口 bicubic/nearest/linear 三档——
// echo 材质 ≡ 无材质标准路逐位。
func TestP37MatTexelEchoQuadBitwise(t *testing.T) {
	requireFilterOpsGPU(t)
	img := p37BChecker(24, 24)
	for _, tc := range []struct {
		name   string
		interp render.InterpolationMode
	}{
		{"bicubic", render.InterpBicubic},
		{"nearest", render.InterpNearest},
		{"linear", render.InterpBilinear},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := p37MPEchoEffect(t, p37MPEchoWGSL)
			defer e.Close()
			corners := [4]render.Point{{X: 8, Y: 8}, {X: 56, Y: 8}, {X: 56, Y: 56}, {X: 8, Y: 56}}

			dcPlain := render.NewContext(64, 64)
			defer func() { _ = dcPlain.Close() }()
			dcPlain.ClearWithColor(render.White)
			if err := dcPlain.DrawImageQuad(img, corners, render.QuadDrawOptions{Interpolation: tc.interp}); err != nil {
				t.Fatalf("plain: %v", err)
			}
			dcMat := render.NewContext(64, 64)
			defer func() { _ = dcMat.Close() }()
			dcMat.ClearWithColor(render.White)
			if err := dcMat.DrawImageQuad(img, corners, render.QuadDrawOptions{Interpolation: tc.interp, Material: e}); err != nil {
				t.Fatalf("material+%s: %v（纹素契约放行）", tc.name, err)
			}
			for _, dc := range []*render.Context{dcPlain, dcMat} {
				if err := dc.FlushGPU(); err != nil {
					t.Fatalf("FlushGPU: %v", err)
				}
			}
			a := p37BRegion(t, dcPlain, 8, 8, 48, 48)
			b := p37BRegion(t, dcMat, 8, 8, 48, 48)
			if n, worst := p37BDiff(a, b); n != 0 {
				t.Fatalf("echo material vs plain: %d differing channels, worst %d — 引擎纹素≠标准采样", n, worst)
			}
		})
	}
}

// TestP37MatTexelEchoOwnBindingsBitwise：用户自带纹理对声明——引擎复用用户
// 变量名（bicubic 档）。
func TestP37MatTexelEchoOwnBindingsBitwise(t *testing.T) {
	requireFilterOpsGPU(t)
	img := p37BChecker(24, 24)
	e := p37MPEchoEffect(t, p37MPEchoOwnBindingsWGSL)
	defer e.Close()

	corners := [4]render.Point{{X: 8, Y: 8}, {X: 56, Y: 8}, {X: 56, Y: 56}, {X: 8, Y: 56}}
	dcPlain := render.NewContext(64, 64)
	defer func() { _ = dcPlain.Close() }()
	dcPlain.ClearWithColor(render.White)
	if err := dcPlain.DrawImageQuad(img, corners, render.QuadDrawOptions{Interpolation: render.InterpBicubic}); err != nil {
		t.Fatalf("plain: %v", err)
	}
	dcMat := render.NewContext(64, 64)
	defer func() { _ = dcMat.Close() }()
	dcMat.ClearWithColor(render.White)
	if err := dcMat.DrawImageQuad(img, corners, render.QuadDrawOptions{Interpolation: render.InterpBicubic, Material: e}); err != nil {
		t.Fatalf("material+bicubic: %v", err)
	}
	for _, dc := range []*render.Context{dcPlain, dcMat} {
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("FlushGPU: %v", err)
		}
	}
	a := p37BRegion(t, dcPlain, 8, 8, 48, 48)
	b := p37BRegion(t, dcMat, 8, 8, 48, 48)
	if n, worst := p37BDiff(a, b); n != 0 {
		t.Fatalf("echo(own bindings) bicubic vs plain: %d differing channels, worst %d", n, worst)
	}
}

// TestP37MatTexelEchoPerspBitwise：纹素骑透视正确插值的 UV——echo+透视
// ≡ 无材质透视逐位（A 批权重透传 × B 批纹素契约的组合态）。采样档取 linear：
// 透视+bicubic 是先于材质的既有边界（透视槽无 bicubic 变体 ⇒ ErrQuadPerspectiveBicubic，
// 三合一维持该拒——材质路虽可实现，plain 路无对照锚，归透视槽 bicubic 变体另项）。
func TestP37MatTexelEchoPerspBitwise(t *testing.T) {
	requireFilterOpsGPU(t)
	img := p37BChecker(24, 24)
	e := p37MPEchoEffect(t, p37MPEchoWGSL)
	defer e.Close()

	corners := [4]render.Point{{X: 10, Y: 8}, {X: 54, Y: 6}, {X: 50, Y: 54}, {X: 8, Y: 50}}
	weights := [4]float64{1.5, 1.2, 1.0, 2.0}
	dcPlain := render.NewContext(64, 64)
	defer func() { _ = dcPlain.Close() }()
	dcPlain.ClearWithColor(render.White)
	if err := dcPlain.DrawImageQuad(img, corners, render.QuadDrawOptions{
		Perspective: true, Weights: weights, Interpolation: render.InterpBilinear,
	}); err != nil {
		t.Fatalf("plain: %v", err)
	}
	dcMat := render.NewContext(64, 64)
	defer func() { _ = dcMat.Close() }()
	dcMat.ClearWithColor(render.White)
	if err := dcMat.DrawImageQuad(img, corners, render.QuadDrawOptions{
		Perspective: true, Weights: weights, Interpolation: render.InterpBilinear, Material: e,
	}); err != nil {
		t.Fatalf("echo+persp+linear: %v", err)
	}
	for _, dc := range []*render.Context{dcPlain, dcMat} {
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("FlushGPU: %v", err)
		}
	}
	a := p37BRegion(t, dcPlain, 8, 6, 47, 49)
	b := p37BRegion(t, dcMat, 8, 6, 47, 49)
	if n, worst := p37BDiff(a, b); n != 0 {
		t.Fatalf("echo+persp+bicubic vs plain persp bicubic: %d differing channels, worst %d", n, worst)
	}
}

// TestP37MatTexelEchoSpriteAndImageBitwise：批绘口与图像口各一轮 bicubic
// echo ≡ 标准路逐位（材质+bicubic 三口全通）。
func TestP37MatTexelEchoSpriteAndImageBitwise(t *testing.T) {
	requireFilterOpsGPU(t)
	img := p37BChecker(24, 24)
	e := p37MPEchoEffect(t, p37MPEchoWGSL)
	defer e.Close()

	// 批绘口：bicubic 精灵 echo ≡ plain。
	spPlain := render.AtlasSprite{SrcX: 0, SrcY: 0, SrcW: 24, SrcH: 24, DstX: 8, DstY: 8, DstW: 48, DstH: 48, Filter: render.InterpBicubic}
	dcPlain := render.NewContext(64, 64)
	defer func() { _ = dcPlain.Close() }()
	dcPlain.ClearWithColor(render.White)
	if _, err := dcPlain.DrawAtlasEx(img, []render.AtlasSprite{spPlain}, render.AtlasDrawOptions{}); err != nil {
		t.Fatalf("plain sprite: %v", err)
	}
	dcMat := render.NewContext(64, 64)
	defer func() { _ = dcMat.Close() }()
	dcMat.ClearWithColor(render.White)
	spMat := spPlain
	spMat.Material = e
	if _, err := dcMat.DrawAtlasEx(img, []render.AtlasSprite{spMat}, render.AtlasDrawOptions{}); err != nil {
		t.Fatalf("material sprite: %v", err)
	}

	// 图像口：bicubic DrawImageEx echo ≡ plain。
	dcPlain2 := render.NewContext(64, 64)
	defer func() { _ = dcPlain2.Close() }()
	dcPlain2.ClearWithColor(render.White)
	dcPlain2.DrawImageEx(img, render.DrawImageOptions{X: 8, Y: 8, DstWidth: 48, DstHeight: 48, Interpolation: render.InterpBicubic})
	dcMat2 := render.NewContext(64, 64)
	defer func() { _ = dcMat2.Close() }()
	dcMat2.ClearWithColor(render.White)
	dcMat2.DrawImageEx(img, render.DrawImageOptions{X: 8, Y: 8, DstWidth: 48, DstHeight: 48, Interpolation: render.InterpBicubic, Material: e})

	for _, dc := range []*render.Context{dcPlain, dcMat, dcPlain2, dcMat2} {
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("FlushGPU: %v", err)
		}
	}
	if n, worst := p37BDiff(p37BRegion(t, dcPlain, 8, 8, 48, 48), p37BRegion(t, dcMat, 8, 8, 48, 48)); n != 0 {
		t.Fatalf("echo sprite bicubic vs plain: %d differing channels, worst %d", n, worst)
	}
	if n, worst := p37BDiff(p37BRegion(t, dcPlain2, 8, 8, 48, 48), p37BRegion(t, dcMat2, 8, 8, 48, 48)); n != 0 {
		t.Fatalf("echo image bicubic vs plain: %d differing channels, worst %d", n, worst)
	}
}
