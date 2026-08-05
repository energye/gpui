// HbShaper — HarfBuzz Go 移植（go-text/typesetting/harfbuzz）驱动的 shaper。
//
// ENGINE_TEXT_SHAPING_PLAN（2026-08-05）：shaping 层（A 类）替换为 HarfBuzz，
// 像素层（B 类：hint / 光栅化 / 缓存）保持自研。M0 目标：Latin 文本走 HarfBuzz
// 输出，与自研 OwnShaper 行为对齐（glyph 序列 + advance）。
//
// 分工对照（用户确认）：
//   - A 类（换用 HarfBuzz）：GSUB/GPOS 引擎、Indic/Arabic 复杂脚本、NFC、AAT…
//   - B 类（自研保留）：tt_*.go（TT 指令引擎）、autohint_*.go、光栅化、缓存层
//
// 缩放约定：Font.XScale/YScale = size<<12（16.12 定点，Position 单位为
// 1/4096 像素），输出像素 = Position >> hbScaleBits。高精度定点使 HbShaper
// 与自研 float64 全精度的 advance 误差趋近于零（÷4096 ≤ 0.0003px）。
package text

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"sync"

	"github.com/go-text/typesetting/font"
	hb "github.com/go-text/typesetting/harfbuzz"
	"github.com/go-text/typesetting/language"
)

// hbScaleBits is the fixed-point fraction bits for HarfBuzz positions
// (16.12: units of 1/4096 px, error ≤ 0.0003px vs float64).
const hbScaleBits = 12

// hbFont bundles the typesetting font.Face and its HarfBuzz wrapper.
type hbFont struct {
	face *font.Face
	hbf  *hb.Font
}

// HbShaper implements the Shaper interface with the HarfBuzz Go port.
// It caches parsed fonts per FontSource. Safe for concurrent use.
type HbShaper struct {
	mu    sync.RWMutex
	faces map[*FontSource]*hbFont
}

// NewHbShaper creates a new HarfBuzz-backed shaper.
func NewHbShaper() *HbShaper {
	return &HbShaper{faces: make(map[*FontSource]*hbFont)}
}

// Shape implements the Shaper interface.
func (s *HbShaper) Shape(text string, face Face) []ShapedGlyph {
	if text == "" || face == nil {
		return nil
	}
	source := face.Source()
	if source == nil {
		return nil
	}
	hbf, err := s.getOrCreateHbFont(source)
	if err != nil || hbf == nil {
		return nil
	}

	size := face.Size()
	// 16.12 定点缩放：Position 单位为 1/4096 像素（远高于 26.6，逼近
	// 自研 float64 全精度）。渲染层拿到像素后自行取整。
	hbf.hbf.XScale = int32(math.Round(size * (1 << hbScaleBits)))
	hbf.hbf.YScale = hbf.hbf.XScale

	runes := []rune(text)

	// Buffer 填充：跳过控制字符（与 OwnShaper runeToGlyphs 一致），
	// tab 映射为 space（advance 在输出阶段按 tab-stop 放大）。
	buf := hb.NewBuffer()
	buf.Flags = hb.Bot | hb.Eot
	buf.ClusterLevel = hb.MonotoneGraphemes
	buf.Props.Direction = hbDirection(face.Direction())
	buf.Props.Script = detectHBScript(runes)
	buf.Props.Language = language.NewLanguage(face.Language())

	var tabClusters []int
	for i, r := range runes {
		switch {
		case r == '\t':
			buf.AddRune(' ', i)
			tabClusters = append(tabClusters, i)
		case r < 0x20:
			continue // control chars dropped (OwnShaper parity)
		default:
			buf.AddRune(r, i)
		}
	}

	buf.Shape(hbf.hbf, hbFeatures(face.Features()))

	return hbToShapedGlyphs(buf, runes, size, tabClusters)
}

// ClearCache removes all cached font data.
func (s *HbShaper) ClearCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.faces = make(map[*FontSource]*hbFont)
}

// RemoveSource removes cached data for a specific FontSource.
func (s *HbShaper) RemoveSource(source *FontSource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.faces, source)
}

// getOrCreateHbFont parses a FontSource's raw data once per source.
func (s *HbShaper) getOrCreateHbFont(source *FontSource) (*hbFont, error) {
	s.mu.RLock()
	if e, ok := s.faces[source]; ok {
		s.mu.RUnlock()
		return e, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.faces[source]; ok {
		return e, nil
	}
	e, err := buildHbFont(source)
	if err != nil {
		return nil, err
	}
	s.faces[source] = e
	return e, nil
}

// buildHbFont parses raw font bytes with typesetting (TTF/OTF, TTC fallback).
func buildHbFont(source *FontSource) (*hbFont, error) {
	if source == nil || len(source.data) == 0 {
		return nil, errors.New("text: empty font data")
	}
	rd := bytes.NewReader(source.data)
	f, err := font.ParseTTF(rd)
	if err != nil {
		// TTC/OTC fallback (collection).
		faces, ttcErr := font.ParseTTC(bytes.NewReader(source.data))
		if ttcErr != nil || len(faces) == 0 {
			return nil, err
		}
		idx := source.config.collectionIndex
		if idx < 0 || idx >= len(faces) {
			idx = 0
		}
		f = faces[idx]
	}
	return &hbFont{face: f, hbf: hb.NewFont(f)}, nil
}

// hbDirection maps the text.Direction to HarfBuzz direction.
func hbDirection(dir Direction) hb.Direction {
	switch dir {
	case DirectionRTL:
		return hb.RightToLeft
	case DirectionBTT:
		return hb.TopToBottom
	case DirectionTTB:
		return hb.BottomToTop
	default:
		return hb.LeftToRight
	}
}

// detectHBScript returns the HarfBuzz script for the dominant rune.
func detectHBScript(runes []rune) language.Script {
	for _, r := range runes {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			continue
		}
		return language.LookupScript(r)
	}
	latn, _ := language.ParseScript("Latn")
	return latn
}

// hbFeatures converts user + default features to HarfBuzz features.
// Default feature set matches OwnShaper's collectDesiredFeatures so the
// two backends agree on which OpenType features are active (M0 parity).
func hbFeatures(userFeatures []FontFeature) []hb.Feature {
	gsubTags, gposTags := collectDesiredFeatures(userFeatures)
	features := make([]hb.Feature, 0, len(gsubTags)+len(gposTags)+len(userFeatures))
	enable := func(t [4]byte) {
		features = append(features, hb.Feature{
			Tag:   font.Tag(binary.BigEndian.Uint32(t[:])),
			Value: 1,
			Start: hb.FeatureGlobalStart,
			End:   hb.FeatureGlobalEnd,
		})
	}
	for _, t := range gsubTags {
		enable(t)
	}
	for _, t := range gposTags {
		enable(t)
	}
	// User-disabled features: explicitly disable so HarfBuzz's own default
	// set is overridden (parity with OwnShaper's delete-from-default).
	for _, f := range userFeatures {
		if f.Value == 0 {
			features = append(features, hb.Feature{
				Tag:   font.Tag(binary.BigEndian.Uint32(f.Tag[:])),
				Value: 0,
				Start: hb.FeatureGlobalStart,
				End:   hb.FeatureGlobalEnd,
			})
		}
	}
	return features
}

// hbToShapedGlyphs converts a HarfBuzz buffer result to ShapedGlyph output.
// Position units are 1/64 px (26.6). HarfBuzz emits visual order for RTL.
func hbToShapedGlyphs(buf *hb.Buffer, runes []rune, size float64, tabClusters []int) []ShapedGlyph {
	if len(buf.Info) == 0 {
		return nil
	}
	tabSet := make(map[int]bool, len(tabClusters))
	for _, c := range tabClusters {
		tabSet[c] = true
	}

	out := make([]ShapedGlyph, 0, len(buf.Info))
	var x, y float64
	for i := range buf.Info {
		info := &buf.Info[i]
		pos := &buf.Pos[i]
		xAdv := float64(pos.XAdvance) / (1 << hbScaleBits)
		yAdv := float64(pos.YAdvance) / (1 << hbScaleBits)
		xOff := float64(pos.XOffset) / (1 << hbScaleBits)
		yOff := float64(pos.YOffset) / (1 << hbScaleBits)

		if tabSet[info.Cluster] {
			xAdv *= float64(globalTabWidth)
		}

		var cjk bool
		if info.Cluster >= 0 && info.Cluster < len(runes) {
			cjk = IsCJKRune(runes[info.Cluster])
		}

		out = append(out, ShapedGlyph{
			GID:      GlyphID(info.Glyph),
			Cluster:  info.Cluster,
			IsCJK:    cjk,
			X:        x + xOff,
			Y:        y + yOff,
			XAdvance: xAdv,
			YAdvance: yAdv,
		})
		x += xAdv
		y += yAdv
	}
	return out
}
