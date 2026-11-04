package sprite_test

// P3.7 批 2 步 2b：sprite 两套排序副本改委托 camera 的逐位对拍钉。
//
// 判据来源：testdata/p37_zsort_parity.json（scripts/gen_p37_zsort_parity.py 生成），
// 里面每条 want_order 都是「改委托之前」的旧判据离线复算结果
//（flat: layer 升 → feetY 升 → 输入稳定序；deep: depth 降 → layer → feetY → 输入稳定序）。
// 委托必须是"动实现不动行为"，所以这里要求逐元素相等；不等就是行为变了，停下来重拍。
// 另两枚钉管"真委托"：容差判等与带→z 桶饱和这两条只有 camera 才有的行为，
// 必须在 sprite 的输出口上看得见（把公式重新抄回 sprite 就红）。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/engine/sprite"
)

type parityItem struct {
	Name  string  `json:"name"`
	Depth float64 `json:"depth"`
	Layer int     `json:"layer"`
	FeetY float64 `json:"feetY"`
}

type parityCase struct {
	Name      string       `json:"name"`
	Mode      string       `json:"mode"`
	Items     []parityItem `json:"items"`
	WantOrder []string     `json:"want_order"`
}

type parityFile struct {
	Note  string       `json:"note"`
	Cases []parityCase `json:"cases"`
}

func loadParityCases(t *testing.T) parityFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "p37_zsort_parity.json"))
	if err != nil {
		t.Fatalf("read corpus: %v", err)
	}
	var f parityFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode corpus: %v", err)
	}
	if len(f.Cases) == 0 {
		t.Fatal("empty corpus")
	}
	return f
}

func parityFlatItems(in []parityItem) []sprite.Item {
	out := make([]sprite.Item, len(in))
	for i, it := range in {
		out[i] = sprite.Item{Name: it.Name, Layer: sprite.Layer(it.Layer), FeetY: it.FeetY}
	}
	return out
}

func parityDeepItems(in []parityItem) []sprite.DeepItem {
	out := make([]sprite.DeepItem, len(in))
	for i, it := range in {
		out[i] = sprite.DeepItem{Name: it.Name, Depth: it.Depth, Layer: sprite.Layer(it.Layer), FeetY: it.FeetY}
	}
	return out
}

func parityFlatNames(in []sprite.Item) []string {
	out := make([]string, len(in))
	for i, it := range in {
		out[i] = it.Name
	}
	return out
}

func parityDeepNames(in []sprite.DeepItem) []string {
	out := make([]string, len(in))
	for i, it := range in {
		out[i] = it.Name
	}
	return out
}

// firstDiff reports the first rank where got and want part ways ("" = equal).
func firstDiff(got, want []string) string {
	if len(got) != len(want) {
		return fmt.Sprintf("长度 %d≠%d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			return fmt.Sprintf("第 %d 位 got=%s want=%s", i, got[i], want[i])
		}
	}
	return ""
}

func reverseFlat(in []sprite.Item) []sprite.Item {
	out := make([]sprite.Item, len(in))
	for i := range in {
		out[i] = in[len(in)-1-i]
	}
	return out
}

func reverseDeep(in []sprite.DeepItem) []sprite.DeepItem {
	out := make([]sprite.DeepItem, len(in))
	for i := range in {
		out[i] = in[len(in)-1-i]
	}
	return out
}

func TestP37SpriteSortParityBitwise(t *testing.T) {
	f := loadParityCases(t)
	flatN, deepN := 0, 0
	for _, c := range f.Cases {
		t.Run(c.Name, func(t *testing.T) {
			switch c.Mode {
			case "flat":
				flatN++
				in := parityFlatItems(c.Items)
				snapshot := append([]sprite.Item(nil), in...)
				got, err := sprite.Sort(in)
				if err != nil {
					t.Fatalf("Sort: %v", err)
				}
				if msg := firstDiff(parityFlatNames(got), c.WantOrder); msg != "" {
					t.Fatalf("逐位对拍失败：%s", msg)
				}
				if !sprite.IsSorted(got) {
					t.Fatal("Sort 结果不自称已排序")
				}
				// 首末键真的不同 ⇒ 倒序必须判未排序（否则 IsSorted 的判据形同虚设）。
				if n := len(got); n > 1 && (got[0].Layer != got[n-1].Layer || got[0].FeetY != got[n-1].FeetY) {
					if sprite.IsSorted(reverseFlat(got)) {
						t.Fatal("倒序输入必须判未排序")
					}
				}
				if len(snapshot) != len(in) {
					t.Fatal("输入长度被改")
				}
				for i := range snapshot {
					if snapshot[i] != in[i] {
						t.Fatalf("Sort 改了输入切片第 %d 项", i)
					}
				}
			case "deep":
				deepN++
				in := parityDeepItems(c.Items)
				got, err := sprite.DepthSort(in)
				if err != nil {
					t.Fatalf("DepthSort: %v", err)
				}
				if msg := firstDiff(parityDeepNames(got), c.WantOrder); msg != "" {
					t.Fatalf("逐位对拍失败：%s", msg)
				}
				if !sprite.IsDepthSorted(got) {
					t.Fatal("DepthSort 结果不自称已排序")
				}
				if n := len(got); n > 1 && (got[0].Depth != got[n-1].Depth ||
					got[0].Layer != got[n-1].Layer || got[0].FeetY != got[n-1].FeetY) {
					if sprite.IsDepthSorted(reverseDeep(got)) {
						t.Fatal("倒序输入必须判未排序")
					}
				}
			default:
				t.Fatalf("unknown mode %q", c.Mode)
			}
		})
	}
	if flatN == 0 || deepN == 0 {
		t.Fatalf("语料家族不齐：flat=%d deep=%d", flatN, deepN)
	}
}

// 真委托钉 1：脚位差落在 camera 容差内时，先后由输入序号给 —— 旧副本按精确
// 浮点比 Y，会把 Y 小的排前面；只有把判据交给 camera 才会是下面这个结果。
func TestP37SpriteSortUsesCameraTolerance(t *testing.T) {
	far, _ := sprite.NewItem("far", sprite.LayerWorld, 20+1e-9)
	near, _ := sprite.NewItem("near", sprite.LayerWorld, 20)
	got, err := sprite.Sort([]sprite.Item{far, near})
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}
	if got[0].Name != "far" || got[1].Name != "near" {
		t.Fatalf("容差内平票必须按输入序号（far,near），实际 %v,%v：判据没交给 camera",
			got[0].Name, got[1].Name)
	}
	d := []sprite.DeepItem{
		{Name: "far", Depth: 500, Layer: sprite.LayerWorld, FeetY: 20 + 1e-9},
		{Name: "near", Depth: 500, Layer: sprite.LayerWorld, FeetY: 20},
	}
	gotD, err := sprite.DepthSort(d)
	if err != nil {
		t.Fatalf("DepthSort: %v", err)
	}
	if gotD[0].Name != "far" {
		t.Fatalf("DepthSort 同深度平票也必须按输入序号，实际 %v", gotD[0].Name)
	}
}

// 真委托钉 2：带→z 桶走 camera 的 ±YSortZRange 饱和，越界的带并进最后一桶后
// 由脚位 Y 决定先后（旧副本里 layer 永远压 Y，不可能出现这个结果）。
func TestP37SpriteSortBandSaturatesLikeCamera(t *testing.T) {
	if got := sprite.ZIndexForLayer(sprite.LayerWorld); got != -4000 {
		t.Fatalf("LayerWorld 槽位=%d，期望 -4000（camera.YSortZ(0)）", got)
	}
	world, fx, ui := sprite.ZIndexForLayer(sprite.LayerWorld), sprite.ZIndexForLayer(sprite.LayerFX), sprite.ZIndexForLayer(sprite.LayerUI)
	if !(world < fx && fx < ui) {
		t.Fatalf("带序必须单调：world=%d fx=%d ui=%d", world, fx, ui)
	}
	farBand := sprite.Layer(1_000_000)
	nearBand := sprite.Layer(2_000_000)
	if sprite.ZIndexForLayer(farBand) != sprite.ZIndexForLayer(nearBand) {
		t.Fatal("两条越界带必须饱和进同一桶")
	}
	hi, _ := sprite.NewItem("hi", farBand, 500)
	lo, _ := sprite.NewItem("lo", nearBand, 10)
	got, err := sprite.Sort([]sprite.Item{hi, lo})
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}
	if got[0].Name != "lo" {
		t.Fatalf("同桶内必须按脚位 Y 排（饱和后 band 不再压 Y），实际 %v", got[0].Name)
	}
}
