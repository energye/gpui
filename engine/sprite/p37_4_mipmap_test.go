package sprite

// P3.7 批 4：精灵的 mipmap 档位必须能走到渲染命令上。
//
// 判据正本 `evidence/P3.7/design.md` §5 与 `assessment.md` §5，两处断点实测在：
//   - `AtlasSprite` 根本没有档位承载（P3.6 批 2 给 render 加了 `AtlasSprite.Mipmap`，
//     引擎侧一个字段都没有 ⇒ 玩家/场景设不了）；
//   - `AtlasSprite.ToRender()` 逐字段转 render 时从不填 `Mipmap`。
// 形状按拍板：引擎不再自造第三套过滤枚举——`Filter` 用 `render.InterpolationMode`
// （零值未指定→render 归一 Bilinear，老路逐位不变），档位用 `render.MipmapMode`
// （零值 `MipmapNone` = 单档历史路逐位不变）。bicubic 这条活能力 1:1 保留。

import (
	"testing"

	"github.com/energye/gpui/engine/core"
	"github.com/energye/gpui/render"
)

func mipFixture(t *testing.T) AtlasSprite {
	t.Helper()
	s, err := NewAtlasSprite(core.AssetID("tex/atlas"),
		core.NewRect(0, 0, 8, 8), core.NewRect(2, 3, 6, 6),
		1, 0, core.V2(3, 3), core.Color{}, FilterUnset, false, false)
	if err != nil {
		t.Fatalf("NewAtlasSprite: %v", err)
	}
	return s
}

// 钉 1：档位有承载，且逐字段转到渲染命令上（今天编译不过 = 断点复现）。
func TestP374AtlasSpriteCarriesMipmapToRender(t *testing.T) {
	for _, tc := range []struct {
		name string
		mip  render.MipmapMode
	}{
		{"none", render.MipmapNone},
		{"nearest", render.MipmapNearest},
		{"linear", render.MipmapLinear},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := mipFixture(t)
			s.Mipmap = tc.mip
			got, err := s.ToRender()
			if err != nil {
				t.Fatalf("ToRender: %v", err)
			}
			if got.Mipmap != tc.mip {
				t.Fatalf("档位没传到渲染命令：%d want %d", got.Mipmap, tc.mip)
			}
		})
	}
}

// 钉 2：零值必须逐位保持历史路——不指定档位 = MipmapNone（单档），不指定过滤 = Bilinear。
// 这是「存量画面不许变」的边界钉：默认值一旦映射成 Nearest/Linear，所有 sprite 的采样键都会变。
func TestP374ZeroValuesKeepHistoricalRoad(t *testing.T) {
	s := mipFixture(t)
	if s.Filter != FilterUnset || s.Mipmap != render.MipmapNone {
		t.Fatalf(" fixture 零值不齐：filter=%d mip=%d", s.Filter, s.Mipmap)
	}
	got, err := s.ToRender()
	if err != nil {
		t.Fatalf("ToRender: %v", err)
	}
	if got.Mipmap != render.MipmapNone {
		t.Fatalf("零值档位被改了：%d want MipmapNone(0)——所有存量精灵的采样键都会漂", got.Mipmap)
	}
	// 零值必须原样透传：改造前 AtlasFilterDefault 也是把 0 交给渲染层，由 render 自己
	// 归一成 Bilinear（老路）。引擎提前改判成 2 会动到渲染层的归一判据，等于换路。
	if got.Filter != FilterUnset {
		t.Fatalf("零值过滤被引擎提前改判：%d want %d（透传给 render 归一）", got.Filter, FilterUnset)
	}
	// 显式 Bilinear 与零值是两个不同值（前者 2、后者 0），各自都得原样过去。
	s2 := mipFixture(t)
	s2.Filter = render.InterpBilinear
	got2, err := s2.ToRender()
	if err != nil {
		t.Fatalf("ToRender bilinear: %v", err)
	}
	if got2.Filter != render.InterpBilinear {
		t.Fatalf("显式 Bilinear 被改了：%d", got2.Filter)
	}
}

// 钉 3：bicubic 这条活能力 1:1 保留（有专档逐位断言，不许随枚举合一被裁掉）。
func TestP374BicubicAxisSurvives(t *testing.T) {
	s := mipFixture(t)
	s.Filter = render.InterpBicubic
	s.Mipmap = render.MipmapLinear
	got, err := s.ToRender()
	if err != nil {
		t.Fatalf("ToRender bicubic: %v", err)
	}
	if got.Filter != render.InterpBicubic || got.Mipmap != render.MipmapLinear {
		t.Fatalf("两轴没各自到位：filter=%d mip=%d", got.Filter, got.Mipmap)
	}
}

// 钉 4：非法档位与非法过滤一律具名拒绝，不猜默认值（与旧 AtlasFilter(99) 同纪律）。
func TestP374InvalidFilterAndMipmapRejected(t *testing.T) {
	bad := mipFixture(t)
	bad.Filter = render.InterpolationMode(99)
	if _, err := bad.ToRender(); err == nil {
		t.Fatal("非法过滤值被放过了")
	}
	bad2 := mipFixture(t)
	bad2.Mipmap = render.MipmapMode(7)
	if _, err := bad2.ToRender(); err == nil {
		t.Fatal("非法档位值被放过了")
	}
	if _, err := NewAtlasSprite(core.AssetID("tex/atlas"),
		core.NewRect(0, 0, 8, 8), core.NewRect(2, 3, 6, 6),
		1, 0, core.V2(3, 3), core.Color{}, render.InterpolationMode(99), false, false); err == nil {
		t.Fatal("构造口放过非法过滤值（旧 AtlasFilter(99) 同纪律）")
	}
}

// 钉 5：字符串词汇随迁不裁剪（存档/测试数据里的 default/nearest/bilinear/bicubic 仍可用），
// 并新增档位词 none/nearest/linear；非法词一律报错。
func TestP374FilterVocabularyKept(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want render.InterpolationMode
	}{{"", 0}, {"default", 0}, {"nearest", render.InterpNearest},
		{"bilinear", render.InterpBilinear}, {"bicubic", render.InterpBicubic}} {
		got, err := ParseFilter(tc.in)
		if err != nil {
			t.Fatalf("ParseFilter(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("ParseFilter(%q) = %d want %d", tc.in, got, tc.want)
		}
	}
	if _, err := ParseFilter("sharp"); err == nil {
		t.Fatal("非法过滤词被放过")
	}
	for _, tc := range []struct {
		in   string
		want render.MipmapMode
	}{{"", render.MipmapNone}, {"none", render.MipmapNone},
		{"nearest", render.MipmapNearest}, {"linear", render.MipmapLinear}} {
		got, err := ParseMipmap(tc.in)
		if err != nil {
			t.Fatalf("ParseMipmap(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("ParseMipmap(%q) = %d want %d", tc.in, got, tc.want)
		}
	}
	if _, err := ParseMipmap("trilinear"); err == nil {
		t.Fatal("非法档位词被放过")
	}
	if FilterName(render.InterpBicubic) != "bicubic" || MipmapName(render.MipmapLinear) != "linear" {
		t.Fatal("稳定日志名不齐（存档与门禁对比要用）")
	}
}
