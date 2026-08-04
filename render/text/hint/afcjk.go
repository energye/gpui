package hint

import (
	"github.com/energye/gpui/render/text"
)

// afcjk.go —— FreeType autohinter「afcjk」脚本的移植区（CJK light 拟合）。
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
// 进度（docs/ENGINE_TEXT_HINT_LIGHT_PLAN.md §5）：
//	M0 ✔ 顶沿/基线锚 + 顶-次间距保持（本文件）。
//	M1 （横笔捕捉 stem 分组）→ 全横边网格排布。
//	M2 （轮廓点 Y 平移/伸缩传播）。
//	M3 （16.16 舍入顺序 & 边界字号逐条对齐）。

// AFCJK_TOP_BLUE_FU 是 Noto Sans CJK 的顶蓝线参考位（字体单位）。
// 来源：ftexp 实测——FT-light 顶横锚 = round(816*px/1000) 在 10–16px
// 主字号域与 FT 输出精确一致（FU=816 由 12px 锚=10.0 反推）。
// 其他 CJK 字体的该值不同，M3 加入字体蓝区提取后替换。
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

// hintAfcjk 是 afcjk 脚本的 M0 实现：
// 顶横锚定蓝线 + 次横间距保持（1/64）+ 基线下横锚 baseline。
//
// 单位约定：GlyphOutline 为 Y-down（Y=0 基线，Y<0 上方）。内部计算用
// Y-down 直接做（顶横 = min y），蓝线锚转成 Y-down 负值。
//
// 注意：OutlineSegment 是连续折线——LineTo 段只用 Points[0]（新端点），
// 段跨度 = (上一端点 → Points[0])。水平检测必须用相邻点对。
func (e *Engine) hintAfcjk(font text.ParsedFont, gid text.GlyphID, size float64, outline *text.GlyphOutline) (*text.GlyphOutline, error) {
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
		case text.OutlineOpMoveTo:
			prev = pt{x: s.Points[0].X, y: s.Points[0].Y, seg: i}
			prevSet = true
		case text.OutlineOpLineTo:
			cur := pt{x: s.Points[0].X, y: s.Points[0].Y, seg: i}
			if prevSet && edgeOf(prev, cur) {
				edges = append(edges, he{y: float64(prev.y), idxs: []int{prev.seg, i}})
			}
			pts = append(pts, cur)
			prev = cur
			prevSet = true
		case text.OutlineOpQuadTo, text.OutlineOpCubicTo:
			// 曲线段：三个点 (c1, c2, end)，水平检测只对直线段做（M0 范围）。
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

	// 应用到轮廓（仅 Y 方向；X 不动）。凡 Points[0] 的 y 命中位移即平移
	// （MoveTo 与 LineTo 的共享端点被同值覆盖；曲线端点 M1 处理）。
	for i := range outline.Segments {
		s := &outline.Segments[i]
		dy, ok := deltas[float64(s.Points[0].Y)]
		if !ok {
			continue
		}
		switch s.Op {
		case text.OutlineOpMoveTo, text.OutlineOpLineTo:
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
