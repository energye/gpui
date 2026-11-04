//go:build !nogpu

package render

import (
	"testing"

	"github.com/energye/gpui/render/text"
)

// P3.7 批 1d 内部钉：各组非仿射表态的具名计数器 + projTotalAffine 语义。
// 外部钉（p37_projection_test.go）验画面行为；这里验「表态确实被计数」——
// 防止降级/跳过悄悄发生而无人知晓（非静默压扁纪律的观测面）。

func TestP37ProjectionAffineSemantic(t *testing.T) {
	c := NewContext(64, 64)
	defer func() { _ = c.Close() }()

	// 默认：projTotalAffine 必须原样返回 totalMatrix（逐位同旧）。
	tm := c.totalMatrix()
	aff, ok := c.projTotalAffine()
	if !ok || aff != tm {
		t.Fatalf("恒等投影下 projTotalAffine≠totalMatrix（ok=%v）", ok)
	}

	// 纯提升仿射投影：组合成单一仿射（ok=true）。
	c.SetProjection(Matrix4FromMatrix(Translate(7, -3)))
	aff2, ok2 := c.projTotalAffine()
	if !ok2 {
		t.Fatal("仿射投影被 projTotalAffine 拒绝")
	}
	want := Translate(7, -3)
	if aff2.A != want.A || aff2.B != want.B || aff2.C != want.C || aff2.D != want.D || aff2.E != want.E || aff2.F != want.F {
		t.Fatalf("仿射投影组合错误：%v want %v", aff2, want)
	}

	// 真透视（w 行非平凡）：ok=false ⇒ 调用方必须表态。
	c.SetProjection(Matrix4Homography(Translate(100, 65), 0, -0.003, 1))
	if _, ok3 := c.projTotalAffine(); ok3 {
		t.Fatal("透视投影被 projTotalAffine 放行（静默压扁风险）")
	}
}

func TestP37ProjectionCounters(t *testing.T) {
	c := NewContext(320, 220)
	defer func() { _ = c.Close() }()

	src, err := text.NewFontSourceFromFile("text/testdata/source-sans/OTF/SourceSans3-Regular.otf")
	if err != nil {
		t.Skipf("字体缺失：%v", err)
	}
	c.SetFont(src.Face(20))

	c.SetProjection(Matrix4Homography(Translate(100, 65), 0, -0.003, 1))

	// 文字跳过计数。
	c.SetRGBA(0, 0, 0, 1)
	c.DrawString("SKIP", 10, 30)
	if c.projTextSkips == 0 {
		t.Fatal("文字跳过未被计数（E 组表态没有观测面）")
	}

	// 裁剪降级计数。
	c.ClipRect(0, 0, 100, 40)
	if c.projClipDegrades == 0 {
		t.Fatal("裁剪降级未被计数（C 组表态没有观测面）")
	}

	// 路径映射计数。
	c.NewSubPath()
	c.MoveTo(10, 10)
	c.LineTo(60, 10)
	c.LineTo(60, 50)
	c.ClosePath()
	c.SetRGBA(0.2, 0.4, 0.8, 1)
	if err := c.Fill(); err != nil {
		t.Fatalf("投影下填充：%v", err)
	}
	if c.projPathExpands == 0 {
		t.Fatal("路径映射未被计数（D 组表态没有观测面）")
	}

	// 刷烘焙降级计数（非实心刷才走 applyDeviceSpaceBrush；用渐变刷触发）。
	c.Push()
	gb := NewLinearGradientBrush(200, 0, 280, 0)
	gb.AddColorStop(0, RGBA{R: 1, A: 1})
	gb.AddColorStop(1, RGBA{B: 1, A: 1})
	c.SetFillBrush(gb)
	c.NewSubPath()
	c.MoveTo(200, 10)
	c.LineTo(280, 10)
	c.LineTo(280, 60)
	c.ClosePath()
	if err := c.Fill(); err != nil {
		t.Fatalf("投影下渐变填充：%v", err)
	}
	c.Pop()
	if c.projBrushDegrades == 0 {
		t.Fatal("刷烘焙降级未被计数（F 组表态没有观测面）")
	}
}

// 批 1d 存量回归钉（D6 状态钉抓到过的那一类）：投影层的保存/恢复绝不允许把
// **零值 Matrix4** 当成「已激活」装回上下文——零值的 IsIdentity()=false，一旦
// 装回，后续每次绘制都被全零矩阵映射到空（画面直接画白）。
func TestP37ProjectionZeroValueNeverActive(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	dc := NewContext(16, 16)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(White)
	dc.SetRGB(0, 0, 1)

	// 从未设过投影：Push/Pop 一轮之后投影位必须仍是关的，且照旧画得出蓝。
	dc.Push()
	dc.Pop()
	if dc.projectionActive {
		t.Fatal("Push/Pop 之后投影位被零值矩阵点亮")
	}
	if got := dc.Projection(); !got.IsIdentity() {
		t.Fatalf("Projection() 未返回恒等：%v", got)
	}
	dc.DrawRectangle(0, 0, 2, 2)
	if err := dc.Fill(); err != nil {
		t.Fatalf("fill: %v", err)
	}
	if r, g, b, _ := pixelAt(dc.PixmapForTest(), 1, 1); r > 40 || g > 40 || b < 200 {
		t.Fatalf("Push/Pop 之后蓝色填充丢失：(%d,%d,%d)", r, g, b)
	}

	// 零值显式入链也要当关闭（SetProjection 的文档口径）。
	dc.SetProjection(Matrix4Homography(Translate(10, 10), 0, -0.003, 1))
	if !dc.projectionActive {
		t.Fatal("正常投影未点亮投影位")
	}
	dc.SetProjection(Matrix4{})
	if dc.projectionActive {
		t.Fatal("零值 Matrix4 被当成有效投影装入")
	}
	if got := dc.Projection(); !got.IsIdentity() {
		t.Fatalf("零值入链后 Projection() 应归一为恒等：%v", got)
	}

	// 设过投影再 Push/Pop：必须原样恢复，不能丢也不能凭空激活。
	h := Matrix4Homography(Translate(10, 10), 0, -0.003, 1)
	dc.SetProjection(h)
	dc.Push()
	dc.SetProjection(Matrix4Identity())
	dc.Pop()
	if !dc.projectionActive {
		t.Fatal("Pop 没有把投影层恢复到 Push 前的值")
	}
	if dc.Projection() != h {
		t.Fatalf("Pop 恢复的投影不等于 Push 前：%v want %v", dc.Projection(), h)
	}
	// 空栈 Pop 只许 no-op。
	dc.SetProjection(Matrix4Identity())
	dc.PopProjection()
	if dc.projectionActive {
		t.Fatal("空栈 PopProjection 误置激活位")
	}
}
