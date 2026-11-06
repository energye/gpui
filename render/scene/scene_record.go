//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package scene

import (
	"errors"
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/recording"
)

// ErrSceneReplayUnsupported reports a paint target that can neither run a
// device-direct scene replay nor record one: the capability gap stays a
// named refusal instead of silently painting nothing (P3.2 fail-closed
// discipline, same family as the material-leg refusals).
var ErrSceneReplayUnsupported = errors.New("scene: target canvas supports no scene replay")

// Compile-time proof of the payload protocol (Skia drawPicture /
// Flutter drawDisplayList shape, P3.7 批 6 收官 H21).
var _ recording.ScenePayload = (*Scene)(nil)

// RenderSceneInto replays this scene through a live render.Context (the
// recording.ScenePayload contract). It is the same decoder walk
// GPUSceneRenderer always performed — no second implementation, this is a
// carrier, not a re-do.
func (s *Scene) RenderSceneInto(dc *render.Context) error {
	if dc == nil {
		return errors.New("scene.RenderSceneInto: nil context")
	}
	return NewGPUSceneRenderer(dc).RenderScene(s)
}

// RenderSceneTo paints a scene onto any paint target the pipeline can hand
// a window: a live *render.Context replays it directly; a *recording.Recorder
// (the per-frame UI walk) records a nested scene command so the replayed
// display list — what actually reaches the screen — carries the scene.
// Before this verb existed, scene replay was live-only and never reached a
// presented frame (H21: found by human eyes on ui_wr_p37_scene /
// ui_render_base_scene card A).
func RenderSceneTo(dc render.Canvas, s *Scene) error {
	if s == nil {
		return nil
	}
	if dc == nil {
		return fmt.Errorf("scene.RenderSceneTo: %w (nil canvas)", ErrSceneReplayUnsupported)
	}
	switch t := dc.(type) {
	case *render.Context:
		return s.RenderSceneInto(t)
	case *recording.Recorder:
		// 录制腿：命令进显示表，回放时在 CanvasBackend.RenderScene 执行
		// （Recorder 动词一律 void——与 Save/FillPath 全家族一致，错误在
		// Playback 面上报）。
		t.RenderScene(s)
		return nil
	default:
		return fmt.Errorf("scene.RenderSceneTo: %w (%T)", ErrSceneReplayUnsupported, dc)
	}
}
