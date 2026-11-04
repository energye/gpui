//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// //----------------------------------------

package recording

import (
	"github.com/energye/gpui/render"
)

// P3.7 批 1d：Recorder 的投影层四动词——录制期取值、回放只读（PathEffect 先例）。
// ProjectionCommand{Mode} 落命令流；CanvasBackend 回放时调目标 dc 同名方法。
// scene 公开面的投影编码仍归批 3e（本批不动 render/scene）。

// SetProjection records a projection-layer replace (render.Canvas 契约).
func (r *Recorder) SetProjection(m render.Matrix4) {
	if m == (render.Matrix4{}) {
		m = render.Matrix4Identity() // 零值不可用：按恒等=关闭（与 Context 同口径）
	}
	r.projection = m
	r.projectionPush = !m.IsIdentity()
	r.commands = append(r.commands, ProjectionCommand{Mode: render.ProjectionSet, Proj: m})
}

// PushProjection records a projection-layer save+replace.
func (r *Recorder) PushProjection(m render.Matrix4) {
	// 存归一化快照（与 Context 同口径）：零值 Matrix4 的 IsIdentity()=false，
	// 直接存会把「全零投影」当成激活态恢复回来。
	r.projectionStack = append(r.projectionStack, r.Projection())
	r.SetProjection(m)
	r.commands = append(r.commands, ProjectionCommand{Mode: render.ProjectionPush, Proj: m})
}

// PopProjection records a projection-layer restore (empty stack = no-op,
// matching render.Context).
func (r *Recorder) PopProjection() {
	if len(r.projectionStack) > 0 {
		r.projection = r.projectionStack[len(r.projectionStack)-1]
		r.projectionStack = r.projectionStack[:len(r.projectionStack)-1]
	}
	r.projectionPush = !r.projection.IsIdentity()
	if r.projection == (render.Matrix4{}) {
		r.projection = render.Matrix4Identity()
		r.projectionPush = false
	}
	r.commands = append(r.commands, ProjectionCommand{Mode: render.ProjectionPop})
}

// Projection returns the recorder's current projection layer (恒等时返回
// Matrix4Identity()，与 Context 同口径)。
func (r *Recorder) Projection() render.Matrix4 {
	if !r.projectionPush {
		return render.Matrix4Identity()
	}
	return r.projection
}
