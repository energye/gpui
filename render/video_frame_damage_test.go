//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package render

import (
	"image"
	"testing"
)

// TestVideoFrameDamageAndPixels 钉住 DrawImageEx 的两条路径都必须登记增量
// damage 且真的把像素画出来。
//
// 回归背景：GPU textured-quad 快路径（tryGPUDrawImage 成功）在 Fill() 之前
// 直接 return，而 damage 是 Fill() 里通过 c.trackDamage(c.path.Bounds())
// 登记的。带 GPU 会话时 DrawVideoFrame 因此只排队了 quad、既没登记脏区，
// 增量 present 就会漏掉这块画面（三个 video 单测报 "left no damage"）。
//
// 本测同时断言 damage 矩形与读回像素：只补一个脏区标记而不真画，同样算红。
// 门禁在带/不带 WGPU_NATIVE_PATH 两种环境下都会跑到，覆盖 CPU 与 GPU 两路。
func TestVideoFrameDamageAndPixels(t *testing.T) {
	dc := NewContext(64, 64)
	img, err := NewImageBuf(64, 36, FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	d := img.Data()
	for i := 0; i < len(d); i += 4 {
		d[i], d[i+1], d[i+2], d[i+3] = 220, 30, 30, 255
	}
	img.NotifyPixelsChanged()

	if !dc.DrawVideoFrame(img, VideoDrawOptions{X: 4, Y: 4, DstWidth: 32, DstHeight: 18}) {
		t.Fatal("DrawVideoFrame=false, want true")
	}

	want := image.Rect(4, 4, 36, 22)
	found := false
	for _, r := range dc.FrameDamage() {
		if r == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("FrameDamage()=%v, want a rect == %v (GPU quad path must register damage too)", dc.FrameDamage(), want)
	}

	std := dc.Image()
	for _, pt := range [][2]int{{5, 5}, {10, 10}, {35, 21}} {
		r, g, b, a := std.At(pt[0], pt[1]).RGBA()
		if int(r>>8) != 220 || int(g>>8) != 30 || int(b>>8) != 30 || int(a>>8) != 255 {
			t.Fatalf("pixel (%d,%d) = %d,%d,%d,%d; want 220,30,30,255 (quad must really land)",
				pt[0], pt[1], r>>8, g>>8, b>>8, a>>8)
		}
	}
	// 区域外必须保持空白，防止整屏铺色的假绿。
	_, _, _, a := std.At(50, 50).RGBA()
	if a>>8 != 0 {
		t.Fatalf("pixel (50,50) alpha=%d, want 0 (draw must be confined to dest rect)", a>>8)
	}
}
