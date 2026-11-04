//go:build !nogpu

package render_test

// P3.7 批 3a 前置探针 P3 的生产腿对照（internal/gpu 侧探针文件的姊妹档）：
// 同一份梯度 shader 走生产填充路（P3.2 既有 RT 读回链，p32 同款 harness），
// 对 internal/gpu 探针 quad 片元腿的解析期望逐像素对拍，实测两路真实差；
// 棋盘图走生产 DrawImage 出逐位摘要，与 internal/gpu 恒等材质腿跨包对拍。
// 证据落 evidence/P3.7/batch3a/probe-p3-premul.txt。

import (
	"crypto/sha256"
	"fmt"
	"image"
	"math"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

// 与 internal/gpu/p37_3a_probe_test.go 保持逐字同式（探针判据源，改要两边一起改）。
const p37AProbeGradWGSL = `@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let a = 0.3 + 0.6 * uv.x;
    return vec4<f32>(0.8 * a, 0.35 * a, 0.1 * a, a);
}`

func p37AProbeGradAnalytic(u float64) (r, g, b, a float64) {
	a = 0.3 + 0.6*u
	return 0.8 * a, 0.35 * a, 0.1 * a, a
}

func p37AProbeRound8(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 255
	}
	return uint8(math.Round(v * 255))
}

func p37AProbeGradOverWhite(r, g, b, a float64) [4]uint8 {
	return [4]uint8{p37AProbeRound8(r + 1 - a), p37AProbeRound8(g + 1 - a), p37AProbeRound8(b + 1 - a), p37AProbeRound8(a + 1 - a)}
}

// TestP37AProbeFillLegGradient 生产填充腿（RT 读回链）premul 契约钉：修复后
// fillShaderAsImage 交 premul 字节（与图像腿 level0 原样上传 + premul 合成链同一
// 语义），生产填充腿对 quad 片元腿解析期望逐像素一致（±1 = 修复前 u8 舍入差已被
// 消除，本钉按 ±1 收紧防采样链微差）。修复前本钉锁的是回绕行为（2026-11-05
// 探针实测，证据 batch3a/probe-p3-premul.txt §四）。
func TestP37AProbeFillLegGradient(t *testing.T) {
	requireFilterOpsGPU(t)
	e, err := render.NewShaderEffect(p37AProbeGradWGSL)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	defer e.Close()
	dc := render.NewContext(32, 32)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.White)
	dc.SetShader(e)
	dc.DrawRectangle(4, 4, 24, 24) // 与 internal/gpu 探针 quad 腿同一设备矩形
	if err := dc.Fill(); err != nil {
		t.Fatalf("Fill: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}

	interior := make([]byte, 0, 24*24*4)
	matchN, offN := 0, 0
	worst, worstAt := 0, ""
	for py := 4; py < 28; py++ {
		for px := 4; px < 28; px++ {
			u := (float64(px) + 0.5 - 4.0) / 24.0
			r, g, b, a := p37AProbeGradAnalytic(u)
			want := p37AProbeGradOverWhite(r, g, b, a)
			ir, ig, ib, ia := dc.Image().At(px, py).RGBA()
			got := [4]uint8{uint8(ir >> 8), uint8(ig >> 8), uint8(ib >> 8), uint8(ia >> 8)}
			interior = append(interior, got[0], got[1], got[2], got[3])
			match := true
			for c := 0; c < 4; c++ {
				d := int(got[c]) - int(want[c])
				if d < 0 {
					d = -d
				}
				if d > 1 { // u8 量化链 ±1
					match = false
					if d > worst {
						worst, worstAt = d, fmt.Sprintf("(%d,%d)c%d", px, py, c)
					}
				}
			}
			if match {
				matchN++
			} else {
				offN++
			}
		}
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(interior))
	t.Logf("probe fill leg: premul-contract match=%d off=%d worst=%d at %s interior digest=%s", matchN, offN, worst, worstAt, digest[:16])
	if offN > 0 {
		t.Fatalf("fill leg premul contract broken: %d/%d pixels off the premul-src-over model, worst %d at %s — 填充腿 premul 交接回归？", offN, matchN+offN, worst, worstAt)
	}
}

// TestP37AProbeImageCheckerDigest 生产 DrawImage 腿的棋盘逐位摘要：与
// internal/gpu 恒等材质腿共享同一字节面（sha256 一致 = 组合腿与生产图像腿逐位同）。
func TestP37AProbeImageCheckerDigest(t *testing.T) {
	requireFilterOpsGPU(t)
	const imgW, imgH = 16, 16
	m := image.NewNRGBA(image.Rect(0, 0, imgW, imgH))
	for y := 0; y < imgH; y++ {
		for x := 0; x < imgW; x++ {
			o := y*m.Stride + x*4
			m.Pix[o] = uint8(40 + ((x*7 + y*13) % 200))
			m.Pix[o+1] = uint8(30 + ((x*11 + y*5) % 220))
			m.Pix[o+2] = uint8(20 + ((x*3 + y*17) % 230))
			m.Pix[o+3] = 255
		}
	}
	buf := render.ImageBufFromImage(m)

	dc := render.NewContext(32, 32)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.White)
	dc.DrawImage(buf, 4, 4) // 1:1，设备矩形 (4,4)-(20,20)，与 internal/gpu 探针恒等腿同矩形
	if err := dc.Fill(); err != nil {
		t.Fatalf("Fill: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}

	interior := make([]byte, 0, imgW*imgH*4)
	nonWhite := 0
	for py := 4; py < 4+imgH; py++ {
		for px := 4; px < 4+imgW; px++ {
			ir, ig, ib, ia := dc.Image().At(px, py).RGBA()
			if ir>>8 != 255 || ig>>8 != 255 || ib>>8 != 255 {
				nonWhite++
			}
			interior = append(interior, uint8(ir>>8), uint8(ig>>8), uint8(ib>>8), uint8(ia>>8))
		}
	}
	if nonWhite == 0 {
		t.Fatalf("production DrawImage painted nothing — harness broken")
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(interior))
	t.Logf("probe image leg: interior digest=%s nonWhite=%d", digest[:16], nonWhite)
}
