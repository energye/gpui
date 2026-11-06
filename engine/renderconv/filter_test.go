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
	"testing"

	"github.com/energye/gpui/engine/tex"
	"github.com/energye/gpui/render"
)

// 缺口 4 红测（2026-11-06）：tex.Filter → 渲染层两枚档位枚举的唯一转换路。
// 映射语义按 gap4-tex-filter-assessment.md 拍板口径钉死：
//   - 同轴 nearest/nearest → InterpNearest，同轴 linear/linear → InterpBilinear；
//   - 混合轴 / Unknown Kind / aniso>1 具名拒绝（宪法「不给默认」+ 不吞开关）；
//   - Mip 原样透传不归一（tex 零值 Mip = render.MipmapNone ≡ 历史单层路，H16 口径）。
// C 证据（像素）不适用：转换不画东西，画面证据在 ④/⑥ 真窗 tex 驱动面板。

func TestFilterToRenderCarriesSameAxisFilters(t *testing.T) {
	cases := []struct {
		name  string
		in    tex.Filter
		wantF render.InterpolationMode
		wantM render.MipmapMode
	}{
		{"nearest_axes", tex.Filter{Near: tex.KindNearest, Far: tex.KindNearest, Mip: tex.MipNearest, MaxAniso: 1},
			render.InterpNearest, render.MipmapNearest},
		{"nearest_axes_linear_mip_rides", tex.Filter{Near: tex.KindNearest, Far: tex.KindNearest, Mip: tex.MipLinear, MaxAniso: 1},
			render.InterpNearest, render.MipmapLinear},
		{"default_filter", tex.DefaultFilter(),
			render.InterpBilinear, render.MipmapNearest},
		{"linear_axes_trilinear", tex.Filter{Near: tex.KindLinear, Far: tex.KindLinear, Mip: tex.MipLinear, MaxAniso: 1},
			render.InterpBilinear, render.MipmapLinear},
		// 零值 Mip 透传：tex「未选」= render.MipmapNone = 历史单层路（归一是
		// Normalized 的职责，转换不偷语义）。aniso 0 与 1 同为「关」。
		{"zero_mip_passes_through_as_none", tex.Filter{Near: tex.KindNearest, Far: tex.KindNearest},
			render.InterpNearest, render.MipmapNone},
		{"aniso_zero_means_off", tex.Filter{Near: tex.KindLinear, Far: tex.KindLinear, Mip: tex.MipNearest, MaxAniso: 0},
			render.InterpBilinear, render.MipmapNearest},
	}
	for _, c := range cases {
		gotF, gotM, err := FilterToRender(c.in)
		if err != nil {
			t.Errorf("%s: FilterToRender(%+v) 拒绝了合法输入: %v", c.name, c.in, err)
			continue
		}
		if gotF != c.wantF || gotM != c.wantM {
			t.Errorf("%s: FilterToRender(%+v) = (%v, %v), want (%v, %v)",
				c.name, c.in, gotF, gotM, c.wantF, c.wantM)
		}
	}
}

func TestFilterToRenderRefusesUncarryable(t *testing.T) {
	cases := []struct {
		name string
		in   tex.Filter
		want error
	}{
		{"mixed_axes_near_nearest_far_linear",
			tex.Filter{Near: tex.KindNearest, Far: tex.KindLinear, Mip: tex.MipNearest, MaxAniso: 1},
			ErrFilterMixedAxes},
		{"mixed_axes_near_linear_far_nearest",
			tex.Filter{Near: tex.KindLinear, Far: tex.KindNearest, Mip: tex.MipNearest, MaxAniso: 1},
			ErrFilterMixedAxes},
		{"unknown_near_axis",
			tex.Filter{Far: tex.KindLinear, Mip: tex.MipNearest, MaxAniso: 1},
			ErrFilterUnknownKind},
		{"unknown_far_axis",
			tex.Filter{Near: tex.KindLinear, Mip: tex.MipNearest, MaxAniso: 1},
			ErrFilterUnknownKind},
		{"zero_value_filter_is_unset_not_default", tex.Filter{}, ErrFilterUnknownKind},
		{"aniso_above_one_has_no_sprite_landing", tex.RealisticFilter(), ErrFilterAnisoUnsupported},
		{"aniso_above_one_on_nearest",
			tex.Filter{Near: tex.KindNearest, Far: tex.KindNearest, Mip: tex.MipNearest, MaxAniso: 2},
			ErrFilterAnisoUnsupported},
	}
	for _, c := range cases {
		gotF, gotM, err := FilterToRender(c.in)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: FilterToRender(%+v) err = %v, want %v（具名拒绝）", c.name, c.in, err, c.want)
		}
		if err == nil {
			t.Errorf("%s: FilterToRender(%+v) 放行了不可携带输入 → (%v, %v)", c.name, c.in, gotF, gotM)
		}
	}
}

// 拒绝必须在折叠进采样器之前发生（fold 会把 nearest 的 mip/aniso 屏蔽掉，
// 转换层不借 fold 的屏蔽装作无事）。
func TestFilterToRenderRefusalPrecedesSamplerMasking(t *testing.T) {
	// nearest 路在 EffectiveImageSamplerKey 里屏蔽 mip/aniso，但转换层必须
	// 仍然拒绝 aniso>1——屏蔽是采样折叠的内部事实，不是放行理由。
	f := tex.Filter{Near: tex.KindNearest, Far: tex.KindNearest, Mip: tex.MipLinear, MaxAniso: 8}
	if _, _, err := FilterToRender(f); !errors.Is(err, ErrFilterAnisoUnsupported) {
		t.Errorf("nearest+aniso8: err = %v, want ErrFilterAnisoUnsupported（不许借屏蔽放行）", err)
	}
}
