//go:build !nogpu

package camera

import (
	"math"
	"testing"

	"github.com/energye/gpui/engine/core"
)

// P3.7 批 2 步 2a：camera z 序真源升级的钉。判据按 Godot 真源写死：
// 外层 z 桶（相对 z 逐级累加 + 每级 CLAMP，renderer_canvas_cull.cpp:145-150）→
// 层内 origin Y 容差判等（ItemYSort + Math::is_equal_approx，
// renderer_canvas_cull.h:117-127、math_funcs.h:552-562）→ 容器 DFS 序号破平票。

func flatItems(pos ...core.Vec3) []YSortItem {
	out := make([]YSortItem, len(pos))
	for i, p := range pos {
		out[i] = YSortItem{Pos: p, Order: i}
	}
	return out
}

func TestP37YSortZBucketBeatsY(t *testing.T) {
	// A 在近处（Y 大）但 z 桶更低 ⇒ 仍应先画（z 是外层键，Godot「同 z_index 内才比 Y」）。
	items := []YSortItem{
		{Pos: core.V3(0, 5, 0), Order: 0, ZIndex: 10},
		{Pos: core.V3(0, 50, 0), Order: 1, ZIndex: -10},
	}
	got, err := YSort(items)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Order != 1 || got[1].Order != 0 {
		t.Fatalf("z 桶没压过 Y：%+v（期望 z=-10 的先画）", got)
	}
	if !YSortLess(items[1], items[0]) || YSortLess(items[0], items[1]) {
		t.Fatal("比较器对 z 桶不是严格全序")
	}
}

func TestP37YSortEqualApproxIsGodotFormula(t *testing.T) {
	// 平坦小值：容差下限 = CMP_EPSILON=1e-5 ⇒ 1e-9 判等、0.5 单位差别被判等。
	if !ySortEqualApprox(20, 20+1e-9) {
		t.Fatal("1e-9 差应判等（Godot is_equal_approx）")
	}
	if ySortEqualApprox(20, 20.5) {
		t.Fatal("0.5 单位差被判等，容差过大")
	}
	// 大值：Godot 容差按 |a| 放大 = max(CMP_EPSILON, CMP_EPSILON*|a|) ⇒ |a|=1e5 时容差 1.0。
	if !ySortEqualApprox(1e5, 1e5+0.5) {
		t.Fatal("1e5 量级下 0.5 差应落在放大容差内")
	}
	if ySortEqualApprox(1e5, 1e5+2) {
		t.Fatal("超出放大容差(1.0)的差不该判等")
	}
	// ±Inf：Godot 先走精确相等分支，Inf 与自己相等、与有限值不等。
	if !ySortEqualApprox(math.Inf(1), math.Inf(1)) || ySortEqualApprox(math.Inf(1), 1e300) {
		t.Fatal("±Inf 分支不对")
	}
}

func TestP37YSortTieUsesOrderNotGeometry(t *testing.T) {
	// 容差内的两个脚位（1e-9）：谁 Y 更小不许决定先后，必须由 Order 给 ——
	// 这条正是"退回精确比较必红"的判据（物理插值让脚位每帧抖 1e-9 时不闪）。
	near := YSortItem{Pos: core.V3(0, 20+1e-9, 0), Order: 0}
	far := YSortItem{Pos: core.V3(0, 20, 0), Order: 1}
	if !YSortLess(near, far) || YSortLess(far, near) {
		t.Fatalf("容差内平票必须由 Order 定向：Less(near,far)=%v Less(far,near)=%v",
			YSortLess(near, far), YSortLess(far, near))
	}
	// 默认键只看 Y：同 Y 不同 X 不比几何，先后只能由 Order 给
	//（x+z 微扰是具名变体，见 TestP37YSortKeysAreNamedAndSelectable）。
	l := YSortItem{Pos: core.V3(0, 9, 0), Order: 1}
	r := YSortItem{Pos: core.V3(3, 9, 0), Order: 0}
	if !YSortLess(r, l) || YSortLess(l, r) {
		t.Fatalf("同 Y 不同 X 的先后必须由 Order 给：Less(r,l)=%v Less(l,r)=%v", YSortLess(r, l), YSortLess(l, r))
	}
	// Order 也相同 = 完全平票：两个方向都不许成立（不许抛硬币）。
	x := YSortItem{Pos: core.V3(0, 9, 0), Order: 7}
	y := YSortItem{Pos: core.V3(3, 9, 0), Order: 7}
	if YSortLess(x, y) || YSortLess(y, x) {
		t.Fatal("完全平票不许比出先后")
	}
}

func TestP37YSortKeysAreNamedAndSelectable(t *testing.T) {
	p := core.V3(100, 7, 50)
	if YSortKeyY(p) != 7 {
		t.Fatalf("标准键应等于纯 Y：%v", YSortKeyY(p))
	}
	if got := YSortKeySlightXZ(p); got <= 7 {
		t.Fatalf("变体键应带上 0.001*(x+z)：%v", got)
	}
	// 同 Y 不同 X，故意让 Order 与几何"顶风"：默认键听 Order，变体键听 x+z。
	items := []YSortItem{
		{Pos: core.V3(3, 9, 0), Order: 0},
		{Pos: core.V3(0, 9, 0), Order: 1},
	}
	std, err := YSort(items)
	if err != nil {
		t.Fatal(err)
	}
	if std[0].Order != 0 || std[1].Order != 1 {
		t.Fatalf("默认键应按 Order 破平票：%+v", std)
	}
	variant, err := YSortWithKey(items, YSortKeySlightXZ)
	if err != nil {
		t.Fatal(err)
	}
	if variant[0].Order != 1 || variant[1].Order != 0 {
		t.Fatalf("变体键没按 x+z 分先后：%+v", variant)
	}
	// nil 键 = 标准键（护栏，不许拿 nil 猜）。
	nilKey, err := YSortWithKey(items, nil)
	if err != nil || nilKey[0].Order != std[0].Order {
		t.Fatalf("nil 键应等于标准键：%+v err=%v", nilKey, err)
	}
	// 树入口同口径：两个根同 Y 不同 X，标准键听 DFS 序号，变体键听 x+z。
	nodes := []YSortNode{
		{Parent: -1, Pos: core.V3(3, 9, 0)},
		{Parent: -1, Pos: core.V3(0, 9, 0)},
	}
	treeStd, err := YSortTree(nodes)
	if err != nil {
		t.Fatal(err)
	}
	if treeStd[0] != 0 || treeStd[1] != 1 {
		t.Fatalf("树的标准键应按 DFS 序号破平票：%v", treeStd)
	}
	treeVariant, err := YSortTreeWithKey(nodes, YSortKeySlightXZ)
	if err != nil {
		t.Fatal(err)
	}
	if treeVariant[0] != 1 || treeVariant[1] != 0 {
		t.Fatalf("树的变体键没按 x+z 分先后：%v", treeVariant)
	}
	if treeNil, err := YSortTreeWithKey(nodes, nil); err != nil || treeNil[0] != treeStd[0] {
		t.Fatalf("树的 nil 键应等于标准键：%v err=%v", treeNil, err)
	}
}

func TestP37YSortTreeZChainClamps(t *testing.T) {
	nodes := []YSortNode{
		{Parent: -1, Pos: core.V3(0, 0, 0), ZIndex: 3000},                    // 0 根
		{Parent: 0, Pos: core.V3(0, 1, 0), ZIndex: 3000, ZAsRelative: true},  // 1 累加 6000 ⇒ 饱和 4096
		{Parent: 1, Pos: core.V3(0, 2, 0), ZIndex: -9000, ZAsRelative: true}, // 2 每级 CLAMP：4096-9000 ⇒ -4904 ⇒ 饱和 -4096
		{Parent: 0, Pos: core.V3(0, 3, 0), ZIndex: -7, ZAsRelative: false},   // 3 绝对化：忽略父链 3000，取 -7
		{Parent: -1, Pos: core.V3(0, 4, 0)},                                  // 4 另一根：z=0，用来验绝对化真的改了先后
	}
	order, err := YSortTree(nodes)
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != len(nodes) {
		t.Fatalf("丢节点： %+v", order)
	}
	// 有效 z：2=-4096、3=-7、4=0、0=3000、1=4096 ⇒ 全序 [2 3 4 0 1]。
	//（3 若被当成相对 z 会变成 2993，排到 4 之后 ⇒ 这条顺序同时钉住"绝对化不吃父链"。）
	for i, want := range []int{2, 3, 4, 0, 1} {
		if order[i] != want {
			t.Fatalf("z 链/CLAMP 结果不对：order=%v want=[2 3 4 0 1]（第 %d 位）", order, i)
		}
	}
	if z, err := YSortEffectiveZ(nodes, 2); err != nil || z != -YSortZRange {
		t.Fatalf("三级链应饱和到 -%d：%d err=%v", YSortZRange, z, err)
	}
	if z, err := YSortEffectiveZ(nodes, 3); err != nil || z != -7 {
		t.Fatalf("absolute z 不该累加父链：%d err=%v", z, err)
	}
	if z, err := YSortEffectiveZ(nodes, 1); err != nil || z != YSortZRange {
		t.Fatalf("正向累加应饱和到 +%d：%d err=%v", YSortZRange, z, err)
	}
}

func TestP37YSortTreeSortsOwnYNotSubtree(t *testing.T) {
	// Godot 口径：压平是 DFS，但比较的是节点自己在容器系里的 Y —— 孙节点 Y 远就排后面，
	// 父子关系不聚簇（ysort_xform.origin.y 只带祖先平移量，不带「整棵子树跟着走」）。
	nodes := []YSortNode{
		{Parent: -1, Pos: core.V3(0, 0, 0)}, // 0 宿主（占序号 0）
		{Parent: 0, Pos: core.V3(0, 10, 0)}, // 1 第一子
		{Parent: 1, Pos: core.V3(0, 99, 0)}, // 2 孙（Y 很远 ⇒ 排最后）
		{Parent: 0, Pos: core.V3(0, 20, 0)}, // 3 第二子
	}
	order, err := YSortTree(nodes)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []int{0, 1, 3, 2} {
		if order[i] != want {
			t.Fatalf("没按各自 Y 排（子树不聚簇）：order=%v want=[0 1 3 2]（第 %d 位）", order, i)
		}
	}
	// 真平票（同 Y）才轮到 DFS 序号：子 1 先于兄弟 3，孙 2 紧跟其父 1。
	tied := []YSortNode{
		{Parent: -1, Pos: core.V3(0, 5, 0)},
		{Parent: 0, Pos: core.V3(1, 5, 0)},
		{Parent: 1, Pos: core.V3(2, 5, 0)},
		{Parent: 0, Pos: core.V3(3, 5, 0)},
	}
	got, err := YSortTree(tied)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []int{0, 1, 2, 3} {
		if got[i] != want {
			t.Fatalf("平票没按 DFS 序定序：order=%v want=[0 1 2 3]（第 %d 位）", got, i)
		}
	}
}

func TestP37YSortTreeRejectsBadInput(t *testing.T) {
	t.Run("环", func(t *testing.T) {
		if got, err := YSortTree([]YSortNode{{Parent: 1}, {Parent: 0}}); err == nil || got != nil {
			t.Fatalf("父子成环必须拒绝：%v %v", got, err)
		}
		if got, err := YSortTree([]YSortNode{{Parent: 0}}); err == nil || got != nil {
			t.Fatalf("自父亲环必须拒绝：%v %v", got, err)
		}
		if _, err := YSortEffectiveZ([]YSortNode{{Parent: 1}, {Parent: 0}}, 0); err == nil {
			t.Fatal("YSortEffectiveZ 同环必须拒绝")
		}
	})
	t.Run("坏父索引", func(t *testing.T) {
		if got, err := YSortTree([]YSortNode{{Parent: 7}}); err == nil || got != nil {
			t.Fatalf("越界父索引必须拒绝：%v %v", got, err)
		}
		if _, err := YSortEffectiveZ([]YSortNode{{Parent: -1}}, 5); err == nil {
			t.Fatal("越界下标必须拒绝")
		}
	})
	t.Run("非有限位置", func(t *testing.T) {
		if got, err := YSortTree([]YSortNode{{Parent: -1, Pos: core.V3(math.NaN(), 0, 0)}}); err == nil || got != nil {
			t.Fatalf("NaN 位置必须拒绝：%v %v", got, err)
		}
		if got, err := YSortTree([]YSortNode{{Parent: -1, Pos: core.V3(0, math.Inf(-1), 0)}}); err == nil || got != nil {
			t.Fatalf("-Inf 位置必须拒绝：%v %v", got, err)
		}
	})
	t.Run("超上限", func(t *testing.T) {
		nodes := make([]YSortNode, MaxYSort25D+1)
		for i := range nodes {
			nodes[i] = YSortNode{Parent: -1}
		}
		if _, err := YSortTree(nodes); err == nil {
			t.Fatal("超过 MaxYSort25D 必须拒排")
		}
		if _, err := YSortTreeWithKey(nodes, YSortKeySlightXZ); err == nil {
			t.Fatal("变体键入口同上限必须拒排")
		}
		if _, err := YSort(flatItems(make([]core.Vec3, MaxYSort25D+1)...)); err == nil {
			t.Fatal("扁平入口同上限必须拒排")
		}
	})
}

func TestP37YSortIsDeterministicTotalOrder(t *testing.T) {
	// 300 个真实量级脚位（整数 Y、含同 Y 同 z）：结果与输入排列无关，且不靠排序稳定性。
	seed := uint64(20261104)
	next := func() float64 {
		seed = seed*6364136223846793005 + 1442695040888963407
		return float64((seed>>33)%1000) / 10.0
	}
	items := make([]YSortItem, 300)
	for i := range items {
		items[i] = YSortItem{Pos: core.V3(next(), math.Round(next()), 0), Order: i, ZIndex: int(math.Round(next())) - 50}
	}
	got, err := YSort(items)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(got); i++ {
		if YSortLess(got[i], got[i-1]) {
			t.Fatalf("第 %d 位逆序", i)
		}
	}
	for i := 0; i < len(got); i++ {
		for j := i + 1; j < len(got); j++ {
			if YSortLess(got[i], got[j]) == YSortLess(got[j], got[i]) {
				t.Fatalf("非严格全序（i=%d j=%d）", i, j)
			}
		}
	}
	// 输入乱序重排后结果一致（Order 是唯一破平票依据，不是算法稳定性）。
	shuffled := make([]YSortItem, len(items))
	copy(shuffled, items)
	for i := len(shuffled) - 1; i > 0; i-- {
		seed = seed*6364136223846793005 + 1442695040888963407
		j := int((seed >> 33) % uint64(i+1))
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}
	again, err := YSort(shuffled)
	if err != nil {
		t.Fatal(err)
	}
	for i := range got {
		if got[i].Order != again[i].Order || got[i].ZIndex != again[i].ZIndex {
			t.Fatalf("乱序输入结果不稳定（第 %d 位）", i)
		}
	}
}
