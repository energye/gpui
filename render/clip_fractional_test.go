//go:build !nogpu

package render_test

//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/render/internal/testutil/imagediff"
)

// P4.1 批 2（子项②）：分数裁剪矩形 AA 能力测试（先红后绿）。
//
// 现状（Skia PLAN §6 登记的半成品）：非整数裁剪矩形在 GPU 走整数 scissor
// Floor/Ceil 外扩（溢出内容无 AA 硬边）、在 CPU 走 bounds Contains 二值判定
// （边缘无分数覆盖）——Skia 对非整数矩形裁剪走 analytic coverage（ClipStack.cpp
// 总策略 analytic FP → stencil → atlas → SW mask）。修法：设备矩形带分数时
// 升级为 Radius=0 的 rrect entry（rrectCoverage/GPU WGSL SDF 对 r=0 自然退化，
// 0.75px smoothstep AA），整数矩形保持纯 scissor 零变化。
//
// 场景数据 render/testdata/clip_fractional_cases.json：clip 边缘 10.5 半覆盖
// 像素 10（中心 10.5），像素 9 保持白，像素 11 全内；integer_rect_lock 锁定
// 整数 clip 的像素对齐硬边语义零变化。

type clipFractionalCase struct {
	Name       string       `json:"name"`
	Note       string       `json:"note"`
	Canvas     [2]int       `json:"canvas"`
	Background [4]float64   `json:"background"`
	Clip       [4]float64   `json:"clip"`
	Fill       [4]float64   `json:"fill"`
	Op         string       `json:"op"`
	Probes     []vertsProbe `json:"probes"`
}

type clipFractionalCasesFile struct {
	Cases []clipFractionalCase `json:"cases"`
}

func loadClipFractionalCases(t *testing.T) []clipFractionalCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "clip_fractional_cases.json"))
	if err != nil {
		t.Fatalf("read testdata/clip_fractional_cases.json: %v", err)
	}
	var f clipFractionalCasesFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode clip_fractional_cases.json: %v", err)
	}
	if len(f.Cases) == 0 {
		t.Fatal("clip_fractional_cases.json: no cases")
	}
	return f.Cases
}

func runClipFractionalCase(t *testing.T, c clipFractionalCase) *render.Context {
	t.Helper()
	dc := render.NewContext(c.Canvas[0], c.Canvas[1])
	defer func() {
		if t.Failed() {
			dc.Close()
		}
	}()
	dc.ClearWithColor(render.RGBA{
		R: c.Background[0], G: c.Background[1], B: c.Background[2], A: c.Background[3],
	})
	if c.Op == "replace" {
		dc.ClipRectOp(c.Clip[0], c.Clip[1], c.Clip[2], c.Clip[3], render.ClipOpReplace)
	} else {
		dc.ClipRect(c.Clip[0], c.Clip[1], c.Clip[2], c.Clip[3])
	}
	dc.SetRGBA(c.Fill[0], c.Fill[1], c.Fill[2], c.Fill[3])
	dc.DrawRectangle(0, 0, float64(c.Canvas[0]), float64(c.Canvas[1]))
	_ = dc.Fill()
	return dc
}

// TestClipFractionalCasesCPU：强制 CPU 栅格跑全部场景。
func TestClipFractionalCasesCPU(t *testing.T) {
	withVertsCPU(t)
	for _, c := range loadClipFractionalCases(t) {
		t.Run(c.Name, func(t *testing.T) {
			dc := runClipFractionalCase(t, c)
			defer dc.Close()
			vertsCheckProbes(t, dc, c.Probes)
		})
	}
}

// TestClipFractionalCasesGPU：GPU 路径跑全部场景。
func TestClipFractionalCasesGPU(t *testing.T) {
	requireVertsGPU(t)
	for _, c := range loadClipFractionalCases(t) {
		t.Run(c.Name, func(t *testing.T) {
			dc := runClipFractionalCase(t, c)
			defer dc.Close()
			if err := dc.FlushGPU(); err != nil {
				if errors.Is(err, render.ErrFallbackToCPU) {
					t.Skipf("native GPU path unavailable: %v", err)
				}
				t.Fatalf("flush: %v", err)
			}
			vertsCheckProbes(t, dc, c.Probes)
		})
	}
}

// TestClipFractionalCPUMatchesGPU：同场景 CPU 与 GPU 画面一致。
func TestClipFractionalCPUMatchesGPU(t *testing.T) {
	requireVertsGPU(t)
	for _, c := range loadClipFractionalCases(t) {
		t.Run(c.Name, func(t *testing.T) {
			gpu := runClipFractionalCase(t, c)
			defer gpu.Close()
			if err := gpu.FlushGPU(); err != nil {
				if errors.Is(err, render.ErrFallbackToCPU) {
					t.Skipf("native GPU path unavailable: %v", err)
				}
				t.Fatalf("gpu flush: %v", err)
			}
			gpuImg := gpu.Image()

			t.Setenv("GOGPU_RENDER_MODE", "cpu")
			cpu := runClipFractionalCase(t, c)
			defer cpu.Close()
			cpuImg := cpu.Image()

			st, err := imagediff.Images(cpuImg, gpuImg)
			if err != nil {
				t.Fatalf("diff: %v", err)
			}
			total := int64(gpuImg.Bounds().Dx() * gpuImg.Bounds().Dy())
			// AA 边缘允许小差异（GPU smoothstep vs CPU rrectCoverage 同为 0.75px
			// smoothstep，数值实现舍入差异小）；内部逐位一致。
			if int64(st.ChangedPixels) > total/200 || st.MaxDelta > 6 {
				t.Fatalf("GPU vs CPU changed=%d/%d max=%d mean=%.3f, want <=0.5%% and max<=6",
					st.ChangedPixels, total, st.MaxDelta, st.MeanAbs)
			}
		})
	}
}
