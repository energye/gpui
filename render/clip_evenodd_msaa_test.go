//go:build !nogpu

package render_test

//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
//----------------------------------------

// P4.1 批 4（子项①）：MSAA>1 下 even-odd 裁剪孔洞能力测试（先红后绿）。
//
// 现状（PLAN §6 登记，depth_clip.go:625 实测）：sampleCount>1 时 BuildClipMask
// 跳过（R8 coverage mask 是唯一认 even-odd 的路），RecordDraw 的 stencil 填充
// 只有 winding 规则（IncrWrap/DecrWrap）——even-odd 孔洞区域 winding=2≠0 →
// 深度写入 → 孔洞内容被错误保留。
//
// Skia 标准（evidence/P4.1/assessment.md §2）：MSAA 下 even-odd 走 stencil
// INVERT——每 fragment 翻转 stencil（gEOStencilPass=kInvert），盖奇数次=1 画、
// 偶次=0 镂空；winding 保持 IncrWrap/DecrWrap（零变化）；cover 阶段以
// stencil NotEqual-0 门控写深度（gEOColorPass=kNotEqual，现有 Phase 2 已是）。
//
// 场景数据 render/testdata/clip_evenodd_msaa_cases.json：
// evenodd_ring_clip_msaa4   双同心圆 even-odd 环孔（红测：孔中心=红=洞没挖出）
// winding_ring_clip_msaa4_lock  winding 锁定卡（修 even-odd 不得动 winding）
//
// 硬前置：SetMSAASampleCount 先于 GPU 会话创建（同 gles_msaa_resolve_test.go）。

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

type clipEvenOddMSAACase struct {
	Name        string       `json:"name"`
	Note        string       `json:"note"`
	Canvas      [2]int       `json:"canvas"`
	Background  [4]float64   `json:"background"`
	ClipCircles [][3]float64 `json:"clip_circles"`
	FillRule    string       `json:"fill_rule"`
	Fill        [4]float64   `json:"fill"`
	FillRect    [4]float64   `json:"fill_rect"`
	Probes      []vertsProbe `json:"probes"`
}

type clipEvenOddMSAACasesFile struct {
	Cases []clipEvenOddMSAACase `json:"cases"`
}

func loadClipEvenOddMSAACases(t *testing.T) []clipEvenOddMSAACase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "clip_evenodd_msaa_cases.json"))
	if err != nil {
		t.Fatalf("read testdata/clip_evenodd_msaa_cases.json: %v", err)
	}
	var f clipEvenOddMSAACasesFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode clip_evenodd_msaa_cases.json: %v", err)
	}
	if len(f.Cases) == 0 {
		t.Fatal("clip_evenodd_msaa_cases.json: no cases")
	}
	return f.Cases
}

func runClipEvenOddMSAACase(t *testing.T, c clipEvenOddMSAACase) *render.Context {
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
	// 裁剪路径：多同心圆。构建后 SetFillRule 决定 clip 的填充规则
	// （context_clip.go：Clip 读 paint.FillRule → clipStack.PushPath + gpuClipFillRule）。
	clipPath := render.NewPath()
	for _, circ := range c.ClipCircles {
		clipPath.Circle(circ[0], circ[1], circ[2])
	}
	dc.SetPath(clipPath)
	switch c.FillRule {
	case "evenodd":
		dc.SetFillRule(render.FillRuleEvenOdd)
	case "nonzero":
		dc.SetFillRule(render.FillRuleNonZero)
	default:
		t.Fatalf("unknown fill_rule %q", c.FillRule)
	}
	dc.Clip()
	dc.SetRGBA(c.Fill[0], c.Fill[1], c.Fill[2], c.Fill[3])
	dc.DrawRectangle(c.FillRect[0], c.FillRect[1], c.FillRect[2], c.FillRect[3])
	if err := dc.Fill(); err != nil {
		t.Fatalf("fill: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		if errors.Is(err, render.ErrFallbackToCPU) {
			t.Skipf("GPU path unavailable: %v", err)
		}
		t.Fatalf("flush: %v", err)
	}
	return dc
}

// TestClipEvenOddMSAACasesGPU：MSAA=4 GPU 路跑全部场景（先红后绿）。
func TestClipEvenOddMSAACasesGPU(t *testing.T) {
	if render.Accelerator() == nil {
		t.Skip("GPU accelerator not registered")
	}
	render.SetMSAASampleCount(render.MSAASampleCount4)
	defer render.SetMSAASampleCount(0)
	if err := setupGLDevice(t); err != nil {
		t.Skipf("GL device setup unavailable: %v", err)
	}
	for _, c := range loadClipEvenOddMSAACases(t) {
		t.Run(c.Name, func(t *testing.T) {
			dc := runClipEvenOddMSAACase(t, c)
			defer dc.Close()
			if st := dc.RenderPathStats(); st.GPUOps == 0 {
				t.Fatalf("GPU path did not run (cpu fallback?): %s", st.LogLine())
			}
			vertsCheckProbes(t, dc, c.Probes)
		})
	}
}

// TestClipEvenOddMSAACasesCPU：CPU 栅格（1x 语义）跑全部场景——
// CPU 的 Coverage/mask 路一直就支持 even-odd，必须恒绿（回归护栏）。
func TestClipEvenOddMSAACasesCPU(t *testing.T) {
	withVertsCPU(t)
	for _, c := range loadClipEvenOddMSAACases(t) {
		t.Run(c.Name, func(t *testing.T) {
			dc := runClipEvenOddMSAACase(t, c)
			defer dc.Close()
			vertsCheckProbes(t, dc, c.Probes)
		})
	}
}

// TestClipEvenOddMSAAMatchesCPU：同场景 MSAA=4 GPU 与 CPU 画面一致（孔洞区域逐点同色）。
func TestClipEvenOddMSAAMatchesCPU(t *testing.T) {
	if render.Accelerator() == nil {
		t.Skip("GPU accelerator not registered")
	}
	render.SetMSAASampleCount(render.MSAASampleCount4)
	defer render.SetMSAASampleCount(0)
	if err := setupGLDevice(t); err != nil {
		t.Skipf("GL device setup unavailable: %v", err)
	}
	for _, c := range loadClipEvenOddMSAACases(t) {
		t.Run(c.Name, func(t *testing.T) {
			gpu := runClipEvenOddMSAACase(t, c)
			defer gpu.Close()
			// CPU 版重跑（GOGPU_RENDER_MODE=cpu）后逐探针对比 GPU 读回值。
			withVertsCPU(t)
			cpu := runClipEvenOddMSAACase(t, c)
			defer cpu.Close()
			for i, p := range c.Probes {
				gotG := vertsSample(t, gpu, p.X, p.Y)
				gotC := vertsSample(t, cpu, p.X, p.Y)
				if gotG != gotC {
					t.Errorf("probe[%d] %s (%d,%d): gpu=%v cpu=%v", i, p.Desc, p.X, p.Y, gotG, gotC)
				}
			}
		})
	}
}
