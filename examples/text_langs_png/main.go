// Command text_langs_png — M0–M7 文本渲染全语言可视化输出（GPU 优先）。
//
// 用 GPUI 文本引擎（HintingVertical=FT light + HbShaper/go-text shaping，
// GPU 走 Tier6 glyph-mask/MSDF 管线）把所有支持的语言/脚本样本渲染进
// 大图，按「字号 + 字体样式」命名输出 PNG。
//
// 渲染路径（render.Context.DrawString 自动调度）：
//   - GPU：空引入 render/gpu 注册 VelloAccelerator（headless Vulkan，
//     Intel/NVIDIA/lavapipe 均可），DrawString 走 GPU glyph-mask/MSDF
//     （GOGPU_DISABLE_GPU=1 可强制 CPU 降级对照）
//   - CPU 降级：无 GPU/设备失败时自动回落 freetype 像素路径
//
// 每个「样式」= 主字体 + 全套脚本 fallback 的 MultiFace 链（缺字自动
// 回退），一张图内所有语言真正显示。竖排样本用 TTB 方向 MultiFace。
//
// 命名: samples_<字号>px_<样式>.png（例 samples_16px_NotoSansCJK.png）
// 输出目录: ./out/（可用 $OUT_DIR 覆盖）。
//
// 覆盖：Latin（连字/kern）/Cyrillic/Greek/CJK（中/日/韩 + 竖排）/
// Thai/Devanagari/Bengali/Tamil/Arabic(RTL)/Hebrew(RTL)/Myanmar/Lao。
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
	text  string
	vert  string
}

type styleDef struct {
	name    string
	primary string
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
		{name: "NotoSansCJK", primary: "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc", chain: chainCjk()},
		{name: "FreeSans", primary: "/usr/share/fonts/truetype/freefont/FreeSans.ttf", chain: chainFree()},
		{name: "DejaVuSans", primary: "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", chain: chainFree()},
	}
	sizes := []float64{16, 24, 32}
	samples := langSamples()

	for _, st := range styles {
		primary, err := text.NewFontSourceFromFile(st.primary, text.WithParser("own"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "font %s: %v\n", st.primary, err)
			continue
		}
		var fallbacks []*text.FontSource
		for _, p := range st.chain {
			if p == st.primary {
				continue
			}
			s, err := text.NewFontSourceFromFile(p, text.WithParser("own"))
			if err != nil {
				continue
			}
			fallbacks = append(fallbacks, s)
			defer s.Close()
		}

		for _, size := range sizes {
			faces := []text.Face{primary.Face(size)}
			for _, s := range fallbacks {
				faces = append(faces, s.Face(size))
			}
			mf, err := text.NewMultiFace(faces...)
			if err != nil {
				fmt.Fprintf(os.Stderr, "style %s %.0fpx: %v\n", st.name, size, err)
				continue
			}
			all := append([]*text.FontSource{primary}, fallbacks...)
			var vfaces []text.Face
			for _, s := range all {
				vfaces = append(vfaces, s.Face(size, text.WithDirection(text.DirectionTTB)))
			}
			vmf, verr := text.NewMultiFace(vfaces...)
			if verr != nil {
				vmf = nil // 竖排不可用则跳过竖排区
			}

			img := renderGrid(mf, vmf, samples, size)
			fname := fmt.Sprintf("samples_%dpx_%s.png", int(size), st.name)
			if err := img.SavePNG(filepath.Join(outDir, fname)); err != nil {
				fmt.Fprintln(os.Stderr, "save "+fname+":", err)
				continue
			}
			img.Close()
			fmt.Fprintf(os.Stderr, "wrote %s/%s\n", outDir, fname)
		}
		primary.Close()
	}
	fmt.Fprintln(os.Stderr, "done. 输出目录:", outDir)
}

// renderGrid 用 GPU 优先的 render.Context 排版：左列标签 + 主文本 + 竖排。
func renderGrid(mf text.Face, vmf text.Face, samples []langSample, size float64) *render.Context {
	const (
		width  = 1800
		rowH   = 130
		labelW = 230
		margin = 20
		vertW  = 120
	)
	height := margin*2 + rowH*len(samples)

	ctx := render.NewContext(width, height)
	defer ctx.Close()
	ctx.SetFont(mf)

	ink := color.RGBA{20, 20, 20, 255}
	labelCol := color.RGBA{90, 90, 90, 255}
	missing := color.RGBA{200, 90, 90, 255}

	ctx.ClearWithColor(render.RGBA{R: 250, G: 250, B: 250, A: 255})

	for i, s := range samples {
		rowTop := margin + i*rowH
		baseline := float64(rowTop) + float64(rowH)*0.5 + size*0.35

		ctx.SetFont(mf)
		col := ink
		if !faceCovers(mf, s.text) {
			col = missing
		}
		ctx.SetColor(labelCol)
		ctx.DrawString(s.label, margin, baseline)
		ctx.SetColor(col)
		ctx.DrawString(s.text, margin+labelW, baseline)

		if s.vert != "" && vmf != nil {
			ctx.SetFont(vmf)
			vcol := ink
			if !faceCovers(vmf, s.vert) {
				vcol = missing
			}
			ctx.SetColor(vcol)
			vx := float64(width - margin - vertW)
			y := float64(rowTop) + float64(rowH)*0.5 - size*1.2
			ctx.DrawString(s.vert, vx, y)
		}
	}

	// SavePNG 内部 FlushGPU + GPU→CPU 回读。
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
		{"Latin", "Hello World — The quick brown fox jumps over 13 lazy dogs", ""},
		{"Latin ligatures", "Fidelity office — fi ffi fl fft — AVATAR To", ""},
		{"Cyrillic", "Привет мир — Съешь ещё этих мягких французских булок", ""},
		{"Greek", "Καλημέρα κόσμε — Ξεσκεπάζω την ψυχοφθόρα βδελυγμία", ""},
		{"Chinese", "你好，世界。文本渲染引擎支持汉字提示与竖排。", "你好世界，文本渲染！"},
		{"Japanese", "こんにちは世界 — 漢字かな交じり、ひらがなとカタカナ。", "こんにちは世界。"},
		{"Korean", "안녕하세요 세계 — 한글 음절 완성형 11172", ""},
		{"Thai", "สวัสดีชาวโลก — ภาษาไทย ระบบการเขียน", ""},
		{"Devanagari", "नमस्ते दुनिया — हिन्दी भाषा", ""},
		{"Bengali", "নমস্কার বিশ্ব — বাংলা ভাষা", ""},
		{"Tamil", "வணக்கம் உலகம் — தமிழ் மொழி", ""},
		{"Arabic RTL", "مرحبا بالعالم — اللغة العربية", ""},
		{"Hebrew RTL", "שלום עולם — עברית", ""},
		{"Myanmar", "မင်္ဂလာပါ ကမ္ဘာ — မြန်မာစာ", ""},
		{"Lao", "ສະບາຍດີ ໂລກ — ພາສາລາວ", ""},
	}
}

func faceCovers(f text.Face, s string) bool {
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' {
			continue
		}
		if !f.HasGlyph(r) {
			return false
		}
	}
	return true
}