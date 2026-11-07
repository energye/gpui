package svg

import (
	_ "embed"
	"image"
	"testing"
)

// P3.8 批 6 补遗：真实复杂度组合回归钉。三处组合洞由真实语料实测暴露
// （图标/图表/插画三层样例），修复为：
//   1. clipPathUnits 缺省 userSpaceOnUse（spec；Units 零值=oBB 陷阱，批 2
//      修 pattern 时漏了 clipPath——缺省 units 的 clip 全体被错误缩放裁空）
//   2. maskContentUnits 缺省 userSpaceOnUse（批 2 已修，此处为回归钉）
//   3. elementBBox 补 *UseElement——纯 use 的 <g mask> 整组丢弃
// 组合钉（隔离矩阵实测值）：
//   clip(g)+下半区 → 下半有墨上半背景；mask(渐变)+use → 亮度半透明；
//   clip(g)>circle(filter+径向渐变) → 橙色可见；clipPathUnits=oBB 显式
//   声明 → 分数坐标语义保持（显式 oBB 行为不变）。

//go:embed testdata/p38/realworld_l1_icon.svg
var realworldL1 string

//go:embed testdata/p38/realworld_l3_illustration.svg
var realworldL3 string

func TestP38RealWorld_IconRenders(t *testing.T) {
	doc, err := Parse([]byte(realworldL1))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Warnings) != 0 {
		t.Errorf("icon warnings = %d, want 0", len(doc.Warnings))
	}
	img := doc.Render(128, 128)
	// 主环墨迹采样（外环笔画上）
	if c, ok := sampleRGBA8(img, 118, 64); !ok || c[0] < 150 {
		t.Errorf("outer ring ink at (118,64) = %v, want red ring", c)
	}
}

func TestP38RealWorld_IllustrationCombos(t *testing.T) {
	doc, err := Parse([]byte(realworldL3))
	if err != nil {
		t.Fatal(err)
	}
	img := doc.Render(480, 280)
	// 1. clip(g)+filter+径向渐变：光晕中心 (240,180) 应为亮橙
	if c, ok := sampleRGBA8(img, 240, 180); !ok || c[0] < 200 {
		t.Errorf("glow center = %v, want bright orange through clip+filter", c)
	}
	// 2. mask(渐变)+use 星星：星星中心亮度被径向 mask 调制（非背景、非全白）
	for _, pt := range [][2]int{{60, 60}, {120, 90}, {380, 50}, {420, 110}} {
		c, ok := sampleRGBA8(img, pt[0], pt[1])
		if !ok {
			t.Errorf("star at %v outside image", pt)
			continue
		}
		lum := (int(c[0]) + int(c[1]) + int(c[2])) / 3
		bg := 60 // 天空顶部暗紫的近似亮度
		if lum <= bg+10 {
			t.Errorf("star at %v lost (lum=%d) — mask+use combo broken", pt, lum)
		}
	}
	// 3. clip(g) 下半区语义：clip 下边界外（y=135，上半）无光晕
	if c, ok := sampleRGBA8(img, 240, 100); ok {
		// 上半区是天空渐变（暗紫），不应出现亮橙光晕
		if c[0] > 120 && c[2] < 130 {
			t.Errorf("glow leaked above clip at (240,100) = %v", c)
		}
	}
}

func TestP38RealWorld_ClipUnitsDefault(t *testing.T) {
	// 缺省 units：用户空间坐标 clip（下半区可见）
	def := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100">
<defs><clipPath id="c"><rect x="0" y="50" width="100" height="50"/></clipPath></defs>
<rect width="100" height="100" fill="#333"/>
<circle cx="50" cy="75" r="20" fill="#fff" clip-path="url(#c)"/></svg>`
	img := renderFor(def)
	if c, ok := sampleRGBA8(img, 50, 75); !ok || c[0] < 200 {
		t.Errorf("default-units clip lost content: (50,75)=%v", c)
	}
	// 显式 oBB：分数坐标语义保持（0..1 覆盖全画布的 rect）
	oBB := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100">
<defs><clipPath id="c" clipPathUnits="objectBoundingBox"><rect x="0" y="0" width="1" height="1"/></clipPath></defs>
<rect width="100" height="100" fill="#333"/>
<circle cx="50" cy="75" r="20" fill="#fff" clip-path="url(#c)"/></svg>`
	img2 := renderFor(oBB)
	if c, ok := sampleRGBA8(img2, 50, 75); !ok || c[0] < 200 {
		t.Errorf("explicit oBB clip semantics changed: (50,75)=%v", c)
	}
}

func TestP38RealWorld_MaskedUseGroup(t *testing.T) {
	// 纯 use 的 masked group：批 6 补遗前整组丢失
	src := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100">
<defs><mask id="m"><rect width="100" height="100" fill="#fff"/></mask>
<g id="star"><circle r="4" fill="#fff"/></g></defs>
<rect width="100" height="100" fill="#333"/>
<g mask="url(#m)"><use href="#star" x="50" y="50"/></g></svg>`
	img := renderFor(src)
	if c, ok := sampleRGBA8(img, 50, 50); !ok || c[0] < 200 {
		t.Errorf("masked use group lost: (50,50)=%v", c)
	}
}

func renderFor(src string) *image.RGBA {
	doc, err := Parse([]byte(src))
	if err != nil {
		return nil
	}
	return doc.Render(100, 100)
}

func sampleRGBA8(img *image.RGBA, x, y int) ([3]uint8, bool) {
	if img == nil {
		return [3]uint8{}, false
	}
	b := img.Bounds()
	if x < b.Min.X || y < b.Min.Y || x >= b.Max.X || y >= b.Max.Y {
		return [3]uint8{}, false
	}
	r, g, bl, a := img.At(x, y).RGBA()
	if a == 0 {
		return [3]uint8{}, false
	}
	return [3]uint8{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8)}, true
}
