// P3.7 批 6 收官修复（H21）：scene 侧载荷协议、双路逐位对拍、外层矩阵、
// 状态零泄漏、不支持 Canvas 具名拒。目标态 API：
// scene.RenderSceneTo(dc render.Canvas, s *Scene) error /
// (*scene.Scene).RenderSceneInto(*render.Context) error /
// recording.ScenePayload。
package scene_test

import (
	"errors"
	"image"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/recording"
	"github.com/energye/gpui/render/scene"
)

const recProbeSize = 120

// 载荷协议断言（编译不过即红钉）。
var _ recording.ScenePayload = (*scene.Scene)(nil)

func p376Scene() *scene.Scene {
	s := scene.NewScene()
	s.Fill(scene.FillNonZero, scene.IdentityAffine(),
		scene.SolidBrush(render.RGBA{R: 0.85, G: 0.2, B: 0.25, A: 1}),
		scene.NewRectShape(10, 10, 60, 50))
	s.Fill(scene.FillNonZero, scene.NewAffine(1, 0, 0, 1, 6, 6),
		scene.SolidBrush(render.RGBA{R: 0.2, G: 0.4, B: 0.9, A: 1}),
		scene.NewRectShape(10, 10, 40, 30))
	return s
}

func p376Paint(fn func(dc *render.Context)) []byte {
	dc := render.NewContext(recProbeSize, recProbeSize)
	defer dc.Close()
	dc.SetRGB(0.06, 0.06, 0.08)
	dc.DrawRectangle(0, 0, recProbeSize, recProbeSize)
	if err := dc.Fill(); err != nil {
		panic(err)
	}
	fn(dc)
	if err := dc.FlushGPU(); err != nil {
		panic(err)
	}
	return p376Bytes(dc.Image())
}

func p376Bytes(img image.Image) []byte {
	b := img.Bounds()
	out := make([]byte, 0, b.Dx()*b.Dy()*4)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			out = append(out, uint8(r>>8), uint8(g>>8), uint8(bl>>8), uint8(a>>8))
		}
	}
	return out
}

func p376Diff(a, b []byte) int {
	n := 0
	for i := range a {
		if a[i] != b[i] {
			n++
		}
	}
	return n
}

// 钉①：显示表路径（Recorder 录制 → CanvasBackend 回放）与 live 直连路逐位同，
// 且画面确有场景内容（防空转绿）。
func TestP376RecordedSceneReplayEqualsLivePath(t *testing.T) {
	s := p376Scene()
	live := p376Paint(func(dc *render.Context) {
		if err := scene.RenderSceneTo(dc, s); err != nil {
			t.Fatalf("live RenderSceneTo: %v", err)
		}
	})
	viaList := p376Paint(func(dc *render.Context) {
		rec := recording.NewRecorder(recProbeSize, recProbeSize)
		if err := scene.RenderSceneTo(rec, s); err != nil {
			t.Fatalf("recorder RenderSceneTo: %v", err)
		}
		cb := recording.NewCanvasBackend(dc)
		if err := rec.FinishRecording().Playback(cb); err != nil {
			t.Fatalf("Playback: %v", err)
		}
	})
	if d := p376Diff(live, viaList); d != 0 {
		t.Fatalf("显示表路径与 live 路差 %d 字节 want 0（回放腿没执行场景？）", d)
	}
	empty := p376Paint(func(dc *render.Context) {})
	if d := p376Diff(empty, viaList); d < 2000 {
		t.Fatalf("场景内容与空白对照只差 %d 字节（<2000）——回放可能没画出东西", d)
	}
}

// 钉②：外层矩阵由显示表携带并作用到场景回放（录制端 Translate(30,20)）。
func TestP376OuterTransformAppliesToReplayedScene(t *testing.T) {
	s := p376Scene()
	shifted := p376Paint(func(dc *render.Context) {
		rec := recording.NewRecorder(recProbeSize, recProbeSize)
		rec.Save()
		rec.Translate(30, 20)
		if err := scene.RenderSceneTo(rec, s); err != nil {
			t.Fatalf("RenderSceneTo: %v", err)
		}
		rec.Restore()
		if err := rec.FinishRecording().Playback(recording.NewCanvasBackend(dc)); err != nil {
			t.Fatalf("Playback: %v", err)
		}
	})
	plain := p376Paint(func(dc *render.Context) {
		if err := scene.RenderSceneTo(dc, s); err != nil {
			t.Fatalf("RenderSceneTo: %v", err)
		}
	})
	if d := p376Diff(shifted, plain); d < 2000 {
		t.Fatalf("外层平移没作用到场景回放（差 %d 字节 <2000）——回放把外层矩阵吞了？", d)
	}
}

// 钉③：回放后 Context 状态零泄漏——之后画的东西落在恒等 CTM 原位。
func TestP376ReplayLeaksNoState(t *testing.T) {
	s := p376Scene()
	after := p376Paint(func(dc *render.Context) {
		rec := recording.NewRecorder(recProbeSize, recProbeSize)
		if err := scene.RenderSceneTo(rec, s); err != nil {
			t.Fatalf("RenderSceneTo: %v", err)
		}
		if err := rec.FinishRecording().Playback(recording.NewCanvasBackend(dc)); err != nil {
			t.Fatalf("Playback: %v", err)
		}
		dc.SetRGB(0.1, 0.8, 0.3)
		dc.DrawRectangle(90, 90, 20, 20)
		if err := dc.Fill(); err != nil {
			t.Fatal(err)
		}
	})
	i := (95*recProbeSize + 95) * 4
	got := [4]uint8{after[i], after[i+1], after[i+2], after[i+3]}
	off := func(a, b uint8) int {
		if a > b {
			return int(a - b)
		}
		return int(b - a)
	}
	if off(got[0], 26) > 3 || off(got[1], 204) > 3 || off(got[2], 77) > 3 {
		t.Fatalf("回放后新画绿块落点色=(%d,%d,%d) want (26,204,77)±3——状态泄漏（CTM/投影没复位）", got[0], got[1], got[2])
	}
}

// 钉④：不支持场景回放的 Canvas 具名拒绝（不假装支持、不静默画错）。
type bareCanvasStub struct {
	render.Canvas
}

func TestP376UnsupportedCanvasRefusesNamed(t *testing.T) {
	s := p376Scene()
	err := scene.RenderSceneTo(bareCanvasStub{}, s)
	if err == nil {
		t.Fatal("不支持的 Canvas 被放行了（该具名拒）")
	}
	if !errors.Is(err, scene.ErrSceneReplayUnsupported) {
		t.Fatalf("err=%v want errors.Is ErrSceneReplayUnsupported", err)
	}
}
