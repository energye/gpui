//go:build !nogpu

package gpu

import (
	"encoding/binary"
	"math"
	"testing"

	intImage "github.com/energye/gpui/render/internal/image"
)

func p37F32(b []byte) float32 { return math.Float32frombits(binary.LittleEndian.Uint32(b)) }

// P3.7 批 1a 钉：透视 quad 之间必须合批。
//
// P3.5 批 3 只把透视接成「一张一 draw call」：谓词对 Perspective 无条件 return
// false，发射处写死 vertCnt=6，而同图 1000 张仿射 quad 是一枚多顶点 Draw。2.5D
// 精灵批绘正是这个量级（实测基线见 evidence/P3.7/probes/persp-batch-cost.txt：
// N=1000 时透视散成 1000 draw call + 250KB uniform slab/帧）。
// 四角与权重都是逐顶点数据（与 tint 同性质），合批不需要逐张换绑定组。

func p37PerspCmd(img *intImage.ImageBuf, x float32, w [4]float32) *ImageDrawCommand {
	cmd := p36CmdOf(img)
	cmd.Perspective = true
	cmd.Weights = w
	cmd.TLX, cmd.TLY = x, 4
	cmd.TRX, cmd.TRY = x+8, 4
	cmd.BRX, cmd.BRY = x+8, 12
	cmd.BLX, cmd.BLY = x, 12
	cmd.DstX, cmd.DstY = x, 4
	cmd.DstW, cmd.DstH = 8, 8
	return cmd
}

// TestP37PerspectiveMergePredicate 钉谓词本身的四种组合。
func TestP37PerspectiveMergePredicate(t *testing.T) {
	img, err := intImage.NewImageBuf(16, 16, intImage.FormatRGBAPremul)
	if err != nil {
		t.Fatal(err)
	}
	defer img.Dispose()

	a := p37PerspCmd(img, 0, [4]float32{100, 110, 120, 130})
	b := p37PerspCmd(img, 16, [4]float32{200, 210, 220, 230})
	if !canMergeImageDraw(a, b) {
		t.Fatal("同图同采样同透明度的两张透视 quad 不合批：2.5D 精灵批绘会退化成逐张提交")
	}
	affine := p36CmdOf(img)
	if canMergeImageDraw(a, affine) || canMergeImageDraw(affine, b) {
		t.Fatal("透视与仿射不得同槽：两者顶点布局（36B/32B）与管线不同")
	}
	// 权重是逐顶点数据，不得进合批键；透明度/采样/图源版本必须进。
	diffWeights := p37PerspCmd(img, 32, [4]float32{1, 2, 3, 4})
	if !canMergeImageDraw(a, diffWeights) {
		t.Fatal("仅权重不同也分家：权重应像 tint 一样逐顶点携带")
	}
	otherOpacity := p37PerspCmd(img, 48, [4]float32{100, 110, 120, 130})
	otherOpacity.Opacity = 0.5
	if canMergeImageDraw(a, otherOpacity) {
		t.Fatal("透明度不同必须分家（绑定组按槽共享 uniform）")
	}
	img.Fill(10, 200, 10, 255)
	img.NotifyPixelsChanged()
	nextGen := p37PerspCmd(img, 64, [4]float32{100, 110, 120, 130})
	if canMergeImageDraw(a, nextGen) {
		t.Fatal("跨内容版本不得合批（旧帧会吞掉新帧）")
	}
}

// TestP37PerspectiveBatchBuildsOneDraw 走真会话：4 张透视 quad 必须合成
// 一次 Draw（firstVertex=0、vertCnt=4*6），且顶点字节进 36B 透视缓冲。
func TestP37PerspectiveBatchBuildsOneDraw(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	defer cleanup()
	img, err := intImage.NewImageBuf(16, 16, intImage.FormatRGBAPremul)
	if err != nil {
		t.Fatal(err)
	}
	defer img.Dispose()

	s := NewGPURenderSession(device, queue, 1)
	defer s.Destroy()

	cmds := []ImageDrawCommand{
		*p37PerspCmd(img, 0, [4]float32{100, 110, 120, 130}),
		*p37PerspCmd(img, 16, [4]float32{200, 210, 220, 230}),
		*p37PerspCmd(img, 32, [4]float32{300, 310, 320, 330}),
		*p37PerspCmd(img, 48, [4]float32{400, 410, 420, 430}),
	}
	res, err := s.buildImageResources(cmds, 64, 64, []bool{false, false, false, false})
	if err != nil {
		t.Fatalf("buildImageResources: %v", err)
	}
	if res == nil || len(res.drawCalls) != 1 {
		got := 0
		if res != nil {
			got = len(res.drawCalls)
		}
		t.Fatalf("四张透视 quad 合批后 draw call=%d，want 1", got)
	}
	dc := res.drawCalls[0]
	if !dc.perspective {
		t.Fatal("透视槽丢了 perspective 标记")
	}
	if dc.vertexCount != uint32(4*6) || dc.firstVertex != 0 {
		t.Fatalf("合并跨度 firstVertex=%d vertCnt=%d，want 0/%d（跨度错=只画第一张或读到邻张顶点）",
			dc.firstVertex, dc.vertexCount, 4*6)
	}
	if dc.cmdStart != 0 || dc.cmdCount != 4 {
		t.Fatalf("命令区间 cmdStart=%d cmdCount=%d，want 0/4（裁剪分组按命令区间判定，P3.5 收口的第④洞）",
			dc.cmdStart, dc.cmdCount)
	}
	if dc.vertBuf != s.imagePerspVertBuf {
		t.Fatal("透视合并槽必须钉 36B 透视缓冲")
	}
	if draws, quads := s.LastImageBatchStats(); draws != 1 || quads != 4 {
		t.Fatalf("LastImageBatchStats=(%d,%d)，want (1,4)", draws, quads)
	}
}

// TestP37PerspectiveRunSplitsAtSealAndAffine 钉合并边界：裁剪封段与夹进来的
// 仿射命令都必须把透视链切开（不能跨边界共用一次 Draw）。
func TestP37PerspectiveRunSplitsAtSealAndAffine(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	defer cleanup()
	img, err := intImage.NewImageBuf(16, 16, intImage.FormatRGBAPremul)
	if err != nil {
		t.Fatal(err)
	}
	defer img.Dispose()

	s := NewGPURenderSession(device, queue, 1)
	defer s.Destroy()

	sealCase := []ImageDrawCommand{
		*p37PerspCmd(img, 0, [4]float32{100, 110, 120, 130}),
		*p37PerspCmd(img, 16, [4]float32{200, 210, 220, 230}),
		*p37PerspCmd(img, 32, [4]float32{300, 310, 320, 330}),
	}
	res, err := s.buildImageResources(sealCase, 64, 64, []bool{false, true, false})
	if err != nil {
		t.Fatalf("seal buildImageResources: %v", err)
	}
	if res == nil || len(res.drawCalls) != 2 {
		t.Fatalf("裁剪封段处应切成 2 次 Draw，got %v", res)
	}
	if res.drawCalls[0].vertexCount != 6 || res.drawCalls[1].vertexCount != 12 {
		t.Fatalf("封段后跨度=%d/%d，want 6/12", res.drawCalls[0].vertexCount, res.drawCalls[1].vertexCount)
	}
	if res.drawCalls[1].firstVertex != 6 {
		t.Fatalf("第二段 firstVertex=%d，want 6（透视打包按命令序递增，跨段不能重影）", res.drawCalls[1].firstVertex)
	}

	mixed := []ImageDrawCommand{
		*p37PerspCmd(img, 0, [4]float32{100, 110, 120, 130}),
		*p37PerspCmd(img, 16, [4]float32{200, 210, 220, 230}),
		*p36CmdOf(img), // 仿射夹在中间
		*p37PerspCmd(img, 48, [4]float32{300, 310, 320, 330}),
	}
	res2, err := s.buildImageResources(mixed, 64, 64, []bool{false, false, false, false})
	if err != nil {
		t.Fatalf("mixed buildImageResources: %v", err)
	}
	if res2 == nil || len(res2.drawCalls) != 3 {
		t.Fatalf("透视/透视/仿射/透视应切 3 次 Draw，got %d", func() int {
			if res2 == nil {
				return -1
			}
			return len(res2.drawCalls)
		}())
	}
	var perspDraws, perspQuads int
	for _, dc := range res2.drawCalls {
		if dc.perspective {
			perspDraws++
			perspQuads += int(dc.vertexCount / 6)
		}
	}
	if perspDraws != 2 || perspQuads != 3 {
		t.Fatalf("透视侧 draws=%d quads=%d，want 2/3（被仿射打断的两段共 3 张）", perspDraws, perspQuads)
	}
}

// TestP37PerspRunEndGuardsPackingGap 钉合并跨度的守卫本身：打包里出现空洞
// （降级命令 pIdx=-1）时必须在空洞前断开，否则一次 Draw 会读到邻居的顶点。
func TestP37PerspRunEndGuardsPackingGap(t *testing.T) {
	img, err := intImage.NewImageBuf(16, 16, intImage.FormatRGBAPremul)
	if err != nil {
		t.Fatal(err)
	}
	defer img.Dispose()

	cmds := []ImageDrawCommand{
		*p37PerspCmd(img, 0, [4]float32{1, 1, 1, 1}),
		*p37PerspCmd(img, 16, [4]float32{1, 1, 1, 1}),
		*p37PerspCmd(img, 32, [4]float32{1, 1, 1, 1}),
	}
	if got := perspRunEnd(cmds, []int32{0, 1, 2}, 0, 3); got != 3 {
		t.Fatalf("连续 pIdx 应整段合并，got %d", got)
	}
	if got := perspRunEnd(cmds, []int32{0, 2, 3}, 0, 3); got != 1 {
		t.Fatalf("pIdx 跳号应在空洞前断开，got %d", got)
	}
	if got := perspRunEnd(cmds, []int32{0, -1, 1}, 0, 3); got != 1 {
		t.Fatalf("降级命令（pIdx=-1）应在其前断开，got %d", got)
	}
	degraded := cmds[1]
	degraded.Perspective = false
	gapCmds := []ImageDrawCommand{cmds[0], degraded, cmds[2]}
	if got := perspRunEnd(gapCmds, []int32{0, -1, 1}, 0, 3); got != 1 {
		t.Fatalf("非透视成员应在其前断开，got %d", got)
	}
}

// 批 1b 顶点级钉：透视打包必须把命令的 premul tint 写进 36B 布局的 @location(2)
// 槽（offset 16..27），零结构体退回 identity 白（与仿射孪生体同式）。
func TestP37PerspVerticesCarryTint(t *testing.T) {
	img, err := intImage.NewImageBuf(16, 16, intImage.FormatRGBAPremul)
	if err != nil {
		t.Fatal(err)
	}
	defer img.Dispose()

	cmd := p37PerspCmd(img, 0, [4]float32{100, 110, 120, 130})
	cmd.TintR, cmd.TintG, cmd.TintB, cmd.TintA = 0.5, 0.25, 0.75, 1
	buf := make([]byte, 6*imagePerspVertexStride)
	buildPerspVerticesInto(buf, cmd)
	for v := 0; v < 6; v++ {
		off := v*imagePerspVertexStride + 16
		got := [4]float32{
			p37F32(buf[off : off+4]), p37F32(buf[off+4 : off+8]),
			p37F32(buf[off+8 : off+12]), p37F32(buf[off+12 : off+16]),
		}
		want := [4]float32{0.5, 0.25, 0.75, 1}
		if got != want {
			t.Fatalf("vert[%d] tint=%v want %v（透视腿染色通道没接上）", v, got, want)
		}
	}

	// 零结构体 = identity 白，且权重/几何不受影响。
	plain := p37PerspCmd(img, 0, [4]float32{100, 110, 120, 130})
	buildPerspVerticesInto(buf, plain)
	off := 16
	if got := p37F32(buf[off : off+4]); got != 1 {
		t.Fatalf("零结构体 tint.R=%v want 1（存量画面被改变）", got)
	}
	if got := p37F32(buf[32:36]); got != 100 {
		t.Fatalf("第一角权重=%v want 100", got)
	}
}

// p37AtlasPerspCmd 造一条「图集透视精灵」形态的命令（P3.7 批 1c）：
// 透视 + 子矩形 UV + tint，与 DrawAtlasEx 经 QueueImageDrawQuadW 下发的字节面一致。
func p37AtlasPerspCmd(img *intImage.ImageBuf, x float32, u0, v0, u1, v1 float32) *ImageDrawCommand {
	cmd := p37PerspCmd(img, x, [4]float32{100, 110, 120, 130})
	cmd.U0, cmd.V0, cmd.U1, cmd.V1 = u0, v0, u1, v1
	cmd.TintR, cmd.TintG, cmd.TintB, cmd.TintA = 0.25, 0.5, 0.75, 1
	return cmd
}

// TestP37AtlasPerspMerges 钉批 1c：图集透视精灵（子矩形 UV + tint）与 quad 透视贴图
// 同图混排必须同槽合批——1a 谓词不看 UV 与 tint（都是逐顶点数据），图集批量精灵
// 的透视腿不能退化成逐张提交。
func TestP37AtlasPerspMerges(t *testing.T) {
	img, err := intImage.NewImageBuf(16, 16, intImage.FormatRGBAPremul)
	if err != nil {
		t.Fatal(err)
	}
	defer img.Dispose()

	// 谓词级：图集透视命令与 quad 透视命令（UV 0..1）同图必合。
	atlasCmd := p37AtlasPerspCmd(img, 0, 0.25, 0.5, 0.5, 1)
	quadCmd := p37PerspCmd(img, 16, [4]float32{100, 110, 120, 130})
	if !canMergeImageDraw(atlasCmd, quadCmd) {
		t.Fatal("图集透视精灵与 quad 透视贴图同图不合批：UV/tint 不该进合批键")
	}

	// 真会话：3 张图集透视精灵（子矩形 UV 各异）→ 1 次 Draw，跨度=18 顶点。
	device, queue, cleanup := createNativeTestDevice(t)
	defer cleanup()
	s := NewGPURenderSession(device, queue, 1)
	defer s.Destroy()
	cmds := []ImageDrawCommand{
		*p37AtlasPerspCmd(img, 0, 0.0, 0.0, 0.5, 0.5),
		*p37AtlasPerspCmd(img, 16, 0.5, 0.0, 1.0, 0.5),
		*p37AtlasPerspCmd(img, 32, 0.0, 0.5, 0.5, 1.0),
	}
	res, err := s.buildImageResources(cmds, 64, 64, []bool{false, false, false})
	if err != nil {
		t.Fatalf("buildImageResources: %v", err)
	}
	if res == nil || len(res.drawCalls) != 1 {
		got := 0
		if res != nil {
			got = len(res.drawCalls)
		}
		t.Fatalf("三张图集透视精灵合批后 draw call=%d，want 1", got)
	}
	dc := res.drawCalls[0]
	if !dc.perspective || dc.vertexCount != uint32(3*6) || dc.firstVertex != 0 {
		t.Fatalf("合并槽形态 perspective=%v firstVertex=%d vertCnt=%d，want true/0/18", dc.perspective, dc.firstVertex, dc.vertexCount)
	}
	if draws, quads := s.LastImageBatchStats(); draws != 1 || quads != 3 {
		t.Fatalf("LastImageBatchStats=(%d,%d)，want (1,3)", draws, quads)
	}

	// 顶点级：子矩形 UV 必须逐角进 36B 透视缓冲（批 1b 的 tint 槽同位验证）。
	buf := make([]byte, 6*imagePerspVertexStride)
	buildPerspVerticesInto(buf, atlasCmd)
	if got := p37F32(buf[8:12]); got != 0.25 {
		t.Fatalf("图集透视顶点 TL.u=%v want 0.25（子矩形 UV 没进透视槽）", got)
	}
	if got := p37F32(buf[12:16]); got != 0.5 {
		t.Fatalf("图集透视顶点 TL.v=%v want 0.5", got)
	}
	if got := p37F32(buf[8+imagePerspVertexStride : 12+imagePerspVertexStride]); got != 0.5 {
		t.Fatalf("图集透视顶点 TR.u=%v want 0.5", got)
	}
}
