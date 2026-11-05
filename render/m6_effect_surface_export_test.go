//go:build !nogpu

package render_test

// M6 长期钉：`SetEffectSurface(true)` 那条路下，导出面（`ExportImageBuf`）必须与不开
// 效果面时**逐通道等值**。原取证见 `evidence/P3.7/m6-export-byteorder.md`（一次性探针，
// 本批转正成钉）。
//
// 根因一句话：把 GPU 面同步进 pixmap 时，选哪个读回函数是按「接口能不能用 + 失败再退」
// 硬编的（先试 BGRA 翻转读口，报错或长度不足才退 RGBA 直取），而不是按**那张面真实的格式**；
// 效果面那条路发布的是 RGBA8Unorm，被当成 BGRA8 翻了一次 ⇒ 红蓝整体互换，
// 绿与 alpha 不动、白底不变（所以画面肉眼像是对的）。

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// m6DrawExport 画一次「白底 + 红块 + 蓝块」，按 effectSurface 两档分别导出，读回采样点。
func m6DrawExport(t *testing.T, effectSurface bool) [3][4]uint8 {
	t.Helper()
	dc := render.NewContext(16, 16)
	defer dc.Close()
	dc.SetEffectSurface(effectSurface)
	dc.ClearWithColor(render.White)
	dc.SetRGBA(1, 0, 0, 1)
	dc.DrawRectangle(2, 2, 8, 8)
	dc.Fill()
	dc.SetRGBA(0, 0, 1, 1)
	dc.DrawRectangle(12, 12, 3, 3)
	dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("effectSurface=%v FlushGPU: %v", effectSurface, err)
	}
	st := dc.RenderPathStats()
	if st.GPUOps == 0 || st.CPUFallbackOps != 0 {
		t.Fatalf("effectSurface=%v 没真走 GPU（gpu_ops=%d cpu_fallback=%d last=%q）：两档对照不成立",
			effectSurface, st.GPUOps, st.CPUFallbackOps, st.LastCPUFallbackReason)
	}
	var img *render.ImageBuf
	if !dc.ExportImageBuf(&img) || img == nil {
		t.Fatalf("effectSurface=%v ExportImageBuf 失败", effectSurface)
	}
	defer img.Dispose()
	data, stride := img.Data(), img.Stride()
	if stride < 16*4 {
		t.Fatalf("effectSurface=%v stride=%d，容不下 16 像素/行", effectSurface, stride)
	}
	var out [3][4]uint8
	for i, p := range [][2]int{{1, 1}, {5, 5}, {13, 13}} { // 白底 / 红块 / 蓝块
		off := p[1]*stride + p[0]*4
		out[i] = [4]uint8{data[off], data[off+1], data[off+2], data[off+3]}
	}
	return out
}

// 钉 M6-1：两档导出必须逐通道等值，且红/蓝各归其位（画面对，不是数值巧合）。
func TestM6EffectSurfaceExportKeepsChannelOrder(t *testing.T) {
	requireNativeGPU(t)
	plain := m6DrawExport(t, false)
	effect := m6DrawExport(t, true)

	// 绘制落位自检：非基准档也得先证明红蓝没串（否则「两档相等」可以和「两档都错」同时成立）。
	if plain[1][0] < 200 || plain[1][2] > 60 {
		t.Fatalf("基准档红块采样(%d,%d,%d,%d)：绘制本身没落位", plain[1][0], plain[1][1], plain[1][2], plain[1][3])
	}
	if plain[2][2] < 200 || plain[2][0] > 60 {
		t.Fatalf("基准档蓝块采样(%d,%d,%d,%d)：绘制本身没落位", plain[2][0], plain[2][1], plain[2][2], plain[2][3])
	}

	names := []string{"白底", "红块", "蓝块"}
	for i := range plain {
		if plain[i] != effect[i] {
			t.Fatalf("%s 两档导出不等值：effectSurface=false %v / true %v —— 效果面那条路被多翻（或少翻）了一次通道",
				names[i], plain[i], effect[i])
		}
	}
}

// 钉 M6-2：格式声明随读回一起出来——同一效果面重复导出必须逐位稳定（幂等），
// 且红蓝在任何一档都不许互换（把「只靠 fallback 侥幸对上一次」的形状钉死）。
func TestM6EffectSurfaceExportIsRepeatable(t *testing.T) {
	requireNativeGPU(t)
	for _, effectSurface := range []bool{false, true} {
		first := m6DrawExport(t, effectSurface)
		second := m6DrawExport(t, effectSurface)
		if first != second {
			t.Fatalf("effectSurface=%v 重复导出逐位不稳：%v vs %v", effectSurface, first, second)
		}
		if first[1][0] < 200 || first[1][2] > 60 {
			t.Fatalf("effectSurface=%v 红块被读成 (%d,%d,%d)：R/B 互换未修净", effectSurface, first[1][0], first[1][1], first[1][2])
		}
		if first[2][2] < 200 || first[2][0] > 60 {
			t.Fatalf("effectSurface=%v 蓝块被读成 (%d,%d,%d)：R/B 互换未修净", effectSurface, first[2][0], first[2][1], first[2][2])
		}
	}
}
