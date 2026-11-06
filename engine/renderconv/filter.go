//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package renderconv

import (
	"errors"

	"github.com/energye/gpui/engine/tex"
	"github.com/energye/gpui/render"
)

// tex.Filter → 渲染层档位枚举的唯一转换路（P3.7 审计缺口 4，档 A：tex.Filter
// 为真源）。映射语义按 `docs/render_gpu_converge/evidence/P3.7/batch6/
// gap4-tex-filter-assessment.md` 拍板：
//
//   - 同轴映射：Near==Far==Nearest → InterpNearest；Near==Far==Linear →
//     InterpBilinear。渲染侧 mag/min 永远同档（GPU 无分离采样器，
//     image_pipeline 两族采样器描述符 MagFilter==MinFilter），双轴 Filter 只有
//     同轴组合有承载形。
//   - 混合轴 / Unknown Kind 具名拒绝：静默选一边或补默认都违反本包宪法
//     （「逐字段拷贝、不钳位、不给默认」）；「未选」要默认是调用方的职责
//     （先过 tex.Filter.Normalized 或用 DefaultFilter 预设）。
//   - Mip 原样透传，不归一：tex 的零值 Mip（MipUnknown）语义是「未选」，透传
//     即 render.MipmapNone ≡ 历史单层路（H16「零值即老路」钉）；归一到
//     MipNearest 是 tex.Normalized 的职责，转换不偷语义（批 4 登记「归一语义
//     与单档历史路不兼容」的本体）。
//   - MaxAniso：渲染层精灵/单图公开路都没有 aniso 承载字段（批 4「aniso 不进
//     sprite，不承诺不存在的能力」）。0 与 1 都是「关」，放行；>1 具名拒绝，
//     不静默吞开关——包括 nearest 组合（EffectiveImageSamplerKey 会在采样折叠
//     时屏蔽 aniso，但屏蔽是折叠的内部事实，不是转换层放行的理由）。
var (
	// ErrFilterMixedAxes reports a Near≠Far tex.Filter: render carries one
	// filter for both magnify and minify, so the pair has no lossless form.
	ErrFilterMixedAxes = errors.New("renderconv: tex.Filter Near/Far differ but render samples magnify and minify with one filter (no split sampler)")
	// ErrFilterUnknownKind reports an unset axis: "not chosen" maps to
	// nothing; normalize via tex (Normalized/SetFilter) before converting.
	ErrFilterUnknownKind = errors.New("renderconv: tex.Filter has an unset (Unknown) axis; normalize via tex.Filter.Normalized first (conversion invents no defaults)")
	// ErrFilterAnisoUnsupported reports MaxAniso>1: the render sprite/image
	// roads carry no anisotropy field, so the switch would be silently lost.
	ErrFilterAnisoUnsupported = errors.New("renderconv: tex.Filter MaxAniso>1 has no landing field on the render sprite/image roads (aniso would be silently dropped)")
)

// FilterToRender maps a tex.Filter to the sampling pair the render sprite and
// image roads carry (render.InterpolationMode + render.MipmapMode). It never
// normalizes and never invents defaults (package rules): unset axes and
// uncarryable switches come back as the named errors above, and Mip rides
// through untouched so a zero tex.Filter.Mip keeps the historic single-level
// road (render.MipmapNone). Picture evidence: ④ p37_mip_exit arm F and ⑥
// p37_scene panel E drive real draws through this conversion.
func FilterToRender(f tex.Filter) (render.InterpolationMode, render.MipmapMode, error) {
	if f.Near != tex.KindNearest && f.Near != tex.KindLinear {
		return 0, 0, ErrFilterUnknownKind
	}
	if f.Far != tex.KindNearest && f.Far != tex.KindLinear {
		return 0, 0, ErrFilterUnknownKind
	}
	if f.Near != f.Far {
		return 0, 0, ErrFilterMixedAxes
	}
	if f.MaxAniso > tex.AnisoMin {
		return 0, 0, ErrFilterAnisoUnsupported
	}
	if f.Near == tex.KindNearest {
		return render.InterpNearest, f.Mip, nil
	}
	return render.InterpBilinear, f.Mip, nil
}
