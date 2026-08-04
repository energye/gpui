package hint

import (
	"github.com/energye/gpui/render/text"
)

// latin_light.go —— TrueType 字节码解释器「light 模式」的移植区（拉丁 light 拟合）。
//
// 对齐目标：FT_LOAD_TARGET_LIGHT 下对带 bytecode hint 程序的字体
// （Latin 主字体：DejaVu Sans / Noto Sans 都带 fpgm/prep）走 TrueType
// 解释器，以 light 渲染模式执行 hint 程序：
//
//	- 只保留 Y 方向（纵轴）的网格调整；X 方向指令（IUP 后的 X 调整、
//	   MDAP/MDRP 的 X 对齐等）被 light 模式抑制。
//	- 结果等价于"只修横笔、不动竖笔"，正是浏览器中文/正文的清晰度来源。
//
// 现有实现：render/text/tt_engine.go 已实现 bytecode 全量执行（HintingFull），
// 本包需要它支持 light 语义（新增渲染模式 flag，抑制 X 方向指令）。
//
// 验证方式见 docs/ENGINE_TEXT_HINT_LIGHT_PLAN.md §4 逐字 diff 闭环。

// hintLatinLight 是 bytecode light 模式骨架入口。
//
// 当前阶段（骨架）：直接返回原始轮廓。实现两阶段：
//
//	L0 在 tt_engine 增加 light 渲染模式（解释器抑制 X 方向调整）。
//	L1 与 FT light 输出的逐字 diff：拉丁全集（ASCII+重音）归零。
func (e *Engine) hintLatinLight(gid text.GlyphID, size float64, outline *text.GlyphOutline) (*text.GlyphOutline, error) {
	// TODO(ft-light): L0/L1 移入 tt_engine light 模式，见文档 §5 进度表。
	return outline, nil
}

// latinLightControlValue 预留：light 模式下被抑制的指令分类。
// TODO(ft-light): tt_engine 按此分类抑制 X 方向 MDAP/MDRP/SHP/... 等指令。
type latinLightControlValue uint8

const (
	lvXAdjust latinLightControlValue = iota // 需抑制的 X 方向调整
	lvYAdjust                               // 保留的 Y 方向调整
)
