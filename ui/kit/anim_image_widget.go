//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// AnimatedImageBox 是动图组件的渲染侧宿主（WIDGET_MODEL Render 分面）。
//
// Node 是被驱动的 rendering.RenderImage（解码器所有权与 Paint 期推进都在节点）；
// Inst 是节奏状态机（Tick/NextWake 实现 scheduler 契约，注册走 AddTicker）。
// 宿主把 Node 挂树后调 Inst.Play() 开始驱动；关闭时调 Inst.Release()。
type AnimatedImageBox struct {
	Node *rendering.RenderImage
	Inst *AnimatedImageInstance
}

// BuildAnimatedImage 装配动图组件（WIDGET_MODEL Build，唯一构造入口）。
//
// ctx: 组件上下文（动效开关/减弱动效偏好在 Tick 里生效）。
// anim: 已解码打开的多帧容器（render.LoadAnimatedImage 的产物）；所有权随装配
// 交给渲染节点（Clear/SetError/换装/Release 时由节点释放）。
// props: 尺寸与倍速；尺寸零值用解码画布尺寸。
//
// 装配后第 0 帧在节点首个 Paint 期显示；播放从 Inst.Play() 开始（节奏归消费者）。
func BuildAnimatedImage(ctx ScopeCtx, anim *render.AnimatedImage, props AnimatedImageProps) *AnimatedImageBox {
	node := rendering.NewRenderImage(props.Width, props.Height)
	if anim != nil {
		// 装图前先让节点拿到正确的画布尺寸（props 零值回填写回 props）。
		w, h := anim.Canvas()
		if props.Width <= 0 {
			props.Width = float64(w)
		}
		if props.Height <= 0 {
			props.Height = float64(h)
		}
		node.Width, node.Height = props.Width, props.Height
	}
	node.SetAnimatedImage(anim)
	inst := newAnimatedImageInstance(ctx, anim, props, node)
	return &AnimatedImageBox{Node: node, Inst: inst}
}
