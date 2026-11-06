//go:build !nogpu

package scene_test

//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// H22（P3.7 批 6 立，2026-11-07 修）：scene 回放腿吞 quad 口具名拒——直调口
// 拿得到错，回放腿 `_ =` 丢弃且四可观察面全空（复现实测：err=nil、
// MaterialSkips=0、ShaderBackendFails=0、无日志；呈现帧走录制→CanvasBackend
// 回放→正是这条路）。修=回放腿拒绝账：Refusals 计数 + LastRefusal 具名 +
// 每 reason 一次 stderr 日志，quad 口与同文件材质门 fill/stroke 四处 `_ =`
// 全部记账表态；拒笔照旧不画、别笔照旧画、RenderScene 保持 nil（帧不死，
// ③/⑥ 冻结语义——单笔拒不升级为整场景错误，否则 recorder Playback 会掀帧）。
// 画面证据：⑥ ui_wr_p37_scene probe（h22_replay_quad_refusal_accounted）。
// 腿路口径：离屏单测无 GPU 加速器注册（`_ "render/gpu"` 只在真窗进程），
// 干净 quad 走静默 CPU 分裂腿、材质+透视在校验期具名拒——「画出了」一律用
// 像素断言（contextPaint+paintDiff，3e quad 先例），不假设计数腿路。

import (
	"errors"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/scene"
)

// h22RefusalScene 透视 quad + 粘滞材质 = quad 口校验期具名拒
// （ErrQuadMaterialUnsupported，m4_quadimage.go 材质腿组合边界）。
func h22RefusalScene(t *testing.T) *scene.Scene {
	s := scene.NewScene()
	s.SetMaterial(p37E3Effect(t))
	s.DrawImageQuad(quadImage(), trapezoidCorners(10, 10), render.QuadDrawOptions{
		Perspective: true, Weights: quadWeights(), Opacity: 1,
	})
	return s
}

// 钉 1（红测）：回放腿的拒绝必须可查——计数、具名、帧不死。
func TestH22ReplayQuadRefusalAccounted(t *testing.T) {
	s := h22RefusalScene(t)
	dc := render.NewContext(200, 200)
	defer dc.Close()
	rr := scene.NewGPUSceneRenderer(dc)
	if err := rr.RenderScene(s); err != nil {
		t.Fatalf("RenderScene = %v, want nil（单笔拒不掀帧）", err)
	}
	if got := rr.Refusals(); got != 1 {
		t.Fatalf("Refusals = %d, want 1（拒绝必须可查，不许 _ = 吞掉）", got)
	}
	if last := rr.LastRefusal(); !errors.Is(last, render.ErrQuadMaterialUnsupported) {
		t.Fatalf("LastRefusal = %v, want ErrQuadMaterialUnsupported（具名）", last)
	}
}

// 钉 2（反假阳）：同 quad 不带材质 = 画得出且回放≡直调逐位；账必须干净。
func TestH22ReplayCleanQuadNoRefusal(t *testing.T) {
	img := quadImage()
	corners := trapezoidCorners(10, 10)
	opts := render.QuadDrawOptions{Perspective: true, Weights: quadWeights(), Opacity: 1}

	direct := contextPaint(func(dc *render.Context) {
		dc.DrawImageQuad(quadBuf(t, img), corners, opts)
	})
	var rr *scene.GPUSceneRenderer
	replay := contextPaint(func(dc *render.Context) {
		s := scene.NewScene()
		s.DrawImageQuad(img, corners, opts)
		rr = scene.NewGPUSceneRenderer(dc)
		if err := rr.RenderScene(s); err != nil {
			t.Fatalf("RenderScene: %v", err)
		}
	})
	if n := paintDiff(direct, replay); n != 0 {
		t.Fatalf("回放 vs 直调 diff=%d，want 0（干净 quad 必须真画出）", n)
	}
	if got := rr.Refusals(); got != 0 {
		t.Fatalf("Refusals = %d, want 0（干净场景不许记账）", got)
	}
	if got := rr.LastRefusal(); got != nil {
		t.Fatalf("LastRefusal = %v, want nil", got)
	}
}

// 钉 3（拒不污染后续）：拒笔在前、可画笔在后——后者照常画出（混合回放 ≡
// 仅可画笔回放逐位，因拒笔什么都不贡献），账只记一笔。
func TestH22RefusalDoesNotPoisonFollowingDraws(t *testing.T) {
	img := quadImage()
	mixed := scene.NewScene()
	mixed.SetMaterial(p37E3Effect(t))
	mixed.DrawImageQuad(img, trapezoidCorners(10, 10), render.QuadDrawOptions{
		Perspective: true, Weights: quadWeights(), Opacity: 1,
	})
	mixed.SetMaterial(nil)
	mixed.DrawImageQuad(img, parallelogramCorners(10, 10), render.QuadDrawOptions{Opacity: 1})

	cleanOnly := scene.NewScene()
	cleanOnly.DrawImageQuad(img, parallelogramCorners(10, 10), render.QuadDrawOptions{Opacity: 1})

	var rr *scene.GPUSceneRenderer
	mixedPM := contextPaint(func(dc *render.Context) {
		rr = scene.NewGPUSceneRenderer(dc)
		if err := rr.RenderScene(mixed); err != nil {
			t.Fatalf("RenderScene mixed: %v", err)
		}
	})
	cleanPM := contextPaint(func(dc *render.Context) {
		if err := scene.NewGPUSceneRenderer(dc).RenderScene(cleanOnly); err != nil {
			t.Fatalf("RenderScene clean: %v", err)
		}
	})
	if n := paintDiff(mixedPM, cleanPM); n != 0 {
		t.Fatalf("混合回放 vs 仅可画笔回放 diff=%d，want 0（拒笔不画、后续照画）", n)
	}
	if got := rr.Refusals(); got != 1 {
		t.Fatalf("Refusals = %d, want 1（只记被拒那一笔）", got)
	}
	if last := rr.LastRefusal(); !errors.Is(last, render.ErrQuadMaterialUnsupported) {
		t.Fatalf("LastRefusal = %v, want ErrQuadMaterialUnsupported", last)
	}
}
