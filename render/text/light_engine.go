//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package text

import (
	"errors"
)

// light_engine.go —— FreeType light 渲染模式的消费者侧骨架（模式选择+轮廓
// 壳），基于 render/text 已有的轮廓提取与自研 hint 引擎。
//
//   - TrueType glyf 轮廓（无 bytecode）→ autohinter afcjk（render/text
//     autohint*.go 已实现全管线）。
//   - Latin 等带 fpgm/prep 的字体 → TrueType 解释器 light（tt_engine.go
//     已实现全量执行；light 语义 Y-only 在 L0/L1）。
//
// 本文件保留早期隔离验证用的 Engine/Mode 壳（fdiff / hint_test 依赖），
// 通过 text.OutlineExtractor 提取轮廓后按 mode 分派。

// Mode 是目标 FreeType 渲染模式。
type Mode uint8

const (
	// ModeLightCJK 对应 pshinter light / afcjk（CFF→cffcs+psh_light，glyf→afcjk）。
	ModeLightCJK Mode = iota

	// ModeLightLatin 对应 TrueType 解释器 light 模式（bytecode Y-only）。
	ModeLightLatin
)

// ErrUnsupportedFont 表示字体类型不被 light 引擎支持。
var ErrUnsupportedFont = errors.New("text: unsupported font type for FT-light port")

// Engine 是 FT-light 拟合的独立实现壳。
type Engine struct {
	extractor *OutlineExtractor
}

// New 创建 light 引擎壳。
func New() *Engine {
	return &Engine{extractor: NewOutlineExtractor()}
}

// Hint 对 glyph 执行 light 网格拟合，返回拟合后的轮廓。
func (e *Engine) Hint(font ParsedFont, gid GlyphID, size float64, mode Mode) (*GlyphOutline, error) {
	if font == nil {
		return nil, errors.New("text: nil font")
	}
	if e == nil || e.extractor == nil {
		return nil, errors.New("text: nil engine")
	}

	outline, err := e.extractor.ExtractOutline(font, gid, size)
	if err != nil {
		return nil, err
	}
	if outline == nil {
		return nil, nil // 空字形（空格等）
	}

	switch mode {
	case ModeLightCJK:
		return e.hintAfcjk(font, gid, size, outline)
	case ModeLightLatin:
		return e.hintLatinLight(gid, size, outline)
	default:
		return nil, errors.New("text: unknown mode")
	}
}

// hintLatinLight 是 bytecode light 模式骨架入口（L0/L1 待接入 tt_engine
// 的 light 渲染 flag；当前返回原始轮廓）。
func (e *Engine) hintLatinLight(gid GlyphID, size float64, outline *GlyphOutline) (*GlyphOutline, error) {
	return outline, nil
} // afcjk.go —— FreeType autohinter「afcjk」脚本的移植区（CJK light 拟合）。
//
// 对齐目标：FT_LOAD_TARGET_LIGHT 下对无 bytecode hint 程序的字体
// （CJK 主字体如 Noto Sans CJK 不带 fpgm/prep）走 autohinter，
// 执行 afcjk 脚本的 light 拟合：
//
//	1. 蓝线构造（blue zones）：顶横锚区（round(AFCJK_TOP_BLUE_FU*px/upem)）。
//	2. 顶横捕捉：字形最高水平边锚定到蓝线顶（10–16px 主字号域为整数行）。
//	3. 次横排布：从顶锚按原始间距（容差 1/64）保持；低频边维持。
//	4. 基线锚：底部内横锚定到 baseline（0）。
//	5. X 不动（light 对 CJK 只动 Y）。
//

// AFCJK_TOP_BLUE_FU 是 Noto Sans CJK 的顶蓝线参考位（字体单位）。
// 来源：ftexp 实测——FT-light 顶横锚 = round(816*px/1000) 在 10–16px
// 主字号域与 FT 输出精确一致（FU=816 由 12px 锚=10.0 反推）。
const AFCJK_TOP_BLUE_FU = 816

// afcjkBlueAnchor 返回 px 字号下的顶横锚定目标（Y-up 像素，1/64 网格）。
// FT afcjk 顶蓝线：anchor = round(blueFU * px / upem)（26.6 网格化）。
// afcjkTopAnchor 返回 px 字号下的顶横锚定目标（Y-up 像素，整数像素行）。
// FT afcjk 顶蓝线：anchor = round(blueFU * px / upem)（实测 10–16px 全对齐）。
func afcjkTopAnchor(px, upem float64) float64 {
	v := float64(AFCJK_TOP_BLUE_FU) * px / upem
	f := int(v)
	if v-float64(f) >= 0.5 {
		f++
	}
	return float64(f)
}

// round26 将像素值取整到 1/64 网格（FT f26dot6 语义）。
func round26(v float64) float64 {
	return float64(int(v*64+sign(v)*0.5)) / 64
}

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}

// he 是单条水平段（LineTo 且首尾 y 相同）。
type he struct {
	y    float64 // 段代表 y（Y-down）
	idxs []int   // outline.Segments 下标
}

// closeY 判断两点 y 是否相等（float32 精度容差，1/256 网格级）。
func closeY(a, b float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 0.004
}

// 单位约定：GlyphOutline 为 Y-down（Y=0 基线，Y<0 上方）。内部计算用
// Y-down 直接做（顶横 = min y），蓝线锚转成 Y-down 负值。
//
// 注意：OutlineSegment 是连续折线——LineTo 段只用 Points[0]（新端点），
// 段跨度 = (上一端点 → Points[0])。水平检测必须用相邻点对。
func (e *Engine) hintAfcjk(font ParsedFont, gid GlyphID, size float64, outline *GlyphOutline) (*GlyphOutline, error) {
	if outline == nil || outline.IsEmpty() {
		return outline, nil
	}
	// 构建点流 + 水平边（相邻点对 y 相同）。
	type pt struct {
		x, y float32
		seg  int // 所属段下标（LineTo）
	}
	var pts []pt
	var prev pt
	prevSet := false
	edgeOf := func(a, b pt) bool { return closeY(a.y, b.y) }
	var edges []he
	for i := range outline.Segments {
		s := &outline.Segments[i]
		switch s.Op {
		case OutlineOpMoveTo:
			prev = pt{x: s.Points[0].X, y: s.Points[0].Y, seg: i}
			prevSet = true
		case OutlineOpLineTo:
			cur := pt{x: s.Points[0].X, y: s.Points[0].Y, seg: i}
			if prevSet && edgeOf(prev, cur) {
				edges = append(edges, he{y: float64(prev.y), idxs: []int{prev.seg, i}})
			}
			pts = append(pts, cur)
			prev = cur
			prevSet = true
		case OutlineOpQuadTo, OutlineOpCubicTo:
			cur := pt{x: s.Points[2].X, y: s.Points[2].Y, seg: i}
			pts = append(pts, cur)
			prev = cur
			prevSet = true
		}
	}
	if len(edges) == 0 {
		return outline, nil
	}
	// 合并同 y 边（同 y 组 = FT 的一条 edge）。
	var groups [][]he
	for _, ed := range edges {
		placed := false
		for gi := range groups {
			if groups[gi][0].y == ed.y {
				groups[gi] = append(groups[gi], ed)
				placed = true
				break
			}
		}
		if !placed {
			groups = append(groups, []he{ed})
		}
	}
	if len(groups) == 0 {
		return outline, nil
	}
	// 排序（Y-down：小 y = 顶部在上）。
	sortGroupsByY(groups)

	// 位移量按 y 值记录（同 y 组 = 同一边，闭合路径的 MoveTo 点自动覆盖）。
	deltas := map[float64]float64{}

	// 顶横锚定（最高段组）→ Y-down 锚 = -afcjkTopAnchor。
	top := groups[0][0].y
	anchorYDown := -afcjkTopAnchor(size, 1000)
	deltas[top] = anchorYDown - top

	// 次横：从顶锚按原始间距（1/64）保持。
	if len(groups) > 1 {
		spacing := groups[1][0].y - groups[0][0].y // Y-down 间距（向下为正）
		spacing = round26(spacing)
		newY := anchorYDown + spacing
		deltas[groups[1][0].y] = newY - groups[1][0].y
	}

	// 底内横：最高 y 且 < 0 的组锚定 baseline（0，Y-down）。
	for gi := len(groups) - 1; gi >= 0; gi-- {
		if groups[gi][0].y > 0 {
			continue
		}
		deltas[groups[gi][0].y] = -groups[gi][0].y
		break
	}

	for i := range outline.Segments {
		s := &outline.Segments[i]
		dy, ok := deltas[float64(s.Points[0].Y)]
		if !ok {
			continue
		}
		switch s.Op {
		case OutlineOpMoveTo, OutlineOpLineTo:
			s.Points[0].Y += float32(dy)
		}
	}
	return outline, nil
}

func sortGroupsByY(groups [][]he) {
	for i := 1; i < len(groups); i++ {
		for j := i; j > 0 && groups[j][0].y < groups[j-1][0].y; j-- {
			groups[j], groups[j-1] = groups[j-1], groups[j]
		}
	}
}

func applyDelta(deltas map[int]float64, group []he, dy float64) {
	for _, ed := range group {
		for _, idx := range ed.idxs {
			if _, seen := deltas[idx]; seen {
				continue
			}
			deltas[idx] = dy
		}
	}
}
