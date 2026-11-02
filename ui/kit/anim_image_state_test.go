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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/render"
	
	"github.com/energye/gpui/ui/rendering"
)

// 动图组件（P3.6 批 1d 步骤 4）的逻辑钉。
//
// 契约：Tick 返回「是否继续驱动」（暂停/播完不循环/未挂树/减弱动效 = false，
// 注册表自动摘除）；NextWake 报出当前帧剩余时长（Duration=0 报 0「立刻再来」）；
// 循环按容器真值（负 = 无限、N = 额外 N 轮），组件不改写帧时长。

// kitLoadAnim 读批 1b 的动图正本素材，缺失时 Skip（不静默假绿）。
func kitLoadAnim(t *testing.T, name string) *render.AnimatedImage {
	t.Helper()
	path := filepath.Join("..", "..", "render", "testdata", "imgfmt", name)
	a, err := render.LoadAnimatedImage(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("animated sample missing: %s (%v)", path, err)
		}
		t.Fatalf("LoadAnimatedImage(%s): %v", path, err)
	}
	return a
}

// mountAnimated 建组件并挂树（owner 可查注册态）。
func mountAnimated(t *testing.T, anim *render.AnimatedImage) (*AnimatedImageBox, *rendering.PipelineOwner) {
	t.Helper()
	box := BuildAnimatedImage(DefaultScopeCtx(), anim, DefaultAnimatedImageProps(40, 30))
	root := rendering.NewAbsoluteBox(200, 200)
	root.Place(box.Node, 10, 10)
	owner := rendering.NewPipelineOwner(root)
	return box, owner
}

// 帧推进按容器时长：gif_once = 2 帧 150ms/250ms、不循环（批 1b 实测口径）。
func TestKitAnimatedImage_TickAdvancesByFrameDuration(t *testing.T) {
	box, owner := mountAnimated(t, kitLoadAnim(t, "gif_once.gif"))
	inst := box.Inst

	// 未 Play 不驱动。
	if inst.Tick(0.2) {
		t.Fatal("a paused instance must refuse to tick")
	}
	if owner.HasTickers() {
		t.Fatal("no ticker may be registered before Play")
	}
	inst.Play()
	if !owner.HasTickers() {
		t.Fatal("Play must register the ticker with the tree owner")
	}
	// 帧 0 时长 150ms：100ms 未到点，不推进。
	if !inst.Tick(0.1) {
		t.Fatal("mid-frame tick must keep driving")
	}
	if inst.FrameIndex() != 0 || inst.FrameIndex() != box.Node.AnimatedFrameIndex() {
		t.Fatalf("frame 0 (150ms) must still be current after 100ms, got %d", inst.FrameIndex())
	}
	// 累计 200ms 跨过 150ms：推进请求置位（帧在节点 Paint 期才落地）。
	if !inst.Tick(0.1) {
		t.Fatal("crossing the frame duration must keep driving")
	}
	if box.Node.AnimatedFrameIndex() != 0 {
		t.Fatalf("the decode must not happen outside Paint, got %d", box.Node.AnimatedFrameIndex())
	}
	if inst.FrameIndex() != 0 {
		t.Fatalf("timeline advanced but display must lag until Paint, got %d", inst.FrameIndex())
	}
	// Paint 期落地推进；NextWake 换算到帧 1 的剩余（250ms 相位保留 50ms）。
	dc := render.NewContext(80, 80)
	dc.BeginFrame()
	box.Node.Paint(rendering.NewPaintContext(dc, 1))
	if box.Node.AnimatedFrameIndex() != 1 {
		t.Fatalf("paint must land the pending advance, got %d", box.Node.AnimatedFrameIndex())
	}
	if d, ok := inst.NextWake(); !ok || d < 190*time.Millisecond || d > 210*time.Millisecond {
		t.Fatalf("NextWake after advance = %v,%v want ~200ms,true", d, ok)
	}
}

// 播完不循环返回 false 并被注册表自动摘除（泵空转自摘的组件层收益）。
func TestKitAnimatedImage_FinishStopsAndUnregisters(t *testing.T) {
	box, owner := mountAnimated(t, kitLoadAnim(t, "gif_once.gif"))
	inst := box.Inst
	inst.Play()
	// 跨过帧 0（150ms）。
	if !inst.Tick(0.2) {
		t.Fatal("frame 0 to frame 1 must keep driving")
	}
	// 跨过帧 1（250ms）到末帧之后：一轮播完，loop=0 → 停止并被摘除。
	if inst.Tick(0.3) {
		t.Fatal("gif_once (loop=0) must stop after one full play")
	}
	if inst.Playing() {
		t.Fatal("Playing must read false after the final frame")
	}
	if owner.HasTickers() {
		t.Fatal("the registry must drop a finished ticker automatically")
	}
}

// 无限循环容器跨过末帧回到帧 0，持续驱动。
func TestKitAnimatedImage_LoopForever(t *testing.T) {
	box, _ := mountAnimated(t, kitLoadAnim(t, "gif_keep.gif")) // 3 帧 100/200/300ms、loop=-1
	inst := box.Inst
	inst.Play()
	// 一轮 600ms：分段 tick 走完必须回到帧 0 且仍继续驱动。
	if !inst.Tick(0.1) || !inst.Tick(0.15) || !inst.Tick(0.2) {
		t.Fatal("frames 0-1 must keep driving")
	}
	if !inst.Tick(0.35) {
		t.Fatal("wrapping to frame 0 must keep driving (loop=-1)")
	}
	if inst.FrameIndex() != 0 {
		t.Fatalf("after one full round the timeline must be back at frame 0, got %d", inst.FrameIndex())
	}
	if _, loop := inst.Info(); loop >= 0 {
		t.Fatalf("gif_keep must report negative loop, got %d", loop)
	}
}

// NextWake 报当前帧剩余；暂停后报无截止。
func TestKitAnimatedImage_NextWakeFollowsPhase(t *testing.T) {
	box, _ := mountAnimated(t, kitLoadAnim(t, "gif_once.gif"))
	inst := box.Inst
	if _, ok := inst.NextWake(); ok {
		t.Fatal("a paused instance must report no wake deadline")
	}
	inst.Play()
	if d, ok := inst.NextWake(); !ok || d != 150*time.Millisecond {
		t.Fatalf("NextWake = %v,%v want 150ms,true", d, ok)
	}
	inst.Tick(0.05)
	if d, ok := inst.NextWake(); !ok || d < 90*time.Millisecond || d > 110*time.Millisecond {
		t.Fatalf("NextWake after 50ms = %v,%v want ~100ms,true", d, ok)
	}
	inst.Pause()
	if owner := box.Node.Owner(); owner.HasTickers() {
		t.Fatal("Pause must unregister the ticker")
	}
	if _, ok := inst.NextWake(); ok {
		t.Fatal("a paused instance must report no wake deadline")
	}
}

// 不可见即停：未挂树的实例 Play 后也拒绝驱动；减弱动效同样停。
func TestKitAnimatedImage_InvisibleAndReducedMotionStop(t *testing.T) {
	// 未挂树：syncTickerRegistration 是空操作，Tick 直接拒绝。
	bare := BuildAnimatedImage(DefaultScopeCtx(), kitLoadAnim(t, "gif_once.gif"), DefaultAnimatedImageProps(40, 30))
	bare.Inst.Play()
	if bare.Inst.Tick(0.2) {
		t.Fatal("an unmounted node must not drive")
	}
	// 减弱动效：系统偏好优先于播放态。
	reduced := DefaultScopeCtx().WithMotion(ScopeMotionConfig{Enabled: true, ReducedMotion: true})
	box := BuildAnimatedImage(reduced, kitLoadAnim(t, "gif_once.gif"), DefaultAnimatedImageProps(40, 30))
	root := rendering.NewAbsoluteBox(200, 200)
	root.Place(box.Node, 10, 10)
	rendering.NewPipelineOwner(root)
	box.Inst.Play()
	if box.Inst.Tick(0.2) {
		t.Fatal("reduced motion must stop the ticker")
	}
}

// 控制动词：Seek 归零相位并落帧；Speed 换倍速后按新时长推进。
func TestKitAnimatedImage_SeekAndSpeed(t *testing.T) {
	box, _ := mountAnimated(t, kitLoadAnim(t, "gif_keep.gif")) // 3 帧 100/200/300ms
	inst := box.Inst
	if err := inst.SeekFrame(2); err != nil {
		t.Fatalf("SeekFrame(2): %v", err)
	}
	inst.Play()
	// 帧 2 时长 300ms：换 2 倍速后 100ms 就该跨过。
	inst.SetSpeed(2)
	if !inst.Tick(0.05) {
		t.Fatal("half of 300ms at 2x must keep driving")
	}
	if !inst.Tick(0.1) {
		t.Fatal("300ms at 2x crosses after 150ms; wrapping keeps driving")
	}
	if inst.FrameIndex() != 0 {
		t.Fatalf("wrapping at 2x must land back on frame 0, got %d", inst.FrameIndex())
	}
	// 释放后一切停。
	inst.Release()
	if inst.Tick(0.2) {
		t.Fatal("a released instance must not drive")
	}
}
