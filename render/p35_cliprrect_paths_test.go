//go:build !nogpu

package render_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

// 深审 L3-A3（D12，方案 F 拍板 2026-11-03）钉：ClipRRect 分腿必须跨守卫连续。
// 三腿格局：
//   - 快路（轴对齐·严格等比）：设备 AABB + 原圆半径（历史逐位语义，不许变）。
//   - 解析定向腿（方案 F 扩面）：轴对齐非等比 → 椭圆解析（真形语义，与旧路径
//     栅格逐位吻合）；旋转等比 → 角度解析（与快路同式同参）。
//   - 路径栅格腿：四角异径/九宫格/复杂型，维持精确几何。
// 修复前实测（证据登记）：r=2 微姿态两腿差 17/69/255、旋转 0.01° 差 96/255。

// d12Coverage 在指定 CTM 姿态下画圆角裁剪并取右下角弧带的覆盖亮度。
func d12Coverage(t *testing.T, rr render.RRect, skew, rotDeg float64) map[[2]int]uint8 {
	t.Helper()
	dc := render.NewContext(48, 48)
	defer dc.Close()
	dc.ClearWithColor(render.Black)
	if skew != 0 {
		dc.Scale(1, 1+skew)
	}
	if rotDeg != 0 {
		dc.RotateAbout(math.Pi*rotDeg/180, 24, 24)
	}
	dc.ClipRRect(rr)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, 48, 48)
	if err := dc.Fill(); err != nil {
		t.Fatalf("fill: %v", err)
	}
	pm := dc.PixmapForTest()
	data := pm.Data()
	pts := [][2]int{{43, 43}, {44, 43}, {45, 43}, {43, 44}, {43, 45}, {44, 44}, {45, 44}, {44, 45}}
	out := map[[2]int]uint8{}
	for _, p := range pts {
		i := (p[1]*pm.Width() + p[0]) * 4
		out[p] = data[i]
	}
	return out
}

func d12MaxDiff(a, b map[[2]int]uint8) (int, int) {
	maxDiff, nDiff := 0, 0
	for p, av := range a {
		d := int(av) - int(b[p])
		if d < 0 {
			d = -d
		}
		if d > 0 {
			nDiff++
		}
		if d > maxDiff {
			maxDiff = d
		}
	}
	return maxDiff, nDiff
}

// F1 跨守卫连续（可修面）：微旋转与严格均匀逐位一致（解析腿=快路同式同参）。
// 缩放跨守卫（均匀→非等比 ε→0）存在 96/255 单像素跳：那是快路「AABB+原圆
// 半径」的均匀语义与真形语义在像素中心边界的量化差（几何差仅 3e-8px），
// 消灭它必须放弃快路均匀语义（会改存量 UI 的圆角裁剪像素）——已按方案 F 拍板
// 保留，此处只观测不断言，登记见证据 D12。
func TestD12ClipRRectGuardContinuity(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	for _, r := range []float64{2, 8, 12, 30} {
		rr := render.NewRRectXY(render.Rect{Min: render.Pt(4, 4), Max: render.Pt(44, 44)}, r, r)
		base := d12Coverage(t, rr, 0, 0)
		maxB, nB := d12MaxDiff(base, d12Coverage(t, rr, 0, 0.01))
		maxS, nS := d12MaxDiff(base, d12Coverage(t, rr, 1e-9, 0))
		t.Logf("r=%-4.0f 微旋转 %d/%d 差≤%d（断言≤2）；微非等比 %d/%d 差≤%d（观测：快路均匀语义固有量化差）",
			r, nB, len(base), maxB, nS, len(base), maxS)
		if maxB > 2 {
			t.Errorf("r=%v: 旋转跨守卫覆盖跳变 %d > 2/255（解析腿未接住）", r, maxB)
		}
	}
}

// F2 存量零变动：近均匀（守卫容差内）与严格均匀逐位一致——快路像素不许动。
func TestD12ClipRRectUniformPixelsUnchanged(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	for _, r := range []float64{2, 8, 12, 30} {
		rr := render.NewRRectXY(render.Rect{Min: render.Pt(4, 4), Max: render.Pt(44, 44)}, r, r)
		a := d12Coverage(t, rr, 0, 0)
		b := d12Coverage(t, rr, 1e-13, 0) // 仍在快路守卫容差内
		if maxDiff, nDiff := d12MaxDiff(a, b); nDiff != 0 {
			t.Errorf("r=%v: 近均匀微扰动了 %d 像素（最大差 %d）— 快路守卫被放宽/收紧，存量像素会变", r, nDiff, maxDiff)
		}
	}
}

// F3 真形推进留档 + 边界钉：非等比变形扫掠（0.1%→5%）逐像素序列钉住解析腿行为。
// 对照表（(43,43)，实测）：
//
//	sk:         1e-9   0.001  0.01  0.02  0.05
//	解析腿:       0      6     176   224   127   ← 本测试钉
//	旧栅格腿:      0      2     109   187   127   ← 负向对照（腿摘除）
//
// 中段差 ≤67 是「user-space cubic 展平后设备变形」对真椭圆弧的近似误差（旧栅格
// 固有，非本腿引入；ε→0 与 ε=5% 边界两处逐位重合）。L3-A3 的主诉（同一几何跨
// 守卫换表示导致跳变）已由解析腿消除；栅格 cubic 近似的残余属另一码事，登记另议。
// 禁止项：发散/非法值、5% 形变后角弧不回扫 (43,44)。
func TestD12ClipRRectDeformContinuity(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	steps := []float64{1e-9, 0.001, 0.01, 0.02, 0.05}
	want4343 := []uint8{0, 6, 176, 224, 127}
	for _, r := range []float64{2, 8, 12, 30} {
		rr := render.NewRRectXY(render.Rect{Min: render.Pt(4, 4), Max: render.Pt(44, 44)}, r, r)
		seq := make([]map[[2]int]uint8, 0, len(steps))
		for _, sk := range steps {
			seq = append(seq, d12Coverage(t, rr, sk, 0))
		}
		if r == 2 {
			for i, p := range seq {
				if got := p[[2]int{43, 43}]; got != want4343[i] {
					t.Errorf("r=2 sk=%g: (43,43)=%d want %d（raster 语义保持钉，实测真值表见注释）", steps[i], got, want4343[i])
				}
			}
		}
		last := seq[len(seq)-1]
		if r == 2 && last[[2]int{43, 44}] == 0 {
			t.Errorf("r=2: 5%% 形变后角弧应扫过 (43,44)（覆盖应>0），得 0——弧族行为发散")
		}
	}
}
