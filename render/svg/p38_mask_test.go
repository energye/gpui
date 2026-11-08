//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

import (
	"math"
	"testing"
)

// P3.8 批 2c（SVG mask）红测。语义口径：CSS Masking Level 1——luminance
// 遮罩（白显/黑隐/灰半）、maskUnits 区域、悬空引用 fail-closed（元素不画）。
// 实现=用户空间三离屏组合（mask 内容离屏→LuminanceMask→内容离屏→ApplyMask→
// DrawImage 回主 dc），CTM 天然正确（design.md §2.1-2c）。

func TestP38MaskLuminance(t *testing.T) {
	doc := mustParseFile(t, "mask_luminance.svg")
	img := doc.Render(100, 60)
	// 白区全显（蓝 74/144/226），黑区全隐，灰区半透（蓝×~0.5 混白底→浅蓝）。
	white := img.RGBAAt(15, 30)
	if white.B < 200 {
		t.Errorf("white zone = %+v, want full blue", white)
	}
	if a := img.RGBAAt(45, 30).A; a != 0 {
		t.Errorf("black zone = %+v, want fully hidden", img.RGBAAt(45, 30))
	}
	gray := img.RGBAAt(75, 30)
	// luminance(0x80)≈0.216(线性) → 蓝线性 0.216×原线性；sRGB 回投约 0.5 → A≈128±30。
	if a := gray.A; a < 80 || a > 180 {
		t.Errorf("gray zone = %+v, want ~half alpha", gray)
	}
	if gray.B >= white.B {
		t.Errorf("gray zone = %+v, want dimmer than white zone %+v", gray, white)
	}
}

func TestP38MaskRegionAndMissing(t *testing.T) {
	doc := mustParseFile(t, "mask_region_missing.svg")
	img := doc.Render(100, 80)
	// 区域 [10,50)：内显外隐。
	if c := img.RGBAAt(20, 15); c.G < 150 {
		t.Errorf("region inside = %+v, want green", c)
	}
	if a := img.RGBAAt(80, 15).A; a != 0 {
		t.Errorf("region outside = %+v, want hidden", img.RGBAAt(80, 15))
	}
	// 悬空 mask：整元素不画 + Warning。
	if a := img.RGBAAt(50, 50).A; a != 0 {
		t.Errorf("missing mask = %+v, want unpainted", img.RGBAAt(50, 50))
	}
	found := false
	for _, w := range doc.Warnings {
		if w.Reason == warnUnresolvedRef && w.Attribute == "mask" {
			found = true
		}
	}
	if !found {
		t.Errorf("missing mask warning missing: %+v", doc.Warnings)
	}
	// 对照 rect 仍绿。
	if got := img.RGBAAt(30, 75); got.G < 200 {
		t.Errorf("contrast rect = %+v, want green", got)
	}
}

func TestP38MaskOnGroup(t *testing.T) {
	// mask 挂 <g>：整组一起被遮。
	src := `<svg xmlns="http://www.w3.org/2000/svg" width="80" height="40" viewBox="0 0 80 40">
	  <defs>
	    <mask id="left" maskUnits="userSpaceOnUse" x="0" y="0" width="40" height="40">
	      <rect x="0" y="0" width="40" height="40" fill="#ffffff"/>
	    </mask>
	  </defs>
	  <g mask="url(#left)">
	    <rect x="0" y="0" width="80" height="40" fill="#e24a4a"/>
	  </g>
	</svg>`
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	img := doc.Render(80, 40)
	if c := img.RGBAAt(20, 20); c.R < 150 {
		t.Errorf("group mask inside = %+v, want red", c)
	}
	if a := img.RGBAAt(60, 20).A; a != 0 {
		t.Errorf("group mask outside = %+v, want hidden", img.RGBAAt(60, 20))
	}
}

// 批 4c 收尾洞：mask 区域 W/H 在 maskUnits=userSpaceOnUse 下是绝对长度，
// 不是坐标——旧实现 `- rx/- ry` 把每个非零原点区域裁坏（showcase 参照对拍
// 抓出：region (680,240,330,170) 算成 (680,240,-350,-70) → 整组不渲染）。
func TestP38MaskRegionUserSpaceOffsets(t *testing.T) {
	cases := []struct {
		name string
		src  string
		xy   [2]int
		want [3]float64
	}{
		{
			name: "非零原点区域+白内容",
			src: `<svg xmlns="http://www.w3.org/2000/svg" width="1024" height="640">
				<defs><mask id="m" maskUnits="userSpaceOnUse" x="100" y="50" width="200" height="100">
					<rect x="100" y="50" width="200" height="100" fill="#fff"/></mask></defs>
				<rect width="1024" height="640" fill="#1b2430"/>
				<rect x="100" y="50" width="200" height="100" fill="#4a90e2" mask="url(#m)"/>
			</svg>`,
			xy:   [2]int{150, 100},
			want: [3]float64{0.29, 0.565, 0.886},
		},
		{
			name: "组目标+非零原点",
			src: `<svg xmlns="http://www.w3.org/2000/svg" width="1024" height="640">
				<defs><mask id="m" maskUnits="userSpaceOnUse" x="680" y="240" width="330" height="170">
					<rect x="680" y="240" width="330" height="170" fill="#fff"/></mask></defs>
				<rect width="1024" height="640" fill="#1b2430"/>
				<g mask="url(#m)"><rect x="680" y="240" width="330" height="170" fill="#4a90e2"/></g>
			</svg>`,
			xy:   [2]int{845, 325},
			want: [3]float64{0.29, 0.565, 0.886},
		},
		{
			name: "渐变内容+非零原点（luminance 衰减）",
			src: `<svg xmlns="http://www.w3.org/2000/svg" width="1024" height="640">
				<defs>
					<radialGradient id="mg"><stop offset="0" stop-color="#fff"/><stop offset="1" stop-color="#404040"/></radialGradient>
					<mask id="m" maskUnits="userSpaceOnUse" x="680" y="240" width="330" height="170">
						<rect x="680" y="240" width="330" height="170" fill="url(#mg)"/></mask>
				</defs>
				<rect width="1024" height="640" fill="#1b2430"/>
				<g mask="url(#m)"><rect x="680" y="240" width="330" height="170" fill="#4a90e2"/></g>
			</svg>`,
			xy:   [2]int{845, 325},
			want: [3]float64{0.286, 0.557, 0.882}, // 白心不衰减
		},
	}
	for _, tc := range cases {
		doc, err := Parse([]byte(tc.src))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		img := doc.Render(1024, 640)
		r, g, b, _ := img.At(tc.xy[0], tc.xy[1]).RGBA()
		got := [3]float64{float64(r>>8) / 255, float64(g>>8) / 255, float64(b>>8) / 255}
		for i := range tc.want {
			if math.Abs(got[i]-tc.want[i]) > 0.05 {
				t.Errorf("%s at %v = %v, want %v", tc.name, tc.xy, got, tc.want)
				break
			}
		}
	}
}
