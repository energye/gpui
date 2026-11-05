//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// P3.7 批 5 · 步骤 1（CPU 竖条）：泛光核落在渲染层的 float 域上。
//
// 钉的是「一处算法」的四件事：亮部软斜坡的逐值、可分离高斯用的就是
// internal/filter 那枚共享核（不是盒）、加法不回钳（>1 的光留到 Encode
// 的 Reinhard 收）、以及恒等与边界的逐位不变。摘掉任何一条必红。
package hdr

import (
	"math"
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/internal/filter"
)

const bloomEps = 1e-6

func newFilledTarget(t *testing.T, w, h int, fill func(x, y int) [4]float32) *FloatTarget {
	t.Helper()
	ft, err := NewFloatTarget(w, h)
	if err != nil {
		t.Fatalf("NewFloatTarget(%d,%d): %v", w, h, err)
	}
	pix := ft.FloatPixels()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := fill(x, y)
			i := (y*w + x) * 4
			pix[i], pix[i+1], pix[i+2], pix[i+3] = c[0], c[1], c[2], c[3]
		}
	}
	return ft
}

func copyFloatPixels(src []float32) []float32 {
	out := make([]float32, len(src))
	copy(out, src)
	return out
}

func equalFloatSlice(a, b []float32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// 钉 1：非法参数一律具名错误，不许猜一个默认值。
func TestBloomParamsNamedErrors(t *testing.T) {
	cases := []struct {
		name string
		p    render.BloomParams
		want string
	}{
		{"neg threshold", render.BloomParams{Threshold: -0.1, Sigma: 1, Intensity: 1}, "threshold"},
		{"over threshold", render.BloomParams{Threshold: 1.01, Sigma: 1, Intensity: 1}, "threshold"},
		{"nan threshold", render.BloomParams{Threshold: math.NaN(), Sigma: 1, Intensity: 1}, "threshold"},
		{"inf threshold", render.BloomParams{Threshold: math.Inf(1), Sigma: 1, Intensity: 1}, "threshold"},
		{"neg sigma", render.BloomParams{Threshold: 0.5, Sigma: -1, Intensity: 1}, "sigma"},
		{"nan sigma", render.BloomParams{Threshold: 0.5, Sigma: math.NaN(), Intensity: 1}, "sigma"},
		{"over sigma", render.BloomParams{Threshold: 0.5, Sigma: render.MaxBloomSigma + 1, Intensity: 1}, "sigma"},
		{"neg intensity", render.BloomParams{Threshold: 0.5, Sigma: 1, Intensity: -1}, "intensity"},
		{"nan intensity", render.BloomParams{Threshold: 0.5, Sigma: 1, Intensity: math.NaN()}, "intensity"},
		{"over intensity", render.BloomParams{Threshold: 0.5, Sigma: 1, Intensity: render.MaxBloomIntensity + 1}, "intensity"},
	}
	for _, c := range cases {
		err := c.p.Validate()
		if err == nil {
			t.Errorf("%s: Validate() 却返回 nil", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: 错误没点名参数 %q：%v", c.name, c.want, err)
		}
	}
}

// 钉 2：零值参数必须是合法且不改变画面的默认（与批 4 的 MipmapNone 同理）。
func TestBloomParamsZeroValueIsValidIdentity(t *testing.T) {
	var p render.BloomParams
	if err := p.Validate(); err != nil {
		t.Fatalf("零值参数必须合法，却报：%v", err)
	}
	if !p.IsIdentity() {
		t.Fatal("零值参数必须报恒等")
	}
	if (render.BloomParams{Threshold: 0.5, Sigma: 2, Intensity: 0}).IsIdentity() != true {
		t.Fatal("Intensity==0 必须恒等")
	}
	if (render.BloomParams{Threshold: 0.5, Sigma: 2, Intensity: 1}).IsIdentity() != false {
		t.Fatal("真会发光的参数不许报恒等")
	}
}

// 钉 3：Intensity==0 ⇒ 逐位不变，且不写脏 src。
func TestBloomZeroIntensityBitwiseIdentity(t *testing.T) {
	src := newFilledTarget(t, 4, 3, func(x, y int) [4]float32 {
		return [4]float32{float32(x) / 3, float32(y) / 2, 0.9, 1}
	})
	before := copyFloatPixels(src.FloatPixels())

	out, err := src.Bloom(render.BloomParams{Threshold: 0.2, Sigma: 3, Intensity: 0})
	if err != nil {
		t.Fatalf("Bloom: %v", err)
	}
	if out == src {
		t.Fatal("恒等也必须回新目标，不许把 src 自己交回去")
	}
	if !equalFloatSlice(out.FloatPixels(), before) {
		t.Fatalf("Intensity==0 却改了像素：\nout=%v\nsrc=%v", out.FloatPixels(), before)
	}
	if !equalFloatSlice(src.FloatPixels(), before) {
		t.Fatal("Bloom 改了入参 src：源图必须只读")
	}
}

// 钉 4：Threshold==1 ⇒ 掩膜处处 0 ⇒ 加法项为 0 ⇒ 逐位等价恒等。
func TestBloomThresholdOneEmitsNoGlow(t *testing.T) {
	src := newFilledTarget(t, 5, 5, func(x, y int) [4]float32 {
		v := float32(x+y) / 8
		return [4]float32{v, v, v, 1}
	})
	before := copyFloatPixels(src.FloatPixels())

	out, err := src.Bloom(render.BloomParams{Threshold: 1, Sigma: 2, Intensity: render.MaxBloomIntensity})
	if err != nil {
		t.Fatalf("Bloom: %v", err)
	}
	if !equalFloatSlice(out.FloatPixels(), before) {
		t.Fatalf("Threshold==1 却加了光：\nout=%v\nsrc=%v", out.FloatPixels(), before)
	}
}

// 钉 5：软斜坡逐值。1×1 时高斯所有抽头都夹到同一点，掩膜值即输出增量。
// v=0.75 的灰阶亮度 lum=v，thr=0.5 ⇒ m=(0.75-0.5)/0.5=0.5 ⇒ 增量 0.75*0.5。
func TestBloomBrightMaskRampValue(t *testing.T) {
	src := newFilledTarget(t, 1, 1, func(_, _ int) [4]float32 { return [4]float32{0.75, 0.75, 0.75, 1} })
	out, err := src.Bloom(render.BloomParams{Threshold: 0.5, Sigma: 1, Intensity: 1})
	if err != nil {
		t.Fatalf("Bloom: %v", err)
	}
	got := out.At(0, 0)
	want := float32(0.75 + 0.75*0.5) // 1.125
	for i, name := range [3]string{"R", "G", "B"} {
		if math.Abs(float64(got[i]-want)) > bloomEps {
			t.Errorf("%s=%v，want %v（软斜坡 m=0.5）", name, got[i], want)
		}
	}
	if got[3] != 1 {
		t.Errorf("alpha=%v，加法不改透明度，want 1", got[3])
	}
}

// 钉 6：均匀亮场过完高斯仍均匀 —— 这条同时钉住核归一与边缘夹取
// （补零式边缘会让边界暗下去，正是本钉要抓的形态）。
func TestBloomUniformFieldStaysUniform(t *testing.T) {
	const v = 0.8
	src := newFilledTarget(t, 5, 5, func(_, _ int) [4]float32 { return [4]float32{v, v, v, 1} })
	out, err := src.Bloom(render.BloomParams{Threshold: 0, Sigma: 2, Intensity: 0.5})
	if err != nil {
		t.Fatalf("Bloom: %v", err)
	}
	// thr=0 ⇒ m=v ⇒ 掩膜均匀 v*v ⇒ 模糊后仍 v*v ⇒ out = v + 0.5*v*v
	want := float32(v + 0.5*v*v)
	for y := 0; y < 5; y++ {
		for x := 0; x < 5; x++ {
			got := out.At(x, y)
			for i := 0; i < 3; i++ {
				if math.Abs(float64(got[i]-want)) > bloomEps {
					t.Fatalf("(%d,%d) 分量 %d = %v，want %v（均匀场必须保持均匀）",
						x, y, i, got[i], want)
				}
			}
		}
	}
}

// 钉 7：模糊剖面必须等于共享高斯核，不是盒（批 5 拍的 P-2 两路同式）。
// 盒的形态是窗口内等值，高斯是 exp(-x²/2σ²) 衰减 —— 这条摘掉核替换必红。
// 亮心离两边都隔着一个核半径（15 宽、心在 7、half=3），所以核不会被边缘
// 折叠进来：窗口外必是 0，窗口内必是对应抽头。
func TestBloomProfileIsGaussianNotBox(t *testing.T) {
	const sigma = 1.0
	const width = 15
	const center = width / 2
	src := newFilledTarget(t, width, 1, func(x, _ int) [4]float32 {
		if x == center {
			return [4]float32{1, 1, 1, 1}
		}
		return [4]float32{}
	})
	out, err := src.Bloom(render.BloomParams{Threshold: 0.9, Sigma: sigma, Intensity: 1})
	if err != nil {
		t.Fatalf("Bloom: %v", err)
	}

	k := filter.CachedGaussianKernel(sigma)
	half := len(k) / 2
	if math.Abs(float64(k[half-1]/k[half])-math.Exp(-0.5)) > 1e-3 {
		t.Fatalf("共享核自身不对：K[half-1]/K[half]=%v，want exp(-0.5)=%v",
			k[half-1]/k[half], math.Exp(-0.5))
	}

	flat := true
	for x := 0; x < width; x++ {
		got := out.At(x, 0)[0] - src.At(x, 0)[0]
		want := float32(0)
		if d := x - center; d >= -half && d <= half {
			want = k[half+d] // 亮心掩膜值 1，增量就是那一枚抽头
		}
		if math.Abs(float64(got-want)) > bloomEps {
			t.Fatalf("x=%d 增量 %v，want 核抽头 %v（必须是 internal/filter 那枚高斯核）", x, got, want)
		}
		if math.Abs(float64(got-k[half])) > bloomEps {
			flat = false
		}
	}
	if flat {
		t.Fatal("剖面窗口内等值 —— 那是盒模糊，不是高斯")
	}
}

// 钉 8：横纵两趟可分离 —— 3×3 单亮心的输出必须等于核的一维外积。
func TestBloomIsSeparableOuterProduct(t *testing.T) {
	const sigma = 1.0
	src := newFilledTarget(t, 3, 3, func(x, y int) [4]float32 {
		if x == 1 && y == 1 {
			return [4]float32{1, 1, 1, 1}
		}
		return [4]float32{}
	})
	out, err := src.Bloom(render.BloomParams{Threshold: 0.9, Sigma: sigma, Intensity: 1})
	if err != nil {
		t.Fatalf("Bloom: %v", err)
	}
	k := filter.CachedGaussianKernel(sigma)
	half := len(k) / 2
	// 边界夹取后，一维剖面是 [K[half+1], K[half], K[half-1]]
	p := [3]float32{k[half+1], k[half], k[half-1]}
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			got := out.At(x, y)[0] - src.At(x, y)[0]
			want := p[x] * p[y]
			if math.Abs(float64(got-want)) > bloomEps {
				t.Errorf("(%d,%d) 增量 %v，want 外积 %v（可分离两趟）", x, y, got, want)
			}
		}
	}
}

// 钉 9：加法不回钳 —— >1 的光必须留到 Encode 的 Reinhard 收，
// 这才是 float 域相对 8-bit 的全部意义（提前钳就等于白做）。
func TestBloomKeepsSuperUnityLight(t *testing.T) {
	src := newFilledTarget(t, 1, 1, func(_, _ int) [4]float32 { return [4]float32{3, 3, 3, 1} })
	out, err := src.Bloom(render.BloomParams{Threshold: 0.5, Sigma: 1, Intensity: 1})
	if err != nil {
		t.Fatalf("Bloom: %v", err)
	}
	got := out.At(0, 0)
	// 掩膜用夹到 [0,1] 的亮度算（thr=0.5，lum=1 ⇒ m=1），增量 1；原值 3 不回钳。
	if math.Abs(float64(got[0]-4)) > bloomEps {
		t.Errorf("R=%v，want 4（3 的光 + 1 的辉，不许提前钳到 1）", got[0])
	}
	enc, err := out.Encode(1)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// 对照：同样这块光若被提前钳到 1.0，收口后就是一个固定的暗一档值。
	// Reinhard 是 x/(1+x)，有限光永远到不了 255，所以真正的判据是单调性：
	// 4.0 的光必须比 1.0 的光更亮，否则「留到收口再压」就是句空话。
	clamped := newFilledTarget(t, 1, 1, func(_, _ int) [4]float32 { return [4]float32{1, 1, 1, 1} })
	encClamped, err := clamped.Encode(1)
	if err != nil {
		t.Fatalf("Encode(clamped): %v", err)
	}
	gotR8, _, _, _ := enc.At(0, 0).RGBA()
	refR8, _, _, _ := encClamped.At(0, 0).RGBA()
	gotR, refR := gotR8>>8, refR8>>8
	if refR >= 255 {
		t.Fatalf("对照基线不对：Reinhard 下有限光不许落到纯白，实测 %d", refR)
	}
	if gotR <= refR || gotR >= 255 {
		t.Errorf("4.0 的光收口后 R=%d，必须严格亮于 1.0 的 %d 且不到纯白：提前钳色就是把强光压成同一档", gotR, refR)
	}
}

// 钉 10：alpha 逐纹素保留 src，模糊只碰 RGB。
func TestBloomPreservesAlpha(t *testing.T) {
	src := newFilledTarget(t, 4, 4, func(x, y int) [4]float32 {
		return [4]float32{1, 1, 1, float32(x*4+y) / 15}
	})
	out, err := src.Bloom(render.BloomParams{Threshold: 0.1, Sigma: 2, Intensity: 2})
	if err != nil {
		t.Fatalf("Bloom: %v", err)
	}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			if got, want := out.At(x, y)[3], src.At(x, y)[3]; got != want {
				t.Fatalf("(%d,%d) alpha 被改了：%v，want %v", x, y, got, want)
			}
		}
	}
}

// 钉 11：入参只读 —— 出光的那一帧不许把源帧写脏。
func TestBloomNeverMutatesSource(t *testing.T) {
	src := newFilledTarget(t, 6, 6, func(x, y int) [4]float32 {
		if x == 3 && y == 3 {
			return [4]float32{1, 1, 1, 1}
		}
		return [4]float32{0.2, 0.2, 0.2, 1}
	})
	before := copyFloatPixels(src.FloatPixels())
	if _, err := src.Bloom(render.BloomParams{Threshold: 0.5, Sigma: 3, Intensity: 2}); err != nil {
		t.Fatalf("Bloom: %v", err)
	}
	if !equalFloatSlice(src.FloatPixels(), before) {
		t.Fatal("Bloom 写脏了 src")
	}
}

// 钉 12：nil 源与非法参数都必须具名报错，且不给半个产物。
func TestBloomRejectsNilAndBadParams(t *testing.T) {
	var nilTarget *FloatTarget
	if out, err := nilTarget.Bloom(render.BloomParams{Sigma: 1, Intensity: 1}); err == nil || out != nil {
		t.Fatalf("nil 源必须报错且无产物：out=%v err=%v", out, err)
	}
	src := newFilledTarget(t, 2, 2, func(_, _ int) [4]float32 { return [4]float32{1, 1, 1, 1} })
	if out, err := src.Bloom(render.BloomParams{Sigma: -1, Intensity: 1}); err == nil || out != nil {
		t.Fatalf("非法参数必须报错且无产物：out=%v err=%v", out, err)
	}
}
