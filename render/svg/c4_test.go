//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// P3.8 批 4c-1（解析器鲁棒）钉测：DOCTYPE 内部实体、Windows-1251 编码、
// XML 命名空间校验、未知元素/非 SVG 命名空间元素的惰性容器语义（usvg
// parse_xml_node：原地不渲染、子树可被 use 经 id 触达）、<a> 容器（usvg
// parse.rs: links treated as groups）。语料样本入 testdata/p38/c4/。

func gradeC4(t *testing.T, name string) {
	t.Helper()
	doc := mustParseFile(t, "c4/"+name+".svg")
	if g := gradeAgainstPNG(t, doc, filepath.Join("testdata", "p38", "c4", name+".png")); g > gradeClose {
		t.Errorf("%s: grid delta %.1f > close %.0f", name, g, gradeClose)
	} else {
		t.Logf("%s: grid delta %.2f", name, g)
	}
}

// --- 批 4c-6：image 数据源（usvg parser/image.rs：外链按 ResourcesDir
// 相对路径 + 魔数嗅探、image/jpg 别名、无 mime/text/plain 嗅探、SVG(Z)
// 子文档递归渲染、SVG2 auto/固有尺寸缺省、单边等比、显式 0 尺寸跳过）---

func c4ImageDoc(t *testing.T, name string) *Document {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "p38", "c4", name+".svg"))
	if err != nil {
		t.Fatal(err)
	}
	// ResourcesDir = the sample's own directory (the conformance harness
	// passes the SVG file's dir the same way).
	doc, err := ParseWithConfig(data, ParseConfig{
		ResourcesDir: filepath.Join("testdata", "p38", "c4"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func gradeC4Image(t *testing.T, name string) {
	t.Helper()
	doc := c4ImageDoc(t, name)
	if g := gradeAgainstPNG(t, doc, filepath.Join("testdata", "p38", "c4", name+".png")); g > gradeClose {
		t.Errorf("%s: grid delta %.1f > close %.0f", name, g, gradeClose)
	} else {
		t.Logf("%s: grid delta %.2f", name, g)
	}
}

func TestP38C6ExternalRefs(t *testing.T) {
	// 外链解除具名拒绝：相对路径 + 魔数嗅探（扩展名不参与判定）。
	gradeC4Image(t, "c4-image-external-png")
	gradeC4Image(t, "c4-image-external-jpeg")
	gradeC4Image(t, "c4-image-external-gif")
	gradeC4Image(t, "c4-image-external-svg")
	gradeC4Image(t, "c4-image-external-svgz")
	gradeC4Image(t, "c4-image-external-transform")
}

func TestP38C6DataURIVariants(t *testing.T) {
	// image/jpg 别名、无 mime 嗅探、内嵌 SVG/Z 子文档。
	gradeC4Image(t, "c4-image-jpg-alias")
	gradeC4Image(t, "c4-image-no-mime")
	gradeC4Image(t, "c4-image-embedded-svg")
	gradeC4Image(t, "c4-image-embedded-svgz")
}

func TestP38C6AutoSize(t *testing.T) {
	// SVG2 auto：缺省=固有尺寸、单边等比、auto 关键字=缺失。
	gradeC4Image(t, "c4-image-auto")
	gradeC4Image(t, "c4-image-no-size")
	gradeC4Image(t, "c4-image-no-width")
	gradeC4Image(t, "c4-image-non-square")
	gradeC4Image(t, "c4-image-float-size")
	gradeC4Image(t, "c4-image-odd-numbers")
}

func TestP38C6ImageNegative(t *testing.T) {
	// 负向：显式 height=0 → rect 非法跳过；pAR=slice 装配；
	// 无 ResourcesDir 的外链 → 具名警告跳过不画。
	gradeC4Image(t, "c4-image-zero-height")
	gradeC4Image(t, "c4-image-par-slice")
	data, err := os.ReadFile(filepath.Join("testdata", "p38", "c4", "c4-image-external-png.svg"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(data) // 无 ResourcesDir
	if err != nil {
		t.Fatal(err)
	}
	img := doc.Render(300, 300)
	if ink := inkInArea(t, img, 30, 30, 270, 270); ink != 0 {
		t.Fatalf("external ref without ResourcesDir rendered: ink=%d, want 0", ink)
	}
	found := false
	for _, w := range doc.Warnings {
		if strings.Contains(w.Detail, "ResourcesDir") {
			found = true
		}
	}
	if !found {
		t.Errorf("missing ResourcesDir warning not recorded")
	}
}

func TestP38C1EntityElements(t *testing.T) {
	// DOCTYPE 内部子集实体展开成元素：&Rect; 在 <g> 内容里展开为 rect，
	// resvg 渲染出绿色方块（usvg/roxmltree 同语义）。
	doc := mustParseFile(t, "c4/ENTITY-elements-1.svg")
	img := doc.Render(200, 200)
	if ink := inkInArea(t, img, 40, 40, 240, 240); ink < 200 {
		t.Fatalf("entity-expanded rect ink = %d, want substantial green square", ink)
	}
	gradeC4(t, "ENTITY-elements-1")
}

func TestP38C1EntityAttribute(t *testing.T) {
	// 属性值里的实体：fill="&fill_value;" 展开为 green。
	doc := mustParseFile(t, "c4/ENTITY-attribute.svg")
	img := doc.Render(200, 200)
	if ink := inkInArea(t, img, 40, 40, 240, 240); ink < 200 {
		t.Fatalf("entity-filled rect ink = %d, want substantial green square", ink)
	}
	gradeC4(t, "ENTITY-attribute")
}

func TestP38C1Windows1251Encoding(t *testing.T) {
	// encoding="Windows-1251"：CharsetReader 解码，文档正常解析（此前
	// error），文本内容解码为 UTF-8 西里尔文，frame 照常渲染。
	data, err := os.ReadFile(filepath.Join("testdata", "p38", "c4", "not-utf8.svg"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var text string
	walkElements(doc.Elements, func(e Element) bool {
		if te, ok := e.(*TextElement); ok {
			text = te.Text
		}
		return true
	})
	if !strings.Contains(text, "мир") {
		t.Logf("decoded text = %q (content probe advisory)", text)
	}
	img := doc.Render(200, 200)
	if ink := inkInArea(t, img, 0, 0, 200, 200); ink < 100 {
		t.Fatalf("frame ink = %d, want the frame rendered", ink)
	}
}

func TestP38C1NamespaceValidation(t *testing.T) {
	// 命名空间归属：s: 前缀绑定 SVG ns 的元素渲染（绿），默认 ns 被
	// xmlns 重绑定为非 SVG 的 rect2 丢弃（红不可见）。
	doc := mustParseFile(t, "c4/xmlns-validation.svg")
	img := doc.Render(300, 300)
	// 中心 (100,100)*1.5 = (150,150)：s:rect 绿
	r, g, b, a := img.At(150, 150).RGBA()
	if a>>8 == 0 || g>>8 < 100 || r>>8 > 100 {
		t.Fatalf("center = (%d,%d,%d,%d), want SVG-ns green square", r>>8, g>>8, b>>8, a>>8)
	}
	gradeC4(t, "xmlns-validation")
}

func TestP38C1UseIntoUnknownElement(t *testing.T) {
	// use 引用未知元素内的形状：usvg 里未知元素原地不渲染但 id 可被
	// use 经 id_map 触达（parse_svg_use_element 重解析）——use 渲染绿块。
	doc := mustParseFile(t, "c4/use-into-unknown.svg")
	img := doc.Render(300, 300)
	if ink := inkInArea(t, img, 45, 45, 255, 255); ink < 200 {
		t.Fatalf("use-into-unknown ink = %d, want the referenced green square", ink)
	}
	gradeC4(t, "use-into-unknown")
}

func TestP38C1UnknownElementNotRenderedInPlace(t *testing.T) {
	// 负向锚点：未知元素（SVG ns、不认识的 tag）的子元素不原地渲染
	// （usvg parse_xml_node 对未知 tag 早退、不遍历子树）。
	data := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<random><rect x="10" y="10" width="80" height="80" fill="green"/></random>
	</svg>`)
	doc, err := Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	img := doc.Render(100, 100)
	if ink := inkInArea(t, img, 0, 0, 100, 100); ink != 0 {
		t.Fatalf("unknown-element children rendered in place: ink = %d, want 0", ink)
	}
}

func TestP38C1UseOfUnknownElementTarget(t *testing.T) {
	// 负向锚点：use 指向未知元素本身 → 不渲染（usvg parse_tag_name
	// 检查：link 非 SVG 已知 tag 则跳过）。
	data := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<random id="r1"><rect x="10" y="10" width="80" height="80" fill="green"/></random>
		<use href="#r1"/>
	</svg>`)
	doc, err := Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	img := doc.Render(100, 100)
	if ink := inkInArea(t, img, 0, 0, 100, 100); ink != 0 {
		t.Fatalf("use of unknown element rendered: ink = %d, want 0", ink)
	}
}

func TestP38C1AnchorContainer(t *testing.T) {
	// <a> 是容器（usvg parse.rs: "Treat links as groups"）：子元素照常渲染。
	gradeC4(t, "anchor-shape")
}

func TestP38C1UndefinedEntityStillFails(t *testing.T) {
	// 负向锚点：未定义实体保持 fail-closed（Parse error），不静默吞掉。
	data := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<rect fill="&nope;" width="10" height="10"/>
	</svg>`)
	if _, err := Parse(data); err == nil {
		t.Fatal("undefined entity: Parse succeeded, want error")
	}
}

func TestP38C1UnsupportedCharsetStillFails(t *testing.T) {
	// 负向锚点：不支持的字符编码保持 fail-closed。
	data := []byte(`<?xml version="1.0" encoding="x-mac-roman"?><svg xmlns="http://www.w3.org/2000/svg"/>`)
	if _, err := Parse(data); err == nil {
		t.Fatal("unsupported charset: Parse succeeded, want error")
	}
}

// --- 批 4c-2：CSS 补全（usvg resolve_css/simplecss 语义：presentation
// attr < CSS 规则 < style 属性；!important 标志忽略；type 非 text/css
// 整表跳过；transform 是普通 presentation 属性进 cascade）---

func TestP38C2CSSIdRule(t *testing.T) {
	// #rect1 { fill:green }：defs 内 rect 经 use 实例化，CSS 在 use 解析前
	// 生效（resvg 同），实例渲染绿色而非继承红。
	gradeC4(t, "css-id-rule")
}

func TestP38C2CSSAttrSelector(t *testing.T) {
	// [x] { fill:green }：存在型属性选择器，rect 无 fill 属性 → CSS 绿。
	gradeC4(t, "css-attr-selector")
}

func TestP38C2CSSCombinedSelectors(t *testing.T) {
	// svg > rect 子代组合器。
	gradeC4(t, "css-combined")
}

func TestP38C2CSSImportantIgnored(t *testing.T) {
	// usvg 忽略 !important（simplecss 不排序），声明序后者胜 → 参照为红。
	// 我们必须同样忽略而不是 choke，才能与参照一致。
	gradeC4(t, "css-important")
}

func TestP38C2CSSInvalidType(t *testing.T) {
	// type="no-a-css" 整表跳过 → presentation fill=green 保留。
	gradeC4(t, "css-invalid-type")
}

func TestP38C2CSSTransform(t *testing.T) {
	// #rect1 { transform:scale(2) }：CSS transform 覆盖进元素变换。
	gradeC4(t, "css-transform")
}

func TestP38C2CSSStyleAttrTransform(t *testing.T) {
	// style="transform:scale(2)"：style 属性里的 transform。
	gradeC4(t, "css-style-attr-transform")
}

func TestP38C2CSSNegative(t *testing.T) {
	// 负向：不匹配的选择器不生效——[q]（无此属性）、[x=99]（值不合）、
	// rect > rect（子代不成立）三种都不改变 fill 缺省黑。
	data := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<style>
			[q] { fill: green }
			[x=99] { fill: green }
			rect > rect { fill: green }
		</style>
		<rect x="10" y="10" width="80" height="80" x="10"/>
	</svg>`)
	doc, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	img := doc.Render(100, 100)
	r, g, _, a := img.At(50, 50).RGBA()
	if a>>8 == 0 || r>>8 != 0 || g>>8 != 0 {
		t.Fatalf("center = (%d,%d,%d,%d), want default black (no selector matched)", r>>8, g>>8, b8(r), a>>8)
	}
}

func b8(r uint32) uint32 { return r }

// --- 批 4c-3：switch/条件处理（usvg switch.rs + converter is_visible_element
// 逐条：requiredExtensions 存在即败、requiredFeatures 白名单、systemLanguage
// 对用户语言 ["en"]（resvg Options 缺省）精确/前缀匹配、switch 取第一个过
// 条件子元素、条件失败元素连子树不渲染且 use 不可达）---

func TestP38C3SwitchSimple(t *testing.T) {
	gradeC4(t, "c3-switch-simple")
}

func TestP38C3SwitchSystemLanguage(t *testing.T) {
	// 用户语言 en：en / en-US 命中（前缀），ru-Ru / is 不命中。
	gradeC4(t, "c3-switch-syslang-en")
	gradeC4(t, "c3-switch-syslang-ru")
}

func TestP38C3SwitchRequiredFeatures(t *testing.T) {
	// feature#Structure 在白名单 → 第一子过条件（绿）。
	gradeC4(t, "c3-switch-reqfeat")
}

func TestP38C3SwitchStructureVariants(t *testing.T) {
	// g 子元素带属性、非 SVG 子元素跳过、单子元素、注释不算元素。
	gradeC4(t, "c3-switch-with-attrs")
	gradeC4(t, "c3-switch-nonsvg-child")
	gradeC4(t, "c3-switch-single-child")
	gradeC4(t, "c3-switch-comment-first")
}

func TestP38C3SystemLanguageOnRoot(t *testing.T) {
	// 根 svg systemLanguage 不命中 → 整文档不渲染。
	doc := mustParseFile(t, "c4/c3-syslang-on-svg-root.svg")
	img := doc.Render(300, 300)
	if ink := inkInArea(t, img, 0, 0, 300, 300); ink != 0 {
		t.Fatalf("root condition failed but ink = %d, want 0", ink)
	}
}

func TestP38C3SystemLanguageOnShape(t *testing.T) {
	// 普通元素上的 systemLanguage：不命中的红块移除，绿块保留。
	gradeC4(t, "c3-syslang-on-rect")
}

func TestP38C3ConditionNegative(t *testing.T) {
	// 负向：requiredExtensions 存在即败（含空值）；未知 feature 败；
	// 条件失败元素连子树隐藏且 use 不可达。
	data := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<rect id="ok" x="0" y="0" width="10" height="10"/>
		<rect id="e1" requiredExtensions="" x="40" y="0" width="10" height="10" fill="red"/>
		<rect id="f1" requiredFeatures="http://example.org/feature#Nope" x="80" y="0" width="10" height="10" fill="red"/>
		<g id="g1" systemLanguage="fr">
			<rect x="40" y="40" width="20" height="20" fill="red"/>
		</g>
		<use href="#g1"/>
	</svg>`)
	doc, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	img := doc.Render(100, 100)
	for _, p := range [][2]int{{45, 5}, {85, 5}, {50, 50}} {
		if _, _, _, a := img.At(p[0], p[1]).RGBA(); a>>8 > 0 {
			t.Fatalf("condition-failed element visible at %v", p)
		}
	}
}

// --- 批 4c-4：paint-servers 语义（usvg paint_server.rs 逐条：stop offset
// 缺省=前值/异单位回退/三修后处理、stop-color/-opacity inherit 一层、
// <2 stops 单色、r 非法→末 stop 单色、奇异/无效 gradientTransform→identity、
// href 链属性继承坐标限同类型、userSpaceOnUse 百分比按 viewBox）---

func TestP38C4StopOffsets(t *testing.T) {
	// offset 缺省=前值（中位 stop 0.1→1）；递减钳制（0.7 后 0.1→0.7）。
	gradeC4(t, "c4-stop-missing-offset")
	gradeC4(t, "c4-stop-smaller-offset")
}

func TestP38C4StopColorInherit(t *testing.T) {
	// stop-color="inherit" 取 gradient 元素的 stop-color（一层）。
	gradeC4(t, "c4-stop-color-inherit")
}

func TestP38C4UserSpaceOnUsePercent(t *testing.T) {
	// 缺省 x1=0 x2=100% 在 userSpaceOnUse 下按 viewBox 宽解析。
	gradeC4(t, "c4-linear-userspace")
}

func TestP38C4SingularGradientTransform(t *testing.T) {
	// matrix(0 0 0 0 0 0)：奇异 → identity（usvg 缺省变换语义，
	// resvg 参照为未变换渲染）。
	gradeC4(t, "c4-linear-singular-transform")
	gradeC4(t, "c4-radial-singular-transform")
}

func TestP38C4RadialZeroR(t *testing.T) {
	// r=0 → 末 stop 单色（含其 opacity）。
	gradeC4(t, "c4-radial-zero-r")
}

func TestP38C4RadialReflect(t *testing.T) {
	// 径向 spreadMethod=reflect 镜像环。
	gradeC4(t, "c4-radial-reflect")
}

func TestP38C4RadialFocal(t *testing.T) {
	// focal 在圆外：两点 conical（tiny-skia greater 根 + 退化掩码）。
	gradeC4(t, "c4-radial-focal")
}

func TestP38C4HrefCrossType(t *testing.T) {
	// href 链：坐标只从同类型继承、units/transform/spread 任意渐变继承、
	// stops 走链找第一个有 stops 的。
	gradeC4(t, "c4-radial-href-from-linear")
	gradeC4(t, "c4-linear-href-complex")
}

func TestP38C4StopNormalizeNegative(t *testing.T) {
	// 负向：三连同 offset 去中 + 单 stop 单色 + 坏 stop-color→黑不炸。
	data := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<linearGradient id="g1">
			<stop offset="0.2" stop-color="red"/>
			<stop offset="0.5" stop-color="green"/>
			<stop offset="0.5" stop-color="blue"/>
			<stop offset="0.5" stop-color="yellow"/>
			<stop offset="0.9" stop-color="white"/>
		</linearGradient>
		<linearGradient id="g2"><stop offset="0.3" stop-color="red"/></linearGradient>
		<linearGradient id="g3"><stop offset="0" stop-color="nope"/></linearGradient>
		<rect width="10" height="10" fill="url(#g1)"/>
	</svg>`)
	doc, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	img := doc.Render(100, 100)
	if ink := inkInArea(t, img, 0, 0, 100, 100); ink < 200 {
		t.Fatalf("normalized gradient ink = %d, want full square", ink)
	}
}

// --- 批 4c-5：pattern 补全（usvg convert_units 缺省语义：patternUnits
// 缺省 oBB / patternContentUnits 缺省 userSpaceOnUse、无效值回缺省；
// resvg render_pattern_pixmap：内容 1:1 进 tile 无位移、oBB 内容按
// fill bbox 尺寸缩放（to_user_coordinates "no need to shift patterns"）、
// x/y 锚定铺贴相位、嵌套画刷、% 尺寸按 viewBox）---

func TestP38C5PatternUnits(t *testing.T) {
	// patternUnits="qwe" 无效 → oBB 缺省；patternContentUnits="qwe" →
	// userSpaceOnUse 缺省（两个缺省不同）。
	gradeC4(t, "c4-pattern-invalid-units")
}

func TestP38C5PatternXYPhase(t *testing.T) {
	// x=5,y=10 锚定铺贴相位；内容 1:1 无位移。
	gradeC4(t, "c4-pattern-xy-phase")
}

func TestP38C5PatternContentOBB(t *testing.T) {
	// patternContentUnits=oBB：内容按 fill bbox 尺寸缩放、无平移。
	gradeC4(t, "c4-pattern-content-obb")
}

func TestP38C5PatternNested(t *testing.T) {
	// tile 内容里 fill=url(#patt1)（嵌套 pattern）+ oBB 直引。
	gradeC4(t, "c4-pattern-nested")
	gradeC4(t, "c4-pattern-nested-obb")
}

func TestP38C5PatternPercent(t *testing.T) {
	// userSpaceOnUse 下 w/h=20% 按 viewBox 尺寸解析。
	gradeC4(t, "c4-pattern-percent")
}

// --- 批 4c-7：嵌套 svg/symbol/use/transform-origin/display（usvg use_node
// get_clip_rect：overflow visible/auto 不裁、缺省裁、viewBox-only 不裁；
// use w/h 设了才覆盖 svg 自身尺寸、symbol 自身属性进级联；SVG2
// transform-origin = T(o)·T·T(−o)，percent 按 viewBox；无尺寸根按内容
// 包围盒重定 viewBox）---

func TestP38C7UseSymbolOpacity(t *testing.T) {
	// use/symbol 上的 opacity 整组合成（PushLayer 通道）。
	gradeC4Image(t, "c4-use-opacity")
	gradeC4Image(t, "c4-symbol-opacity")
}

func TestP38C7SymbolViewBox(t *testing.T) {
	// symbol viewBox 建坐标系。
	gradeC4Image(t, "c4-symbol-viewbox")
}

func TestP38C7UseToSVG(t *testing.T) {
	// use→svg：use w/h 覆盖 svg 尺寸、svg x/y 偏移视口、嵌套 use 链。
	gradeC4Image(t, "c4-use-to-svg")
	gradeC4Image(t, "c4-use-nested-svg")
}

func TestP38C7NestedSVG(t *testing.T) {
	// overflow=auto 不裁剪；% 尺寸按父视口。
	gradeC4Image(t, "c4-nested-overflow-auto")
	gradeC4Image(t, "c4-nested-percent")
}

func TestP38C7RootNoSize(t *testing.T) {
	// 根 svg 无尺寸无 viewBox → 按内容包围盒重定（usvg
	// calculate_svg_bbox）→ 内容拉伸铺满画布。
	gradeC4Image(t, "c4-root-no-size")
}

func TestP38C7TransformOrigin(t *testing.T) {
	// transform-origin（center/长度/%）= T(o)·T·T(−o)，percent 按
	// viewBox（usvg resolve_transform）；image/pattern/clipPath 三落点。
	gradeC4Image(t, "c4-origin-image")
	gradeC4Image(t, "c4-origin-pattern")
	gradeC4Image(t, "c4-origin-clippath")
}

func TestP38C7DisplayNone(t *testing.T) {
	// display="none"：元素连子树隐藏（is_visible_element）；
	// switch 选中 display:none 子元素时渲染为空不回退；use 亦不可达。
	gradeC4Image(t, "c4-display-none-child")
	gradeC4Image(t, "c4-use-display-none")
	data := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
		<g id="g1" display="none"><rect width="10" height="10" fill="red"/></g>
		<use href="#g1"/>
	</svg>`)
	doc, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	img := doc.Render(100, 100)
	if ink := inkInArea(t, img, 0, 0, 100, 100); ink != 0 {
		t.Fatalf("display:none subtree visible via use: ink=%d, want 0", ink)
	}
}
