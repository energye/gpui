//go:build !nogpu

package recording

// P3.7 批 3e-3 录制面钉：shader 作为录制期抓取的状态随 Fill/Stroke 命令走，
// 一路到 Backend 的尾参数。判据正本 `evidence/P3.7/batch3e3-design.md` §2/§3 钉 7。
//
// 链条四段，逐段可观察：
//   Recorder.SetShader 状态 → FillPathCommand/StrokePathCommand.Shader（录制期取值）
//   → Recording.Playback 交给 Backend.FillPath/StrokePath 的尾参
//   → CanvasBackend 用可选接口断言把 *render.Context 作用域 SetShader/clear。
// 材质没有 CPU 求值器（P3.2），所以最后一段在软件画布上的可观察形态是「整笔空白 +
// 无泄漏」；掉链则照常画得出颜色，鉴别不依赖 GPU 在场。

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
)

func p37E3RecordingEffect(t *testing.T, green string) *render.ShaderEffect {
	t.Helper()
	e, err := render.NewShaderEffect(`@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(0.0, ` + green + `, 0.0, 1.0);
}`)
	if err != nil {
		t.Fatalf("材质编译: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })
	return e
}

// 钉 7a：录制抓取 + save/restore 快照——命令携带 SetShader 当时的指针。
func TestP37E3RecordingCarriesShader(t *testing.T) {
	e := p37E3RecordingEffect(t, "1.0")

	rec := NewRecorder(120, 120)
	rec.SetRGBA(1, 0, 0, 1)
	rec.SetShader(e)
	rec.DrawRectangle(0, 0, 16, 16)
	if err := rec.Fill(); err != nil {
		t.Fatal(err)
	}
	rec.Save()
	rec.SetShader(nil)
	rec.DrawRectangle(0, 0, 16, 16)
	if err := rec.Fill(); err != nil {
		t.Fatal(err)
	}
	rec.Restore()
	rec.DrawRectangle(0, 0, 16, 16)
	if err := rec.Stroke(); err != nil {
		t.Fatal(err)
	}
	// 状态查询口镜像 Context.Shader()：Restore 之后必须回到 e。
	if got := rec.shader; got != e {
		t.Fatalf("Restore 后录制态 shader = %v, want 回滚到 e（快照没带 shader）", got)
	}

	fills, strokes, carried := 0, 0, 0
	for _, c := range rec.FinishRecording().Commands() {
		switch cmd := c.(type) {
		case FillPathCommand:
			fills++
			if cmd.Shader != nil {
				carried++
			}
		case StrokePathCommand:
			strokes++
			if cmd.Shader != nil {
				carried++
			}
		}
	}
	if fills != 2 || strokes != 1 {
		t.Fatalf("fills=%d strokes=%d want 2/1", fills, strokes)
	}
	// Fill(e) + Restore 之后的 Stroke(e) 携带；Save 内 SetShader(nil) 的那笔 fill 不带。
	if carried != 2 {
		t.Fatalf("携带材质的命令 = %d, want 2（录制抓取或 save/restore 快照失效）", carried)
	}
}

// 钉 7b：DrawImage 命令的 Opts.Material 随行（3b 拍板「自动随行」，这里实测钉住克隆不吞）。
func TestP37E3RecordingKeepsDrawImageMaterial(t *testing.T) {
	e := p37E3RecordingEffect(t, "0.8")
	img := render.ImageBufFromImage(image.NewRGBA(image.Rect(0, 0, 4, 4)))
	if img == nil {
		t.Fatal("nil image buf")
	}
	defer img.Dispose()

	rec := NewRecorder(64, 64)
	rec.DrawImageEx(img, render.DrawImageOptions{X: 1, Y: 2, Material: e})
	rec.DrawImageEx(img, render.DrawImageOptions{X: 5, Y: 6})

	var seen []*render.ShaderEffect
	for _, c := range rec.FinishRecording().Commands() {
		if cmd, ok := c.(DrawImageCommand); ok {
			seen = append(seen, cmd.Opts.Material)
		}
	}
	if len(seen) != 2 {
		t.Fatalf("DrawImageCommand 枚数 = %d, want 2", len(seen))
	}
	if seen[0] != e {
		t.Fatal("Opts.Material 在录制/克隆处被丢了")
	}
	if seen[1] != nil {
		t.Fatal("没装材质的 DrawImage 被串上了材质")
	}
}

// shaderSpyBackend 记下每次 FillPath/StrokePath 收到的尾参（Playback 交接面）。
type shaderSpyBackend struct {
	stubBackend
	fillShaders   []*render.ShaderEffect
	strokeShaders []*render.ShaderEffect
}

var _ Backend = (*shaderSpyBackend)(nil)

func (s *shaderSpyBackend) FillPath(_ *render.Path, _ Brush, _ FillRule, _ render.BlendMode, _ render.BlendSpace, shader *render.ShaderEffect) {
	s.fillShaders = append(s.fillShaders, shader)
}

func (s *shaderSpyBackend) StrokePath(_ *render.Path, _ Brush, _ Stroke, _ render.BlendMode, _ render.BlendSpace, shader *render.ShaderEffect) {
	s.strokeShaders = append(s.strokeShaders, shader)
}

// 钉 7c：Playback 把命令携带的 shader 交到 Backend 尾参——命令为空则交 nil。
func TestP37E3PlaybackPassesShaderToBackend(t *testing.T) {
	e := p37E3RecordingEffect(t, "0.6")

	rec := NewRecorder(64, 64)
	rec.SetRGBA(1, 0, 0, 1)
	rec.SetShader(e)
	rec.DrawRectangle(0, 0, 8, 8)
	if err := rec.Fill(); err != nil {
		t.Fatal(err)
	}
	rec.SetShader(nil)
	rec.DrawRectangle(0, 0, 8, 8)
	if err := rec.Fill(); err != nil {
		t.Fatal(err)
	}
	rec.DrawRectangle(0, 0, 8, 8)
	if err := rec.Stroke(); err != nil {
		t.Fatal(err)
	}

	spy := &shaderSpyBackend{}
	if err := rec.FinishRecording().Playback(spy); err != nil {
		t.Fatalf("Playback: %v", err)
	}
	if len(spy.fillShaders) != 2 {
		t.Fatalf("FillPath 交接次数 = %d, want 2", len(spy.fillShaders))
	}
	if spy.fillShaders[0] != e || spy.fillShaders[1] != nil {
		t.Fatalf("Playback 交给 Backend 的 shader 不齐：[%v %v] want [e nil]", spy.fillShaders[0], spy.fillShaders[1])
	}
	// 第三笔没装材质 ⇒ StrokePath 收到 nil（不许把上一笔的残留材质传下去）。
	if len(spy.strokeShaders) != 1 || spy.strokeShaders[0] != nil {
		t.Fatalf("StrokePath 收到的 shader = %v, want [nil]", spy.strokeShaders)
	}
}

// 钉 7d：CanvasBackend 的真实腿——*Context 支持 SetShader 时作用域装/清；
// 材质腿无 CPU 求值器 ⇒ 整笔空白，且回放结束后画布的环境 shader 必须为空（不泄漏）。
func TestP37E3CanvasBackendScopedShaderNoLeak(t *testing.T) {
	e := p37E3RecordingEffect(t, "0.4")

	rec := NewRecorder(120, 120)
	rec.SetRGBA(1, 0, 0, 1)
	rec.SetShader(e)
	rec.DrawRectangle(10, 10, 60, 40)
	if err := rec.Fill(); err != nil {
		t.Fatal(err)
	}
	control := NewRecorder(120, 120)
	control.SetRGBA(1, 0, 0, 1)
	control.DrawRectangle(10, 10, 60, 40)
	if err := control.Fill(); err != nil {
		t.Fatal(err)
	}

	painted := func(r *Recorder) (int, *render.Context) {
		dc := render.NewContext(120, 120)
		dc.ClearWithColor(render.White)
		if err := r.FinishRecording().Playback(NewCanvasBackend(dc)); err != nil {
			t.Fatalf("Playback: %v", err)
		}
		pm := dc.PixmapForTest()
		if pm == nil {
			t.Fatal("拿不到测试画布")
		}
		n := 0
		for y := 0; y < 120; y++ {
			for x := 0; x < 120; x++ {
				if pm.GetPixel(x, y) != render.White {
					n++
				}
			}
		}
		if got := dc.Shader(); got != nil {
			t.Fatalf("回放后画布环境 shader 非空（%v）：作用域 set/clear 没闭合，材质泄漏给后续绘制", got)
		}
		return n, dc
	}

	plainCount, dcPlain := painted(control)
	defer dcPlain.Close()
	if plainCount == 0 {
		t.Fatal("对照组（无材质）什么都没画：这条鉴别不成立")
	}
	matCount, dcMat := painted(rec)
	defer dcMat.Close()
	if matCount != 0 {
		t.Fatalf("带材质的录制回放画出 %d 格：shader 没交到 Context，被 CPU 裸画了", matCount)
	}
}
