// Package hint 是 FreeType light 渲染模式的独立移植库（R21 hinting 对齐）。
//
// 目标：让自研文字渲染的网格拟合（grid-fit）输出与 FreeType
// FT_LOAD_TARGET_LIGHT 逐像素一致，从而与 Linux 桌面浏览器（Chrome 默认
// light hint）的中文/拉丁观感完全相同。
//
// 现状（骨架阶段）：Hint 返回未拟合轮廓（等同 FT_LOAD_NO_HINTING 轮廓 +
// 无网格调整），规则逐条移植中，进度见 docs/ENGINE_TEXT_HINT_LIGHT_PLAN.md。
//
// 设计约束：
//   - 与现有渲染路径（glyph_mask_engine / draw.go）完全解耦：本包只做
//     "轮廓 → 拟合轮廓"的纯变换，不碰窗口/GPU/缓存。
//   - 不依赖系统 libfreetype（运行时零依赖目标）。
//   - 验证闭环：render/text/hint/fdiff 逐字 diff（对照系统 FreeType 输出）。
package hint

import (
	"errors"

	"github.com/energye/gpui/render/text"
)

// Mode 是目标 FreeType 渲染模式。
//
// FreeType 的 FT_LOAD_TARGET_LIGHT 对不同字体走不同引擎：
//   - CJK 等无 bytecode hint 程序的字体 → autohinter 的 afcjk 脚本，
//     只做纵轴（Y 方向）网格拟合；
//   - Latin 等带 fpgm/prep 指令的字体 → TrueType 解释器以 light 模式运行
//     （只保留 Y 方向调整，跳过 X 方向指令）。
type Mode uint8

const (
	// ModeLightCJK 对应 FreeType autohinter afcjk（Y-only grid-fit）。
	ModeLightCJK Mode = iota

	// ModeLightLatin 对应 TrueType 解释器 light 模式（bytecode Y-only）。
	ModeLightLatin
)

// ErrUnsupportedFont 表示字体类型不被移植引擎支持。
var ErrUnsupportedFont = errors.New("hint: unsupported font type for FT-light port")

// Engine 是 FT-light 拟合的独立实现。
//
// 骨架阶段 Hint 返回原始（未拟合）轮廓；规则逐步实现后，
// 每个 Mode 的分派函数负责对应引擎（afcjk.go / latin_light.go）。
type Engine struct {
	extractor *text.OutlineExtractor
}

// New 创建 FT-light 移植引擎。
func New() *Engine {
	return &Engine{extractor: text.NewOutlineExtractor()}
}

// Hint 对 glyph 执行 light 网格拟合，返回拟合后的轮廓。
//
// size 是 ppem（每 em 像素数）。mode 选择拟合引擎。
// 骨架阶段：返回未拟合轮廓（= FT_LOAD_NO_HINTING 轮廓），
// 规则实现后在此分派到 afcjk / latin_light 的拟合流程。
func (e *Engine) Hint(font text.ParsedFont, gid text.GlyphID, size float64, mode Mode) (*text.GlyphOutline, error) {
	if font == nil {
		return nil, errors.New("hint: nil font")
	}
	if e == nil || e.extractor == nil {
		return nil, errors.New("hint: nil engine")
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
		// TODO(ft-light): tt_engine light 模式移植，见 latin_light.go
		return outline, nil
	default:
		return nil, errors.New("hint: unknown mode")
	}
}
