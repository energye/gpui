//go:build !nogpu

package render_test

// fpdiag2 — O13 wgpu 专项批深挖诊断：复刻 framepresent 窗 offleg.go 的完整
// 复杂会话序列（v1 base → var blit 源 → 单 UV blit+damage → multi UV blit+
// 双矩形 damage → 全面 UV blit+damage → plan → full → auto），逐点读回打印
// 实测值，对照 v1 base / v1 bar / qBR / qTL 的期望值域，判定：
//   - multi-untouched (40,170)：期望 v1Bar（y∈[168,182) 底部横条）还是变暗？
//   - scissor-leak (240,150)：multi 腿 R blit（damage R 内）已合法画上 qBR，
//     探针的「保留 v1Base」期望是否只是比较了陈旧基线？
// 每个场景打印 got/want 三通道，形成实证矩阵。

import (
	"fmt"
	"image"
	"os"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

func fp2Sample(t *testing.T, dc *render.Context, label string, pts map[string][2]int) {
	t.Helper()
	img := dc.Image()
	for n, p := range pts {
		r, g, b, _ := img.At(p[0], p[1]).RGBA()
		fmt.Fprintf(os.Stderr, "fpdiag2 %-22s %s(%d,%d) = (%3d,%3d,%3d)\n",
			label+":", n, p[0], p[1], r>>8, g>>8, b>>8)
	}
}

func TestFPDiag2_OffLegSequence(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 320, 200
	v1Base := [3]float64{0.30, 0.34, 0.55} // (76,87,140)
	v1Bar := [3]float64{0.16, 0.18, 0.30}  // (41,46,77)
	qBR := [3]float64{0.90, 0.60, 0.15}    // (229,153,38)
	offRectL := image.Rect(24, 40, 128, 120)
	offRectR := image.Rect(200, 96, 296, 176)

	t.Logf("expect v1Base=%v v1Bar=%v qBR=%v", v1Base, v1Bar, qBR)

	dc := render.NewContextWithScale(w, h, 1)
	defer dc.Close()

	v1View, relV1 := dc.CreateOffscreenTexture(w, h)
	if v1View.IsNil() {
		t.Skip("no offscreen")
	}
	defer relV1()

	// v1 base: full fill + two horizontal bars (y 20-34 and y 168-182).
	dc.BeginGPUFrame()
	p1FillRect(dc, 0, 0, w, h, v1Base, 1)
	p1FillRect(dc, 0, 20, w, 14, v1Bar, 1)
	p1FillRect(dc, 0, 168, w, 14, v1Bar, 1)
	if err := dc.PresentFrame(v1View, w, h, nil); err != nil {
		t.Fatalf("v1 present: %v", err)
	}
	fp2Sample(t, dc, "s0 v1 base", map[string][2]int{
		"mid(40,170)": {40, 170},
		"TR(240,150)": {240, 150},
		"L(76,80)":    {76, 80},
	})

	varView, relVar := dc.CreateOffscreenTexture(w, h)
	defer relVar()
	dc.BeginGPUFrame()
	p1FillRect(dc, 0, 0, w/2, h/2, [3]float64{0.20, 0.70, 0.40}, 1)
	p1FillRect(dc, w/2, 0, w/2, h/2, [3]float64{0.75, 0.25, 0.45}, 1)
	p1FillRect(dc, 0, h/2, w/2, h/2, [3]float64{0.25, 0.35, 0.80}, 1)
	p1FillRect(dc, w/2, h/2, w/2, h/2, qBR, 1)
	if err := dc.PresentFrame(varView, w, h, nil); err != nil {
		t.Fatalf("var present: %v", err)
	}

	// s1: single UV blit (TR quadrant) into bottom-right, damage L — away
	// from the blit dst. L + corner + TR must keep v1.
	dc.DrawGPUTextureWithOpacityUV(varView, 160, 100, 160, 100, 1.0, 0.5, 0, 1, 0.5)
	if err := dc.PresentFrameDamage(v1View, w, h, offRectL, nil); err != nil {
		t.Fatalf("s1 present: %v", err)
	}
	fp2Sample(t, dc, "s1 single", map[string][2]int{
		"L保留(76,80)":   {76, 80},
		"角保留(5,5)":     {5, 5},
		"TR保留(240,150)": {240, 150},
		"blit落点(200,130)": {200, 130},
	})

	// s2: multi UV blits INSIDE their rects, damage L+R.
	dc.DrawGPUTextureWithOpacityUV(varView, float64(offRectL.Min.X), float64(offRectL.Min.Y),
		offRectL.Dx(), offRectL.Dy(), 1.0, 0, 0, 0.5, 0.5)
	dc.DrawGPUTextureWithOpacityUV(varView, float64(offRectR.Min.X), float64(offRectR.Min.Y),
		offRectR.Dx(), offRectR.Dy(), 1.0, 0.5, 0.5, 1, 1)
	if err := dc.PresentFrameDamageRects(v1View, w, h,
		[]image.Rectangle{offRectL, offRectR}, nil); err != nil {
		t.Fatalf("s2 present: %v", err)
	}
	fp2Sample(t, dc, "s2 multi", map[string][2]int{
		"L落点(76,80)":       {76, 80},
		"R落点(248,136)":     {248, 136},
		"未触(40,170)":       {40, 170},
		"未触(40,100)":       {40, 100},
		"未触(160,60)":       {160, 60},
		"R内s1blit(240,150)": {240, 150},
	})

	// s3: whole-view UV blit (BR quadrant stretched) + damage L. Outside-L:
	// (240,150) is INSIDE offRectR — multi's R blit already legally painted
	// qBR there. The honest untouched points are (40,100)/(160,60).
	dc.DrawGPUTextureWithOpacityUV(varView, 0, 0, w, h, 1.0, 0.5, 0.5, 1, 1)
	if err := dc.PresentFrameDamage(v1View, w, h, offRectL, nil); err != nil {
		t.Fatalf("s3 present: %v", err)
	}
	fp2Sample(t, dc, "s3 full-blit", map[string][2]int{
		"L外(240,150)": {240, 150},
		"L外(40,100)":  {40, 100},
		"L外(160,60)":  {160, 60},
		"L内(76,80)":   {76, 80},
	})
}

func p1FillRect(dc *render.Context, x, y, w, h float64, c [3]float64, alpha float64) {
	dc.SetRGBA(c[0], c[1], c[2], alpha)
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Fill()
}
