//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"testing"
)

// 深审 D5 / L1-P1-2 钉：固定功能 13 模式的 Linear 请求必须显式降级为 sRGB
// 域执行（P3.1 拍板 D1「固定功能族双端一致显式降级」）。旧状
// compositeAdvanced 的 `if linear` 分支排在固定功能 switch 之前，13 模式被
// 路由到另一套 Porter-Duff 纯公式（Clear 忽略 coverage=半擦变全擦类），
// 既换算式又与 GPU 固定功能路分叉——「降级」名不副实。

func compositeAdvancedPixel(pm *Pixmap, mode BlendMode, linear bool) (uint8, uint8, uint8, uint8) {
	data := pm.Data()
	i := (4*pm.Width() + 4) * 4
	// 不透明底（premul=直值）：0.6/0.3/0.2 → 153,77,51,255。
	data[i], data[i+1], data[i+2], data[i+3] = 153, 77, 51, 255
	// 半覆盖纯黑源（coverage=128）：降级契约下 linear 与 sRGB 必须逐字节同。
	compositeAdvanced(pm, 4, 4, RGBA{R: 0, G: 0, B: 0, A: 1}, 128, mode, linear)
	r, g, b, a := pixelAt(pm, 4, 4)
	return r, g, b, a
}

func TestFixedFunctionLinearDegradesToSRGB(t *testing.T) {
	fixed := []BlendMode{
		BlendClear, BlendCopy, BlendDestinationOver,
		BlendSourceIn, BlendDestinationIn, BlendSourceOut, BlendDestinationOut,
		BlendSourceAtop, BlendDestinationAtop, BlendXor,
		BlendPlus, BlendModulate,
	}
	for _, mode := range fixed {
		if IsAdvancedBlendMode(mode) {
			t.Fatalf("测试表失准：%d 已是 advanced 模式", mode)
		}
		pmS := NewPixmap(8, 8)
		pmL := NewPixmap(8, 8)
		sr, sg, sb, sa := compositeAdvancedPixel(pmS, mode, false)
		lr, lg, lb, la := compositeAdvancedPixel(pmL, mode, true)
		if sr != lr || sg != lg || sb != lb || sa != la {
			t.Fatalf("mode=%d 固定功能+Linear 未降级：linear=(%d,%d,%d,%d) vs sRGB=(%d,%d,%d,%d)（半覆盖源必须与 sRGB 域逐字节一致）",
				mode, lr, lg, lb, la, sr, sg, sb, sa)
		}
	}
}
