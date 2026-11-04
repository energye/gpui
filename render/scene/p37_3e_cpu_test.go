//go:build !nogpu

package scene

// P3.7 批 3e-1（包内测）：CPU 瓦片腿对投影的显式表态。
//
// Renderer.Render 一有加速器就先走 GPU 腿，所以「CPU 腿遇真投影拒画 + 计数」这条
// 判据只能直接驱动瓦片执行器（照 paint_correctness_test.go 的先例）。

import (
	"testing"

	"github.com/energye/gpui/render"
)

// cpuTileScene 装一份投影再录一块红矩形（与外部钉同一形态：投影只管它之后的内容）。
func cpuTileScene(m render.Matrix4) *Scene {
	s := NewScene()
	s.SetProjection(m)
	s.Fill(FillNonZero, IdentityAffine(), SolidBrush(render.RGBA{R: 1, A: 1}),
		NewRectShape(4, 4, 24, 16))
	return s
}

// renderAllTiles 用 CPU 瓦片腿把编码画进整张网格，返回是否有任一像素非零。
func renderAllTiles(t *testing.T, r *Renderer, s *Scene) (painted bool) {
	t.Helper()
	enc := s.Encoding()
	images := s.Images()
	for ty := 0; ty*64 <= r.height; ty++ {
		for tx := 0; tx*64 <= r.width; tx++ {
			tile := r.tileGrid.TileAt(tx, ty)
			if tile == nil || tile.Width <= 0 || tile.Height <= 0 {
				continue
			}
			r.renderTile(tile, enc, nil, images)
			for _, b := range tile.Data[:tile.Width*tile.Height*4] {
				if b != 0 {
					painted = true
				}
			}
		}
	}
	return painted
}

func projectiveProbe() render.Matrix4 {
	return render.Matrix4FromElements([16]float64{
		1, 0, 0.125, 0,
		0, 1, 0.25, 0,
		0, 0, 1, 0,
		0, 0.03125, 0, 1,
	})
}

// 仿射可表达的投影：折进烘焙链，必须画得出来，且不计数。
func TestCPUTileAffineLiftProjectionDraws(t *testing.T) {
	r := NewRenderer(64, 64, WithWorkers(1))
	lift := render.Matrix4FromMatrix(render.Matrix{A: 1, B: 0, C: 8, D: 0, E: 1, F: 6})
	if !renderAllTiles(t, r, cpuTileScene(lift)) {
		t.Fatal("仿射提升的投影下 CPU 腿什么都没画")
	}
	if got := r.ProjectionDegrades(); got != 0 {
		t.Fatalf("仿射可表达不该计数：%d", got)
	}
}

// 真投影：CPU 腿没有透视采样 ⇒ 整笔拒画 + 具名计数（绝不静默压扁成仿射）。
func TestCPUTileProjectiveProjectionRefusesAndCounts(t *testing.T) {
	r := NewRenderer(64, 64, WithWorkers(1))
	if renderAllTiles(t, r, cpuTileScene(projectiveProbe())) {
		t.Fatal("真投影下 CPU 腿画出了像素：要么静默压扁，要么拒画没生效")
	}
	if got := r.ProjectionDegrades(); got == 0 {
		t.Fatal("拒画没计数：表态丢了（AGENTS 禁止静默降级）")
	} else {
		t.Logf("projective refusals = %d", got)
	}
	r.ResetProjectionDegrades()
	if got := r.ProjectionDegrades(); got != 0 {
		t.Fatalf("ResetProjectionDegrades 没清零：%d", got)
	}
	// 恒等投影：既不拒画也不计数（存量路径逐位不变）。
	if got := r.ProjectionDegrades(); got != 0 {
		t.Fatalf("恒等不该计数：%d", got)
	}
	if !renderAllTiles(t, r, cpuTileScene(render.Matrix4Identity())) {
		t.Fatal("恒等投影下 CPU 腿应当照常画出内容")
	}
	if got := r.ProjectionDegrades(); got != 0 {
		t.Fatalf("恒等投影被计数了：%d", got)
	}
}

// cpuProjectionState 是本文件两条判据的纯函数面：三态划分不许含糊。
func TestCPUTileProjectionStateThreeCases(t *testing.T) {
	if aff, projective := cpuProjectionState(render.Matrix4{}); projective || !aff.IsIdentity() {
		t.Fatalf("零值应按恒等：%+v projective=%v", aff, projective)
	}
	if aff, projective := cpuProjectionState(render.Matrix4Identity()); projective || !aff.IsIdentity() {
		t.Fatalf("恒等不该 projective：%+v projective=%v", aff, projective)
	}
	lift := render.Matrix4FromMatrix(render.Matrix{A: 2, B: 0, C: 3, D: 0, E: 2, F: 5})
	aff, projective := cpuProjectionState(lift)
	if projective || aff.A != 2 || aff.C != 3 || aff.F != 5 {
		t.Fatalf("仿射提升丢了：%+v projective=%v", aff, projective)
	}
	if _, projective := cpuProjectionState(projectiveProbe()); !projective {
		t.Fatal("真透视被当成可仿射表达（会静默压扁）")
	}
}
