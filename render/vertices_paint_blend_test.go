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

// P4.1 批 1（子项③）：顶点网格 paint alpha/blend 接线能力测试（先红后绿）。
//
// 场景数据 render/testdata/verts_paint_blend_cases.json：
//   - alpha_gouraud     逐顶点色 × 实色 paint alpha 0.5（Skia：paint alpha 在
//     primitive blend 后乘，SkGr.cpp:452-478）
//   - porter_duff_plus  Paint.BlendMode=BlendPlus 走固定函数混合（mesh 命令
//     BlendMode 接线 → pipelineForBlend）
//   - advanced_multiply Paint.BlendMode=BlendMultiply 走现有层封装（同
//     fillClipWithPaint 模式，不为 mesh 新建管线）
//   - solid_no_vc_alpha 实色无顶点色回归锁定（solid 复制路 alpha 原本就生效）

type paintBlendCase struct {
	Name       string       `json:"name"`
	Note       string       `json:"note"`
	Canvas     [2]int       `json:"canvas"`
	Background [4]float64   `json:"background"`
	PaintSolid [4]float64   `json:"paint_solid"`
	PaintBlend string       `json:"paint_blend"`
	Positions  [][2]float64 `json:"positions"`
	Colors     [][4]float64 `json:"colors"`
	Probes     []vertsProbe `json:"probes"`
	// GPU-vs-CPU whole-image diff gates (per-case; solid_no_vc_alpha carries
	// the allowed AA-edge-only mesh diff, see its note).
	DiffMaxChangedPct float64 `json:"diff_max_changed_pct"`
	DiffMaxDelta      int     `json:"diff_max_delta"`
}

type paintBlendCasesFile struct {
	Cases []paintBlendCase `json:"cases"`
}

func loadPaintBlendCases(t *testing.T) []paintBlendCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "verts_paint_blend_cases.json"))
	if err != nil {
		t.Fatalf("read testdata/verts_paint_blend_cases.json: %v", err)
	}
	var f paintBlendCasesFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode verts_paint_blend_cases.json: %v", err)
	}
	if len(f.Cases) == 0 {
		t.Fatal("verts_paint_blend_cases.json: no cases")
	}
	return f.Cases
}

func paintBlendMode(s string) render.BlendMode {
	switch s {
	case "plus":
		return render.BlendPlus
	case "multiply":
		return render.BlendMultiply
	default:
		return render.BlendNormal
	}
}

func runPaintBlendCase(t *testing.T, c paintBlendCase) *render.Context {
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
	dc.SetRGBA(c.PaintSolid[0], c.PaintSolid[1], c.PaintSolid[2], c.PaintSolid[3])
	dc.SetBlendMode(paintBlendMode(c.PaintBlend))
	dc.DrawVertices(
		vertsToPoints(c.Positions),
		vertsToColors(c.Colors),
		render.VertexModeTriangles,
	)
	return dc
}

// TestVerticesPaintBlendCasesCPU：强制 CPU 栅格跑全部场景。
func TestVerticesPaintBlendCasesCPU(t *testing.T) {
	withVertsCPU(t)
	for _, c := range loadPaintBlendCases(t) {
		t.Run(c.Name, func(t *testing.T) {
			dc := runPaintBlendCase(t, c)
			defer dc.Close()
			vertsCheckProbes(t, dc, c.Probes)
		})
	}
}

// TestVerticesPaintBlendCasesGPU：GPU 路径跑全部场景。
func TestVerticesPaintBlendCasesGPU(t *testing.T) {
	requireVertsGPU(t)
	for _, c := range loadPaintBlendCases(t) {
		t.Run(c.Name, func(t *testing.T) {
			dc := runPaintBlendCase(t, c)
			defer dc.Close()
			if err := dc.FlushGPU(); err != nil {
				if errors.Is(err, render.ErrFallbackToCPU) {
					t.Skipf("native GPU mesh path unavailable: %v", err)
				}
				t.Fatalf("flush: %v", err)
			}
			// 高级 blend 场景走层封装，层可能落 CPU 合成；其余必须 GPUOps>0。
			st := dc.RenderPathStats()
			if paintBlendMode(c.PaintBlend) == render.BlendNormal && st.GPUOps == 0 {
				t.Fatalf("expected GPU mesh ops, got %s", st.LogLine())
			}
			vertsCheckProbes(t, dc, c.Probes)
		})
	}
}

// TestVerticesPaintBlendCPUMatchesGPU：同场景 CPU 与 GPU 画面一致。
func TestVerticesPaintBlendCPUMatchesGPU(t *testing.T) {
	requireVertsGPU(t)
	for _, c := range loadPaintBlendCases(t) {
		t.Run(c.Name, func(t *testing.T) {
			gpu := runPaintBlendCase(t, c)
			defer gpu.Close()
			if err := gpu.FlushGPU(); err != nil {
				if errors.Is(err, render.ErrFallbackToCPU) {
					t.Skipf("native GPU mesh path unavailable: %v", err)
				}
				t.Fatalf("gpu flush: %v", err)
			}
			gpuImg := gpu.Image()

			t.Setenv("GOGPU_RENDER_MODE", "cpu")
			cpu := runPaintBlendCase(t, c)
			defer cpu.Close()
			cpuImg := cpu.Image()

			st, err := imagediff.Images(cpuImg, gpuImg)
			if err != nil {
				t.Fatalf("diff: %v", err)
			}
			total := int64(gpuImg.Bounds().Dx() * gpuImg.Bounds().Dy())
			maxDelta := c.DiffMaxDelta
			if maxDelta == 0 {
				maxDelta = 4
			}
			maxPct := c.DiffMaxChangedPct
			if maxPct == 0 {
				maxPct = 0.1
			}
			limit := int64(float64(total) * maxPct / 100)
			if int64(st.ChangedPixels) > limit || int(st.MaxDelta) > maxDelta {
				t.Fatalf("GPU vs CPU changed=%d/%d max=%d mean=%.3f, want <=%.2f%% and max<=%d",
					st.ChangedPixels, total, st.MaxDelta, st.MeanAbs, maxPct, maxDelta)
			}
		})
	}
}
