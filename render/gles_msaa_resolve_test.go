//go:build !nogpu

package render_test

//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
//----------------------------------------

// P4.1 批 4：GLES 4x 离屏 MSAA resolve→readback 内容测试（go4x 修复红绿对）。
//
// 现状洞（evidence/P4.1/batch4/go4x-diagnosis.md，2026-11-10 定位）：
// MSAAResolveCommand.ensureResolveFBO 首用分支裸 BindFramebuffer(GL_FRAMEBUFFER, fbo)
// 同时改写 READ+DRAW 两个绑定点（GLES3 §4.4.1），毁掉 blit 源（READ=msaa FBO），
// resolve 变成 resolve FBO 自拷贝 → resolveTex 恒零 → 一切走「离屏 nil-view 读回」
// 的 GLES 4x 画面全丢（真窗快照全黑 nonblack=0、无头 4x 离屏内容丢失）。
// 修法（用户拍板 2026-11-10）：首用分支改 DRAW_FRAMEBUFFER-only 绑定，
// READ 全程保持 msaa FBO；1x 无 MSAA 命令零变化，native 零变化。
//
// 场景数据 render/testdata/gles_msaa_resolve_cases.json（先红后绿）。
//
// 硬前置：SetMSAASampleCount 必须先于设备装箱——SetDeviceProvider 调用时即按
// 当前全局锁 sampleCount（诊断中「4x 探针实际跑 1x」假阳性的根因）。

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

type glesMSAAResolveCase struct {
	Name       string       `json:"name"`
	Note       string       `json:"note"`
	Canvas     [2]int       `json:"canvas"`
	Background [4]float64   `json:"background"`
	Fill       [4]float64   `json:"fill"`
	Rect       [4]float64   `json:"rect"`
	Probes     []vertsProbe `json:"probes"`
}

type glesMSAAResolveCasesFile struct {
	Cases []glesMSAAResolveCase `json:"cases"`
}

func loadGLESMSAAResolveCases(t *testing.T) []glesMSAAResolveCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "gles_msaa_resolve_cases.json"))
	if err != nil {
		t.Fatalf("read testdata/gles_msaa_resolve_cases.json: %v", err)
	}
	var f glesMSAAResolveCasesFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode gles_msaa_resolve_cases.json: %v", err)
	}
	if len(f.Cases) == 0 {
		t.Fatal("gles_msaa_resolve_cases.json: no cases")
	}
	return f.Cases
}

func runGLESMSAAResolveCase(t *testing.T, c glesMSAAResolveCase) *render.Context {
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
	dc.SetRGBA(c.Fill[0], c.Fill[1], c.Fill[2], c.Fill[3])
	dc.DrawRectangle(c.Rect[0], c.Rect[1], c.Rect[2], c.Rect[3])
	if err := dc.Fill(); err != nil {
		t.Fatalf("fill: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		if errors.Is(err, render.ErrFallbackToCPU) {
			t.Skipf("GPU path unavailable: %v", err)
		}
		t.Fatalf("flush: %v", err)
	}
	if st := dc.RenderPathStats(); st.GPUOps == 0 {
		t.Fatalf("GPU path did not run (cpu fallback?): %s", st.LogLine())
	}
	return dc
}

// TestGLESMSAAOffscreenReadback：go/GL 4x 离屏渲染必须能读回内容。
// 修复前红（内容全丢）；修复后绿。1x 对照在 TestGLESMSAA1xUnaffected。
func TestGLESMSAAOffscreenReadback(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skipf("no DISPLAY: pure-Go GL needs an X11 context-provider window")
	}
	if render.Accelerator() == nil {
		t.Skip("GPU accelerator not registered")
	}
	// 先锁 4x，再装箱 GL 设备（顺序即语义：SetDeviceProvider 即锁 sampleCount）。
	render.SetMSAASampleCount(render.MSAASampleCount4)
	defer render.SetMSAASampleCount(0)
	if err := setupGLDevice(t); err != nil {
		t.Skipf("GL device setup unavailable: %v", err)
	}
	// 假绿防护：断言加速器会话真在 4x（防「装箱时机把 sampleCount 锁 1x」）。
	if aw, ok := render.Accelerator().(render.MSAAAware); ok {
		if got := aw.MSAASampleCount(); got != render.MSAASampleCount4 {
			t.Fatalf("accelerator MSAA sample count = %d, want 4 (box-order regression?)", got)
		}
	}
	for _, c := range loadGLESMSAAResolveCases(t) {
		t.Run(c.Name, func(t *testing.T) {
			dc := runGLESMSAAResolveCase(t, c)
			defer dc.Close()
			vertsCheckProbes(t, dc, c.Probes)
		})
	}
}

// TestGLESMSAA1xUnaffected：同场景 1x 必须照常（修复不得动 1x 路）。
func TestGLESMSAA1xUnaffected(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skipf("no DISPLAY: pure-Go GL needs an X11 context-provider window")
	}
	if render.Accelerator() == nil {
		t.Skip("GPU accelerator not registered")
	}
	render.SetMSAASampleCount(render.MSAASampleCount1)
	defer render.SetMSAASampleCount(0)
	if err := setupGLDevice(t); err != nil {
		t.Skipf("GL device setup unavailable: %v", err)
	}
	for _, c := range loadGLESMSAAResolveCases(t) {
		if c.Name != "solid_rect_4x" {
			continue // 1x 对照只用基础场景
		}
		t.Run("solid_rect_1x", func(t *testing.T) {
			dc := runGLESMSAAResolveCase(t, c)
			defer dc.Close()
			vertsCheckProbes(t, dc, c.Probes)
		})
	}
}
