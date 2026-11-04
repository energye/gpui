//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// 一句话：按提交序画一批带深度的精灵，画家算法盖对，排序不归渲染层。
//
// 口径（P3.7 批 2 步 2c 收口分层越界）：
//   - Depth 是随数据走的一张牌：越大越远，与 camera.Projector.DepthToScale 同口径；
//     render 只认**提交序**（远先近后 = 调用方排好再交），自己不再排、也不再提供排序口。
//     排序判据在 engine：camera.YSortLess（z 桶 → 脚位键 → 容器序号）与 sprite.DepthSort。
//   - 输入片不改；同序即同画，不闪。
//
// 保留面（数据格式与绘制能力随迁不裁）：
//   - DepthSprite / DepthDrawResult / ErrDepthNonFinite
//   - NewDepthSprite / (*Context).DrawDepthSprites
//
// 坏路：空批跳过（Skipped），非有限 Depth/精灵返回 ErrDepthNonFinite
// 且什么都不画，未知过滤透传 ErrAtlasUnsupportedFilter。
package render

import (
	"errors"
	"math"
)

// ErrDepthNonFinite 非有限深度精灵（Depth 或内嵌 AtlasSprite 含 NaN/Inf）。
var ErrDepthNonFinite = errors.New("render: non-finite depth sprite")

// DepthCPUFallbackReason 是深度分支 CPU 回退的降级标记，与 Atlas 路对齐。
// 底层走 DrawAtlasEx，染色等仍记 verts:DrawAtlas。
const DepthCPUFallbackReason = AtlasCPUFallbackReason

// DepthSprite 是一张带深度的图集小块。
// Sprite 复用 AtlasSprite 全字段，
// Depth 与 camera.Projector 同口径：越大越远；Name 只做调试键。
// 深度只随数据走，绘制顺序由提交序决定（render 不排序，见本文件头）。
type DepthSprite struct {
	Sprite AtlasSprite
	Depth  float64
	Name   string
}

// DepthDrawResult 是深度分支的降级标记与诊断。
type DepthDrawResult struct {
	// Degraded 为 true 表示走了 CPU 真采样（与 Atlas 路一致）。
	Degraded bool
	// Reason 为降级原因（Degraded 时为 DepthCPUFallbackReason，否则为空）。
	Reason string
	// Skipped 为 true 表示空批或全跳过，什么都没画。
	Skipped bool
	// Drawn 为实际提交的精灵数（D/E 诊断用）。
	Drawn int
}

// depthFinite 报告深度有限。
func depthFinite(d float64) bool {
	return !math.IsNaN(d) && !math.IsInf(d, 0)
}

// NewDepthSprite 建一个深度精灵，Depth 非有限或内嵌精灵非有限返回哨兵错。
// 透视精灵（Sprite.Perspective）同时校验四角/权重/采样组合（P3.7 批 1c）。
func NewDepthSprite(name string, sp AtlasSprite, depth float64) (DepthSprite, error) {
	if !depthFinite(depth) || !isFiniteAtlasSprite(sp) {
		return DepthSprite{}, ErrDepthNonFinite
	}
	filt, err := normalizeAtlasFilter(sp.Filter)
	if err != nil {
		return DepthSprite{}, err
	}
	if sp.Perspective {
		if err := atlasPerspValidate(sp, filt); err != nil {
			return DepthSprite{}, err
		}
	}
	return DepthSprite{Sprite: sp, Depth: depth, Name: name}, nil
}

// DrawDepthSprites 按提交序画一批深度精灵（先提交的先画 = 远先近后由调用方排好）。
// 空批或全跳过返回 Skipped；非有限与未知过滤返回哨兵错且什么都不画；输入片不改。
func (c *Context) DrawDepthSprites(img *ImageBuf, sprites []DepthSprite) (DepthDrawResult, error) {
	if c == nil || img == nil || len(sprites) == 0 {
		return DepthDrawResult{Skipped: true}, nil
	}
	for i := range sprites {
		if !depthFinite(sprites[i].Depth) || !isFiniteAtlasSprite(sprites[i].Sprite) {
			return DepthDrawResult{}, ErrDepthNonFinite
		}
		if _, err := normalizeAtlasFilter(sprites[i].Sprite.Filter); err != nil {
			return DepthDrawResult{}, err
		}
	}
	flat := make([]AtlasSprite, len(sprites))
	for i := range sprites {
		flat[i] = sprites[i].Sprite
	}
	res, err := c.DrawAtlasEx(img, flat, AtlasDrawOptions{})
	if err != nil {
		return DepthDrawResult{}, err
	}
	return DepthDrawResult{
		Degraded: res.Degraded,
		Reason:   res.Reason,
		Skipped:  res.Skipped,
		Drawn:    res.Drawn,
	}, nil
}
