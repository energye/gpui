// Copyright 2026 The gogpu Authors
// SPDX-License-Identifier: MIT

//go:build linux && !(js && wasm)

package gles

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"unsafe"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/gwgpu/gles/egl"
	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// H4-d 帧与送显走通证（离屏，不碰在线 present_target）：
// Configure → Acquire → CreateTextureView(曲面视图) → 句柄装盒 →
// 建管线 → BeginRenderPass(曲面目标) → Draw → Submit → swapchainFBO 读回。
//
// 像素口径：swapchainFBO 直接读回必须与 H4-c 金文件逐位一致（同一三角、
// 同一尺寸、同一朝向，金文件复用 testdata/h4c_triangle_golden.json，容差 0）；
// Y 翻转 blit 另起校验 FBO 验抄写（中心红、四角蓝）。
// 真 Present（swap 到窗）无窗验不了，留 H4-e/P1 真窗；此处只证真正的
// blit 函数空跑不 panic、无窗像素不断言。
// 无 EGL 时 t.Skipf（缺真机数据不假绿）。

func TestH4D_SurfaceFrameOffscreenPixel(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// WGSL 与期望探针一律读数据文件（H4-c 同一套，不另立金文件）。
	vsWGSL := h4cReadWGSL(t, "h4c_triangle_vertex.wgsl")
	fsWGSL := h4cReadWGSL(t, "h4c_triangle_fragment.wgsl")
	rawGolden, err := os.ReadFile(filepath.Join("testdata", "h4c_triangle_golden.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var golden h4cTriangleGolden
	if err := json.Unmarshal(rawGolden, &golden); err != nil {
		t.Fatalf("parse golden: %v", err)
	}
	if golden.Tolerance != 0 {
		t.Fatalf("golden tolerance = %d, want 0 (bit-exact)", golden.Tolerance)
	}
	w, h := golden.Width, golden.Height
	if w <= 0 || h <= 0 {
		t.Fatalf("golden size = %dx%d, want positive", w, h)
	}

	// 真机 EGL 上下文（桌面 GL）；无 EGL 时跳过并写明原因。
	if err := egl.Init(); err != nil {
		t.Skipf("egl.Init unavailable: %v", err)
	}
	eglCfg := egl.DefaultContextConfig()
	eglCfg.GLES = false
	eglCtx, err := egl.NewContext(eglCfg)
	if err != nil {
		t.Skipf("egl.NewContext unavailable (headless?): %v", err)
	}
	defer eglCtx.Destroy()
	if err := eglCtx.MakeCurrent(); err != nil {
		t.Skipf("MakeCurrent unavailable: %v", err)
	}
	glCtx := &gl.Context{}
	if err := glCtx.Load(egl.GetGLProcAddress); err != nil {
		t.Fatalf("GL load: %v", err)
	}
	t.Logf("GL surface-chain: %s", glCtx.GetString(gl.VERSION))

	// 设备走正式 Adapter.Open（能力探测 + 围栏 + 队列，与线上同一条路）。
	actx := NewAdapterContext(eglCtx, glCtx, false)
	prober := actx.Lock()
	caps := queryAdapterCapabilities(prober)
	actx.Unlock()
	opened, err := (&Adapter{ctx: actx, caps: caps}).Open(0, gputypes.DefaultLimits())
	if err != nil {
		t.Fatalf("Adapter.Open: %v", err)
	}
	dev, queue := opened.Device, opened.Queue

	// 曲面：无窗句柄，Configure 只走 swapchainFBO 分配（与在线窗解耦）。
	surf := &Surface{ctx: actx}
	cfg := &hal.SurfaceConfiguration{
		Width:       uint32(w),
		Height:      uint32(h),
		Format:      gputypes.TextureFormatRGBA8Unorm,
		Usage:       gputypes.TextureUsageRenderAttachment,
		PresentMode: hal.PresentModeFifo,
		AlphaMode:   gputypes.CompositeAlphaModeOpaque,
	}
	if err := surf.Configure(nil, cfg); err != nil {
		t.Fatalf("Surface.Configure: %v", err)
	}
	if surf.swapchainFBO == 0 {
		t.Fatal("swapchainFBO == 0 after Configure")
	}

	// 帧：Acquire → 曲面视图（isSurface 真入口）。
	acq, err := surf.AcquireTexture(nil)
	if err != nil {
		t.Fatalf("AcquireTexture: %v", err)
	}
	if surf.current == nil {
		t.Fatal("in-flight frame not tracked after Acquire")
	}
	view, err := dev.CreateTextureView(acq.Texture, nil)
	if err != nil {
		t.Fatalf("CreateTextureView(surface): %v", err)
	}
	tv, ok := view.(*TextureView)
	if !ok || !tv.isSurface || tv.surfaceTex == nil {
		t.Fatalf("surface view = %#v, want isSurface view", view)
	}

	// 句柄装盒：只许盒子这一处生产句柄（黑屏铁律），外来指针解出空。
	packed := gpucontext.PackTextureView(view)
	if gpucontext.UnpackTextureView(packed) != view {
		t.Fatal("box round-trip did not return the same view")
	}
	foreign := 42
	if gpucontext.UnpackTextureView(gpucontext.NewTextureView(unsafe.Pointer(&foreign))) != nil {
		t.Fatal("foreign handle unpacked to non-nil, want fail-closed nil")
	}

	// 管线：WGSL 经缓存编译（与 H4-c 同一路），真建 GL 管线。
	vsMod, err := dev.CreateShaderModule(&ShaderModuleDescriptor{WGSL: vsWGSL})
	if err != nil {
		t.Fatalf("CreateShaderModule(vertex): %v", err)
	}
	fsMod, err := dev.CreateShaderModule(&ShaderModuleDescriptor{WGSL: fsWGSL})
	if err != nil {
		t.Fatalf("CreateShaderModule(fragment): %v", err)
	}
	pipe, err := dev.CreateRenderPipeline(&RenderPipelineDescriptor{
		Vertex: hal.VertexState{Module: vsMod, EntryPoint: "vs_main"},
		Fragment: &hal.FragmentState{
			Module:     fsMod,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{{
				Format:    cfg.Format,
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
	if err != nil {
		t.Fatalf("CreateRenderPipeline: %v", err)
	}

	// 画一帧进 swapchainFBO（曲面目标路径：setupSurfaceTarget）。
	enc, err := dev.CreateCommandEncoder(nil)
	if err != nil {
		t.Fatalf("CreateCommandEncoder: %v", err)
	}
	rp, err := enc.BeginRenderPass(&hal.RenderPassDescriptor{
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View:       view,
			LoadOp:     gputypes.LoadOpClear,
			StoreOp:    gputypes.StoreOpStore,
			ClearValue: gputypes.ColorBlue,
		}},
	})
	if err != nil {
		t.Fatalf("BeginRenderPass: %v", err)
	}
	rp.SetPipeline(pipe)
	rp.Draw(3, 1, 0, 0)
	if err := rp.End(); err != nil {
		t.Fatalf("RenderPass.End: %v", err)
	}
	cmd, err := enc.Finish()
	if err != nil {
		t.Fatalf("Encoder.Finish: %v", err)
	}
	if _, err := queue.Submit(cmd); err != nil {
		t.Fatalf("Queue.Submit: %v", err)
	}

	// 读回 swapchainFBO：与 H4-c 离屏同朝向，md5 必须对金文件逐位一致。
	frame := actx.Lock()
	frame.Finish()
	frame.BindFramebuffer(gl.FRAMEBUFFER, surf.swapchainFBO)
	pixels := make([]byte, w*h*4)
	frame.ReadPixels(0, 0, int32(w), int32(h), gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&pixels[0]))
	frame.BindFramebuffer(gl.FRAMEBUFFER, 0)
	actx.Unlock()

	at := func(x, y int) []int {
		o := (y*w + x) * 4
		return []int{int(pixels[o]), int(pixels[o+1]), int(pixels[o+2]), int(pixels[o+3])}
	}
	eq := func(a, b []int) bool {
		return len(a) == 4 && len(b) == 4 && a[0] == b[0] && a[1] == b[1] && a[2] == b[2] && a[3] == b[3]
	}
	if got := at(w/2, h/2); !eq(got, golden.Center) {
		t.Errorf("surface frame center = %v, want %v", got, golden.Center)
	}
	for _, p := range [][2]int{{0, 0}, {w - 1, 0}, {0, h - 1}, {w - 1, h - 1}} {
		if got := at(p[0], p[1]); !eq(got, golden.Corner) {
			t.Errorf("surface frame corner %v = %v, want %v", p, got, golden.Corner)
		}
	}
	sum := md5.Sum(pixels)
	if gotMD5 := hex.EncodeToString(sum[:]); gotMD5 != golden.PixelsMD5 {
		t.Errorf("surface frame pixels md5 = %s, want %s (golden testdata/h4c_triangle_golden.json)", gotMD5, golden.PixelsMD5)
	} else {
		t.Logf("surface frame pixels md5: %s", gotMD5)
	}

	// Y 翻转 blit 抄写验证：源 swapchainFBO → 自有校验 FBO（同坐标同参数，
	// 与 blitSwapchainToDefaultWith 一致；真 Present 的目标 FBO 0 无窗验不了）。
	blit := actx.Lock()
	blitTex := blit.GenTextures(1)
	blit.BindTexture(gl.TEXTURE_2D, blitTex)
	blit.TexImage2D(gl.TEXTURE_2D, 0, int32(gl.RGBA8), int32(w), int32(h), 0, gl.RGBA, gl.UNSIGNED_BYTE, 0)
	blit.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	blit.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	verifyFBO := blit.GenFramebuffers(1)
	blit.BindFramebuffer(gl.FRAMEBUFFER, verifyFBO)
	blit.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, blitTex, 0)
	if st := blit.CheckFramebufferStatus(gl.FRAMEBUFFER); st != gl.FRAMEBUFFER_COMPLETE {
		actx.Unlock()
		t.Fatalf("verify framebuffer status = %#x, want COMPLETE", st)
	}
	blit.Disable(gl.SCISSOR_TEST)
	blit.BindFramebuffer(gl.READ_FRAMEBUFFER, surf.swapchainFBO)
	blit.BindFramebuffer(gl.DRAW_FRAMEBUFFER, verifyFBO)
	blit.BlitFramebuffer(
		0, int32(h), int32(w), 0, // source Y-flipped (same as present path)
		0, 0, int32(w), int32(h), // dest normal
		gl.COLOR_BUFFER_BIT, gl.NEAREST,
	)
	blit.BindFramebuffer(gl.READ_FRAMEBUFFER, 0)
	blit.BindFramebuffer(gl.DRAW_FRAMEBUFFER, 0)
	blit.BindFramebuffer(gl.FRAMEBUFFER, verifyFBO)
	flipped := make([]byte, w*h*4)
	blit.ReadPixels(0, 0, int32(w), int32(h), gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&flipped[0]))
	blit.BindFramebuffer(gl.FRAMEBUFFER, 0)
	blit.DeleteFramebuffers(verifyFBO)
	blit.DeleteTextures(blitTex)
	actx.Unlock()

	atF := func(x, y int) []int {
		o := (y*w + x) * 4
		return []int{int(flipped[o]), int(flipped[o+1]), int(flipped[o+2]), int(flipped[o+3])}
	}
	if got := atF(w/2, h/2); !eq(got, golden.Center) {
		t.Errorf("blitted center = %v, want %v", got, golden.Center)
	}
	for _, p := range [][2]int{{0, 0}, {w - 1, 0}, {0, h - 1}, {w - 1, h - 1}} {
		if got := atF(p[0], p[1]); !eq(got, golden.Corner) {
			t.Errorf("blitted corner %v = %v, want %v", p, got, golden.Corner)
		}
	}

	// 真 blit 函数空跑不 panic（无窗下 FBO 0 不存在，像素不断言）。
	plain := actx.Lock()
	surf.blitSwapchainToDefaultWith(plain)
	actx.Unlock()

	// 在途帧流转：Discard 放掉，Unconfigure 后 Acquire 报 ErrSurfaceLost。
	surf.DiscardTexture(acq.Texture)
	if surf.current != nil {
		t.Error("Discard did not drop the in-flight frame")
	}
	surf.Unconfigure(nil)
	if _, err := surf.AcquireTexture(nil); !errors.Is(err, hal.ErrSurfaceLost) {
		t.Errorf("post-Unconfigure Acquire = %v, want ErrSurfaceLost", err)
	}
}
