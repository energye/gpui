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

// E2 合并计数锁死（包相等 + 数诚实）：同一棵树、同一脏态下，
// 新 API（BuildFramePacketWithSaveLayerStats，顺手计数）与老三件套
// （CountRepaintBoundaries / CountPictureOps / TreeMeasureCacheStats）
// 必须给出相同的数，且新老建包的层序列与脏号顺序全等。
// 老函数留着给别处用，这里只读它们做对照，不走真帧。
func TestE2Merged_CountsHonestAndPacketEqual(t *testing.T) {
	buildTree := func() (*rendering.RenderBox, *rendering.PipelineOwner) {
		root := rendering.NewRenderBox()
		root.FixedWidth, root.FixedHeight = 800, 600
		// A 分支：边界下挂文本 + 色块（文本走计量缓存）。
		branchA := rendering.NewRenderBox()
		branchA.SetRepaintBoundary(true)
		txtA := rendering.NewRenderText("hello world e2 merged counts")
		txtA.MaxWidth = 400
		leafA := rendering.NewRenderBox()
		leafA.FixedWidth, leafA.FixedHeight = 10, 10
		branchA.AddChild(txtA)
		branchA.AddChild(leafA)
		// B 分支：两层嵌套边界（深度 2），内层再挂文本。
		branchB := rendering.NewRenderBox()
		branchB.SetRepaintBoundary(true)
		nested := rendering.NewRenderBox()
		nested.SetRepaintBoundary(true)
		txtB := rendering.NewRenderText("nested boundary text")
		txtB.MaxWidth = 400
		nested.AddChild(txtB)
		branchB.AddChild(nested)
		// 普通叶：非边界对照。
		plain := rendering.NewRenderBox()
		plain.FixedWidth, plain.FixedHeight = 10, 10
		root.AddChild(branchA)
		root.AddChild(branchB)
		root.AddChild(plain)
		owner := rendering.NewPipelineOwner(root)
		owner.FlushLayout(rendering.Size{Width: 800, Height: 600}, true)
		owner.ConsumeNeedsPaint()
		return root, owner
	}

	root, _ := buildTree()
	// 只脏 1 叶（E2 场景）：数必须照样诚实。
	root.Children()[0].Children()[1].MarkNeedsPaint()

	// 新路径：一次建包顺手出四个数。
	scene.ResetLayerIDGen()
	pktNew, st := rendering.BuildFramePacketWithSaveLayerStats(root, 1, 1, 800, 600, nil, nil)
	if pktNew == nil || pktNew.Root == nil {
		t.Fatal("nil packet")
	}

	// 老对照：同一树同一脏态，不再建包（建包会再暖一次计量缓存，
	// 老读数只许在新包建完后直接读，读的正是同一时刻的累计值）。
	bc, bd := rendering.CountRepaintBoundaries(root)
	opsOld := scene.CountPictureOps(pktNew)
	mh, mm := rendering.TreeMeasureCacheStats(root)

	if st.BoundaryCount != bc || st.BoundaryMaxDepth != bd {
		t.Fatalf("boundary merged=(%d,%d) old=(%d,%d)", st.BoundaryCount, st.BoundaryMaxDepth, bc, bd)
	}
	if st.PictureOpCount != opsOld {
		t.Fatalf("ops merged=%d old=%d", st.PictureOpCount, opsOld)
	}
	if st.MeasureHits != mh || st.MeasureMisses != mm {
		t.Fatalf("measure merged=(%d,%d) old=(%d,%d)", st.MeasureHits, st.MeasureMisses, mh, mm)
	}
	// 非空守卫：数不能全 0 装诚实。
	if bc < 3 || bd < 2 {
		t.Fatalf("boundary too trivial: count=%d depth=%d", bc, bd)
	}
	if opsOld == 0 {
		t.Fatal("picture ops is 0, test records nothing")
	}
	if mh+mm == 0 {
		t.Fatal("measure hits+misses is 0, text cache untouched")
	}
	t.Logf("honest: boundary=(%d,%d) ops=%d measure=(%d,%d)", bc, bd, opsOld, mh, mm)

	// 包相等：同树同脏态，老 API 再建一包，层序列与脏号顺序必须全等。
	// 建包不碰渲染树脏标记，两次脏集相同；层 ID 发号器重置后两次
	// 分配顺序一致，脏号可逐位比。
	scene.ResetLayerIDGen()
	pktOld := rendering.BuildFramePacketWithSaveLayer(root, 1, 1, 800, 600, nil, nil)
	kindsNew := layerKinds(pktNew.Root)
	kindsOld := layerKinds(pktOld.Root)
	if len(kindsNew) != len(kindsOld) {
		t.Fatalf("layer count new=%d old=%d", len(kindsNew), len(kindsOld))
	}
	for i := range kindsNew {
		if kindsNew[i] != kindsOld[i] {
			t.Fatalf("layer %d kind new=%s old=%s", i, kindsNew[i], kindsOld[i])
		}
	}
	if len(pktNew.DirtyLayerIDs) != len(pktOld.DirtyLayerIDs) {
		t.Fatalf("dirty count new=%d old=%d", len(pktNew.DirtyLayerIDs), len(pktOld.DirtyLayerIDs))
	}
	for i := range pktNew.DirtyLayerIDs {
		if pktNew.DirtyLayerIDs[i] != pktOld.DirtyLayerIDs[i] {
			t.Fatalf("dirty %d new=%d old=%d", i, pktNew.DirtyLayerIDs[i], pktOld.DirtyLayerIDs[i])
		}
	}
	t.Logf("packet equal: layers=%d dirty=%d ops=%d", len(kindsNew), len(pktNew.DirtyLayerIDs), opsOld)
}

func layerKinds(l scene.Layer) []string {
	var out []string
	scene.Walk(l, func(n scene.Layer) {
		out = append(out, n.Kind())
	})
	return out
}

// E2 新序列遍数参考：只脏 1 叶时，新序列（一次顺手建包 + 原位清旗，
// 不再调三个老计数）对渲染树的 Children 调用必须降到 400 以下
// （老证据 5 步 724 次，线 360；N=120 时新序列约一遍建包 + 一遍清旗）。
// 孩子调用按根/叶分开记，方便看是谁在走。
func TestE2Merged_NewSequenceWalkCount(t *testing.T) {
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
	n := cfg.WalkNodes

	var rootCalls, leafCalls int64
	root := newE2CountBox2(&rootCalls, 800, 600)
	kids := make([]*e2countBox2, 0, n)
	for i := 0; i < n; i++ {
		k := newE2CountBox2(&leafCalls, 10, 10)
		k.SetRepaintBoundary(true)
		root.AddChild(k)
		kids = append(kids, k)
	}
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 800, Height: 600}, true)
	owner.ConsumeNeedsPaint()
	atomic.StoreInt64(&rootCalls, 0)
	atomic.StoreInt64(&leafCalls, 0)
	kids[0].MarkNeedsPaint()

	// 新序列：一次建包（边界/操作数/字命中顺手出）+ 原位清旗。
	pkt, st := rendering.BuildFramePacketWithSaveLayerStats(root, 1, 1, 800, 600, nil, nil)
	if pkt == nil || pkt.Root == nil {
		t.Fatal("nil packet")
	}
	owner.ConsumeNeedsPaint()

	got := atomic.LoadInt64(&rootCalls) + atomic.LoadInt64(&leafCalls)
	t.Logf("N=%d new sequence Children calls=%d (root=%d leaf=%d) stats=%+v",
		n, got, atomic.LoadInt64(&rootCalls), atomic.LoadInt64(&leafCalls), st)
	if got >= 400 {
		t.Fatalf("Children calls=%d want <400 (merged single-walk sequence)", got)
	}
	if st.BoundaryCount != n {
		t.Fatalf("BoundaryCount=%d want %d", st.BoundaryCount, n)
	}
}

// e2countBox2 是只计数的包皮节点（同 e2_old_evidence_test.go 的 e2countBox，
// 各自计数器分开以便看根/叶分别走了多少遍）。
type e2countBox2 struct {
	*rendering.RenderBox
	calls *int64
}

func newE2CountBox2(calls *int64, w, h float64) *e2countBox2 {
	rb := rendering.NewRenderBox()
	b := &e2countBox2{RenderBox: rb, calls: calls}
	rb.Self = b
	b.FixedWidth, b.FixedHeight = w, h
	return b
}

func (b *e2countBox2) Children() []rendering.RenderObject {
	atomic.AddInt64(b.calls, 1)
	return b.RenderBox.Children()
}
