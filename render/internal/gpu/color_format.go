//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package gpu

import "github.com/energye/gpui/gpu/types"

// M5 HDR 全链 P1（evidence/M5/assessment-design.md §5）：管线颜色目标格式参数化。
//
// 渲染管线在创建时就把 ColorTargetState.Format 烘进管线，必须与 render pass 的
// 颜色附件一致。历史上这里全部硬编码 BGRA8Unorm——8bit 会话目标成立，但 float
// 面（RGBA16Float）无法复用同一批管线。
//
// 标准形态（Skia 的 program/pipeline 键含目标色彩类型、Impeller 的 pipeline
// descriptor 含 color attachment format）是按目标格式出管线变体。本文件的落法：
// 每个管线 owner 带一个 colorFormat 字段，零值表示历史 BGRA8（所以 8bit 路径
// 逐字节零变化），float 目标在创建管线前把它设成 RGBA16Float。
const defaultColorFormat = types.TextureFormatBGRA8Unorm

// colorFormatOr resolves a pipeline owner's color attachment format: the zero
// value (TextureFormatUndefined) keeps the historical BGRA8 session target.
func colorFormatOr(f types.TextureFormat) types.TextureFormat {
	if f == types.TextureFormatUndefined {
		return defaultColorFormat
	}
	return f
}
