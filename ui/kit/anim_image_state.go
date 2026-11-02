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
	"sync"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
)

// 编译期契约断言：状态机直接满足调度器契约（Tick 返回 false 自动摘除、
// NextWake 提供睡眠截止），与光标实现同一形状。
var (
	_ scheduler.Ticker         = (*AnimatedImageInstance)(nil)
	_ scheduler.DeadlineWanter = (*AnimatedImageInstance)(nil)
)

// AnimatedImageInstance 是动图组件的状态机（WIDGET_MODEL State 分面）。
//
// 职责只有「节奏与显示态」：按帧时长累加时间、到点请渲染节点推进（推进本身
// 固定发生在节点 Paint 期内，D19）、按容器真值循环、按播放态同步框架侧驱动
// 注册。解码与绘制实现都归下层（render.AnimatedImage / rendering.RenderImage），
// 这里不碰一个像素。
//
// 驱动契约（批 1d 泵契约合一后组件层白拿的收益，与光标同一形状）：
//   - Tick(dt) 实现 scheduler.Ticker，返回「是否继续驱动」——暂停/播完不循环/
//     未挂树/系统减弱动效返回 false，注册表自动摘除，不空转；
//   - NextWake() 实现 scheduler.DeadlineWanter，报出到下一帧还剩多久，事件循环
//     睡到点再醒，不占帧节拍。
type AnimatedImageInstance struct {
	props AnimatedImageProps
	ctx   scope.Ctx

	mu      sync.Mutex
	node    *rendering.RenderImage // 渲染节点（显示与解码器所有权都在它那）
	anim    *render.AnimatedImage  // 解码器元数据的只读引用（显示归 node）
	elapsed float64                // 当前帧已播秒数（倍速归一后）
	idx     int                    // 时间轴当前帧（已请求推进的帧；显示帧滞后到 Paint 才跟上）
	rounds  int                    // 已完成的整轮数（对照容器的额外播放数）
	playing bool                   // 播放中（Tick 返回值与注册同步的依据）
	mounted bool
}

// newAnimatedImageInstance 建状态机；解码器交给渲染节点接管（所有权），本实例
// 只留一个元数据只读引用。props 的尺寸已由 BuildAnimatedImage 回填为非零。
func newAnimatedImageInstance(ctx ScopeCtx, anim *render.AnimatedImage, props AnimatedImageProps, node *rendering.RenderImage) *AnimatedImageInstance {
	return &AnimatedImageInstance{
		props:   props,
		ctx:     ctx.Normalize(),
		node:    node,
		anim:    anim,
		mounted: true,
	}
}

// Tick 实现 scheduler.Ticker：按 dt 推进动图时间轴。
//
// dt: 距上次推进的真实流逝秒数（嵌道层泵喂墙上间隔）。
//
// 返回「是否继续驱动」：播放中且可见返回 true；暂停、播完且不循环、解码器已
// 释放、未挂树、系统减弱动效（对齐 icon 先例）返回 false，注册表自动摘除。
// 帧推进由 node.AdvanceFrame 受理后在节点 Paint 期内解码；帧本身标脏
// （MarkNeedsPaint），帧跟着脏态来，不由 ticker 活着驱动。
func (in *AnimatedImageInstance) Tick(dt float64) bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if !in.playing || in.node == nil || in.anim == nil || in.anim.Disposed() {
		return false
	}
	// 不可见即停：未挂树（注册本就不可能，防御性再查一次）或零尺寸布局。
	if in.node.Owner() == nil || in.props.Width <= 0 || in.props.Height <= 0 {
		return false
	}
	if !in.ctx.Motion.MotionEnabled() {
		return false
	}
	speed := in.props.Speed
	if speed <= 0 {
		speed = 1
	}
	in.elapsed += dt * speed
	frames := in.anim.FrameCount()
	// 单 tick 内最多推进一整轮的帧数：防全零时长容器在单个 tick 里无限连推。
	// 时间轴用自持的 idx 而不是节点的已显示帧：推进是请求式的（Paint 期才解码），
	// 连续 tick 而 paint 未跟上的间隙里显示帧会滞后，拿它判定会让时间轴卡死。
	for guard := frames; guard > 0; guard-- {
		dur := in.frameDurationLocked(in.idx)
		if in.elapsed < dur {
			return true
		}
		// 到点换帧：非末帧受理 Advance；末帧按容器循环真值重绕或停。
		if in.idx+1 < frames {
			if !in.node.AdvanceFrame() {
				in.stopLocked()
				return false
			}
			in.idx++
			in.elapsed -= dur
			continue
		}
		extra := in.anim.RepetitionCount()
		if extra < 0 || in.rounds < extra {
			in.rounds++
			if err := in.node.SeekFrame(0); err != nil {
				in.stopLocked()
				return false
			}
			in.idx = 0
			in.elapsed -= dur
			continue
		}
		// 播完不循环：停在末帧。除「Tick 返回 false 让注册表自动摘」外再显式
		// 注销一次（与 Pause 同形状）：直接调用 Tick 不经过注册表循环的路径也干净。
		in.stopLocked()
		return false
	}
	return true
}

// stopLocked 停驱动：清播放态并注销框架侧 ticker。调用方须持有 in.mu（注销只
// 触碰 node.Owner 与注册表自身的锁，锁序 in.mu → owner.mu，无环）。
func (in *AnimatedImageInstance) stopLocked() {
	in.playing = false
	in.syncTickerRegistration(false)
}

// NextWake 实现 scheduler.DeadlineWanter：报出当前帧还剩多久播完。
//
// 返回 (剩余时长, true)；不驱动（暂停/停播/无动图/已释放）时 (0, false)。
// 帧时长为 0（合法，动图 WebP 编码器会产出）时报 0——「立刻该再来一次」，
// 与 blinkNextWake 的超支口径一致。
func (in *AnimatedImageInstance) NextWake() (time.Duration, bool) {
	if in == nil {
		return 0, false
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if !in.playing || in.node == nil || in.anim == nil || in.anim.Disposed() {
		return 0, false
	}
	dur := in.frameDurationLocked(in.idx)
	rem := time.Duration((dur - in.elapsed) * float64(time.Second))
	if rem < 0 {
		rem = 0
	}
	return rem, true
}

// frameDurationLocked 读第 i 帧的容器时长；越界/错误按 0 处理（立即走）。
// 调用方须持有 in.mu。
func (in *AnimatedImageInstance) frameDurationLocked(i int) float64 {
	info, err := in.anim.FrameInfo(i)
	if err != nil {
		return 0
	}
	return info.Duration.Seconds()
}

// syncTickerRegistration 按播放态同步框架侧驱动：注册进本节点所在渲染树的
// ticker 注册表（node.Owner()），与光标 syncTickerRegistration 同一形状。
//
// run: true 注册，false 注销（未注册的注销是幂等空操作）。
//
// 调用时机：Play()/Pause() 与 Tick 内的停驱动路径；node 未挂树时什么都不做
// （宿主挂树后再 Play 才生效，与光标焦点事件同一边界）。
func (in *AnimatedImageInstance) syncTickerRegistration(run bool) {
	if in == nil || in.node == nil {
		return
	}
	owner := in.node.Owner()
	if owner == nil {
		return
	}
	if run {
		owner.AddTicker(in)
	} else {
		owner.RemoveTicker(in)
	}
}

// Play 开始播放（幂等）：置播放态、清轮计数（从头播的语义由调用方 Seek 决定，
// 继续播保留相位）、注册驱动，并同步节点的暂停标志。
func (in *AnimatedImageInstance) Play() {
	if in == nil {
		return
	}
	in.mu.Lock()
	in.playing = true
	in.mu.Unlock()
	in.node.SetAnimatedPaused(false)
	in.syncTickerRegistration(true)
}

// Pause 暂停播放（幂等）：注销驱动（驱动没了时间轴就冻结），相位保留，
// 同步节点的暂停标志。
func (in *AnimatedImageInstance) Pause() {
	if in == nil {
		return
	}
	in.mu.Lock()
	in.playing = false
	in.mu.Unlock()
	in.node.SetAnimatedPaused(true)
	in.syncTickerRegistration(false)
}

// SeekFrame 跳到第 i 帧：透传节点的 seek（错误原样返回），时间轴与相位归零到
// 目标帧。
//
// i: 目标帧号，越界报错（与 RenderImage.SeekFrame 同一错误面）。
func (in *AnimatedImageInstance) SeekFrame(i int) error {
	if in == nil {
		return nil
	}
	in.mu.Lock()
	in.elapsed = 0
	in.mu.Unlock()
	err := in.node.SeekFrame(i)
	if err == nil {
		in.mu.Lock()
		in.idx = i
		in.mu.Unlock()
	}
	return err
}

// SetSpeed 换播放倍速；<=0 按 1 处理。
func (in *AnimatedImageInstance) SetSpeed(v float64) {
	if in == nil {
		return
	}
	in.mu.Lock()
	in.props.Speed = v
	in.mu.Unlock()
}

// Playing 报告播放态（「是否继续驱动」的当前值，供宿主与测试观察）。
func (in *AnimatedImageInstance) Playing() bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.playing
}

// FrameIndex 透传节点当前已显示帧。
func (in *AnimatedImageInstance) FrameIndex() int {
	if in == nil || in.node == nil {
		return 0
	}
	return in.node.AnimatedFrameIndex()
}

// Info 透传容器真值：帧数与额外播放次数（负 = 无限）。
func (in *AnimatedImageInstance) Info() (frames int, loop int) {
	if in == nil || in.node == nil {
		return 0, 0
	}
	return in.node.AnimatedInfo()
}

// Node 暴露渲染节点（宿主挂树用；所有权语义见 rendering.RenderImage）。
func (in *AnimatedImageInstance) Node() *rendering.RenderImage {
	if in == nil {
		return nil
	}
	return in.node
}

// Unmount 停驱动并标记宿主死亡（幂等）。渲染资源走 Node.Release。
func (in *AnimatedImageInstance) Unmount() {
	if in == nil {
		return
	}
	in.Pause()
	in.mu.Lock()
	in.mounted = false
	in.mu.Unlock()
}

// Release 释放组件借宿主装上的解码器（渲染节点 SetAnimatedImage(nil) 的语义），
// 并停驱动。关窗/切场景路径调用；重复调用安全。
func (in *AnimatedImageInstance) Release() {
	if in == nil {
		return
	}
	in.Unmount()
	in.node.SetAnimatedImage(nil)
}
