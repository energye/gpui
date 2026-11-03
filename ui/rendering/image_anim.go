//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package rendering

import (
	"fmt"

	"github.com/energye/gpui/render"
)

// RenderImage 的动图承接面。
//
// 所有权（对齐 SetImage 的 owns/demote 语义）：
//   - SetAnimatedImage 接管解码器：换装/清空/出错时由本节点 Dispose 它；当前帧缓冲
//     只共享显示（buf 属于解码器，AnimatedImage.Dispose 会连带释放，本节点不得单独
//     释放，避免双重 Dispose）。
//   - 显式装静态缓冲（SetImage/SetImageShared）、Clear、SetError 都会让动图让位。
//
// 推进（D19，对齐 Flutter 的 Paint 期解码先例）：节奏归组件层——组件按帧时长调
// AdvanceFrame/SeekFrame 置 animPending 并标脏，真正的 DecodeFrame 固定发生在本节点
// 的 Paint 期内，写像素与光栅线程读像素天然错开；publish 换代际后既有图号消费链
// （批 1c）自动重传。本类型不持 timer、不知 fps、不做循环策略。

// SetAnimatedImage 接管动图解码器并显示其当前帧。
//
// a: 解码器；装上后其 Dispose 归本节点（Clear/SetError/换装/装静态图时释放）。
//
// a 为 nil 等价取消动图：释放持有的解码器并回到 Idle 占位。已 Dispose 的解码器被
// 拒收（清场后转 Error，fail closed，对齐 SetImage 对已释放缓冲的处理）。同指针
// 重装是幂等操作（不重复释放，且时间轴回到第 0 帧）。
//
// 调用时机：UI 线程；装上即 MarkNeedsPaint，第 0 帧在下一个 Paint 期解码。
func (im *RenderImage) SetAnimatedImage(a *render.AnimatedImage) {
	if im == nil {
		return
	}
	if a != nil && a.Disposed() {
		im.releaseAnimated()
		im.State = ImageError
		im.MarkNeedsPaint()
		return
	}
	im.releaseAnimatedExcept(a)
	if a == nil {
		im.State = ImageIdle
		im.MarkNeedsPaint()
		return
	}
	im.anim = a
	// 静态 owned 缓冲让位：覆盖 Img 前必须先释放本节点持有的 owned buf，否则泄漏。
	im.releaseOwnedExcept(nil)
	// 共享显示当前帧：buf 的生命周期跟解码器走，本节点不拥有它。
	im.Img = a.Image()
	im.ownsImg = false
	im.animPending = 0
	im.animApplied = 0
	im.State = ImageReady
	im.MarkNeedsPaint()
}

// Animated 报告是否处于动图模式（装着未释放的解码器）。
func (im *RenderImage) Animated() bool {
	return im != nil && im.anim != nil && !im.anim.Disposed()
}

// AnimatedFrameIndex 报告已解码显示的帧号。装上时为 0（首个 Paint 期推进第 0 帧），
// 每次推进在 Paint 期完成后更新。
func (im *RenderImage) AnimatedFrameIndex() int {
	if im == nil {
		return 0
	}
	return im.animApplied
}

// AnimatedInfo 报告帧数与循环数。
//
// frames: 容器帧数（未装动图时 0）。
// loop: 首播之后的额外播放次数，负数表示无限循环（SkCodec 惯例，批 1b 口径）。
func (im *RenderImage) AnimatedInfo() (frames int, loop int) {
	if im == nil || im.anim == nil {
		return 0, 0
	}
	return im.anim.FrameCount(), im.anim.RepetitionCount()
}

// AdvanceFrame 请求推进到下一帧；真正的解码在本节点下一个 Paint 期内执行（D19）。
//
// 返回 true 表示请求已受理（存在下一帧）；false 表示没有动图、解码器已释放、或已
// 停在末帧（组件层据此决定是否继续驱动）。调用后自动 MarkNeedsPaint。
func (im *RenderImage) AdvanceFrame() bool {
	if im == nil || im.anim == nil || im.anim.Disposed() {
		return false
	}
	next := im.animApplied + 1
	if next >= im.anim.FrameCount() {
		return false
	}
	im.animPending = next
	im.MarkNeedsPaint()
	return true
}

// SeekFrame 请求跳到第 i 帧；解码同样发生在下一个 Paint 期。向后跳会在解码器内
// 重放（AnimatedImage.DecodeFrame 的成本模型：按序走、回跳从第 0 帧重放）。
//
// 返回 error：未装动图、解码器已释放、或 i 越界。调用后自动 MarkNeedsPaint。
func (im *RenderImage) SeekFrame(i int) error {
	if im == nil || im.anim == nil {
		return fmt.Errorf("rendering: no animated image to seek")
	}
	if im.anim.Disposed() {
		return fmt.Errorf("rendering: animated image already disposed")
	}
	if i < 0 || i >= im.anim.FrameCount() {
		return fmt.Errorf("rendering: frame index %d out of range (count %d)", i, im.anim.FrameCount())
	}
	im.animPending = i
	im.MarkNeedsPaint()
	return nil
}

// SetAnimatedPaused 设置外部暂停标志。
//
// 标志只表达意图，本节点不拿它 gate 任何路径：暂停期间的节奏（是否继续调
// AdvanceFrame）归组件层，pending 帧仍会被正常画出。供组件同步状态与测试观察。
func (im *RenderImage) SetAnimatedPaused(v bool) {
	if im == nil {
		return
	}
	im.animPaused = v
}

// AnimatedPaused 读取外部暂停标志。
func (im *RenderImage) AnimatedPaused() bool {
	return im != nil && im.animPaused
}

// releaseAnimated 释放本节点持有的解码器（如有）。
func (im *RenderImage) releaseAnimated() {
	im.releaseAnimatedExcept(nil)
}

// releaseAnimatedExcept 释放持有的解码器（keep 除外），并摘掉共享显示的帧缓冲。
// 解码器释放会连带其当前帧 buf，所以 Img 一并置空；没有动图可摘时不得触碰 Img/
// ownsImg——静态 owned 缓冲的释放归 releaseOwnedExcept，这里清了就是孤儿泄漏。
func (im *RenderImage) releaseAnimatedExcept(keep *render.AnimatedImage) {
	if im == nil {
		return
	}
	if im.anim != nil && im.anim != keep {
		im.anim.Dispose()
		im.anim = nil
		im.animPending = -1
		im.Img = nil
		im.ownsImg = false
	}
}
