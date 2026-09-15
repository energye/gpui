// R5 深度分支冻结（S35/W5，1.2 的 render 底，前置 S30 R4，状态落 1.2 行）。
//
// 一句话：远先近后，画家算法盖对，老接口一个不动。
// 本文件是 1.2 render 底的新分支，与 game/sprite/ysort.go 本体无关
// （那是 S41 的事，本分支不碰它，只对齐口径）。
//
// 口径（与 game/camera/project.go 同口径）：
//   - Depth 越大越远（与 Projector.DepthToScale 一致：越大 scale 越小越远），
//     Sort 按 Depth 降序（远先近后），近的后画盖住远的。
//   - 同深用稳定排序保输入序，不闪；输入片不改，返回新片。
//   - DepthTest 开关只在本分支生效：true 强制远先近后，false 保输入序
//    （老味，不排序）。硬件 DepthCompare=GreaterEqual 仍只给裁剪用
//    （depth_clip.wgsl 等），游戏深度硬件比较等 S41 接线，本分支不碰管线。
//
// 新函数新分支，老接口不动：
//   - DepthSprite / DepthDrawOptions / DepthDrawResult / ErrDepthNonFinite
//   - NewDepthSprite / SetDepth / SortDepthSprites / IsDepthSorted
//   - (*Context).DrawDepthSprites（内部调 DrawAtlasEx，不改老签名）
//
// 坏路：空批跳过（Skipped），非有限 Depth/精灵返回 ErrDepthNonFinite
// 且什么都不画，未知过滤透传 ErrAtlasUnsupportedFilter。
// 窗口意图：game_sprite--case=depth 随 P2 建，S35 只留离屏对比。
package render

import (
	"errors"
	"math"
	"sort"
)

// ErrDepthNonFinite 非有限深度精灵（Depth 或内嵌 AtlasSprite 含 NaN/Inf）。
var ErrDepthNonFinite = errors.New("render: non-finite depth sprite")

// DepthCPUFallbackReason 是深度分支 CPU 回退的降级标记，与 Atlas 路对齐。
// 底层走 DrawAtlasEx，染色等仍记 verts:DrawAtlas。
const DepthCPUFallbackReason = AtlasCPUFallbackReason

// DepthSprite 是一张带深度的图集小块（R5 新分支）。
// Sprite 复用 AtlasSprite 全字段（R4 rot/flip/tint/filter 透传），
// Depth 与 camera.Projector 同口径：越大越远；Name 只做调试键。
type DepthSprite struct {
	Sprite AtlasSprite
	Depth  float64
	Name   string
}

// DepthDrawOptions 是 DrawDepthSprites 的选项（R5 预留，零值即默认）。
type DepthDrawOptions struct {
	// DepthTest 为 true 强制远先近后（画家算法）；false 保输入序（老味）。
	DepthTest bool
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
	// Sorted 为 true 表示本次按 DepthTest 开关做了远先近后排序。
	Sorted bool
}

// depthFinite 报告深度有限。
func depthFinite(d float64) bool {
	return !math.IsNaN(d) && !math.IsInf(d, 0)
}

// NewDepthSprite 建一个深度精灵，Depth 非有限或内嵌精灵非有限返回哨兵错。
func NewDepthSprite(name string, sp AtlasSprite, depth float64) (DepthSprite, error) {
	if !depthFinite(depth) || !isFiniteAtlasSprite(sp) {
		return DepthSprite{}, ErrDepthNonFinite
	}
	if _, err := normalizeAtlasFilter(sp.Filter); err != nil {
		return DepthSprite{}, err
	}
	return DepthSprite{Sprite: sp, Depth: depth, Name: name}, nil
}

// SetDepth 改深度值，非法值返回哨兵错且原值不动。
func SetDepth(s *DepthSprite, depth float64) error {
	if s == nil {
		return ErrDepthNonFinite
	}
	if !depthFinite(depth) {
		return ErrDepthNonFinite
	}
	s.Depth = depth
	return nil
}

// SortDepthSprites 按 Depth 降序（远先近后）稳定排序，输入不动，返回新片。
// 同深保输入序不闪；任何非有限 Depth/精灵返回哨兵错且结果为 nil。
func SortDepthSprites(in []DepthSprite) ([]DepthSprite, error) {
	for i := range in {
		if !depthFinite(in[i].Depth) || !isFiniteAtlasSprite(in[i].Sprite) {
			return nil, ErrDepthNonFinite
		}
		if _, err := normalizeAtlasFilter(in[i].Sprite.Filter); err != nil {
			return nil, err
		}
	}
	out := make([]DepthSprite, len(in))
	copy(out, in)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Depth > out[j].Depth })
	return out, nil
}

// IsDepthSorted 报告是否已是远先近后序，空与单片算有序，
// 非有限一律报 false（fail closed），不 panic。
func IsDepthSorted(in []DepthSprite) bool {
	for i := range in {
		if !depthFinite(in[i].Depth) || !isFiniteAtlasSprite(in[i].Sprite) {
			return false
		}
		if i > 0 && in[i].Depth > in[i-1].Depth {
			return false
		}
	}
	return true
}

// DrawDepthSprites 按深度开关画一批（R5 新函数，老路不动）。
// DepthTest=true 先稳定降序再调 DrawAtlasEx（远先画近后盖）；
// false 保输入序直接调。空批或全跳过返回 Skipped；
// 非有限与未知过滤返回哨兵错且什么都不画；输入片不改。
func (c *Context) DrawDepthSprites(img *ImageBuf, sprites []DepthSprite, opts DepthDrawOptions) (DepthDrawResult, error) {
	if c == nil || img == nil || len(sprites) == 0 {
		return DepthDrawResult{Skipped: true, Sorted: opts.DepthTest}, nil
	}
	for i := range sprites {
		if !depthFinite(sprites[i].Depth) || !isFiniteAtlasSprite(sprites[i].Sprite) {
			return DepthDrawResult{}, ErrDepthNonFinite
		}
		if _, err := normalizeAtlasFilter(sprites[i].Sprite.Filter); err != nil {
			return DepthDrawResult{}, err
		}
	}
	ordered := sprites
	sorted := false
	if opts.DepthTest {
		sortedOut, err := SortDepthSprites(sprites)
		if err != nil {
			return DepthDrawResult{}, err
		}
		ordered = sortedOut
		sorted = true
	}
	flat := make([]AtlasSprite, len(ordered))
	for i := range ordered {
		flat[i] = ordered[i].Sprite
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
		Sorted:   sorted,
	}, nil
}
