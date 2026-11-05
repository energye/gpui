//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// P3.7 批 5 · 步骤 2：CPU 8-bit 那条腿必须是 `BloomFloat` 的**薄封装**，不许有第二套算法。
//
// 钉法是「拿同一张源图，一条路走滤镜节点，另一条路直接把同一份源喂给核再量化」——两边逐位相同
// 才算证。这条摘掉（节点里另写一套卷积/另一套掩膜）必红；而它断言的是"两条路同源"，
// 数值的对错由 `render/hdr/bloom_test.go` 的手算期望与 `render/p37_5_bloom_test.go` 的均匀场手算负责。
package filter

import (
	"testing"

	"github.com/energye/gpui/render"
)

// 手写小图（premul uint8，值都取 1/255 的整档，避免量化歧义）：4×4，中间两列亮、边角暗。
var bloomPattern = [][4]uint8{
	{20, 20, 20, 255}, {120, 100, 60, 255}, {120, 100, 60, 255}, {10, 10, 10, 200},
	{30, 30, 30, 255}, {200, 200, 200, 255}, {180, 90, 40, 255}, {0, 0, 0, 0},
	{40, 20, 10, 255}, {220, 220, 220, 255}, {90, 90, 90, 255}, {5, 5, 5, 60},
	{10, 10, 10, 255}, {60, 60, 60, 255}, {255, 255, 255, 255}, {128, 0, 0, 128},
}

func newPatternPixmap(t *testing.T, w, h int) *render.Pixmap {
	t.Helper()
	pm := render.NewPixmap(w, h)
	for i, c := range bloomPattern {
		pm.SetPixelPremul(i%w, i/w, c[0], c[1], c[2], c[3])
	}
	return pm
}

// 钉 1：滤镜节点 ≡ 直接把源喂给 BloomFloat（同一份核，零第二实现）。
func TestP375BloomNodeIsThinOverCore(t *testing.T) {
	const w, h = 4, 4
	src := newPatternPixmap(t, w, h)

	viaNode := render.NewPixmap(w, h)
	f := NewBloomFilter(render.BloomParams{Threshold: 0.3, Sigma: 1.5, Intensity: 2})
	f.Apply(src, viaNode, fullRect(src))

	viaCore := render.NewPixmap(w, h)
	in := make([]float32, w*h*4)
	for i, c := range bloomPattern {
		j := i * 4
		in[j] = float32(c[0]) / 255
		in[j+1] = float32(c[1]) / 255
		in[j+2] = float32(c[2]) / 255
		in[j+3] = float32(c[3]) / 255
	}
	out := make([]float32, w*h*4)
	BloomFloat(out, in, w, h, 0.3, 1.5, 2)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			j := (y*w + x) * 4
			viaCore.SetPixelPremul(x, y, clampUint8(out[j]*255), clampUint8(out[j+1]*255),
				clampUint8(out[j+2]*255), clampUint8(out[j+3]*255))
		}
	}

	a, b := viaNode.Data(), viaCore.Data()
	if len(a) != len(b) {
		t.Fatalf("长度不等 %d/%d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("字节 %d（像素 %d 通道 %d）节点给 %d、核给 %d —— 节点不再是那一份核的薄封装",
				i, i/4, i%4, a[i], b[i])
		}
	}
}

// 钉 2：恒等参数逐位照抄（dst 的出口在两条腿上都得同一件事）。
func TestP375BloomNodeIdentityCopies(t *testing.T) {
	const w, h = 4, 4
	src := newPatternPixmap(t, w, h)
	dst := render.NewPixmap(w, h)
	NewBloomFilter(render.BloomParams{Threshold: 0.2, Sigma: 3, Intensity: 0}).Apply(src, dst, fullRect(src))
	a, b := src.Data(), dst.Data()
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("恒等参数改了字节 %d：%d vs %d", i, b[i], a[i])
		}
	}
}

// 钉 3：非法参数整笔不动（节点不做猜测式兜底，交回调用方的校验口）。
func TestP375BloomNodeRejectsBadParams(t *testing.T) {
	const w, h = 4, 4
	src := newPatternPixmap(t, w, h)
	for _, p := range []render.BloomParams{
		{Threshold: 2, Sigma: 1, Intensity: 1},
		{Threshold: 0.5, Sigma: -1, Intensity: 1},
		{Threshold: 0.5, Sigma: 1, Intensity: -1},
	} {
		if err := p.Validate(); err == nil {
			t.Fatalf("%+v 应被判非法", p)
		}
		dst := newPatternPixmap(t, w, h)
		base := append([]uint8(nil), dst.Data()...)
		NewBloomFilter(p).Apply(src, dst, fullRect(src))
		for i := range base {
			if dst.Data()[i] != base[i] {
				t.Fatalf("%+v 非法参数却改了像素（字节 %d）", p, i)
			}
		}
	}
}

// 量化尺子就是包内那把 clampUint8（0..255 域、四舍五入），节点与对照共用一枚，
// 所以这条钉的是「节点没另起一套算法也没另起一套量化」。
