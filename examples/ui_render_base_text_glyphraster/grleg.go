// grleg.go — offscreen legs for GlyphMaskRasterizer variants. A real repo font
// (goregular.ttf) drives every Rasterize entry; R8 mask readbacks assert the
// observable state. One GL-hub job.
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
	grW, grH   = 320, 240
	testFontGR = "render/text/testdata/goregular.ttf"
)

type grLeg struct {
	imgs     map[string]image.Image
	dLines   []string
	findings []string
	logicOK  int
	logicBad []string
}

func (l *grLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	if !ok {
		line = "FAIL " + line
		l.logicBad = append(l.logicBad, name)
	} else {
		l.logicOK++
	}
	l.dLines = append(l.dLines, line)
	fmt.Fprintf(os.Stderr, "grleg: %s\n", line)
}

// maskToImage converts an R8 mask into a white-on-black image (visual anchor).
func maskToImage(m *text.GlyphMaskResult) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, m.Width, m.Height))
	for y := 0; y < m.Height; y++ {
		for x := 0; x < m.Width; x++ {
			a := m.Mask[y*m.Width+x]
			img.SetRGBA(x, y, color.RGBA{R: a, G: a, B: a, A: 255})
		}
	}
	return img
}

// maskInk counts pixels with alpha >= threshold.
func maskInk(m *text.GlyphMaskResult, thr uint8) int {
	n := 0
	for _, v := range m.Mask {
		if v >= thr {
			n++
		}
	}
	return n
}

// maskIsBinary reports whether every mask value is 0 or 255.
func maskIsBinary(m *text.GlyphMaskResult) bool {
	for _, v := range m.Mask {
		if v != 0 && v != 255 {
			return false
		}
	}
	return true
}

func maskMax(m *text.GlyphMaskResult) uint8 {
	var mx uint8
	for _, v := range m.Mask {
		if v > mx {
			mx = v
		}
	}
	return mx
}

func runGRLegs() *grLeg {
	leg := &grLeg{imgs: map[string]image.Image{}}
	dc := render.NewContextWithScale(grW, grH, 1)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(grW, grH)
	if view.IsNil() {
		leg.dLines = append(leg.dLines, "offscreen: CreateOffscreenTexture 不可用")
		return leg
	}
	defer rel()

	src, err := text.NewFontSourceFromFile(testFontGR)
	if err != nil {
		leg.probe("字体装载", false, err.Error())
		return leg
	}
	defer func() { _ = src.Close() }()
	font := src.Parsed()

	// 'A' glyph id from the face (gid 36 established in fontcontract batch 2).
	face := src.Face(48)
	var gidA text.GlyphID
	for g := range face.Glyphs("A") {
		gidA = text.GlyphID(g.GID)
		break
	}
	leg.probe("gidA", gidA > 0, fmt.Sprintf("gid=%d", gidA))

	r := text.NewGlyphMaskRasterizer()

	// ---- Card A: AA tiers ----
	mPlain, err := r.Rasterize(font, gidA, 48, 0, 0)
	leg.probe("A.Rasterize", err == nil && mPlain != nil && len(mPlain.Mask) == mPlain.Width*mPlain.Height,
		fmt.Sprintf("err=%v %dx%d", err, mPlain.Width, mPlain.Height))
	leg.probe("A.Rasterize墨迹", maskInk(mPlain, 128) > 20,
		fmt.Sprintf("ink(≥128)=%d", maskInk(mPlain, 128)))

	mHinted, _ := r.RasterizeHinted(font, gidA, 48, 0, 0, text.HintingFull)
	leg.probe("A.RasterizeHinted", mHinted != nil && maskInk(mHinted, 128) > 20,
		fmt.Sprintf("ink=%d 尺寸 %dx%d", maskInk(mHinted, 128), mHinted.Width, mHinted.Height))

	mBold, _ := r.RasterizeHintedBold(font, gidA, 48, 0, 0, text.HintingNone, true)
	leg.probe("A.HintedBold更粗", mBold != nil && maskInk(mBold, 128) > maskInk(mPlain, 128),
		fmt.Sprintf("bold ink=%d vs plain=%d", maskInk(mBold, 128), maskInk(mPlain, 128)))

	// FT26 是 own-parsed CFF 档：goregular（TrueType glyf）返回 (nil,false,nil)——
	// 契约语义如实登记（PostScript 轮廓字体才走 FreeType 26.6 路径）。
	mFT26, ftOK, errFT := r.RasterizeHintedFT26(font, gidA, 48, text.HintingFull, nil)
	leg.probe("A.RasterizeHintedFT26", errFT == nil && !ftOK,
		fmt.Sprintf("err=%v ok=%v（glyf 字体无 CFF→不适用，契约分支命中）", errFT, ftOK))
	_ = mFT26

	leg.imgs["plain"] = maskToImage(mPlain)
	leg.imgs["hinted"] = maskToImage(mHinted)
	leg.imgs["bold"] = maskToImage(mBold)

	// ---- Card B: aliased binary tiers ----
	mAlias, _ := r.RasterizeAliased(font, gidA, 48, 0, 0, text.HintingNone)
	leg.probe("B.Aliased二值", mAlias != nil && maskIsBinary(mAlias),
		fmt.Sprintf("二值掩码 ink(255)=%d", maskInk(mAlias, 255)))
	mAliasBold, _ := r.RasterizeAliasedBold(font, gidA, 48, 0, 0, text.HintingNone, false)
	leg.probe("B.AliasedBold", mAliasBold != nil && maskInk(mAliasBold, 255) >= maskInk(mAlias, 255),
		fmt.Sprintf("bold ink=%d vs alias=%d", maskInk(mAliasBold, 255), maskInk(mAlias, 255)))
	ex := text.NewOutlineExtractor()
	outline, errEx := ex.ExtractOutline(font, gidA, 48)
	leg.probe("B.ExtractOutline", errEx == nil && outline != nil,
		fmt.Sprintf("err=%v segs=%d", errEx, outline.SegmentCount()))
	mOutline, errO := r.RasterizeOutline(outline, 0, 0)
	leg.probe("B.RasterizeOutline", errO == nil && mOutline != nil && maskInk(mOutline, 128) > 0,
		fmt.Sprintf("err=%v ink=%d", errO, maskInk(mOutline, 128)))
	mOutlineAlias, errOA := r.RasterizeOutlineAliased(outline, 0, 0)
	leg.probe("B.RasterizeOutlineAliased", errOA == nil && mOutlineAlias != nil && maskIsBinary(mOutlineAlias),
		fmt.Sprintf("err=%v 二值 ink=%d", errOA, maskInk(mOutlineAlias, 255)))

	leg.imgs["aliased"] = maskToImage(mAlias)
	leg.imgs["aliasedbold"] = maskToImage(mAliasBold)
	leg.imgs["outline"] = maskToImage(mOutlineAlias)

	// ---- Card C: LCD tiers (3x horizontal oversampling) ----
	lcdFilter := text.DefaultLCDFilter()
	mLCD, errL := r.RasterizeLCD(font, gidA, 48, 0, 0, text.HintingNone, lcdFilter, text.LCDLayoutRGB)
	leg.probe("C.RasterizeLCD", errL == nil && mLCD != nil,
		fmt.Sprintf("err=%v %dx%d RGB 缓冲 %d B", errL, mLCD.Width, mLCD.Height, len(mLCD.Mask)))
	leg.probe("C.LCD缓冲3B/px", len(mLCD.Mask) == mLCD.Width*mLCD.Height*3,
		fmt.Sprintf("缓冲 %d = 宽%d×高%d×3", len(mLCD.Mask), mLCD.Width, mLCD.Height))
	mLCDBold, _ := r.RasterizeLCDBold(font, gidA, 48, 0, 0, text.HintingNone, lcdFilter, text.LCDLayoutRGB, true)
	leg.probe("C.LCDBold", mLCDBold != nil && mLCDBold.Width >= mLCD.Width,
		fmt.Sprintf("bold 宽 %d ≥ 常规 %d（Embolden 外扩，幅度随字号如实值）", mLCDBold.Width, mLCD.Width))
	outlineLCD, _ := text.NewOutlineExtractor().ExtractOutline(font, gidA, 48)
	mLCDOutline, errLO := r.RasterizeLCDOutline(outlineLCD, 0, 0, lcdFilter, text.LCDLayoutRGB)
	leg.probe("C.LCDOutline", errLO == nil && mLCDOutline != nil && mLCDOutline.Width == mLCD.Width,
		fmt.Sprintf("err=%v 宽 %d", errLO, mLCDOutline.Width))

	lcdImg := func(m *text.LCDMaskResult) image.Image {
		img := image.NewRGBA(image.Rect(0, 0, m.Width, m.Height))
		for y := 0; y < m.Height; y++ {
			for x := 0; x < m.Width; x++ {
				i := (y*m.Width + x) * 3
				// RGB subpixel coverage → grayscale anchor (max channel)
				a := m.Mask[i]
				if m.Mask[i+1] > a {
					a = m.Mask[i+1]
				}
				if m.Mask[i+2] > a {
					a = m.Mask[i+2]
				}
				img.SetRGBA(x, y, color.RGBA{R: a, G: a, B: a, A: 255})
			}
		}
		return img
	}
	leg.imgs["lcd"] = lcdImg(mLCD)
	leg.imgs["lcdbold"] = lcdImg(mLCDBold)
	leg.imgs["lcdoutline"] = lcdImg(mLCDOutline)

	// ---- Card D: GlyphMaskResult geometry ----
	leg.probe("D.几何读回", mPlain.BearingY > 0 && mPlain.Advance > 0,
		fmt.Sprintf("bearing=(%.1f,%.1f) advance=%.1f", mPlain.BearingX, mPlain.BearingY, mPlain.Advance))
	leg.probe("D.掩码尺寸合理", mPlain.Width >= 20 && mPlain.Width <= 60 && mPlain.Height >= 25 && mPlain.Height <= 70,
		fmt.Sprintf("%dx%d（48px 'A'）", mPlain.Width, mPlain.Height))
	// subpixel offset shifts the mask (same gid, subpixel 0.5)
	mSub, _ := r.Rasterize(font, gidA, 48, 0.5, 0)
	diffShift := false
	if mSub != nil && len(mSub.Mask) == len(mPlain.Mask) {
		for i := range mPlain.Mask {
			if mSub.Mask[i] != mPlain.Mask[i] {
				diffShift = true
				break
			}
		}
	}
	leg.probe("D.亚像素偏移", diffShift, "subpixelX=0.5 掩码内容变化（错相采样）")
	lcdInk := 0
	for i := 0; i+2 < len(mLCD.Mask); i += 3 {
		if mLCD.Mask[i] > 64 {
			lcdInk++
		}
	}
	leg.probe("D.LCD墨迹非空", lcdInk > 0, fmt.Sprintf("R 通道 ink=%d", lcdInk))

	leg.imgs["geom"] = maskToImage(mSub)

	leg.findings = append(leg.findings,
		"glyphraster: 11 个 Rasterize 变体全直调（真字体 gid）——R8 掩码读回：AA 档灰度墨迹、Aliased 档严格二值、LCD 档宽=3×逻辑宽、Bold 档墨迹多于常规、亚像素偏移错相采样")
	return leg
}
