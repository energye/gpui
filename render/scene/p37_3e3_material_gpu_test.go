//go:build !nogpu

package scene_test

// P3.7 批 3e-3 包外钉：回放腿的「材质转交」——TagMaterial 装上的粘滞材质必须交到
// item 的每一笔（fill / stroke / image / quad）手上，一笔都不许掉。
//
// 判据正本 `evidence/P3.7/batch3e3-design.md` §1/§3 钉 3。这里量的是**转交**，
// 不是 GPU 求值：材质没有 CPU 求值器（P3.2 契约），所以「这一笔带上了材质」在
// 软件画布上的可观察形态就是**整笔空白 + MaterialSkips 计数**——掉传则照常画得出。
// 真 GPU 求值的像素正确性由 3a/3d 的单元钉与 p32 真窗负责，多纹理/材质真窗随批 6（D7）。

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/scene"
)

// p37E3ItemArms 每一臂是「画在自己 item 上的一笔」：判据只依赖「带材质⇒空白 /
// 不带材质⇒画得出」这一对，两臂共用。
var p37E3ItemArms = []struct {
	name string
	draw func(s *scene.Scene)
}{
	{"fill", func(s *scene.Scene) {
		s.Fill(scene.FillNonZero, scene.IdentityAffine(), scene.SolidBrush(render.Red),
			scene.NewRectShape(10, 10, 60, 40))
	}},
	{"stroke", func(s *scene.Scene) {
		s.Stroke(scene.DefaultStrokeStyle(), scene.IdentityAffine(), scene.SolidBrush(render.Red),
			scene.NewRectShape(10, 10, 60, 40))
	}},
	{"image", func(s *scene.Scene) {
		s.DrawImage(quadImage(), scene.IdentityAffine())
	}},
	{"quad", func(s *scene.Scene) {
		s.DrawImageQuad(quadImage(), parallelogramCorners(10, 10), render.QuadDrawOptions{Opacity: 1})
	}},
}

func p37E3ItemScene(withMaterial bool, draw func(s *scene.Scene), e *render.ShaderEffect) *scene.Scene {
	s := scene.NewScene()
	if withMaterial {
		s.SetMaterial(e)
	}
	draw(s)
	return s
}

// 钉 3a：每一臂的转交鉴别——带材质回放必须整笔空白，不带材质对照组必须画得出。
func TestP37E3ReplayHandsMaterialToEveryArm(t *testing.T) {
	for _, arm := range p37E3ItemArms {
		t.Run(arm.name, func(t *testing.T) {
			e := p37E3Effect(t)
			withMat := p37E3ItemScene(true, arm.draw, e)
			control := p37E3ItemScene(false, arm.draw, nil)

			matPM := contextPaint(func(dc *render.Context) {
				if err := scene.NewGPUSceneRenderer(dc).RenderScene(withMat); err != nil {
					t.Fatalf("material'd %s: %v", arm.name, err)
				}
			})
			plainPM := contextPaint(func(dc *render.Context) {
				if err := scene.NewGPUSceneRenderer(dc).RenderScene(control); err != nil {
					t.Fatalf("control %s: %v", arm.name, err)
				}
			})
			if paintedCount(plainPM) == 0 {
				t.Fatalf("对照组（无材质）%s 一格没画：这条鉴别不成立", arm.name)
			}
			if got := paintedCount(matPM); got != 0 {
				t.Fatalf("%s 臂带材质仍画出 %d 格：材质没交到这一笔手上，被裸画了（P3.2 无 CPU 求值器）", arm.name, got)
			}
		})
	}
}

// 钉 3b：回放 ≡ 手写同一条腿（转交口径逐位对齐，scene 侧不许自己另算一套材质语义）。
func TestP37E3ReplayEqualsHandWrittenMaterialCalls(t *testing.T) {
	e := p37E3Effect(t)

	// image 臂：回放 ≡ 手写 DrawImageEx{Material}。
	img := quadImage()
	replayImg := contextPaint(func(dc *render.Context) {
		s := scene.NewScene()
		s.SetMaterial(e)
		s.DrawImage(img, scene.IdentityAffine())
		if err := scene.NewGPUSceneRenderer(dc).RenderScene(s); err != nil {
			t.Fatalf("replay image: %v", err)
		}
	})
	handImg := contextPaint(func(dc *render.Context) {
		dc.DrawImageEx(quadBuf(t, img), render.DrawImageOptions{
			X: 0, Y: 0, DstWidth: float64(img.Width), DstHeight: float64(img.Height), Material: e,
		})
	})
	if diff := paintDiff(replayImg, handImg); diff != 0 {
		t.Fatalf("scene 的 image 材质回放与手写 DrawImageEx 差 %d 像素：材质语义在 scene 侧被另算了一遍", diff)
	}
	if got := paintedCount(handImg); got != 0 {
		t.Fatalf("手写材质 DrawImageEx 在无加速器画布上画出了 %d 格：P3.2 拒画契约被破坏，对照组失效", got)
	}

	// fill 臂：回放 ≡ 手写 SetShader + FillPath。
	replayFill := contextPaint(func(dc *render.Context) {
		s := scene.NewScene()
		s.SetMaterial(e)
		s.Fill(scene.FillNonZero, scene.IdentityAffine(), scene.SolidBrush(render.Red),
			scene.NewRectShape(10, 10, 60, 40))
		if err := scene.NewGPUSceneRenderer(dc).RenderScene(s); err != nil {
			t.Fatalf("replay fill: %v", err)
		}
	})
	handFill := contextPaint(func(dc *render.Context) {
		path := render.NewPath()
		path.MoveTo(10, 10)
		path.LineTo(70, 10)
		path.LineTo(70, 50)
		path.LineTo(10, 50)
		path.Close()
		dc.SetRGBA(1, 0, 0, 1)
		dc.SetFillRule(render.FillRuleNonZero)
		dc.SetShader(e)
		_ = dc.FillPath(path)
		dc.SetShader(nil)
	})
	if diff := paintDiff(replayFill, handFill); diff != 0 {
		t.Fatalf("scene 的 fill 材质回放与手写 SetShader+FillPath 差 %d 像素", diff)
	}

	// stroke 臂：回放 ≡ 手写 SetShader + StrokePath（3d 的 Paint.Shader  precedence 面）。
	replayStroke := contextPaint(func(dc *render.Context) {
		s := scene.NewScene()
		s.SetMaterial(e)
		s.Stroke(scene.DefaultStrokeStyle(), scene.IdentityAffine(), scene.SolidBrush(render.Red),
			scene.NewRectShape(10, 10, 60, 40))
		if err := scene.NewGPUSceneRenderer(dc).RenderScene(s); err != nil {
			t.Fatalf("replay stroke: %v", err)
		}
	})
	handStroke := contextPaint(func(dc *render.Context) {
		path := render.NewPath()
		path.MoveTo(10, 10)
		path.LineTo(70, 10)
		path.LineTo(70, 50)
		path.LineTo(10, 50)
		path.Close()
		dc.SetRGBA(1, 0, 0, 1)
		dc.SetLineWidth(1)
		dc.SetShader(e)
		_ = dc.StrokePath(path)
		dc.SetShader(nil)
	})
	if diff := paintDiff(replayStroke, handStroke); diff != 0 {
		t.Fatalf("scene 的 stroke 材质回放与手写 SetShader+StrokePath 差 %d 像素：描边臂没接材质", diff)
	}
}

// 钉 3c：转交必须留可观察账——image/quad 臂走 render.Context 的 MaterialSkips。
func TestP37E3ReplayCountsMaterialSkips(t *testing.T) {
	e := p37E3Effect(t)
	dc := render.NewContext(projProbeSize, projProbeSize)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	dc.ResetRenderPathStats()

	s := scene.NewScene()
	s.SetMaterial(e)
	s.DrawImage(quadImage(), scene.IdentityAffine())
	s.DrawImageQuad(quadImage(), parallelogramCorners(10, 10), render.QuadDrawOptions{Opacity: 1})
	if err := scene.NewGPUSceneRenderer(dc).RenderScene(s); err != nil {
		t.Fatalf("RenderScene: %v", err)
	}
	if got := dc.RenderPathStats().MaterialSkips; got < 2 {
		t.Fatalf("两笔材质图像/quad 的拒画只计了 %d：回放没把材质交给它们", got)
	}

	// 同一份几何不带材质 ⇒ 一笔都不计。
	dc.ResetRenderPathStats()
	plain := scene.NewScene()
	plain.DrawImage(quadImage(), scene.IdentityAffine())
	plain.DrawImageQuad(quadImage(), parallelogramCorners(10, 10), render.QuadDrawOptions{Opacity: 1})
	if err := scene.NewGPUSceneRenderer(dc).RenderScene(plain); err != nil {
		t.Fatalf("control RenderScene: %v", err)
	}
	if got := dc.RenderPathStats().MaterialSkips; got != 0 {
		t.Fatalf("无材质场景被计了 %d 次拒画", got)
	}
}

// 钉 3d：粘滞位不许跨场景泄漏——同一个 GPUSceneRenderer 连放两份场景，
// 前一份装了材质、后一份没装，后一份必须照常画（回放结束显式复位，照投影口径）。
func TestP37E3ReplayResetsMaterialBetweenScenes(t *testing.T) {
	e := p37E3Effect(t)
	matScene := scene.NewScene()
	matScene.SetMaterial(e)
	matScene.DrawImage(quadImage(), scene.IdentityAffine())
	flatScene := scene.NewScene()
	flatScene.DrawImage(quadImage(), scene.IdentityAffine())

	dc := render.NewContext(projProbeSize, projProbeSize)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	pm := dc.PixmapForTest()
	if pm == nil {
		t.Fatal("拿不到测试画布")
	}
	r := scene.NewGPUSceneRenderer(dc)
	if err := r.RenderScene(matScene); err != nil {
		t.Fatalf("material'd scene: %v", err)
	}
	if got := paintedCount(pm); got != 0 {
		t.Fatalf("材质场景画出了 %d 格：转交没生效，复位钉的鉴别也随之失效", got)
	}
	if err := r.RenderScene(flatScene); err != nil {
		t.Fatalf("flat scene: %v", err)
	}
	if got := paintedCount(pm); got == 0 {
		t.Fatal("同一份几何、没装材质的第二份场景没画：上一份的粘滞材质跨场景泄漏了（回放结束必须显式复位）")
	}
}

// p37E3Effect 一枚最小合法材质，测试自己持有、结束关闭。
func p37E3Effect(t *testing.T) *render.ShaderEffect {
	t.Helper()
	e, err := render.NewShaderEffect(`@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(0.0, 1.0, 0.0, 1.0);
}`)
	if err != nil {
		t.Fatalf("材质编译: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })
	return e
}
