package rendering_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
)

// E2 老证据重建（只读现行老代码，不引新符号）：
// 重放 pipeline_app.go 一帧内的整树调用序列——
// CountRepaintBoundaries → BuildFramePacketWithSaveLayer →
// scene.CountPictureOps → TreeMeasureCacheStats → ConsumeNeedsPaint，
// 用包皮节点数 Children() 被调次数，证明一帧走多遍整树。
//
// 判定线：只脏 1 叶后跑完序列，Children 调用数 >= 3*N（N=walkNodes）。
// 通过 = 多遍成立；将来修 E2（合并计数、Consume 不动）后应低于此线。
func TestEVerify_E2_OldEvidence(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "e_verify.json"))
	if err != nil {
		t.Skipf("e_verify.json missing: %v", err)
	}
	var cfg struct {
		WalkNodes int `json:"walkNodes"`
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatalf("parse e_verify.json: %v", err)
	}
	if cfg.WalkNodes < 10 {
		t.Fatalf("walkNodes=%d want >=10", cfg.WalkNodes)
	}
	n := cfg.WalkNodes

	var calls int64
	root := rendering.NewRenderBox()
	// 包皮：根与叶子都计数 Children 调用。
	// 用组合而非嵌入（RenderBox 非接口，需自行转发）。
	probe := &e2probe{calls: &calls}
	_ = probe
	_ = root

	// 直接用计数包装：新建 n 个边界叶子挂根下。
	leaves := make([]*rendering.RenderBox, 0, n)
	for i := 0; i < n; i++ {
		leaf := rendering.NewRenderBox()
		leaf.FixedWidth, leaf.FixedHeight = 10, 10
		leaf.SetRepaintBoundary(true)
		root.AddChild(leaf)
		leaves = append(leaves, leaf)
	}
	root.FixedWidth, root.FixedHeight = 800, 600
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 800, Height: 600}, true)
	// 清掉构造时的初始脏（否则全树都脏，脏层数对不上单叶）。
	owner.ConsumeNeedsPaint()

	// 稳态后只脏 1 叶。Children 调用用“包体积法”替代：
	// 建包前记包层数是不可能的（包即全树），改为直接对序列每一步
	// 计整树访问：用独立计数 walk 复述每一步的遍历量。
	leaves[0].MarkNeedsPaint()

	// 1) 数边界：整树一遍。
	rendering.CountRepaintBoundaries(root)
	// 2) 建包：整树一遍（BuildFramePacketWithSaveLayer 内 appendNode 递归）。
	pkt := rendering.BuildFramePacketWithSaveLayer(root, 1, 1, 800, 600, nil, nil)
	if pkt == nil || pkt.Root == nil {
		t.Fatal("nil packet")
	}
	// 3) 数操作数：走包树一遍（与渲染树同规模）。
	scene.CountPictureOps(pkt)
	// 4) 量字统计：整树一遍。
	rendering.TreeMeasureCacheStats(root)
	// 5) 清标记：整树一遍。
	owner.ConsumeNeedsPaint()

	// 包体积证据：只脏 1 叶，包仍覆盖全树（E1 未修时的老行为）。
	total := scene.Walk(pkt.Root, nil)
	t.Logf("N=%d dirtyIDs=%d packetLayers=%d", n, len(pkt.DirtyLayerIDs), total)
	if len(pkt.DirtyLayerIDs) > 2 {
		t.Fatalf("dirtyIDs=%d want <=2", len(pkt.DirtyLayerIDs))
	}
	if total < n {
		t.Fatalf("packetLayers=%d must cover whole tree N=%d", total, n)
	}

	// 遍数证据：用 Children 调用计数器直接量序列。
	// 重建计数树再跑一次序列并计数。
	var calls2 int64
	root2 := newE2CountBox(&calls2, 800, 600)
	kids := make([]*e2countBox, 0, n)
	for i := 0; i < n; i++ {
		k := newE2CountBox(&calls2, 10, 10)
		k.SetRepaintBoundary(true)
		root2.AddChild(k)
		kids = append(kids, k)
	}
	owner2 := rendering.NewPipelineOwner(root2)
	owner2.FlushLayout(rendering.Size{Width: 800, Height: 600}, true)
	owner2.ConsumeNeedsPaint()
	atomic.StoreInt64(&calls2, 0)
	kids[0].MarkNeedsPaint()
	if got := atomic.LoadInt64(&calls2); got != 0 {
		t.Fatalf("dirtying one leaf walked tree: calls=%d want 0", got)
	}
	rendering.CountRepaintBoundaries(root2)
	rendering.BuildFramePacketWithSaveLayer(root2, 1, 1, 800, 600, nil, nil)
	rendering.TreeMeasureCacheStats(root2)
	owner2.ConsumeNeedsPaint()
	got := atomic.LoadInt64(&calls2)
	t.Logf("N=%d Children calls for frame sequence=%d (line %d)", n, got, 3*n)
	if got < int64(3*n) {
		t.Fatalf("Children calls=%d want >=%d (multi full-tree walks)", got, 3*n)
	}
}

// e2probe 占位（本文件改用 e2countBox 计数）。
type e2probe struct {
	calls *int64
}

// e2countBox 是只计数的包皮节点：转发 RenderBox 行为，重写 Children 计数。
type e2countBox struct {
	*rendering.RenderBox
	calls *int64
}

func newE2CountBox(calls *int64, w, h float64) *e2countBox {
	rb := rendering.NewRenderBox()
	b := &e2countBox{RenderBox: rb, calls: calls}
	rb.Self = b
	b.FixedWidth, b.FixedHeight = w, h
	return b
}

func (b *e2countBox) Children() []rendering.RenderObject {
	atomic.AddInt64(b.calls, 1)
	return b.RenderBox.Children()
}
