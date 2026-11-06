// P3.7 批 6 收官修复（H21）：场景回放命令进显示表——录制、回放分发两枚钉。
// 标准对齐：Skia drawPicture / Flutter drawDisplayList（一份录制作为另一份
// 录制的可重放项）+ 本仓 ProjectionCommand（批 1d）、PathEffect（批 4）先例。
package recording

import (
	"testing"

	"github.com/energye/gpui/render"
)

// countingPayload 实现 ScenePayload，记录回放腿拿到的目标 Context。
type countingPayload struct {
	calls int
	last  *render.Context
}

func (p *countingPayload) RenderSceneInto(dc *render.Context) error {
	p.calls++
	p.last = dc
	return nil
}

// captureBackend 在 stubBackend 之上记下场景回放调用（分发钉用）。
type captureBackend struct {
	stubBackend
	scenes []ScenePayload
	saves  int
}

func (b *captureBackend) Save() { b.saves++ }

func (b *captureBackend) RenderScene(p ScenePayload) error {
	b.scenes = append(b.scenes, p)
	return nil
}

func TestP376RecorderCarriesSceneCommand(t *testing.T) {
	p := &countingPayload{}
	r := NewRecorder(20, 20)
	r.RenderScene(p)
	cmds := r.FinishRecording().commands
	if len(cmds) != 1 {
		t.Fatalf("命令数=%d want 1（Recorder.RenderScene 没落命令？）", len(cmds))
	}
	sc, ok := cmds[0].(SceneReplayCommand)
	if !ok {
		t.Fatalf("命令类型 %T want SceneReplayCommand", cmds[0])
	}
	if sc.Type() != CmdSceneReplay {
		t.Fatalf("Type()=%v want CmdSceneReplay", sc.Type())
	}
	if sc.Payload != ScenePayload(p) {
		t.Fatal("payload 没被原样携带（录制期取值、回放只读）")
	}
}

func TestP376PlaybackDispatchesSceneInOrder(t *testing.T) {
	p := &countingPayload{}
	r := NewRecorder(20, 20)
	r.Save()
	r.RenderScene(p)
	b := &captureBackend{}
	if err := r.FinishRecording().Playback(b); err != nil {
		t.Fatalf("Playback: %v", err)
	}
	if b.saves != 1 {
		t.Fatalf("Save 回放数=%d want 1", b.saves)
	}
	if len(b.scenes) != 1 || b.scenes[0] != ScenePayload(p) {
		t.Fatalf("场景回放分发=%v want 1 枚同 payload", b.scenes)
	}
}
