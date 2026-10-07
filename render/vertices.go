//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"errors"
)

//
// 一句话：显卡对三角逐像素插预乘颜色（Gouraud），
// CPU 必须按同样重心权重逐像素插，不许再取平均填纯色；
// 两边只差抗锯齿（当前网格管线 SkipAA，两边都是硬边）。
//
// 约定：
//   - 老 DrawVertices/DrawMesh 签名不动；纯色（无逐点色）仍走原 Fill 抗锯齿路。
//   - 新 DrawVerticesEx/DrawMeshEx 带显式校验加降级标记：
//     空 mesh 跳过（Skipped），非有限点与越界索引返回哨兵错，
//     CPU 真渐变时 Degraded=true（仅差抗锯齿），GPU 时 Degraded=false。
//   - CPU 采样与显卡一致：预乘（R*A/G*A/B*A/A）线性插，
//     再乘夹子/遮罩覆盖做源覆盖混合；混合恒按 Normal（与显卡凸包管线一致）。
//   - GOGPU_RENDER_MODE=cpu 强制走 CPU 真渐变，供 C 两边齐离屏对比取 CPU 真值。

// R2 哨兵错与降级标记：调用方用 errors.Is 判定，不许静默画错。
var (
	// ErrVertsNonFinite 非有限顶点（NaN/Inf）。
	ErrVertsNonFinite = errors.New("render: non-finite vertex")
	// ErrVertsBadIndex 索引越界（DrawMeshEx）。
	ErrVertsBadIndex = errors.New("render: vertex index out of range")
)

// VertCPUFallbackReason 是 CPU 真渐变回退的降级标记（与 RenderPathStats 对齐）。
// CPU 与 GPU 只差抗锯齿时仍记这一条，调用方凭它判定降级。
const VertCPUFallbackReason = "verts:DrawVertices"

// VertDrawOptions 是 DrawVerticesEx/DrawMeshEx 的选项。
type VertDrawOptions struct{}

// VertDrawResult 是 Ex 新函数的降级标记与诊断。
type VertDrawResult struct {
	// Degraded 为 true 表示走了 CPU 真渐变（与 GPU 只差抗锯齿）。
	Degraded bool
	// Reason 为降级原因（Degraded 时为 VertCPUFallbackReason，否则为空）。
	Reason string
	// Skipped 为 true 表示空 mesh/全退化跳过，什么都没画。
	Skipped bool
	// Triangles 为实际光栅化的三角数（D/E 诊断用）。
	Triangles int
}

// VertexMode selects how DrawVertices interprets the position list (V.01).
type VertexMode int

const (
	// VertexModeTriangles groups positions as independent triangles (0,1,2), (3,4,5), ...
	VertexModeTriangles VertexMode = iota
	// VertexModeTriangleFan fans triangles from the first vertex: (0,i,i+1).
	VertexModeTriangleFan
)

// AtlasSprite describes one sub-rect of an atlas image drawn to a destination rect (V.02).
// Source coordinates are in image pixels; destination is in user space (CTM applied).
//
// 老 DrawAtlas 签名不动，只读老字段；新字段零值即老路。
//   - Rot 弧度，与 Rotate 一致（Y 朝下时正角顺时针），绕轴心转。
//   - FlipX/FlipY 先于旋转绕轴心镜像（几何翻转，UV 不动）。
//   - PivotX/PivotY 为 Dst 原点起的偏移（用户单位），零值即左上角。
//   - Tint 零结构体即白不透明（不染色）；其余值按直射相乘（含 A）。
//   - Filter 零值即 Bilinear（老路）；单图选 Nearest/Bilinear/Bicubic。
type AtlasSprite struct {
	SrcX, SrcY, SrcW, SrcH float64
	DstX, DstY, DstW, DstH float64
	// Opacity is 0..1; values <= 0 default to 1.
	Opacity float64
	// Rot rotates the Dst rect about the pivot.
	Rot float64
	// FlipX/FlipY mirror the Dst rect about the pivot before Rot.
	FlipX, FlipY bool
	// PivotX/PivotY is the pivot offset from (DstX,DstY) in Dst units.
	PivotX, PivotY float64
	// Tint multiplies straight source texels; zero struct means white.
	Tint RGBA
	// Filter selects per-sprite sampling; zero means Bilinear.
	Filter InterpolationMode
}

// R4 哨兵错与降级标记：调用方用 errors.Is 判定，不许静默画错。
var (
	// ErrAtlasNonFinite 非有限图集精灵（NaN/Inf，含 Tint）。
	ErrAtlasNonFinite = errors.New("render: non-finite atlas sprite")
	// ErrAtlasUnsupportedFilter 不支持的单图过滤。
	ErrAtlasUnsupportedFilter = errors.New("render: unsupported atlas filter (R4 only Nearest/Bilinear/Bicubic)")
)

// AtlasCPUFallbackReason 是图集 CPU 回退的降级标记（与 RenderPathStats 对齐）。
// tint 染色暂无显卡着色器，整批走 CPU 真采样时记这一条。
const AtlasCPUFallbackReason = "verts:DrawAtlas"

// AtlasDrawOptions 是 DrawAtlasEx 的选项。
type AtlasDrawOptions struct{}

// AtlasDrawResult 是 Ex 新函数的降级标记与诊断。
type AtlasDrawResult struct {
	// Degraded 为 true 表示走了 CPU 真采样（含 tint 整批回退）。
	Degraded bool
	// Reason 为降级原因（Degraded 时为 AtlasCPUFallbackReason，否则为空）。
	Reason string
	// Skipped 为 true 表示空批或全跳过，什么都没画。
	Skipped bool
	// Drawn 为实际提交的精灵数（D/E 诊断用）。
	Drawn int
}
