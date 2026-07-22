//go:build linux || darwin || windows

// ui_ant_compare — render every kit control state to PNG and build an HTML
// matrix against Ant Design 5 acceptance specs (metrics + token colors).
//
//	go run ./examples/ui_ant_compare
//	# open tmp/ant_compare/index.html
//
// Set GPUI_ANT_COMPARE_DIR to override the output directory.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
	"github.com/energye/gpui/ui/visualtest"
)

type cellData struct {
	id, title, antSpec string
	w, h               int
	img                image.Image
}

func main() {
	out := os.Getenv("GPUI_ANT_COMPARE_DIR")
	if out == "" {
		out = filepath.Join("tmp", "ant_compare")
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}
	th := kit.DefaultTheme()
	face := loadFace(14)
	var cells []cellData

	// Capture at DPR=2 so 1px borders / 16px circles match HiDPI AA (closer to Ant in browser).
	const dpr = 2.0

	add := func(id, title, antSpec string, w, h int, root core.Node) {
		img := visualtest.CaptureTreeEx(w, h, root, visualtest.CaptureTreeOptions{
			Theme: th,
			Scale: dpr,
		})
		if img == nil {
			fmt.Fprintf(os.Stderr, "capture failed: %s\n", id)
			return
		}
		path := filepath.Join(out, id+".png")
		if err := visualtest.SavePNG(path, img); err != nil {
			fmt.Fprintf(os.Stderr, "save %s: %v\n", path, err)
			return
		}
		cells = append(cells, cellData{id, title, antSpec, w, h, img})
		fmt.Printf("wrote %s (%dx%d)\n", path, w, h)
	}

	padFrame := func(child core.Node, cw, ch, pad float64) *primitive.Box {
		b := primitive.NewBox(child)
		b.Width, b.Height = cw, ch
		b.Padding = primitive.All(pad)
		b.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		return b
	}
	rowFrame := func(kids ...core.Node) *primitive.Box {
		r := primitive.Row(kids...)
		r.Gap = 12
		r.CrossAlign = core.CrossCenter
		r.Padding = primitive.All(16)
		b := primitive.NewBox(r)
		b.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		return b
	}

	mkBtn := func(label string, typ kit.ButtonType, danger, disabled, loading bool, size kit.ButtonSize) *kit.Button {
		b := kit.NewButton(label)
		b.SetType(typ)
		b.SetSize(size)
		b.SetDanger(danger)
		b.SetDisabled(disabled)
		b.SetLoading(loading)
		if face != nil {
			b.SetFace(face)
		}
		return b
	}
	forceHover := func(b *kit.Button) {
		_ = b.Node()
		b.Root.SetHovered(true)
		b.SyncState()
	}
	forcePress := func(b *kit.Button) {
		_ = b.Node()
		b.Root.SetHovered(true)
		b.Root.HandlePointer(&core.PointerEvent{Type: core.PointerDown, Button: core.ButtonLeft})
		b.SyncState()
	}

	// ── Button ───────────────────────────────────────────────────────────
	for _, typ := range []struct {
		t    kit.ButtonType
		name string
	}{
		{kit.ButtonPrimary, "primary"},
		{kit.ButtonDefault, "default"},
		{kit.ButtonDashed, "dashed"},
		{kit.ButtonText, "text"},
		{kit.ButtonLink, "link"},
	} {
		b := mkBtn("Button", typ.t, false, false, false, kit.ButtonMiddle)
		add(fmt.Sprintf("btn_%s_idle", typ.name),
			fmt.Sprintf("Button %s · idle", typ.name),
			"Ant: h=32, radius=6, font=14, paddingInline=15; primary fill #1677ff; default border #d9d9d9",
			160, 48, padFrame(b.Node(), 160, 48, 8))

		b = mkBtn("Button", typ.t, false, false, false, kit.ButtonMiddle)
		forceHover(b)
		add(fmt.Sprintf("btn_%s_hover", typ.name),
			fmt.Sprintf("Button %s · hover", typ.name),
			"Ant: primary→#4096ff; default border→#4096ff, bg→rgba(0,0,0,0.06) composite",
			160, 48, padFrame(b.Node(), 160, 48, 8))

		b = mkBtn("Button", typ.t, false, false, false, kit.ButtonMiddle)
		forcePress(b)
		add(fmt.Sprintf("btn_%s_pressed", typ.name),
			fmt.Sprintf("Button %s · pressed", typ.name),
			"Ant: primary→#0958d9; 圆角边框必须完整包住，禁止方块罩层",
			160, 48, padFrame(b.Node(), 160, 48, 8))

		b = mkBtn("Button", typ.t, false, true, false, kit.ButtonMiddle)
		add(fmt.Sprintf("btn_%s_disabled", typ.name),
			fmt.Sprintf("Button %s · disabled", typ.name),
			"Ant: text rgba(0,0,0,0.25); primary 半透明实心",
			160, 48, padFrame(b.Node(), 160, 48, 8))
	}
	b := mkBtn("Loading", kit.ButtonPrimary, false, false, true, kit.ButtonMiddle)
	add("btn_primary_loading", "Button primary · loading",
		"Ant: 左侧 spinner + 不可点；圆角保持",
		160, 48, padFrame(b.Node(), 160, 48, 8))

	for _, sz := range []struct {
		s    kit.ButtonSize
		name string
		h    float64
	}{
		{kit.ButtonSmall, "sm", 24},
		{kit.ButtonMiddle, "md", 32},
		{kit.ButtonLarge, "lg", 40},
	} {
		b := mkBtn("Size", kit.ButtonPrimary, false, false, false, sz.s)
		add(fmt.Sprintf("btn_size_%s", sz.name),
			fmt.Sprintf("Button primary · size=%s", sz.name),
			fmt.Sprintf("Ant controlHeight=%g", sz.h),
			160, int(sz.h)+16, padFrame(b.Node(), 160, sz.h+16, 8))
	}
	b = mkBtn("Danger", kit.ButtonPrimary, true, false, false, kit.ButtonMiddle)
	add("btn_danger_primary", "Button danger primary · idle",
		"Ant danger #ff4d4f", 160, 48, padFrame(b.Node(), 160, 48, 8))

	// ── Input ────────────────────────────────────────────────────────────
	in := kit.NewInput("placeholder")
	if face != nil {
		in.SetFace(face)
	}
	in.SetFixedSize(220, 32)
	add("input_idle", "Input · idle",
		"Ant: h=32, paddingInline=11, border #d9d9d9, radius=6",
		240, 48, padFrame(in.Node(), 240, 48, 8))

	in = kit.NewInput("placeholder")
	if face != nil {
		in.SetFace(face)
	}
	in.SetFixedSize(220, 32)
	in.Editor().SetFocused(true)
	add("input_focus", "Input · focus",
		"Ant: border #1677ff",
		240, 48, padFrame(in.Node(), 240, 48, 8))

	in = kit.NewInput("placeholder")
	if face != nil {
		in.SetFace(face)
	}
	in.SetFixedSize(220, 32)
	in.SetDisabled(true)
	add("input_disabled", "Input · disabled",
		"Ant: bg rgba(0,0,0,0.04)",
		240, 48, padFrame(in.Node(), 240, 48, 8))

	in = kit.NewInput("")
	if face != nil {
		in.SetFace(face)
	}
	in.SetFixedSize(220, 32)
	in.SetValue("Hello gpui")
	add("input_value", "Input · with value",
		"Ant: colorText rgba(0,0,0,0.88)",
		240, 48, padFrame(in.Node(), 240, 48, 8))

	// ── Checkbox ─────────────────────────────────────────────────────────
	cb := kit.NewCheckbox("Label")
	if face != nil {
		cb.SetFace(face)
	}
	add("checkbox_off", "Checkbox · off",
		"Ant: 16×16 radius4 border #d9d9d9 — 边角不得毛刺",
		160, 40, padFrame(cb.Node(), 160, 40, 8))

	cb = kit.NewCheckbox("Label")
	if face != nil {
		cb.SetFace(face)
	}
	cb.SetChecked(true)
	add("checkbox_on", "Checkbox · on",
		"Ant: primary 底 + 白色圆头对勾 — 对勾不得锯齿",
		160, 40, padFrame(cb.Node(), 160, 40, 8))

	cb = kit.NewCheckbox("Label")
	if face != nil {
		cb.SetFace(face)
	}
	cb.SetIndeterminate(true)
	add("checkbox_indeterminate", "Checkbox · indeterminate",
		"Ant: primary 底 + 白横条",
		160, 40, padFrame(cb.Node(), 160, 40, 8))

	cb = kit.NewCheckbox("Label")
	if face != nil {
		cb.SetFace(face)
	}
	cb.SetChecked(true)
	cb.SetDisabled(true)
	add("checkbox_on_disabled", "Checkbox · on disabled",
		"Ant: primary 半透明",
		160, 40, padFrame(cb.Node(), 160, 40, 8))

	cb = kit.NewCheckbox("Label")
	if face != nil {
		cb.SetFace(face)
	}
	_ = cb.Node()
	cb.Root.SetHovered(true)
	cb.SyncState()
	add("checkbox_off_hover", "Checkbox · off hover",
		"Ant: border primary",
		160, 40, padFrame(cb.Node(), 160, 40, 8))

	// ── Radio ────────────────────────────────────────────────────────────
	rd := kit.NewRadio("a", "Option")
	if face != nil {
		rd.SetFace(face)
	}
	add("radio_off", "Radio · off",
		"Ant: 16px 真圆 1px 环 — 不得方/毛边",
		160, 40, padFrame(rd.Node(), 160, 40, 8))

	rd = kit.NewRadio("a", "Option")
	if face != nil {
		rd.SetFace(face)
	}
	rd.SetSelected(true)
	add("radio_on", "Radio · on",
		"Ant: 主色环 + 内圆实心（约半外径）— 内外均 AA",
		160, 40, padFrame(rd.Node(), 160, 40, 8))

	rd = kit.NewRadio("a", "Option")
	if face != nil {
		rd.SetFace(face)
	}
	rd.SetSelected(true)
	rd.SetDisabled(true)
	add("radio_on_disabled", "Radio · on disabled",
		"Ant: 半透明主色内圆",
		160, 40, padFrame(rd.Node(), 160, 40, 8))

	rd = kit.NewRadio("a", "Option")
	if face != nil {
		rd.SetFace(face)
	}
	_ = rd.Node()
	rd.Root.SetHovered(true)
	rd.SyncState()
	add("radio_off_hover", "Radio · off hover",
		"Ant: border primary",
		160, 40, padFrame(rd.Node(), 160, 40, 8))

	// ── Switch ───────────────────────────────────────────────────────────
	sw := kit.NewSwitch()
	add("switch_off", "Switch · off",
		"Ant: 44×22 轨道 rgba(0,0,0,0.25)，白拇指 18px 贴左",
		80, 40, padFrame(sw.Node(), 80, 40, 8))

	sw = kit.NewSwitch()
	sw.SetChecked(true)
	add("switch_on", "Switch · on",
		"Ant: 轨道 #1677ff，拇指贴右；运行时点击应有 0.2s 滑动",
		80, 40, padFrame(sw.Node(), 80, 40, 8))

	sw = kit.NewSwitch()
	sw.SetDisabled(true)
	add("switch_disabled", "Switch · disabled",
		"Ant: disabled 底 + 细边",
		80, 40, padFrame(sw.Node(), 80, 40, 8))

	// ── Select ───────────────────────────────────────────────────────────
	sel := kit.NewSelect("Select…", kit.SelectOption{Value: "1", Label: "One"})
	if face != nil {
		sel.SetFace(face)
	}
	add("select_idle", "Select · idle",
		"Ant: 同 Input 高度 32 / padding 11",
		200, 48, padFrame(sel.Node(), 200, 48, 8))

	sel = kit.NewSelect("Select…", kit.SelectOption{Value: "1", Label: "One"})
	if face != nil {
		sel.SetFace(face)
	}
	sel.SetValue("1")
	add("select_valued", "Select · valued",
		"Ant: 正文色标签",
		200, 48, padFrame(sel.Node(), 200, 48, 8))

	// ── Feedback ─────────────────────────────────────────────────────────
	prog := kit.NewProgress(65)
	prog.Width = 200
	add("progress_65", "Progress · 65%",
		"Ant: height 8 胶囊圆角 primary",
		240, 32, padFrame(prog.Node(), 240, 32, 8))

	spin := kit.NewSpin(nil)
	add("spin", "Spin",
		"Ant: ~20px 圆头弧 primary",
		64, 64, padFrame(spin.Node(), 64, 64, 12))

	sk := kit.NewSkeleton(180, 16)
	sk.SetActive(false)
	add("skeleton", "Skeleton",
		"Ant: radius~4 fillSecondary",
		220, 40, padFrame(sk.Node(), 220, 40, 8))

	// ── Icons ────────────────────────────────────────────────────────────
	var iconProps []core.Node
	for _, name := range []string{"check", "close", "plus", "minus", "chevron-right", "chevron-down", "search", "info"} {
		ic := kit.NewIcon(name)
		ic.SetSize(20)
		// icon is geometric; face unused
		iconProps = append(iconProps, ic.Node())
	}
	add("icons_row", "Icons · built-in",
		"Ant 风格线标：圆头线帽，16–20px 无锯齿",
		280, 48, rowFrame(iconProps...))

	// ── Menu ─────────────────────────────────────────────────────────────
	menu := kit.NewMenu(
		kit.MenuItem{Key: "a", Label: "Item A"},
		kit.MenuItem{Key: "b", Label: "Item B"},
		kit.MenuItem{Key: "c", Label: "Item C"},
	)
	if face != nil {
		menu.Face = face
	}
	menu.SetSelected("b")
	add("menu_selected", "Menu · selected B",
		"Ant: selected #E6F4FF + 主色字；item padding 5×12",
		180, 120, padFrame(menu.Node(), 180, 120, 8))

	// ── Token strip ──────────────────────────────────────────────────────
	add("ant_token_strip", "Ant token strip (reference)",
		"Primary / Hover / Active / Error / Border / BgTextHover / PrimaryBg",
		400, 48, tokenStripNode(th, 400, 48))

	// Contact sheet
	sheet := buildContactSheet(cells, 4)
	sheetPath := filepath.Join(out, "CONTACT_SHEET.png")
	if err := visualtest.SavePNG(sheetPath, sheet); err != nil {
		fmt.Fprintf(os.Stderr, "sheet: %v\n", err)
	} else {
		fmt.Printf("wrote %s\n", sheetPath)
	}

	htmlPath := filepath.Join(out, "index.html")
	if err := writeHTML(htmlPath, cells); err != nil {
		fmt.Fprintf(os.Stderr, "html: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\nOpen: file://%s\n", absPath(htmlPath))
	fmt.Println("Ant: https://ant.design/components/overview")
}

func absPath(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return a
}

func tokenStripNode(th *core.Theme, w, h float64) core.Node {
	return primitive.NewCanvas(w, h, func(pc *core.PaintContext, sz core.Size) {
		if pc == nil {
			return
		}
		keys := []string{
			core.TokenColorPrimary,
			core.TokenColorPrimaryHover,
			core.TokenColorPrimaryActive,
			core.TokenColorError,
			core.TokenColorBorder,
			core.TokenColorBgTextHover,
			core.TokenColorPrimaryBg,
		}
		cw := sz.Width / float64(len(keys))
		for i, k := range keys {
			c := th.Color(k)
			if c.A < 0.05 {
				c = render.RGBA{R: 0.9, G: 0.9, B: 0.9, A: 1}
			}
			if c.A < 1 {
				c = render.RGBA{
					R: c.R*c.A + (1 - c.A),
					G: c.G*c.A + (1 - c.A),
					B: c.B*c.A + (1 - c.A),
					A: 1,
				}
			}
			pc.FillLocalRect(float64(i)*cw, 0, cw-2, sz.Height, c)
		}
	})
}

func buildContactSheet(cells []cellData, cols int) image.Image {
	if cols < 1 {
		cols = 4
	}
	const cellW, cellH, gap = 200, 100, 8
	n := len(cells)
	if n == 0 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}
	rows := (n + cols - 1) / cols
	w := cols*cellW + (cols+1)*gap
	h := rows*cellH + (rows+1)*gap
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.RGBA{240, 240, 240, 255}}, image.Point{}, draw.Src)
	for i, c := range cells {
		if c.img == nil {
			continue
		}
		col := i % cols
		row := i / cols
		x := gap + col*(cellW+gap)
		y := gap + row*(cellH+gap)
		draw.Draw(dst, image.Rect(x, y, x+cellW, y+cellH), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
		sb := c.img.Bounds()
		sw, sh := sb.Dx(), sb.Dy()
		// fit into cell if needed
		dx, dy := x+(cellW-sw)/2, y+(cellH-sh)/2
		if sw > cellW || sh > cellH {
			dx, dy = x+4, y+4
		}
		r := image.Rect(dx, dy, dx+sw, dy+sh).Intersect(image.Rect(x, y, x+cellW, y+cellH))
		draw.Draw(dst, r, c.img, sb.Min.Add(image.Pt(r.Min.X-dx, r.Min.Y-dy)), draw.Over)
	}
	return dst
}

func writeHTML(path string, cells []cellData) error {
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html>
<html lang="zh-CN"><head><meta charset="utf-8"/>
<title>gpui kit vs Ant Design 5 — state matrix</title>
<style>
  body { font-family: system-ui, sans-serif; margin: 24px; background: #f5f5f5; color: #1f1f1f; }
  h1 { font-size: 20px; }
  .meta { color: #666; margin-bottom: 24px; max-width: 960px; line-height: 1.55; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: 16px; }
  .card { background: #fff; border: 1px solid #e8e8e8; border-radius: 8px; padding: 12px; }
  .card h3 { margin: 0 0 8px; font-size: 14px; }
  .card img { display: block; max-width: 100%; height: auto; border: 1px dashed #d9d9d9;
              background: #fafafa; image-rendering: pixelated; image-rendering: crisp-edges; }
  /* 放大看锯齿：hover 2x */
  .card img:hover { transform: scale(2); transform-origin: top left; z-index: 2; position: relative;
                    box-shadow: 0 4px 16px rgba(0,0,0,.15); image-rendering: pixelated; }
  .ant { margin-top: 8px; font-size: 12px; color: #595959; line-height: 1.45;
         border-left: 3px solid #1677ff; padding-left: 8px; }
  .id { font-family: ui-monospace, monospace; font-size: 11px; color: #8c8c8c; }
  a { color: #1677ff; }
  .warn { background: #fff7e6; border: 1px solid #ffd591; padding: 12px; border-radius: 6px; margin-bottom: 16px; }
  .sheet { margin: 16px 0; }
  .sheet img { max-width: 100%; border: 1px solid #d9d9d9; }
</style></head><body>
<h1>gpui kit 控件状态矩阵 · 对照 Ant Design 5</h1>
<div class="meta">
  由 <code>go run ./examples/ui_ant_compare</code> 生成（CPU 软件光栅 · deviceScale=2（HiDPI 超采样，更接近浏览器））。
  图为 <b>kit 实际绘制</b>；蓝色条文案为 <b>Ant Design 5 默认规格</b>。
  鼠标悬停图片可 2× 放大看锯齿。请与
  <a href="https://ant.design/components/button" target="_blank">Button</a> ·
  <a href="https://ant.design/components/checkbox" target="_blank">Checkbox</a> ·
  <a href="https://ant.design/components/radio" target="_blank">Radio</a> ·
  <a href="https://ant.design/components/switch" target="_blank">Switch</a> ·
  <a href="https://ant.design/components/input" target="_blank">Input</a>
  并排对照。
</div>
<div class="warn">
  <b>检查清单：</b>① 圆角边框被方块盖住 ② 16px 圆/对勾锯齿 ③ 按下态圆角消失
  ④ Switch 拇指是否贴轨 ⑤ hover 边框是否变主色。
</div>
<div class="sheet"><h2>Contact sheet</h2>
<img src="CONTACT_SHEET.png" alt="contact sheet"/></div>
<div class="grid">
`)
	for _, c := range cells {
		fmt.Fprintf(&b, `<div class="card">
  <div class="id">%s · %dx%d</div>
  <h3>%s</h3>
  <img src="%s.png" width="%d" height="%d" alt="%s"/>
  <div class="ant"><b>Ant 期望：</b>%s</div>
</div>
`, c.id, c.w, c.h, esc(c.title), c.id, c.w, c.h, esc(c.title), esc(c.antSpec))
	}
	b.WriteString(`</div>
<p class="meta">重新运行命令会覆盖本目录全部 PNG 与本 HTML。</p>
</body></html>`)
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func esc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func loadFace(px float64) text.Face {
	for _, path := range []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
		"render/text/testdata/goregular.ttf",
	} {
		if src, err := text.NewFontSourceFromFile(path); err == nil {
			return src.Face(px)
		}
	}
	return nil
}
