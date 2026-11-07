//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"
)

// P3.8 批 2d（SVG <image>）红测。语义口径：data: URI 内嵌栅格（跨文件引用
// 显式拒绝，D2）+ preserveAspectRatio 对图像视口（默认 xMidYMid meet）。
// 样本 PNG = testdata/p38/img_rgba.png（4×2：左半 #e24a4a 右半 #4a90e2）。

func imageDataURL(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/p38/img_rgba.png")
	if err != nil {
		t.Fatalf("read png: %v", err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)
}

func TestP38ImageDataURLMeet(t *testing.T) {
	// 4×2 图放进 80×80 视口：meet 等比 s=20 → 内容 80×40 垂直居中。
	src := `<svg xmlns="http://www.w3.org/2000/svg" width="80" height="80" viewBox="0 0 80 80">
	  <rect x="0" y="0" width="80" height="80" fill="#171a20"/>
	  <image x="0" y="0" width="80" height="80" href="` + imageDataURL(t) + `"/>
	</svg>`
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	imgEl, ok := doc.Elements[1].(*ImageElement)
	if !ok {
		t.Fatalf("Elements[1] = %T, want *ImageElement", doc.Elements[1])
	}
	if !imgEl.HasImage || imgEl.ImgW != 4 || imgEl.ImgH != 2 {
		t.Fatalf("decoded image = has=%v %dx%d, want 4x2", imgEl.HasImage, imgEl.ImgW, imgEl.ImgH)
	}
	img := doc.Render(80, 80)
	// 内容带 y∈[20,60)：上 20px 是底色、带内左红右蓝。
	// 带上方是样本自绘底色 #171a20（不是透明）。
	if c := img.RGBAAt(40, 10); diff(float64(c.R), 23) > 10 || diff(float64(c.G), 26) > 10 {
		t.Errorf("above band = %+v, want sample bg #171a20", c)
	}
	if c := img.RGBAAt(20, 40); c.R < 180 {
		t.Errorf("band left = %+v, want red-ish", c)
	}
	if c := img.RGBAAt(60, 40); c.B < 180 {
		t.Errorf("band right = %+v, want blue-ish", c)
	}
}

func TestP38ImageSliceClips(t *testing.T) {
	// slice：4×2 图进 40×80 视口 → cover s=20 → 内容 80×40，横向溢出被裁。
	src := `<svg xmlns="http://www.w3.org/2000/svg" width="40" height="80" viewBox="0 0 40 80">
	  <image x="0" y="0" width="40" height="80" preserveAspectRatio="xMidYMid slice" href="` + imageDataURL(t) + `"/>
	</svg>`
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	img := doc.Render(40, 80)
	// 全高覆盖：上/中/下都有内容；内容左红右蓝（视口宽 40 < 内容 80，居中裁剪后
	// 视口x∈[20,60] 对应内容 t∈[0.25,0.75] —— 中缝在 x=40 用户即视口中心，左红右蓝）。
	if a := img.RGBAAt(20, 5).A; a == 0 {
		t.Errorf("slice top = %+v, want covered", img.RGBAAt(20, 5))
	}
	if a := img.RGBAAt(20, 75).A; a == 0 {
		t.Errorf("slice bottom = %+v, want covered", img.RGBAAt(20, 75))
	}
	if c := img.RGBAAt(10, 40); c.R < 150 {
		t.Errorf("slice left = %+v, want red-ish", c)
	}
	if c := img.RGBAAt(30, 40); c.B < 150 {
		t.Errorf("slice right = %+v, want blue-ish", c)
	}
}

func TestP38ImageExternalRefused(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 40 40">
	  <image x="0" y="0" width="40" height="40" href="https://example.com/pic.png"/>
	  <rect x="30" y="30" width="8" height="8" fill="#00ff00"/>
	</svg>`
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	img := doc.Render(40, 40)
	if a := img.RGBAAt(10, 10).A; a != 0 {
		t.Errorf("external image = %+v, want unpainted", img.RGBAAt(10, 10))
	}
	found := false
	for _, w := range doc.Warnings {
		if w.Reason == warnUnsupportedRef && w.Attribute == "href" {
			found = true
		}
	}
	if !found {
		t.Errorf("external href warning missing: %+v", doc.Warnings)
	}
	if got := img.RGBAAt(34, 34); got.G < 200 {
		t.Errorf("contrast rect = %+v, want green", got)
	}
}

func TestP38ImageBadDataRefused(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 40 40">
	  <image x="0" y="0" width="40" height="40" href="data:image/png;base64,not-valid-base64!!!"/>
	</svg>`
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Elements[0].(*ImageElement).HasImage {
		t.Fatal("bad data URI decoded")
	}
	img := doc.Render(40, 40)
	if a := img.RGBAAt(20, 20).A; a != 0 {
		t.Errorf("bad data image = %+v, want unpainted", img.RGBAAt(20, 20))
	}
	found := false
	for _, w := range doc.Warnings {
		if w.Reason == warnInvalidValue && w.Attribute == "href" {
			found = true
		}
	}
	if !found {
		t.Errorf("bad data warning missing: %+v", doc.Warnings)
	}
	_ = strings.TrimSpace
}
