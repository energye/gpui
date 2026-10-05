// Command icon is the F1 Icon real window: full-window kit icons only,
// no sidebar, no graphic chrome, metrics JSON only (no on-screen HUD).
//
// RUN_SECONDS=5 go run ./examples/kit/icon
// go run ./examples/kit/icon -auto-only
//
// Window: 1200x800 baseline, user-resizable. One scripted resize
// excursion restores the baseline before gates are read.
// Correctness window (RUN_SECONDS>=5): slope_gate=off, fps gate on.
//
// Layout references ant-design/components/icon for appearance only
// (order, sizes, colors, categories, card shape).
// Behavior and copy follow the Go implementation (ui/kit + docs/antd):
// hover highlights the hit instance, click selects it and logs the Go
// constructor — no web clipboard, no Copied overlay.
// demos (basic/two-tone/custom/iconfont/multi) with official order,
// names, themes, colors, sizes (default 16, panda 32, gap 8);
// catalog follows IconSearch default view:
// 6 categories with Chinese titles, 6-column cards (36px icon + name),
// All themes (Outlined/Filled/TwoTone as available).
// Custom heart/panda and 7 iconfont symbols are true vendored shapes
// (prim custom SVG registry), never placeholder glyphs.
// All icons built via kit public API only.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
	"github.com/energye/gpui/ui/theme"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

type probePoint struct {
	x, y float64
	want [3]float64
	tol  float64
}

type rect struct{ x, y, w, h float64 }

type pixelCheck struct {
	pt   *probePoint
	desc string
}

var pixelResult = map[string]bool{}

// iconBox builds one icon render box via kit public API only.
// size<=0 means the kit default edge; otherwise the size is passed into
// props so the painter draws exactly edge-sized (box == glyph, centered
// by placement, never top-left-anchored small paint).
func iconBox(ctx kit.ScopeCtx, props kit.IconProps, size float64, debug string) (*rendering.RenderBox, *kit.IconInstance) {
	if size > 0 {
		props.Size, props.SizeSet = size, true
	}
	in := kit.BuildIcon(ctx, props)
	spec := in.ResolveIconPaintSpec()
	painter := kit.IconPainterForSpec(spec)
	edge := size
	if edge <= 0 {
		edge = kit.ResolveIconSize(props)
	}
	box := kit.PrimNewCustomPaint(edge, edge, painter, true, nil)
	box.SetDebugName(debug)
	return box, in
}

// textLabel builds draw-ready text via kit prim public API (face attached).
func textLabel(s string, size float64, r, g, b float64, maxW float64) *rendering.RenderText {
	face := wrkit.FaceAt(size)
	t := kit.PrimNewLabel(kit.PrimLabelProps{
		Text: s,
		Style: kit.PrimTextStyle{
			FontSize: size,
			Color:    theme.Color{R: r, G: g, B: b, A: 1},
			HasColor: true,
		},
		MaxWidth: maxW,
		Face:     face,
	})
	return t
}

type card struct {
	key   string
	name  string
	theme string
}

type flatRow struct {
	title string
	cards []card
}

// cardUI tracks one built catalog card cell for hover/select refresh.
type cardUI struct {
	cell         *rendering.AbsoluteBox
	key          string
	name         string
	theme        string
	kebab        string
	rowIdx, col int
	cellX        float64
	blend        float64 // 0 idle .. 2 selected (1 == hover)
	nameX        float64
	nameXSet     bool
}

// Card feedback palette (official greens; hex parsed once, no magic floats).
var (
	greenHover    = theme.Hex("#95de64") // green-4: hover glyph + border
	greenDeep     = theme.Hex("#237804") // green-8: selected glyph + border
	tintHover     = theme.Hex("#f6ffed") // green-1: hover fill
	tintSelected  = theme.Hex("#d9f7be") // green-2: selected fill
	twotoneIdle   = theme.Hex("#333333")  // official IconBase primary
	borderIdleV   = 0.2                   // border fades in from neutral gray
)

func lerp(a, b, f float64) float64 { return a + (b-a)*f }

// blendGreen maps level 0..2 through idle -> hover -> selected.
func blendGreen(level, idleR, idleG, idleB float64) (float64, float64, float64) {
	if level <= 1 {
		return lerp(idleR, greenHover.R, level), lerp(idleG, greenHover.G, level), lerp(idleB, greenHover.B, level)
	}
	f := level - 1
	return lerp(greenHover.R, greenDeep.R, f), lerp(greenHover.G, greenDeep.G, f), lerp(greenHover.B, greenDeep.B, f)
}

// tintAt maps level 0..2 through white -> green-1 -> green-2.
func tintAt(level float64) (float64, float64, float64) {
	if level <= 1 {
		return lerp(1, tintHover.R, level), lerp(1, tintHover.G, level), lerp(1, tintHover.B, level)
	}
	f := level - 1
	return lerp(tintHover.R, tintSelected.R, f), lerp(tintHover.G, tintSelected.G, f), lerp(tintHover.B, tintSelected.B, f)
}

// borderAt maps level 0..2 through neutral gray -> hover green -> deep green.
func borderAt(level float64) (float64, float64, float64) {
	if level <= 1 {
		return lerp(borderIdleV, greenHover.R, level), lerp(borderIdleV, greenHover.G, level), lerp(borderIdleV, greenHover.B, level)
	}
	f := level - 1
	return lerp(greenHover.R, greenDeep.R, f), lerp(greenHover.G, greenDeep.G, f), lerp(greenHover.B, greenDeep.B, f)
}

func hexOf(r, g, b float64) string {
	return fmt.Sprintf("#%02x%02x%02x",
		uint8(math.Round(r*255)),
		uint8(math.Round(g*255)),
		uint8(math.Round(b*255)))
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "F1-icon")
	} else if *autoOnly {
		// Self-check needs the resize excursion to settle + 2s steady
		// state afterwards (damage/fps gates read the tail, not the
		// resize transient). RUN_SECONDS can still override upward.
		secs, secsSet = 8, true
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui kit icon — F1 official 1:1", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	ctx := kit.DefaultScopeCtx()
	// Official default: no global override (IconBase #333/#E6E6E6).

	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: 1, G: 1, B: 1, A: 1}
	root.SetDebugName("f1-icon-root")

	const gap = 8.0
	const defSize = 0.0 // 0 -> DefaultIconSize 16, matches official 1em default

	// ---- basic.tsx: HomeOutlined / SettingFilled / SmileOutlined /
	// SyncOutlined spin / SmileOutlined rotate180 / LoadingOutlined ----
	byTitle := 8.0
	root.Place(textLabel("基本用法", 13, 0.1, 0.1, 0.12, 1184), 8, byTitle)
	root.Place(textLabel("kit.BuildIcon(ctx, kit.DefaultIconProps(name))，Variant 选主题后缀，Spin 走 Tick 旋转，Rotate 静态角度，默认 16。", 11, 0.35, 0.35, 0.38, 1184), 8, byTitle+20)
	by := byTitle + 44
	basicX := 8.0
	mkDef := func(name, variant, debug string) *rendering.RenderBox {
		p := kit.DefaultIconProps(name)
		if variant != "" {
			p.Variant = variant
		}
		b, _ := iconBox(ctx, p, defSize, debug)
		root.Place(b, basicX, by)
		basicX += 16 + gap
		return b
	}
	mkDef("home", "outlined", "f1-icon-basic-home")
	mkDef("setting", "filled", "f1-icon-basic-setting")
	mkDef("smile", "outlined", "f1-icon-basic-smile")
	spinProps := kit.DefaultIconProps("sync")
	spinProps.Variant = "outlined"
	spinProps.Spin = true
	spinBox, spinInst := iconBox(ctx, spinProps, defSize, "f1-icon-basic-sync-spin")
	root.Place(spinBox, basicX, by)
	basicX += 16 + gap
	rotProps := kit.DefaultIconProps("smile")
	rotProps.Variant = "outlined"
	rotProps.Rotate = 180
	rotBox, rotInst := iconBox(ctx, rotProps, defSize, "f1-icon-basic-smile-180")
	root.Place(rotBox, basicX, by)
	basicX += 16 + gap
	mkDef("loading", "outlined", "f1-icon-basic-loading")

	// ---- two-tone.tsx: Smile / Heart #eb2f96 / CheckCircle #52c41a ----
	tyTitle := 108.0
	root.Place(textLabel("多色图标", 13, 0.1, 0.1, 0.12, 1184), 8, tyTitle)
	root.Place(textLabel("Variant=twotone + TwoToneSet 主色；次色走官方色板 generate(primary)[0]，HasSecondary 可显式指定。", 11, 0.35, 0.35, 0.38, 1184), 8, tyTitle+20)
	ty := tyTitle + 44
	twoX := 8.0
	smileTT := kit.DefaultIconProps("smile")
	smileTT.Variant = "twotone"
	bSmileTT, _ := iconBox(ctx, smileTT, defSize, "f1-icon-twotone-smile")
	root.Place(bSmileTT, twoX, ty)
	twoX += 16 + gap
	heartTT := kit.DefaultIconProps("heart")
	heartTT.Variant = "twotone"
	heartTT.TwoToneSet, heartTT.TwoTonePrimary = true, "#eb2f96"
	bHeartTT, heartInst := iconBox(ctx, heartTT, defSize, "f1-icon-twotone-heart")
	root.Place(bHeartTT, twoX, ty)
	twoX += 16 + gap
	checkTT := kit.DefaultIconProps("check-circle")
	checkTT.Variant = "twotone"
	checkTT.TwoToneSet, checkTT.TwoTonePrimary = true, "#52c41a"
	bCheckTT, checkInst := iconBox(ctx, checkTT, defSize, "f1-icon-twotone-check")
	root.Place(bCheckTT, twoX, ty)

	// ---- custom.tsx: HeartIcon hotpink / PandaIcon 32px /
	// Icon component={HomeOutlined} / HomeOutlined ----
	cyTitle := 188.0
	root.Place(textLabel("自定义图标", 13, 0.1, 0.1, 0.12, 1184), 8, cyTitle)
	root.Place(textLabel("CustomPainter 优先于 Name；心形熊猫走 prim 自定义 SVG 真形注册，多路径按 Fill 逐段填。", 11, 0.35, 0.35, 0.38, 1184), 8, cyTitle+20)
	cy := cyTitle + 44
	customX := 8.0
	heartCustom := kit.DefaultIconProps("heart")
	heartCustom.Color, heartCustom.ColorSet = "#ff69b4", true
	heartCustom.CustomPainter, heartCustom.CustomPainterID = "custom-heart-svg", "custom-heart-svg"
	cb1, customInst := iconBox(ctx, heartCustom, 50, "f1-icon-custom-heart")
	// custom.tsx wraps HeartIcon in fontSize 50px; panda is 32px.
	root.Place(cb1, customX, cy-17)
	customX += 50 + gap
	pandaProps := kit.DefaultIconProps("panda")
	pandaProps.CustomPainter, pandaProps.CustomPainterID = "custom-panda-svg", "custom-panda-svg"
	cb2, pandaInst := iconBox(ctx, pandaProps, 32, "f1-icon-custom-panda")
	// 32px box centered against the 50px HeartIcon row (row center cy+8).
	root.Place(cb2, customX, cy-8)
	customX += 32 + gap
	compProps := kit.DefaultIconProps("home")
	compProps.Variant = "outlined"
	compProps.CustomPainter, compProps.CustomPainterID = "home", "home"
	cb3, compInst := iconBox(ctx, compProps, defSize, "f1-icon-custom-component")
	root.Place(cb3, customX, cy)
	customX += 16 + gap
	plainProps := kit.DefaultIconProps("home")
	plainProps.Variant = "outlined"
	cb4, plainInst := iconBox(ctx, plainProps, defSize, "f1-icon-custom-plain")
	root.Place(cb4, customX, cy)
	_ = pandaInst
	_ = compInst
	_ = plainInst

	// ---- iconfont.tsx: tuichu / facebook #1877F2 / twitter (true shapes) ----
	fyTitle := 268.0
	root.Place(textLabel("使用 iconfont.cn", 13, 0.1, 0.1, 0.12, 1184), 8, fyTitle)
	root.Place(textLabel("CreateFromIconfont 建离线族 + Register 绑类型；真形 vendored 入库，永不联网。", 11, 0.35, 0.35, 0.38, 1184), 8, fyTitle+20)
	fy := fyTitle + 44
	fam := kit.CreateFromIconfont(kit.IconfontOptions{Sources: []string{"f1-s1"}})
	fam.Register("f1-s1", "icon-tuichu", "iconfont-tuichu")
	fam.Register("f1-s1", "icon-facebook", "iconfont-facebook")
	fam.Register("f1-s1", "icon-twitter", "iconfont-twitter")
	fontTypes := []struct {
		typ   string
		color string
	}{
		{"icon-tuichu", ""},
		{"icon-facebook", "#1877F2"},
		{"icon-twitter", ""},
	}
	fontX := 8.0
	var fontInsts []*kit.IconInstance
	for i, ft := range fontTypes {
		p := fam.NewIcon(ft.typ)
		if ft.color != "" {
			p.Color, p.ColorSet = ft.color, true
		}
		b, in := iconBox(ctx, p, defSize, fmt.Sprintf("f1-icon-font-%d", i))
		root.Place(b, fontX, fy)
		fontX += 16 + gap
		fontInsts = append(fontInsts, in)
	}

	// ---- scriptUrl.tsx: javascript / java / shoppingcart(f2 wins) / python(f2) ----
	syTitle := 348.0
	root.Place(textLabel("使用 iconfont.cn 的多个资源", 13, 0.1, 0.1, 0.12, 1184), 8, syTitle)
	root.Place(textLabel("后注册源覆盖同名类型（last wins）；同源内合并，旧键保留。", 11, 0.35, 0.35, 0.38, 1184), 8, syTitle+20)
	sy := syTitle + 44
	fam.Register("f1-s1", "icon-javascript", "iconfont-javascript")
	fam.Register("f1-s1", "icon-java", "iconfont-java")
	fam.Register("f1-s1", "icon-shoppingcart", "iconfont-shoppingcart-f1")
	fam.Register("f1-s2", "icon-shoppingcart", "iconfont-shoppingcart-f2")
	fam.Register("f1-s2", "icon-python", "iconfont-python-f2")
	multiTypes := []string{"icon-javascript", "icon-java", "icon-shoppingcart", "icon-python"}
	multiX := 8.0
	var multiInsts []*kit.IconInstance
	var dupInst *kit.IconInstance
	for i, tp := range multiTypes {
		p := fam.NewIcon(tp)
		b, in := iconBox(ctx, p, defSize, fmt.Sprintf("f1-icon-multi-%d", i))
		root.Place(b, multiX, sy)
		multiX += 16 + gap
		multiInsts = append(multiInsts, in)
		if tp == "icon-shoppingcart" {
			dupInst = in
		}
	}

	// ---- Catalog: IconSearch default All view, 6 categories, 6-col cards ----
	// Official order on https://ant.design/components/icon: search header on
	// top, then 5 demos, then the icon list. The window follows the same
	// order: demos at y8..~410, list header at 428, scroll viewport at 492.
	catTitleY := 428.0
	root.Place(textLabel("图标列表", 13, 0.1, 0.1, 0.12, 1184), 8, catTitleY)
	totalIcons := kit.PrimAntdIconCount()
	root.Place(textLabel(fmt.Sprintf("注册表 %d 全量，6 分类虚拟列表只建可见行；滚轮/拖拽连续滚动，点击选中输出 Go 构造代码到日志。", totalIcons), 11, 0.35, 0.35, 0.38, 1184), 8, catTitleY+20)
	root.Place(textLabel("全部 | 线框风格 | 实底风格 | 双色风格", 11, 0.35, 0.35, 0.38, 1184), 8, catTitleY+40)

	// Build base->themes from registry (kit public API only).
	type themeSet map[string]string // theme(lower) -> key
	baseThemes := map[string]themeSet{}
	var allBases []string
	for i := 0; i < totalIcons; i++ {
		key, _, th, ok := kit.PrimAntdIconEntry(i)
		if !ok {
			continue
		}
		var suffix string
		switch th {
		case "outlined":
			suffix = "Outlined"
		case "filled":
			suffix = "Filled"
		case "twotone":
			suffix = "TwoTone"
		default:
			continue
		}
		if !strings.HasSuffix(key, suffix) {
			continue
		}
		base := key[:len(key)-len(suffix)]
		ts, ok := baseThemes[base]
		if !ok {
			ts = themeSet{}
			baseThemes[base] = ts
			allBases = append(allBases, base)
		}
		ts[th] = key
	}
	datumSet := map[string]bool{}
	for _, b := range catDirection {
		datumSet[b] = true
	}
	for _, b := range catSuggestion {
		datumSet[b] = true
	}
	for _, b := range catEditor {
		datumSet[b] = true
	}
	for _, b := range catData {
		datumSet[b] = true
	}
	for _, b := range catLogo {
		datumSet[b] = true
	}
	var otherBases []string
	for _, b := range allBases {
		if !datumSet[b] {
			otherBases = append(otherBases, b)
		}
	}
	sort.Strings(otherBases)
	catNames := []string{"方向性图标", "提示建议性图标", "编辑类图标", "数据类图标", "品牌和标识", "网站通用图标"}
	catBases := [][]string{catDirection, catSuggestion, catEditor, catData, catLogo, otherBases}
	themeOrder := []string{"outlined", "filled", "twotone"}
	keySuffix := map[string]string{"outlined": "Outlined", "filled": "Filled", "twotone": "TwoTone"}

	var rows []flatRow
	// Official ignore list (IconSearch.tsx): same-shape duplicates.
	ignoreBase := map[string]bool{"CopyrightCircle": true, "DollarCircle": true}
	for ci, bases := range catBases {
		rows = append(rows, flatRow{title: catNames[ci]})
		var cards []card
		for _, base := range bases {
			if ignoreBase[base] {
				continue
			}
			ts, ok := baseThemes[base]
			if !ok {
				continue
			}
			for _, th := range themeOrder {
				key, ok := ts[th]
				if !ok {
					continue
				}
				cards = append(cards, card{key: key, name: key, theme: th})
				_ = keySuffix
			}
		}
		for i := 0; i < len(cards); i += 6 {
			end := i + 6
			if end > len(cards) {
				end = len(cards)
			}
			rows = append(rows, flatRow{cards: cards[i:end]})
		}
	}
	rowCount := len(rows)
	const cardW = 184.0
	const cardH = 100.0
	const rowGap = 16.0
	const cardRowH = cardH + rowGap
	const titleRowH = 32.0
	extentAt := func(i int) float64 {
		if i < 0 || i >= len(rows) {
			return cardRowH
		}
		if rows[i].title != "" {
			return titleRowH
		}
		return cardRowH
	}
	// Card interaction state (Go-native demo behavior: hover highlights the
	// hit instance, click selects it and logs the Go constructor).
	// Active visuals cross-fade over cardFadeSec (paint-only, no layout).
	hoverKey, selectKey := "", ""
	cardUIs := map[string]*cardUI{}
	const cardFadeSec = 0.18
	// Row content offsets for hit testing (variable extents).
	rowOffsets := make([]float64, len(rows)+1)
	for i := range rows {
		rowOffsets[i+1] = rowOffsets[i] + extentAt(i)
	}
	// Scripted proof row: first card row that holds both a plain theme
	// card (for select) and a twotone card (for hover), so one viewport
	// position proves both feedback paths in the snapshot.
	demoRowIdx := -1
	demoSelectKey, demoHoverKey := "", ""
	for i, r := range rows {
		if len(r.cards) < 2 {
			continue
		}
		plain, two := "", ""
		for _, cd := range r.cards {
			if cd.theme == "twotone" && two == "" {
				two = cd.key
			}
			if cd.theme != "twotone" && plain == "" {
				plain = cd.key
			}
		}
		if plain != "" && two != "" {
			demoRowIdx, demoSelectKey, demoHoverKey = i, plain, two
			break
		}
	}
	if demoRowIdx < 0 {
		for _, r := range rows {
			if len(r.cards) > 0 {
				demoSelectKey, demoHoverKey = r.cards[0].key, r.cards[0].key
				break
			}
		}
	}
	// fillCard (re)builds one card cell for a level in [0,2]:
	// 0 idle (theme icon, gray name), 1 hover (light-green icon),
	// 2 selected (deep-green icon). Every theme (outlined/filled/twotone)
	// drives the glyph through the same greens; twotone drives its primary
	// (secondary derives via generate(primary)[0]).
	// The highlight background is a sibling rounded fill + green border
	// (NOT a DecoratedBox clip parent: clip nodes hid icon paint on GLES
	// after click): hover green-1 #f6ffed + green-4 border, selected
	// green-2 #d9f7be + green-8 border, opaque.
	// Icon and name stay centered in the fixed 184x100 slot at every
	// level; the icon grows upward only (bottom pinned at 56) so the
	// 14px gap to the name at y=70 never collapses on hover/select.
	// Green tints come from the official green palette.
	fillCard := func(cui *cardUI, level float64) {
		if level < 0 {
			level = 0
		}
		if level > 2 {
			level = 2
		}
		// Drain first: ranging over Children() while removing aliases the
		// same backing array and skips every other child (stale paint).
		for len(cui.cell.Children()) > 0 {
			cui.cell.RemoveChild(cui.cell.Children()[0])
		}
		p := kit.DefaultIconProps(cui.kebab)
		p.Variant = cui.theme
		lift := level
		if lift > 1 {
			lift = 1
		}
		edge := 36.0 + 11.0*lift // 36->47 grows upward, bottom pinned
		ix := (cardW - edge) / 2
		const iconBottom = 56.0
		const nameY = 70.0
		iy := iconBottom - edge
		// Highlight tint behind the glyph: hover green-1 #f6ffed, selected
		// green-2 #d9f7be, plain sibling custom-paint (no clip node, so the
		// GLES icon paint survives click). t=0 idle has no tint box at all.
		// Every theme (including twotone) gets both glyph + tint feedback.
		if t := level; t > 0 {
			tr, tg, tb := tintAt(level)
			// Border follows the glyph greens so the feedback survives
			// even where the pale fill is hard to tell from white.
			or, og, ob := borderAt(level)
			tint := kit.PrimNewCustomPaint(cardW, cardH,
				func(pc *rendering.PaintContext, size rendering.Size) {
					rendering.FillRoundRect(pc, 0, 0, size.Width, size.Height, 4, tr, tg, tb, 1)
					rendering.StrokeRoundRect(pc, 1, 1, size.Width-2, size.Height-2, 3, 1.5, or, og, ob, 1)
				},
				true, nil)
			tint.SetDebugName("f1-icon-cardbg")
			cui.cell.Place(tint, 0, 0)
		}
		// Glyph feedback converges here: one blend for every theme.
		// Twotone drives its primary (secondary derives via
		// generate(primary)[0]); outlined/filled drive the single color.
		// Idle keeps the official look (#333/#E6E6E6 twotone, theme text
		// otherwise); hover/select walk idle -> green-4 -> green-8.
		if level > 0 {
			idleR, idleG, idleB := theme.DefaultTokens().ColorText.R, theme.DefaultTokens().ColorText.G, theme.DefaultTokens().ColorText.B
			if cui.theme == "twotone" {
				idleR, idleG, idleB = twotoneIdle.R, twotoneIdle.G, twotoneIdle.B
			}
			rr, gg, bb := blendGreen(level, idleR, idleG, idleB)
			if cui.theme == "twotone" {
				p.TwoToneSet, p.TwoTonePrimary = true, hexOf(rr, gg, bb)
				p.HasSecondary = false
			} else {
				p.Color, p.ColorSet = hexOf(rr, gg, bb), true
			}
		}
		b, _ := iconBox(ctx, p, edge, "f1-icon-glyph")
		cui.cell.Place(b, ix, iy)
		nt := textLabel(cui.name, 10, 0.33, 0.33, 0.35, cardW-16)
		if !cui.nameXSet {
			nw := nt.MeasureWidth(cui.name)
			nx := (cardW - nw) / 2
			if nx < 0 {
				nx = 0
			}
			cui.nameX, cui.nameXSet = nx, true
		}
		cui.cell.Place(nt, cui.nameX, nameY)
		// Paint-only: cell/row sizes are fixed absolute geometry, so a
		// content swap must never trigger layout (keeps p95 flat).
		cui.cell.MarkNeedsPaint()
		cui.blend = level
	}
	// refreshCards updates the old/new active cards after a state change.
	// Cards listed here animate toward their target blend in the ticker.
	animating := map[string]bool{}
	refreshCards := func(keys ...string) {
		for _, k := range keys {
			if _, ok := cardUIs[k]; ok && k != "" {
				animating[k] = true
			}
		}
	}
	// stepCards advances running levels toward target (selected 2,
	// hover 1, idle 0) and rebuilds only cards whose level moved.
	stepCards := func(dt float64) {
		if len(animating) == 0 {
			return
		}
		step := dt / cardFadeSec
		for k := range animating {
			cui, ok := cardUIs[k]
			if !ok {
				delete(animating, k)
				continue
			}
			want := 0.0
			if k == selectKey {
				want = 2.0
			} else if k == hoverKey {
				want = 1.0
			}
			next := cui.blend
			if next < want {
				next = math.Min(want, next+step)
			} else if next > want {
				next = math.Max(want, next-step)
			}
			if next != cui.blend {
				fillCard(cui, next)
			}
			if next == want {
				delete(animating, k)
			}
		}
	}
	// blendOf reports the settled level for a card key.
	blendOf := func(key string) float64 {
		if key == "" {
			return 0.0
		}
		if key == selectKey {
			return 2.0
		}
		if key == hoverKey {
			return 1.0
		}
		return 0.0
	}
	// hitCard is defined after vb (needs viewport scroll + catY).
	_ = refreshCards
	_ = stepCards
	_ = animating
	_ = blendOf
	// Catalog viewport follows the window: width = winW-16, height fills
	// from catY to the bottom margin. Row boxes use width 0 (fill the
	// tight viewport constraint at layout), so they track resizes without
	// a rebuild; cards stay 6-per-row left-aligned at fixed 184x100.
	catY := 492.0
	const listMargin = 8.0
	// listRowsCache keeps builder output stable across viewport relayouts:
	// rows are width-agnostic (fixed cards), so a resize must not pay for
	// N card rebuilds (text measure + icon resolve) on the p95 path.
	rowCache := map[int]rendering.RenderObject{}
	listSizeFor := func(ww, wh float64) (float64, float64) {
		w := ww - 2*listMargin
		if w < 320 {
			w = 320
		}
		h := wh - catY - listMargin
		if h < 120 {
			h = 120
		}
		return w, h
	}
	listW, listH := listSizeFor(winW, winH)
	buildRow := func(idx int) rendering.RenderObject {
		if idx < 0 || idx >= len(rows) {
			return rendering.NewAbsoluteBox(0, cardRowH)
		}
		r := rows[idx]
		if r.title != "" {
			box := rendering.NewAbsoluteBox(0, titleRowH)
			box.Place(textLabel(r.title, 12, 0.1, 0.1, 0.12, 1184), 0, 6)
			box.SetDebugName(fmt.Sprintf("f1-icon-cat-title-%d", idx))
			return box
		}
		box := rendering.NewAbsoluteBox(0, cardRowH)
		box.SetDebugName(fmt.Sprintf("f1-icon-row-%d", idx))
		for c, cd := range r.cards {
			x := float64(c)*(cardW+rowGap) + 8
			kebab := kebabForKey(cd.key)
			cui := &cardUI{
				cell: rendering.NewAbsoluteBox(cardW, cardH),
				key:  cd.key, name: cd.name, theme: cd.theme, kebab: kebab,
				rowIdx: idx, col: c, cellX: x,
			}
			cui.cell.SetDebugName(fmt.Sprintf("f1-icon-card-%d-%d", idx, c))
			box.Place(cui.cell, x, 0)
			fillCard(cui, blendOf(cui.key))
			cardUIs[cui.key] = cui
		}
		return box
	}
	vb := kit.PrimNewVariableViewportBox(rowCount, cardRowH, extentAt, func(idx int) rendering.RenderObject {
		if b, ok := rowCache[idx]; ok {
			return b
		}
		b := buildRow(idx)
		rowCache[idx] = b
		return b
	})
	vb.Viewport.FixedWidth, vb.Viewport.FixedHeight = listW, listH
	root.Place(vb.Viewport, 8, catY)

	// hitCard maps window-logical pointer coords to a catalog card key.
	hitCard := func(px, py float64) string {
		lx, ly := px-8, py-catY
		if lx < 0 || ly < 0 || lx >= listW || ly >= listH {
			return ""
		}
		sy := ly + vb.Viewport.ScrollOffset().Y
		lo, hi := 0, len(rows)
		for lo < hi {
			mid := (lo + hi) / 2
			if rowOffsets[mid+1] <= sy {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		if lo < 0 || lo >= len(rows) || len(rows[lo].cards) == 0 {
			return ""
		}
		rel := lx - 8
		if rel < 0 {
			return ""
		}
		pitch := cardW + rowGap
		col := int(rel / pitch)
		if col < 0 || col >= len(rows[lo].cards) {
			return ""
		}
		if math.Mod(rel, pitch) >= cardW {
			return ""
		}
		return rows[lo].cards[col].key
	}
	// goSnippet reports the Go constructor for a card (logged on select).
	goSnippet := func(key string) string {
		cui, ok := cardUIs[key]
		if !ok {
			return fmt.Sprintf("kit.DefaultIconProps(%q)", key)
		}
		return fmt.Sprintf("p := kit.DefaultIconProps(%q); p.Variant = %q", cui.kebab, cui.theme)
	}
	// selectCard pins selection and logs the Go constructor.
	selectCard := func(key string) {
		old := selectKey
		selectKey = key
		refreshCards(old, key)
		if key != "" {
			fmt.Fprintf(os.Stderr, "kit icon: select %s -> %s\n", key, goSnippet(key))
		}
	}
	downKey := ""
	downX, downY := 0.0, 0.0
	hoverSeen, clickLogged := false, false
	scriptedHover, scriptedClick := false, false

	snapDir := filepath.Join("examples", "kit", "icon", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}

	geoValid := false
	resizeEvents := 0
	// Probes use official demo solid areas (no extra fake row).
	ptPanda := probePoint{want: [3]float64{1.0, 0.92, 0.82}, tol: 120.0 / 255}   // panda face #FFEBD2
	ptGreen := probePoint{want: [3]float64{0.32, 0.77, 0.10}, tol: 120.0 / 255}  // #52c41a check
	ptHot := probePoint{want: [3]float64{1.0, 0.41, 0.71}, tol: 120.0 / 255}     // hotpink custom heart
	ptInner := probePoint{want: [3]float64{0.9, 0.9, 0.9}, tol: 16.0 / 255}   // official default secondary #E6E6E6
	var goldenRects []rect
	resolveGeometry := func() {
		if geoValid {
			return
		}
		fx, fy := cb2.Offset().X, cb2.Offset().Y
		ptPanda.x, ptPanda.y = fx+16, fy+16
		cx, cy := bCheckTT.Offset().X, bCheckTT.Offset().Y
		ptGreen.x, ptGreen.y = cx+8, cy+8
		px, py := cb1.Offset().X, cb1.Offset().Y
		// 50px hotpink heart: sample inside the top stroke band.
		ptHot.x, ptHot.y = px+8, py+10
		sx, sy := bSmileTT.Offset().X, bSmileTT.Offset().Y
		ptInner.x, ptInner.y = sx+8, sy+8
		// Golden covers first 3 static basic icons (16px, gap 8).
		goldenRects = []rect{{x: 8, y: by, w: 64, h: 16}}
		geoValid = true
	}

	app := embedder.NewPipelineApp(host, root, embedder.PipelineOptions{
		ClearR: 1, ClearG: 1, ClearB: 1, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		SnapshotPath: func() string {
			if secsSet {
				return filepath.Join(snapDir, "showcase_icon.png")
			}
			return ""
		}(),
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "kit icon: close (%s)\n", win.Backend())
			case platform.EventPointer:
				// Hover/select first (Go-native card behavior), then route
				// to the catalog Scrollable for wheel/drag continuity.
				switch ev.Pointer {
				case platform.PointerMove:
					if k := hitCard(ev.X, ev.Y); k != hoverKey {
						old := hoverKey
						hoverKey = k
						if k != "" {
							hoverSeen = true
						}
						refreshCards(old, k)
					}
				case platform.PointerDown:
					downKey, downX, downY = hitCard(ev.X, ev.Y), ev.X, ev.Y
				case platform.PointerUp:
					if k := hitCard(ev.X, ev.Y); k != "" && k == downKey &&
						math.Abs(ev.X-downX) < 6 && math.Abs(ev.Y-downY) < 6 {
						selectCard(k)
						clickLogged = true
					}
					downKey = ""
				}
				// Continuous scroll: wheel/drag anywhere routes to the
				// catalog Scrollable (official page scrolls as one).
				// Persistent ticker presents the next frame; no extra
				// ScheduleFrame needed here.
				vb.Scroll.HandlePointer(ev)
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					resizeEvents++
					root.FixedWidth, root.FixedHeight = float64(ev.Width), float64(ev.Height)
					listW, listH = listSizeFor(float64(ev.Width), float64(ev.Height))
					vb.Viewport.FixedWidth, vb.Viewport.FixedHeight = listW, listH
					vb.Viewport.MarkNeedsLayout()
					root.MarkNeedsLayout()
					geoValid = false
				}
			}
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)

	var elapsed float64
	resizeDone, resizeBack := false, false
	tourSteps := 0
	scrolledTotal := 0.0
	spinAngle0 := spinInst.EffectiveAngle()
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		// Blend animation is paint-only (fixed slots, no layout).
		stepCards(dt)
		if spinInst.Tick(dt) {
			spec := spinInst.ResolveIconPaintSpec()
			painter := kit.IconPainterForSpec(spec)
			spinBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
				painter(pc, size)
			}
			spinBox.MarkNeedsPaint()
		}
		// Continuous auto tour (timed runs only; manual mode is user-driven
		// so hover is never fought by auto scroll): human-speed 3px/frame
		// drift (E6 slow mode), wrapping to top at the bottom. Black out
		// near the end so the gated damage number reflects steady state,
		// not a scroll transient.
		// tourSteps counts passed 120px bands (== one card row incl gap).
		blackout := secsSet && secs > 0 && elapsed >= float64(secs)-2.0
		if secsSet && !blackout {
			before := vb.Viewport.ScrollOffset().Y
			vb.Viewport.ScrollBy(0, 3)
			after := vb.Viewport.ScrollOffset().Y
			scrolledTotal += after - before
			if after >= vb.Viewport.MaxScrollY()-1e-6 {
				vb.Viewport.ScrollToIndex(0)
			}
			steps := int(scrolledTotal / 120)
			if steps > tourSteps {
				tourSteps = steps
			}
		}
		if ctl != nil && !resizeDone && elapsed >= 2.0 {
			resizeDone = true
			ctl.SetSize(winW-160, winH-100)
		}
		if ctl != nil && resizeDone && !resizeBack && elapsed >= 4.5 {
			resizeBack = true
			ctl.SetSize(winW, winH)
		}
		// Scripted mouse proof (timed runs only): hover lands on a twotone
		// card, select lands on a plain-theme card in the same row, so the
		// snapshot proves both feedback paths at once.
		if secsSet && !scriptedHover && elapsed >= 1.0 {
			scriptedHover = true
			if demoHoverKey != "" {
				old := hoverKey
				hoverKey = demoHoverKey
				hoverSeen = true
				refreshCards(old, hoverKey)
			}
		}
		if secsSet && scriptedHover && !scriptedClick && elapsed >= 2.0 {
			scriptedClick = true
			if demoSelectKey != "" {
				selectCard(demoSelectKey)
				clickLogged = true
			}
		}
		// Park the proof row at the top once scrolling stops, so the gated
		// snapshot actually shows hover + selected cards (previously the
		// tour left them scrolled out of view).
		if secsSet && blackout && demoRowIdx >= 0 {
			vb.Viewport.ScrollToIndex(demoRowIdx)
		}
		app.ScheduleFrame()
		proc.Sample()
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	// Pre-flight: demos + true shapes must resolve.
	if !rotInst.Known() || !spinInst.Known() || !customInst.Known() || !dupInst.Known() {
		fmt.Fprintln(os.Stderr, "FAIL: demo instances must all resolve Known()")
		os.Exit(1)
	}
	if !heartInst.Known() || !checkInst.Known() {
		fmt.Fprintln(os.Stderr, "FAIL: two-tone instances must resolve")
		os.Exit(1)
	}
	for _, in := range fontInsts {
		if !in.Known() {
			fmt.Fprintln(os.Stderr, "FAIL: iconfont instances must resolve")
			os.Exit(1)
		}
	}
	for _, in := range multiInsts {
		if !in.Known() {
			fmt.Fprintln(os.Stderr, "FAIL: multi-source instances must resolve")
			os.Exit(1)
		}
	}
	if !kit.PrimIsKnownCustomSVG("custom-heart-svg") || !kit.PrimIsKnownCustomSVG("custom-panda-svg") {
		fmt.Fprintln(os.Stderr, "FAIL: custom heart/panda must be registered")
		os.Exit(1)
	}
	for _, k := range []string{"iconfont-tuichu", "iconfont-facebook", "iconfont-twitter", "iconfont-javascript", "iconfont-java", "iconfont-shoppingcart-f2", "iconfont-python-f2"} {
		if !kit.PrimIsKnownCustomSVG(k) {
			fmt.Fprintf(os.Stderr, "FAIL: iconfont %s must be registered\n", k)
			os.Exit(1)
		}
	}
	if dupSpec := dupInst.ResolveIconPaintSpec(); dupSpec.CustomKey != "iconfont-shoppingcart-f2" {
		fmt.Fprintf(os.Stderr, "FAIL: shoppingcart override=%q want iconfont-shoppingcart-f2\n", dupSpec.CustomKey)
		os.Exit(1)
	}
	if rowCount < 100 || rowCount > 200 {
		fmt.Fprintf(os.Stderr, "FAIL: catalog rows=%d want 100..200\n", rowCount)
		os.Exit(1)
	}

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsedSec := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	bind, total := rendering.LastVirtualBind()

	finalW, finalH := winW, winH
	if ctl != nil {
		if w, h := ctl.Size(); w > 0 && h > 0 {
			finalW, finalH = w, h
		}
	}

	resolveGeometry()
	finalChecks := []pixelCheck{
		{pt: &ptPanda, desc: "custom_panda_face_FFEBD2"},
		{pt: &ptGreen, desc: "twotone_check_52c41a"},
		{pt: &ptHot, desc: "custom_heart_hotpink"},
		{pt: &ptInner, desc: "twotone_default_E6E6E6"},
	}
	runPixelChecks(loadImage(filepath.Join(snapDir, "showcase_icon.png")), finalChecks, pixelResult)
	scriptedOK, scriptedTotal := 0, len(pixelResult)
	for _, ok := range pixelResult {
		if ok {
			scriptedOK++
		}
	}
	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapDir, goldenRects)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "F1-icon",
		Scenario:      "kit_icon",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: int64(finalW * finalH),
		Warmup:        true,
		Extra: map[string]any{
			"slope_gate":             "off",
			"icons_total":            totalIcons,
			"catalog_rows":           rowCount,
			"demo_count":             20,
			"virtual_bind":           bind,
			"virtual_items":          total,
			"tour_steps":             tourSteps,
			"spin_advances":          spinInst.EffectiveAngle() != spinAngle0,
			"hover_ok":               hoverSeen,
			"select_logged":          clickLogged,
			"select_key":             selectKey,
			"scripted_ok":            scriptedOK,
			"scripted_total":         scriptedTotal,
			"pixel_golden_diff_pct":  goldenDiffPct,
			"pixel_golden_total_px":  goldenTotalPx,
			"pixel_golden_first_run": goldenFirstRun,
			"resize_events":          resizeEvents,
			"resize_restored":        finalW == winW && finalH == winH,
			"client_px":              fmt.Sprintf("%dx%d", finalW, finalH),
			"list_px":                fmt.Sprintf("%.0fx%.0f", listW, listH),
			"list_follows_window":    listW == float64(finalW)-2*listMargin && listH == float64(finalH)-catY-listMargin,
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:           1,
		RequireRetainedPolicy: true,
		RequirePersistentFPS:  true,
		MinFPSWall:            55,
		MaxP95Ms:              22,
		MaxDamageRatio:        0.35,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	if snap.VSyncSource == "" {
		fmt.Fprintln(os.Stderr, "FAIL: vsync_source missing")
		os.Exit(1)
	}
	if scriptedTotal != 4 || scriptedOK != scriptedTotal {
		fmt.Fprintf(os.Stderr, "FAIL: scripted=%d/%d want 4/4 (pixel probes)\n", scriptedOK, scriptedTotal)
		os.Exit(1)
	}
	if !goldenFirstRun && goldenDiffPct != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: pixel_golden_diff_pct=%.4f want 0 over %d px\n", goldenDiffPct, goldenTotalPx)
		os.Exit(1)
	}
	if total > 0 && bind >= total {
		fmt.Fprintf(os.Stderr, "FAIL: virtual bind=%d not << items=%d\n", bind, total)
		os.Exit(1)
	}
	if tourSteps < 3 {
		fmt.Fprintf(os.Stderr, "FAIL: tour=%d want >=3 (catalog auto tour)\n", tourSteps)
		os.Exit(1)
	}
	if secsSet && !hoverSeen {
		fmt.Fprintln(os.Stderr, "FAIL: hover never highlighted a card")
		os.Exit(1)
	}
	if secsSet && !clickLogged {
		fmt.Fprintln(os.Stderr, "FAIL: select never logged a Go constructor")
		os.Exit(1)
	}
	if !spinInst.Tick(0.016) && spinInst.EffectiveAngle() == spinAngle0 {
		fmt.Fprintln(os.Stderr, "FAIL: spin did not advance")
		os.Exit(1)
	}
	if !resizeBack || finalW != winW || finalH != winH {
		fmt.Fprintf(os.Stderr, "FAIL: resize round trip incomplete (back=%v %dx%d)\n", resizeBack, finalW, finalH)
		os.Exit(1)
	}
	if listW != float64(finalW)-2*listMargin || listH != float64(finalH)-catY-listMargin {
		fmt.Fprintf(os.Stderr, "FAIL: list %.0fx%.0f does not follow window %dx%d\n", listW, listH, finalW, finalH)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "kit icon: OK presents=%d scripted=%d/%d golden=%.4f%% bind=%d/%d tour=%d resize=%v %dx%d elapsed=%.1fs\n",
		app.PresentCount(), scriptedOK, scriptedTotal, goldenDiffPct, bind, total, tourSteps, resizeBack, finalW, finalH, elapsedSec)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

// keyToKebab maps PascalCase key back to kebab name via registry scan.
var kebabCache = map[string]string{}

func kebabForKey(key string) string {
	if n, ok := kebabCache[key]; ok {
		return n
	}
	total := kit.PrimAntdIconCount()
	for i := 0; i < total; i++ {
		k, name, _, ok := kit.PrimAntdIconEntry(i)
		if !ok {
			continue
		}
		if k == key {
			kebabCache[key] = name
			return name
		}
	}
	kebabCache[key] = strings.ToLower(key)
	return kebabCache[key]
}

func runPixelChecks(img image.Image, checks []pixelCheck, result map[string]bool) {
	dpr := 1.0
	if img != nil {
		dpr = float64(img.Bounds().Dx()) / winW
	}
	for _, c := range checks {
		r, g, b, valid := sampleLogical(img, dpr, c.pt.x, c.pt.y)
		ok := valid && nearC(r, g, b, c.pt.want, c.pt.tol)
		fmt.Fprintf(os.Stderr, "kit icon: pixel %-22s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
			c.desc, c.pt.x, c.pt.y, r, g, b, c.pt.want[0], c.pt.want[1], c.pt.want[2], ok)
		result[c.desc] = ok
	}
}

func loadImage(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pixel checks: %v\n", err)
		return nil
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pixel checks: decode %s: %v\n", path, err)
		return nil
	}
	return img
}

func sampleLogical(img image.Image, dpr float64, lx, ly float64) (r, g, b float64, valid bool) {
	if img == nil {
		return 0, 0, 0, false
	}
	px, py := int(lx*dpr), int(ly*dpr)
	if px < 0 || py < 0 || px >= img.Bounds().Dx() || py >= img.Bounds().Dy() {
		return 0, 0, 0, false
	}
	r32, g32, b32, _ := img.At(px, py).RGBA()
	return float64(r32>>8) / 255, float64(g32>>8) / 255, float64(b32>>8) / 255, true
}

func nearC(r, g, b float64, want [3]float64, tol float64) bool {
	return math.Abs(r-want[0]) <= tol && math.Abs(g-want[1]) <= tol && math.Abs(b-want[2]) <= tol
}

func evaluateGolden(snapDir string, rects []rect) (diffPct float64, totalPx int64, firstRun bool) {
	cur := filepath.Join(snapDir, "showcase_icon.png")
	// Backends rasterize static pixels differently (native vs go differ
	// ~3% on AA fringe), so each backend owns its baseline.
	suffix := ""
	if os.Getenv("GPUI_BACKEND") == "go" {
		suffix = "_go"
	}
	base := filepath.Join(snapDir, "showcase_icon_base"+suffix+".png")
	if _, err := os.Stat(base); err != nil {
		if os.Getenv("GPUI_ACCEPT_GOLDEN") != "1" {
			fmt.Fprintf(os.Stderr, "golden baseline missing: %s (re-run with GPUI_ACCEPT_GOLDEN=1 to seed, then review + commit)\n", base)
			return 100, 0, false
		}
		data, err := os.ReadFile(cur)
		if err != nil {
			fmt.Fprintf(os.Stderr, "golden: current snapshot %s missing (%v)\n", cur, err)
			return 100, 0, false
		}
		if err := os.WriteFile(base, data, 0o644); err == nil {
			fmt.Fprintf(os.Stderr, "golden baseline stored: %s\n", base)
			return 0, 0, true
		}
		return 100, 0, false
	}
	diff, total, err := comparePNG(base, cur, rects)
	if err != nil {
		fmt.Fprintf(os.Stderr, "golden compare: %v\n", err)
		return 100, 0, false
	}
	fmt.Fprintf(os.Stderr, "golden %s: diff=%.4f%% over %d px\n", filepath.Base(cur), diff, total)
	return diff, total, false
}

func comparePNG(basePath, curPath string, rects []rect) (pct float64, total int64, err error) {
	a, b := loadImage(basePath), loadImage(curPath)
	if a == nil || b == nil {
		return 0, 0, fmt.Errorf("decode failed")
	}
	if a.Bounds() != b.Bounds() {
		return 0, 0, fmt.Errorf("size mismatch %v vs %v", a.Bounds(), b.Bounds())
	}
	dpr := float64(a.Bounds().Dx()) / winW
	var diff int64
	for _, r := range rects {
		x0, y0 := int(r.x*dpr), int(r.y*dpr)
		x1, y1 := int((r.x+r.w)*dpr), int((r.y+r.h)*dpr)
		for py := y0; py < y1 && py < a.Bounds().Dy(); py++ {
			for px := x0; px < x1 && px < a.Bounds().Dx(); px++ {
				ar, ag, ab, aa := a.At(px, py).RGBA()
				br, bg, bb, ba := b.At(px, py).RGBA()
				total++
				if ar != br || ag != bg || ab != bb || aa != ba {
					diff++
				}
			}
		}
	}
	if total == 0 {
		return 0, 0, fmt.Errorf("empty golden mask")
	}
	return float64(diff) * 100 / float64(total), total, nil
}
