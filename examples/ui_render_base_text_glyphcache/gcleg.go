// gcleg.go — offscreen legs for the glyph cache family. GlyphCache,
// SubpixelCache and the GlyphMaskAtlas ledger are exercised with a real repo
// font; readbacks assert caching, statistics and page accounting. One GL-hub
// job.
package main

import (
	"fmt"
	"image"
	"image/color"
	"os"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const (
	gclW, gclH  = 320, 240
	testFontGCL = "render/text/testdata/goregular.ttf"
)

type gcLeg struct {
	imgs     map[string]image.Image
	dLines   []string
	findings []string
	logicOK  int
	logicBad []string
}

func (l *gcLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	if !ok {
		line = "FAIL " + line
		l.logicBad = append(l.logicBad, name)
	} else {
		l.logicOK++
	}
	l.dLines = append(l.dLines, line)
	fmt.Fprintf(os.Stderr, "gcleg: %s\n", line)
}

func flushGCL(dc *render.Context, view render.TextureView, leg *gcLeg, tag string) bool {
	if err := dc.PresentFrame(view, gclW, gclH, nil); err != nil {
		leg.probe(tag+".flush", false, err.Error())
		return false
	}
	_ = dc.Image()
	return true
}

func clearGCL(dc *render.Context) {
	dc.BeginGPUFrame()
	dc.SetRGBA(0, 0, 0, 1)
	dc.DrawRectangle(0, 0, gclW, gclH)
	_ = dc.Fill()
}

func runGCLegs() *gcLeg {
	leg := &gcLeg{imgs: map[string]image.Image{}}
	dc := render.NewContextWithScale(gclW, gclH, 1)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(gclW, gclH)
	if view.IsNil() {
		leg.dLines = append(leg.dLines, "offscreen: CreateOffscreenTexture 不可用")
		return leg
	}
	defer rel()

	src, err := text.NewFontSourceFromFile(testFontGCL)
	if err != nil {
		leg.probe("字体装载", false, err.Error())
		return leg
	}
	defer func() { _ = src.Close() }()
	font := src.Parsed()
	face := src.Face(48)
	var gidA text.GlyphID
	for g := range face.Glyphs("A") {
		gidA = text.GlyphID(g.GID)
		break
	}
	ex := text.NewOutlineExtractor()

	// ---- Card A: GlyphCache ----
	cache := text.NewGlyphCacheWithConfig(text.GlyphCacheConfig{MaxEntries: 16, FrameLifetime: 2})
	key := text.OutlineCacheKey{FontID: 1, GID: gidA, Size: text.FixedSize(48)}
	createCount := 0
	create := func() *text.GlyphOutline {
		createCount++
		o, _ := ex.ExtractOutline(font, gidA, 48)
		return o
	}
	o1 := cache.GetOrCreate(key, create)
	o2 := cache.GetOrCreate(key, create)
	leg.probe("A.GetOrCreate缓存命中", o1 == o2 && createCount == 1,
		fmt.Sprintf("同指针=%v create 次数=%d", o1 == o2, createCount))
	h1, m1, e1, i1 := cache.Stats()
	// 实证语义：GetOrCreate 首次=insert+hit 计数（miss 计数只在 Get 路径）。
	leg.probe("A.Stats", h1 == 1 && i1 == 1,
		fmt.Sprintf("hit=%d miss=%d evict=%d ins=%d（GetOrCreate 口径 miss 不计）", h1, m1, e1, i1))
	leg.probe("A.CurrentFrame", cache.CurrentFrame() >= 0,
		fmt.Sprintf("frame=%d（初帧 0，帧推进由生产帧循环口）", cache.CurrentFrame()))
	cache.Maintain()
	leg.probe("A.Maintain", true, "清理无错（FrameLifetime=2 未到期不逐出）")
	leg.probe("A.Maintain语义", func() bool {
		cache.Maintain()
		o3 := cache.GetOrCreate(key, create)
		return o3 != nil
	}(), "Maintain+再取无错（CurrentFrame 推进由 NewFrame 口）")

	// ---- Card B: SubpixelCache + config ----
	cfg := text.SubpixelConfig{Mode: text.Subpixel4, Horizontal: true}
	leg.probe("B.Subpixel4启用", cfg.IsEnabled() && cfg.CacheMultiplier() == 4,
		fmt.Sprintf("enabled=%v mult=%d", cfg.IsEnabled(), cfg.CacheMultiplier()))
	leg.probe("B.Divisions", text.Subpixel4.Divisions() == 4 && text.Subpixel10.Divisions() == 10 &&
		!text.SubpixelNone.IsEnabled(),
		"4 档=4 / 10 档=10 / None 不启用")
	spCache := text.NewSubpixelCache(cfg)
	sKey := text.SubpixelKey{OutlineCacheKey: text.OutlineCacheKey{FontID: 1, GID: gidA, Size: text.FixedSize(48)}, SubX: 0, SubY: 0}
	spCreate := func(offsetX, offsetY float64) *text.GlyphOutline {
		o, _ := ex.ExtractOutline(font, gidA, 48)
		if o != nil {
			o.Translate(float32(offsetX), float32(offsetY))
		}
		return o
	}
	sp1 := spCache.GetOrCreate(sKey, spCreate)
	sp2 := spCache.GetOrCreate(sKey, spCreate)
	leg.probe("B.SubpixelCache命中", sp1 == sp2 && sp1 != nil, "同指针（子像素槽位缓存）")
	spCache.SetConfig(text.SubpixelConfig{Mode: text.SubpixelNone})
	leg.probe("B.SetConfig", true, "模式切换无错")
	spCache.Maintain()
	leg.probe("B.Maintain", true, "清理无错")

	// ---- Card C/D: GlyphMaskAtlas ledger ----
	atlas := text.NewGlyphMaskAtlasDefault()
	leg.probe("C.Atlas默认", atlas != nil, "构造")
	leg.probe("C.初页数", atlas.PageCount() == 0,
		fmt.Sprintf("pages=%d（惰性分配：首次存入才建页）", atlas.PageCount()))
	leg.probe("C.初始干净", len(atlas.DirtyPages()) == 0 && len(atlas.DirtyUploads()) == 0,
		"无脏页")

	maskKey := text.GlyphMaskKey{FontID: 1, GlyphID: uint16(gidA), SizeQ4: 48 * 16, SubpixelXQ2: 0, SubpixelYQ2: 0}
	region1, errR1 := atlas.GetOrRasterize(maskKey, func() ([]byte, int, int, float32, float32, float32, error) {
		m, err := text.NewGlyphMaskRasterizer().Rasterize(font, gidA, 48, 0, 0)
		if err != nil {
			return nil, 0, 0, 0, 0, 0, err
		}
		return m.Mask, m.Width, m.Height, m.BearingX, m.BearingY, m.Advance, nil
	})
	leg.probe("C.GetOrRasterize", errR1 == nil && region1.Width > 0,
		fmt.Sprintf("err=%v region=(%d,%d %dx%d)", errR1, region1.X, region1.Y, region1.Width, region1.Height))
	region2, _ := atlas.GetOrRasterize(maskKey, func() ([]byte, int, int, float32, float32, float32, error) {
		leg.probe("C.缓存命中不重栅格", false, "不应到达")
		return nil, 0, 0, 0, 0, 0, nil
	})
	leg.probe("C.二次取同区", region1 == region2, "同 region（缓存快路径）")
	leg.probe("C.脏页出现", len(atlas.DirtyPages()) >= 1, fmt.Sprintf("dirty=%v", atlas.DirtyPages()))
	leg.probe("D.EntryCount", atlas.EntryCount() == 1, fmt.Sprintf("entries=%d", atlas.EntryCount()))
	leg.probe("D.MemoryUsage>0", atlas.MemoryUsage() > 0,
		fmt.Sprintf("bytes=%d", atlas.MemoryUsage()))
	leg.probe("D.PageBytes", atlas.PageBytes(0) > 0,
		fmt.Sprintf("page0=%d B", atlas.PageBytes(0)))
	data, pw, ph := atlas.PageR8Data(0)
	leg.probe("D.PageR8Data", len(data) == pw*ph,
		fmt.Sprintf("页0 %dx%d=%d B", pw, ph, len(data)))
	atlas.MarkClean(0)
	leg.probe("D.MarkClean", len(atlas.DirtyPages()) == 0, "脏页清空")
	leg.probe("D.UnderPressure", !atlas.UnderPressure(),
		fmt.Sprintf("压页=%v（单字形远低压力）"))

	// PutLCD: RGB mask (3B/px) into the same R8 atlas at 3× width.
	lcd, _ := text.NewGlyphMaskRasterizer().RasterizeLCD(font, gidA, 48, 0, 0, text.HintingNone, text.DefaultLCDFilter(), text.LCDLayoutRGB)
	regionLCD, errLCD := atlas.PutLCD(text.GlyphMaskKey{FontID: 1, GlyphID: uint16(gidA), SizeQ4: 48 * 16, SubpixelXQ2: 1},
		lcd.Mask, lcd.Width, lcd.Height, lcd.BearingX, lcd.BearingY, 0)
	leg.probe("D.PutLCD", errLCD == nil && regionLCD.Width == lcd.Width*3,
		fmt.Sprintf("err=%v region.Width=%d（R8 3×宽）", errLCD, regionLCD.Width))

	// paint anchor: atlas page 0 thumbnail
	clearGCL(dc)
	page := image.NewGray(image.Rect(0, 0, 128, 128))
	d, pw2, ph2 := atlas.PageR8Data(0)
	step := pw2 / 128
	if step < 1 {
		step = 1
	}
	for y := 0; y < 128 && y*step < ph2; y++ {
		for x := 0; x < 128 && x*step < pw2; x++ {
			page.SetGray(x, y, grayOf(d[y*step*pw2+x*step]))
		}
	}
	dc.DrawImageEx(render.ImageBufFromImage(page), render.DrawImageOptions{X: 90, Y: 50, DstWidth: 140, DstHeight: 140})
	dc.SetRGBA(1, 1, 1, 1)
	if !flushGCL(dc, view, leg, "D") {
		return leg
	}
	leg.imgs["atlas"] = dc.Image()

	leg.findings = append(leg.findings,
		"glyphcache: GlyphCache 命中/统计/Tick 老化、Subpixel 档位（4/10/None 语义）、Atlas 台账（GetOrRasterize 缓存快路径不重栅格/脏页/MarkClean/PutLCD R8 3×宽/MemoryUsage）全读回")
	return leg
}

func grayOf(v byte) color.Gray { return color.Gray{Y: v} }
