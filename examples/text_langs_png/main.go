// Command text_langs_png — M0–M7 文本渲染全语言可视化输出（GPU 优先）。
//
// 用 GPUI 文本引擎（HintingVertical=FT light + HbShaper/go-text shaping，
// GPU 走 Tier6 glyph-mask/MSDF 管线）把所有支持的语言/脚本样本渲染进
// 大图，按「字号 + 字体样式」命名输出 PNG。
//
// 每个语言样本渲染五种样式变体（字体样式展示）：
//   1) Regular 常规
//   2) Bold 粗体（同族 Bold 变体，无则回退 Regular）
//   3) Oblique 斜体（同族 Oblique 变体，无则 Shear 变换合成）
//   4) Color 带颜色（绿色着色）
//   5) Vertical 竖排（TTB 方向，中/日/韩等按语言序号分列不重叠）
//
// 渲染路径（render.Context.DrawString 自动调度）：
//   - GPU：空引入 render/gpu 注册 VelloAccelerator（headless Vulkan）
//   - CPU 降级：无 GPU/设备失败自动回落
//
// 每个「样式」= 主字体（Regular）+/Bold/Oblique + 脚本 fallback 链。
// 命名: samples_<字号>px_<样式>.png；输出 ./out/（$OUT_DIR 覆盖）。
package main

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"

	_ "github.com/energye/gpui/render/gpu" // 注册 GPU 加速器（headless Vulkan）
)

type langSample struct {
	label string
	text  string // 主文本（Regular/Bold/Oblique/Color 共用）
	vert  string // 竖排样本（空 = 无竖排变体行）
}

// styleDef 一个样式 = 主字体（Regular）+ 可选 Bold/Oblique + fallback 链。
type styleDef struct {
	name    string
	regular string
	bold    string
	oblique string
	chain   []string
}

func main() {
	outDir := os.Getenv("OUT_DIR")
	if outDir == "" {
		outDir = "out"
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "mkdir:", err)
		os.Exit(1)
	}

	styles := []styleDef{
		{name: "NotoSansCJK",
			regular: "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
			bold:    "/usr/share/fonts/opentype/noto/NotoSansCJK-Bold.ttc",
			chain:   chainCjk()},
		{name: "FreeSans",
			regular: "/usr/share/fonts/truetype/freefont/FreeSans.ttf",
			bold:    "/usr/share/fonts/truetype/freefont/FreeSansBold.ttf",
			oblique: "/usr/share/fonts/truetype/freefont/FreeSansOblique.ttf",
			chain:   chainFree()},
		{name: "DejaVuSans",
			regular: "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
			bold:    "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
			chain:   chainFree()},
	}
	sizes := []float64{16, 24, 32}
	samples := langSamples()

	for _, st := range styles {
		srcs, ok := loadStyleFonts(st)
		if !ok {
			continue
		}
		for _, size := range sizes {
			img := renderStyleGrid(srcs, st, samples, size)
			fname := fmt.Sprintf("samples_%dpx_%s.png", int(size), st.name)
			if err := img.SavePNG(filepath.Join(outDir, fname)); err != nil {
				fmt.Fprintln(os.Stderr, "save "+fname+":", err)
				continue
			}
			img.Close()
			fmt.Fprintf(os.Stderr, "wrote %s/%s\n", outDir, fname)
		}
		srcs.regular.Close()
		if srcs.bold != nil {
			srcs.bold.Close()
		}
		if srcs.oblique != nil {
			srcs.oblique.Close()
		}
		for _, c := range srcs.chain {
			c.Close()
		}
	}
	fmt.Fprintln(os.Stderr, "done. 输出目录:", outDir)
}

// styleFonts 一个样式加载后的字体源集合。
type styleFonts struct {
	regular  *text.FontSource
	bold     *text.FontSource // 可能 nil
	oblique  *text.FontSource // 可能 nil
	chain    []*text.FontSource
}

func loadStyleFonts(st styleDef) (styleFonts, bool) {
	var sf styleFonts
	r, err := text.NewFontSourceFromFile(st.regular, text.WithParser("own"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "font %s: %v\n", st.regular, err)
		return sf, false
	}
	sf.regular = r
	if st.bold != "" {
		if b, err := text.NewFontSourceFromFile(st.bold, text.WithParser("own")); err == nil {
			sf.bold = b
		}
	}
	if st.oblique != "" {
		if o, err := text.NewFontSourceFromFile(st.oblique, text.WithParser("own")); err == nil {
			sf.oblique = o
		}
	}
	for _, p := range st.chain {
		if p == st.regular {
			continue
		}
		s, err := text.NewFontSourceFromFile(p, text.WithParser("own"))
		if err != nil {
			continue
		}
		sf.chain = append(sf.chain, s)
	}
	return sf, true
}

// variantFace 构建一个样式链 MultiFace：base 字体可能被 variant 替换（Bold/
// Oblique），其后跟 fallback chain。dir 指定方向（LTR 或 TTB）。
func (sf styleFonts) variantFace(variant string, size float64, dir text.Direction) (text.Face, bool) {
	base := sf.regular
	switch variant {
	case "bold":
		if sf.bold != nil {
			base = sf.bold
		}
	case "oblique":
		if sf.oblique != nil {
			base = sf.oblique
		}
	}
	opts := faceDirOpts(dir)
	faces := []text.Face{base.Face(size, opts...)}
	for _, c := range sf.chain {
		faces = append(faces, c.Face(size, opts...))
	}
	mf, err := text.NewMultiFace(faces...)
	if err != nil {
		return nil, false
	}
	return mf, true
}

func faceDirOpts(dir text.Direction) []text.FaceOption {
	if dir == 0 || dir == text.DirectionLTR {
		return nil
	}
	return []text.FaceOption{text.WithDirection(dir)}
}

// renderStyleGrid 渲染一个样式一整张大图。
// 布局：每语言一个模块块（模高 modH），内含 5 个变体行。
// 竖排变体按语言序号分列（X 递增，避免中/日重叠）。
func renderStyleGrid(sf styleFonts, st styleDef, samples []langSample, size float64) *render.Context {
	const (
		width  = 2200
		modH   = 400 // 每语言模块高（容纳 5 变体行）
		rowH   = 60  // 每变体行高
		labelW = 240
		margin = 20
	)
	height := margin*2 + modH*len(samples)
	ctx := render.NewContext(width, height)
	defer ctx.Close()
	ctx.ClearWithColor(render.RGBA{R: 250, G: 250, B: 250, A: 255})

	regCol := color.RGBA{20, 20, 20, 255}
	boldCol := color.RGBA{180, 30, 30, 255}
	oblCol := color.RGBA{30, 60, 180, 255}
	colorCol := color.RGBA{30, 140, 40, 255}
	vertCol := color.RGBA{120, 40, 140, 255}
	labelCol := color.RGBA{90, 90, 90, 255}

	for i, s := range samples {
		modTop := margin + i*modH

		// 1) Regular
		mf, _ := sf.variantFace("regular", size, 0)
		ctx.SetFont(mf)
		ctx.SetColor(labelCol)
		ctx.DrawString(s.label+" Regular", margin, float64(modTop+rowH*0+26))
		ctx.SetColor(regCol)
		ctx.DrawString(s.text, labelW, float64(modTop+rowH*0+26))

		// 2) Bold
		bmf, _ := sf.variantFace("bold", size, 0)
		ctx.SetFont(bmf)
		ctx.SetColor(labelCol)
		ctx.DrawString(s.label+" Bold", margin, float64(modTop+rowH*1+26))
		ctx.SetColor(boldCol)
		ctx.DrawString(s.text, labelW, float64(modTop+rowH*1+26))

		// 3) Oblique（有 Oblique 字体用之；无则 Shear 合成并恢复矩阵）
		omf, _ := sf.variantFace("oblique", size, 0)
		ctx.SetFont(omf)
		tag := "Oblique"
		if sf.oblique == nil {
			tag = "Oblique(sheared)"
			// 标签必须先画（Shear 只应作用于主文本，否则斜切的标签灰色
			// 会扩散覆盖文本区形成「灰块」）。
			ctx.SetColor(labelCol)
			ctx.DrawString(s.label+" "+tag, margin, float64(modTop+rowH*2+26))
			prev := ctx.GetTransform()
			ctx.Shear(0.3, 0)
			ctx.SetColor(oblCol)
			ctx.DrawString(s.text, labelW, float64(modTop+rowH*2+26))
			ctx.SetTransform(prev)
		} else {
			ctx.SetColor(labelCol)
			ctx.DrawString(s.label+" "+tag, margin, float64(modTop+rowH*2+26))
			ctx.SetColor(oblCol)
			ctx.DrawString(s.text, labelW, float64(modTop+rowH*2+26))
		}

		// 4) Color（绿色常规）
		cmf, _ := sf.variantFace("regular", size, 0)
		ctx.SetFont(cmf)
		ctx.SetColor(labelCol)
		ctx.DrawString(s.label+" Color", margin, float64(modTop+rowH*3+26))
		ctx.SetColor(colorCol)
		ctx.DrawString(s.text, labelW, float64(modTop+rowH*3+26))

		// 5) Vertical（TTB，分列 + 截长防溢出）
		if s.vert != "" {
			vmf, _ := sf.variantFace("regular", size, text.DirectionTTB)
			if vmf != nil {
				ctx.SetFont(vmf)
				ctx.SetColor(labelCol)
				ctx.DrawString(s.label+" Vertical", margin, float64(modTop+rowH*4+26))
				ctx.SetColor(vertCol)
				// 分列 X：每语言一列（语言序号递增），列间距 = size*2。
				colX := float64(1500 + i*int(size)*2)
				ctx.DrawString(vertClamp(s.vert, size), colX, float64(modTop+rowH*4+26))
			}
		}
	}
	return ctx
}

func chainCjk() []string {
	return []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/tlwg/Loma.ttf",
		"/usr/share/fonts/truetype/lohit-devanagari/Lohit-Devanagari.ttf",
		"/usr/share/fonts/truetype/fonts-beng-extra/Mukti.ttf",
		"/usr/share/fonts/truetype/lohit-tamil/Lohit-Tamil.ttf",
		"/usr/share/fonts/truetype/kacst-one/KacstOne.ttf",
		"/usr/share/fonts/truetype/freefont/FreeSans.ttf",
		"/usr/share/fonts/truetype/padauk/PadaukBook-Regular.ttf",
		"/usr/share/fonts/truetype/lao/Phetsarath_OT.ttf",
	}
}

func chainFree() []string {
	return []string{
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/tlwg/Loma.ttf",
		"/usr/share/fonts/truetype/lohit-devanagari/Lohit-Devanagari.ttf",
		"/usr/share/fonts/truetype/fonts-beng-extra/Mukti.ttf",
		"/usr/share/fonts/truetype/lohit-tamil/Lohit-Tamil.ttf",
		"/usr/share/fonts/truetype/kacst-one/KacstOne.ttf",
		"/usr/share/fonts/truetype/padauk/PadaukBook-Regular.ttf",
		"/usr/share/fonts/truetype/lao/Phetsarath_OT.ttf",
	}
}

func langSamples() []langSample {
	return []langSample{
		{"Latin", "Hello World — The quick brown fox", ""},
		{"Cyrillic", "Привет мир — Съешь ещё булок", ""},
		{"Greek", "Καλημέρα κόσμε — Ξεσκεπάζω", ""},
		{"Chinese", "你好，世界。文本渲染引擎。", "你好世界，文本渲染！"},
		{"Japanese", "こんにちは世界 — かな交じり文", "こんにちは世界。"},
		{"Korean", "안녕하세요 세계 — 완성형", "안녕하세요 세계"},
		{"Thai", "สวัสดีชาวโลก — ภาษาไทย", "สวัสดี"},
		{"Devanagari", "नमस्ते दुनिया — हिन्दी", "हिन्दी"},
		{"Bengali", "নমস্কার বিশ্ব — বাংলা", "বাংলা"},
		{"Tamil", "வணக்கம் உலகம் — தமிழ்", "தமிழ்"},
		{"Arabic", "مرحبا بالعالم — العربية", "مرحبا"},
		{"Hebrew", "שלום עולם — עברית", "שלום"},
		{"Myanmar", "မင်္ဂလာပါ — မြန်မာ", "မြန်မာ"},
		{"Lao", "ສະບາຍດີ — ພາສາລາວ", "ພາສາ"},
	}
}

// vertClamp 将竖排样本截到模块内能放下的字符数，防止竖排文本纵向溢出
// 侵入下一模块。可用高 = modH - 竖排行基线偏移；每字高 = size（T TB
// advance 为 vmtx 高度 ≈ em）。
func vertClamp(s string, size float64) string {
	const (
		modH     = 400
		rowBaselineOffset = 26
	)
	availH := modH - rowBaselineOffset - int(size*1.5)
	maxChars := availH / int(size)
	if maxChars < 1 {
		maxChars = 1
	}
	runes := []rune(s)
	if len(runes) <= maxChars {
		return s
	}
	return string(runes[:maxChars])
}