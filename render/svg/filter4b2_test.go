//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

import (
	"testing"

	_ "github.com/energye/gpui/render/filters"
)

// P3.8 批 4b-2（SVG 滤镜光照+算术合成）钉测。feDiffuseLighting /
// feSpecularLighting（三光源×两模式，resvg lighting.rs 逐字直译）+
// feComposite arithmetic（resvg composite.rs：premul 域 k 系数、RGB 钳到
// 新 alpha）。resvg 语料样本入 testdata/p38/filter4b/（三光源目录样本带
// 目录前缀），感知分级 ≤ close + 关键像素逻辑探针 + 负向（specularExponent
// 出域 → 透明输出）。

func TestP384b2DiffuseDistantSeagreen(t *testing.T) {
	// feDistantLight azimuth=45 elevation=45、surfaceScale=10、sRGB 域：
	// 输出为不透明 lit 图（diffuse alpha=255），seagreen 光色在场。
	doc := mustParseFile(t, "filter4b/lighting-color=seagreen.svg")
	img := doc.Render(200, 200)
	c := img.RGBAAt(100, 100)
	if c.A != 255 {
		t.Errorf("diffuse center alpha = %d, want 255 (opaque output)", c.A)
	}
	if !(c.G >= c.R && c.G >= c.B) {
		t.Errorf("diffuse center = %+v, want seagreen-dominant light", c)
	}
	gradeAgainstPNG4b2(t, doc, "lighting-color=seagreen")
}

func TestP384b2DiffuseDistantDefaults(t *testing.T) {
	gradeAgainstPNG4b2(t, mustParseFile(t, "filter4b/feDistantLight-default-attributes.svg"), "feDistantLight-default-attributes")
}

func TestP384b2DiffusePointLight(t *testing.T) {
	gradeAgainstPNG4b2(t, mustParseFile(t, "filter4b/fePointLight-custom-attributes.svg"), "fePointLight-custom-attributes")
}

func TestP384b2DiffuseSpotConeZero(t *testing.T) {
	// limitingConeAngle=0：cos(0)=1，光锥外全黑（resvg light_color 钳制）。
	gradeAgainstPNG4b2(t, mustParseFile(t, "filter4b/feSpotLight-limitingConeAngle=0.svg"), "feSpotLight-limitingConeAngle=0")
}

func TestP384b2SpecularDistant(t *testing.T) {
	// specular 输出 alpha=max(R,G,B)（premul 域）——非 lit 区应全透明。
	doc := mustParseFile(t, "filter4b/feSpecularLighting-with-feDistantLight.svg")
	img := doc.Render(300, 300)
	// flat 区 (15,15)：lit 直出 (38,115,72,115)±2（premul↔straight 往返
	// 固有 ±1 取整；resvg 一致）。双乘回归的特征值是 (17,51,32,115)——
	// premul 字节被当 straight 又 premul 化一次，G 掉到 51 一眼可辨。
	c := img.RGBAAt(150, 150)
	if c.G < 100 || c.G < c.R || c.R < 30 {
		t.Errorf("specular flat output = %v, want ~(38,115,72,115) not double-multiplied", c)
	}
	gradeAgainstPNG4b2(t, doc, "feSpecularLighting-with-feDistantLight")
}

func TestP384b2SpecularExponentOutOfRange(t *testing.T) {
	// specularExponent=0/256 出域 [1,128] → dummy primitive → 透明输出。
	for _, name := range []string{"specularExponent=0", "specularExponent=256"} {
		doc := mustParseFile(t, "filter4b/"+name+".svg")
		img := doc.Render(200, 200)
		if a := img.RGBAAt(100, 100).A; a != 0 {
			t.Errorf("%s: center alpha = %d, want 0 (out-of-range exponent)", name, a)
		}
		gradeAgainstPNG4b2(t, doc, name)
	}
}

func TestP384b2Arithmetic(t *testing.T) {
	// resvg composite.rs：result = k1·i1·i2 + k2·i1 + k3·i2 + k4（premul 域，
	// RGB 钳到新 alpha；alpha≈0 → 全透明）。
	for _, name := range []string{
		"operator=arithmetic",
		"operator=arithmetic-with-opacity",
		"operator=arithmetic-without-k1-4",
		"operator=arithmetic-on-sRGB",
	} {
		gradeAgainstPNG4b2(t, mustParseFile(t, "filter4b/"+name+".svg"), name)
	}
}

func TestP384b2ArithmeticParse(t *testing.T) {
	doc := mustParseFile(t, "filter4b/operator=arithmetic.svg")
	f := filterOf(t, doc, "filter1")
	var found bool
	for _, p := range f.Primitives {
		if p.Kind == "feComposite" && (p.K1 != 0 || p.K2 != 0 || p.K3 != 0 || p.K4 != 0) {
			found = true
		}
	}
	if !found {
		t.Fatal("arithmetic feComposite with non-zero k not parsed")
	}
}

func TestP384b2LightingParse(t *testing.T) {
	doc := mustParseFile(t, "filter4b/lighting-color=seagreen.svg")
	f := filterOf(t, doc, "filter1")
	p := f.Primitives[0]
	if p.Kind != "feDiffuseLighting" || p.SurfaceScale != 10 {
		t.Fatalf("lighting parse: kind=%s surfaceScale=%v", p.Kind, p.SurfaceScale)
	}
	if p.Light == nil || p.Light.Type != 0 || p.Light.Azimuth != 45 || p.Light.Elevation != 45 {
		t.Fatalf("light source = %+v, want distant 45/45", p.Light)
	}
	if p.LightingColor != "seagreen" {
		t.Errorf("lighting-color = %q", p.LightingColor)
	}
}

// gradeAgainstPNG4b2 是 filter4b_test.go 分级辅助的 4b-2 别名入口（同阈值）。
func gradeAgainstPNG4b2(t *testing.T, doc *Document, name string) {
	t.Helper()
	_ = doc
	gradeSample(t, name)
}
