//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package rendering_test

import (
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// 动图承接面（P3.6 批 1d 步骤 3）的钉。
//
// 契约：所有权照 SetImage 的 owns/demote 语义（解码器被接管、帧缓冲只共享）；
// 推进是「组件置 pending + Paint 期解码」（D19：写像素与光栅线程读像素错开，
// Paint 前不推进是可观察行为）；循环/时长元数据只透传，不在引擎层做策略。

// loadAnim 读批 1b 的动图正本素材（render/testdata/imgfmt），缺失时 Skip。
func loadAnim(t *testing.T, name string) *render.AnimatedImage {
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

// framePixelAt 把解码器第 i 帧画布上 (x,y) 的像素读成 RGBA8（画布按 RGBA8 布局）。
func framePixelAt(t *testing.T, a *render.AnimatedImage, i, x, y int) color.RGBA {
	t.Helper()
	if err := a.DecodeFrame(i); err != nil {
		t.Fatalf("DecodeFrame(%d): %v", i, err)
	}
	w, _ := a.Canvas()
	off := (y*w + x) * 4
	d := a.Image().Data()
	return color.RGBA{R: d[off], G: d[off+1], B: d[off+2], A: d[off+3]}
}

func TestRenderImage_AnimatedOwnership(t *testing.T) {
	im := rendering.NewRenderImage(40, 30)

	// 装上：接管解码器、Ready、帧缓冲共享（节点不拥有 buf）。
	a := loadAnim(t, "gif_once.gif")
	im.SetAnimatedImage(a)
	if !im.Animated() || im.State != rendering.ImageReady {
		t.Fatalf("after SetAnimatedImage: animated=%v state=%v want true/Ready", im.Animated(), im.State)
	}
	if im.OwnsImage() {
		t.Fatal("the frame buffer belongs to the decoder; the node must not own it")
	}
	if !im.NeedsPaint() {
		t.Fatal("SetAnimatedImage must mark the node for repaint")
	}
	// 同指针重装：幂等，解码器不被误放。
	im.SetAnimatedImage(a)
	if !im.Animated() || a.Disposed() {
		t.Fatalf("re-installing the same decoder must be a no-op: animated=%v disposed=%v", im.Animated(), a.Disposed())
	}
	// 装静态图：动图让位（解码器连带释放），新图 Ready。
	static, err := render.NewImageBuf(40, 30, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	defer static.Dispose()
	im.SetImage(static)
	if !a.Disposed() {
		t.Fatal("installing a static buffer must dispose the adopted decoder")
	}
	if im.Animated() || im.State != rendering.ImageReady {
		t.Fatalf("after SetImage: animated=%v state=%v want false/Ready", im.Animated(), im.State)
	}
	// 再装动图、Clear：释放解码器回 Idle。
	a2 := loadAnim(t, "gif_once.gif")
	im.SetAnimatedImage(a2)
	im.Clear()
	if !a2.Disposed() || im.Animated() || im.State != rendering.ImageIdle {
		t.Fatalf("Clear must release the decoder: disposed=%v animated=%v state=%v", a2.Disposed(), im.Animated(), im.State)
	}
	// 拒收已释放的解码器：Error，不接管（fail closed）。
	im.SetAnimatedImage(a2)
	if im.State != rendering.ImageError || im.Animated() {
		t.Fatalf("a disposed decoder must be refused: state=%v animated=%v", im.State, im.Animated())
	}
	// 未装动图时控制动词的静默边界：无动画、无信息、推进拒绝、seek 报错。
	im2 := rendering.NewRenderImage(10, 10)
	if im2.Animated() {
		t.Fatal("fresh node must not be animated")
	}
	if _, loop := im2.AnimatedInfo(); loop != 0 {
		t.Fatalf("AnimatedInfo on a static node must report loop=0, got %d", loop)
	}
	if im2.AdvanceFrame() {
		t.Fatal("AdvanceFrame without an animated image must be refused")
	}
	if err := im2.SeekFrame(0); err == nil {
		t.Fatal("SeekFrame without an animated image must error")
	}
}

// 静态 owned 缓冲被动图覆盖时必须先释放（交付后审查 P0-1 钉）：
// SetImage 装的 owned buf → SetAnimatedImage → 旧 buf 必须 Disposed，不许泄漏。
func TestRenderImage_AnimatedReplacesOwnedStatic(t *testing.T) {
	im := rendering.NewRenderImage(40, 30)
	static, err := render.NewImageBuf(40, 30, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	im.SetImage(static)
	if !im.OwnsImage() {
		t.Fatal("precondition: SetImage must adopt the buffer")
	}
	a := loadAnim(t, "gif_once.gif")
	im.SetAnimatedImage(a)
	if !static.Disposed() {
		t.Fatalf("switching static owned -> animated must dispose the old buffer (leak): disposed=%v", static.Disposed())
	}
	if im.OwnsImage() {
		t.Fatal("the animated frame buffer must not be owned by the node")
	}
	// 换装动图后动图路径语义不受牵连：解码器接管、Clear 连带释放。
	if !im.Animated() || im.State != rendering.ImageReady {
		t.Fatalf("after switch: animated=%v state=%v want true/Ready", im.Animated(), im.State)
	}
	im.Clear()
	if a.Disposed() != true || im.Animated() {
		t.Fatalf("Clear after animated switch must release the decoder: disposed=%v animated=%v", a.Disposed(), im.Animated())
	}
}

func TestRenderImage_AnimatedAdvancePaint(t *testing.T) {
	const W, H = 80, 80
	a := loadAnim(t, "gif_once.gif")
	// 素材前提：2 帧、不循环（loop=0）；两帧在画布中心必须不同，否则像素断言不适用。
	if frames, loop := a.FrameCount(), a.RepetitionCount(); frames != 2 || loop != 0 {
		t.Fatalf("precondition: gif_once frames=%d loop=%d want 2/0", frames, loop)
	}
	f0 := framePixelAt(t, a, 0, 20, 15)
	f1 := framePixelAt(t, a, 1, 20, 15)
	if f0 == f1 {
		t.Skipf("sample frames are identical at canvas center (%#v); pixel assertion not applicable", f0)
	}

	im := rendering.NewRenderImage(40, 30)
	im.SetAnimatedImage(a)
	root := rendering.NewAbsoluteBox(W, H)
	root.Background = &rendering.Color{R: 1, G: 1, B: 1, A: 1}
	root.Place(im, 20, 25)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: W, Height: H}, true)

	paintAt := func() color.RGBA {
		dc := render.NewContext(W, H)
		dc.BeginFrame()
		dc.ClearWithColor(render.White)
		pc := rendering.NewPaintContext(dc, 1)
		root.Paint(pc)
		r, g, b, al := dc.Image().At(40, 40).RGBA()
		return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(al >> 8)}
	}

	// 第 0 帧在首个 Paint 期解码（D19 的可观察面：Paint 前不推进）。
	if im.AnimatedFrameIndex() != 0 {
		t.Fatalf("before first paint frame index must read 0, got %d", im.AnimatedFrameIndex())
	}
	c0 := paintAt()
	if im.AnimatedFrameIndex() != 0 {
		t.Fatalf("first paint must decode frame 0, got %d", im.AnimatedFrameIndex())
	}
	// 无 pending 的重绘显示同一帧。
	c0again := paintAt()
	if c0again != c0 {
		t.Fatalf("repaint without pending advance must show the same frame: %#v vs %#v", c0again, c0)
	}
	// AdvanceFrame 受理 → 下一次 Paint 推进到第 1 帧，画面跟着变。
	if !im.AdvanceFrame() {
		t.Fatal("advance to frame 1 must be accepted")
	}
	if im.AnimatedFrameIndex() != 0 {
		t.Fatalf("the decode must not happen before Paint, got index %d", im.AnimatedFrameIndex())
	}
	c1 := paintAt()
	if im.AnimatedFrameIndex() != 1 {
		t.Fatalf("paint must advance to frame 1, got %d", im.AnimatedFrameIndex())
	}
	if c1 == c0 {
		t.Fatalf("frame 1 must differ from frame 0 on canvas: %#v vs %#v", c1, c0)
	}
	// 末帧即停：gif_once loop=0，末帧之后推进拒绝（组件层据此停驱动）。
	if im.AdvanceFrame() {
		t.Fatal("advancing past the last frame must be refused for loop=0")
	}
	// SeekFrame 向后跳：重放第 0 帧，画面与第 0 帧逐位一致。
	if err := im.SeekFrame(0); err != nil {
		t.Fatalf("seek to frame 0: %v", err)
	}
	cs := paintAt()
	if im.AnimatedFrameIndex() != 0 || cs != c0 {
		t.Fatalf("seek back to frame 0 must replay it: index=%d pixel=%#v want index=0 pixel=%#v", im.AnimatedFrameIndex(), cs, c0)
	}
}

func TestRenderImage_AnimatedSeekAndPause(t *testing.T) {
	a := loadAnim(t, "gif_keep.gif") // 3 帧、无限循环
	im := rendering.NewRenderImage(40, 30)
	im.SetAnimatedImage(a)
	// 先画一次清掉装图时的脏态，后面才断言「拒绝的 seek 不标脏」。
	dc0 := render.NewContext(80, 80)
	dc0.BeginFrame()
	im.Paint(rendering.NewPaintContext(dc0, 1))

	// 越界 seek 报错且不标脏推进。
	if err := im.SeekFrame(3); err == nil {
		t.Fatal("SeekFrame beyond the frame count must error")
	}
	if im.NeedsPaint() {
		t.Fatal("a refused seek must not mark the node for repaint")
	}
	// Seek 到第 2 帧：受理后 Paint 期应用。
	if err := im.SeekFrame(2); err != nil {
		t.Fatalf("SeekFrame(2): %v", err)
	}
	dc := render.NewContext(80, 80)
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	pc := rendering.NewPaintContext(dc, 1)
	im.Paint(pc)
	if im.AnimatedFrameIndex() != 2 {
		t.Fatalf("paint after seek must land on frame 2, got %d", im.AnimatedFrameIndex())
	}
	// paused 标志只表达意图：pending 帧仍被正常画出。
	im.SetAnimatedPaused(true)
	if !im.AnimatedPaused() {
		t.Fatal("AnimatedPaused must read back the flag")
	}
	if err := im.SeekFrame(0); err != nil {
		t.Fatalf("seek while paused: %v", err)
	}
	im.Paint(pc)
	if im.AnimatedFrameIndex() != 0 {
		t.Fatalf("a pending frame must still be painted while paused, got %d", im.AnimatedFrameIndex())
	}
	// AnimatedInfo 透传容器真值（3 帧、无限循环负数）。
	frames, loop := im.AnimatedInfo()
	if frames != 3 || loop >= 0 {
		t.Fatalf("AnimatedInfo = (%d,%d) want (3, negative)", frames, loop)
	}
}
