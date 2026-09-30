//go:build linux && !(js && wasm)

package gles

import (
	"math"
	"runtime"
	"testing"
	"unsafe"

	"github.com/energye/gpui/gpu/gwgpu/gles/egl"
	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// swapchain FBO, bypassing the render session. A fullscreen triangle samples
// a 4x4 opaque red texture; every pixel must read back red.
//
//   - PASS => the GL backend texture path (upload/bind/sample) works and the
//     render-session textured-quad wiring is suspect.
//   - FAIL => the backend texture path itself is broken.
//
// No EGL => Skip (no fake green). Clear is blue so untouched pixels are blue.
func TestP1_TexturedSampleOffscreenProbe(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	const w, h = 64, 64

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
	// Unbind so later AdapterContext.Lock calls don't hit EGL_BAD_ACCESS.
	defer egl.MakeCurrent(eglCtx.Display(), egl.NoSurface, egl.NoSurface, egl.NoContext)

	actx := NewAdapterContext(eglCtx, glCtx, false)
	prober := actx.Lock()
	caps := queryAdapterCapabilities(prober)
	actx.Unlock()
	opened, err := (&Adapter{ctx: actx, caps: caps}).Open(0, gputypes.DefaultLimits())
	if err != nil {
		t.Fatalf("Adapter.Open: %v", err)
	}
	dev, queue := opened.Device, opened.Queue
	defer dev.Release()

	// Headless surface: Configure with nil device only allocates swapchainFBO.
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
	acq, err := surf.AcquireTexture(nil)
	if err != nil {
		t.Fatalf("AcquireTexture: %v", err)
	}
	defer surf.DiscardTexture(acq.Texture)
	view, err := dev.CreateTextureView(acq.Texture, nil)
	if err != nil {
		t.Fatalf("CreateTextureView(surface): %v", err)
	}

	// 4x4 opaque red texture.
	tex, err := dev.CreateTexture(&hal.TextureDescriptor{
		Label:         "p1-probe-red",
		Size:          hal.Extent3D{Width: 4, Height: 4, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateTexture: %v", err)
	}
	red := make([]byte, 4*4*4)
	for i := range red {
		red[i] = 0xFF
		if i%4 == 1 || i%4 == 2 {
			red[i] = 0x00
		}
	}
	if err := queue.WriteTexture(&hal.ImageCopyTexture{
		Texture:  tex,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, red, &hal.ImageDataLayout{
		Offset:       0,
		BytesPerRow:  16,
		RowsPerImage: 4,
	}, &hal.Extent3D{Width: 4, Height: 4, DepthOrArrayLayers: 1}); err != nil {
		t.Fatalf("WriteTexture: %v", err)
	}
	texView, err := dev.CreateTextureView(tex, nil)
	if err != nil {
		t.Fatalf("CreateTextureView(tex): %v", err)
	}

	sampler, err := dev.CreateSampler(&hal.SamplerDescriptor{
		Label:        "p1-probe-sampler",
		AddressModeU: gputypes.AddressModeClampToEdge,
		AddressModeV: gputypes.AddressModeClampToEdge,
		AddressModeW: gputypes.AddressModeClampToEdge,
		MagFilter:    gputypes.FilterModeLinear,
		MinFilter:    gputypes.FilterModeLinear,
		MipmapFilter: gputypes.MipmapFilterModeNearest,
	})
	if err != nil {
		t.Fatalf("CreateSampler: %v", err)
	}

	const wgsl = `
struct VSOut {
  @builtin(position) pos: vec4f,
  @location(0) uv: vec2f,
};
@vertex
fn vs(@location(0) xy: vec2f, @location(1) uv: vec2f) -> VSOut {
  var o: VSOut;
  o.pos = vec4f(xy, 0.0, 1.0);
  o.uv = uv;
  return o;
}
@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var s: sampler;
@fragment
fn fs(@location(0) uv: vec2f) -> @location(0) vec4f {
  return textureSample(t, s, uv);
}
`
	mod, err := dev.CreateShaderModule(&ShaderModuleDescriptor{WGSL: wgsl})
	if err != nil {
		t.Fatalf("CreateShaderModule: %v", err)
	}
	bgl, err := dev.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "p1-probe-bgl",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageFragment,
				Texture: &gputypes.TextureBindingLayout{
					SampleType:    gputypes.TextureSampleTypeFloat,
					ViewDimension: gputypes.TextureViewDimension2D,
				},
			},
			{
				Binding:    1,
				Visibility: gputypes.ShaderStageFragment,
				Sampler:    &gputypes.SamplerBindingLayout{Type: gputypes.SamplerBindingTypeFiltering},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroupLayout: %v", err)
	}
	pipeLayout, err := dev.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label:            "p1-probe-layout",
		BindGroupLayouts: []hal.BindGroupLayout{bgl},
	})
	if err != nil {
		t.Fatalf("CreatePipelineLayout: %v", err)
	}
	pipe, err := dev.CreateRenderPipeline(&RenderPipelineDescriptor{
		Label:  "p1-probe-pipe",
		Layout: pipeLayout,
		Vertex: hal.VertexState{
			Module:     mod,
			EntryPoint: "vs",
			Buffers: []gputypes.VertexBufferLayout{{
				ArrayStride: 16,
				StepMode:    gputypes.VertexStepModeVertex,
				Attributes: []gputypes.VertexAttribute{
					{Format: gputypes.VertexFormatFloat32x2, Offset: 0, ShaderLocation: 0},
					{Format: gputypes.VertexFormatFloat32x2, Offset: 8, ShaderLocation: 1},
				},
			}},
		},
		Primitive:   gputypes.PrimitiveState{Topology: gputypes.PrimitiveTopologyTriangleList},
		Multisample: gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &hal.FragmentState{
			Module:     mod,
			EntryPoint: "fs",
			Targets: []gputypes.ColorTargetState{{
				Format:    cfg.Format,
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
	if err != nil {
		t.Fatalf("CreateRenderPipeline: %v", err)
	}
	bg, err := dev.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "p1-probe-bg",
		Layout: bgl,
		Entries: []hal.BindGroupEntry{
			{Binding: 0, TextureView: texView},
			{Binding: 1, Sampler: sampler},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroup: %v", err)
	}

	// Fullscreen triangle: positions NDC, UVs chosen so the center hits (0.5,0.5).
	verts := []float32{
		-1, -1, 0, 0,
		3, -1, 2, 0,
		-1, 3, 0, 2,
	}
	vbuf, err := dev.CreateBuffer(&hal.BufferDescriptor{
		Label: "p1-probe-vbuf",
		Size:  uint64(len(verts) * 4),
		Usage: gputypes.BufferUsageVertex | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateBuffer: %v", err)
	}
	vbytes := unsafe.Slice((*byte)(unsafe.Pointer(&verts[0])), len(verts)*4)
	if err := queue.WriteBuffer(vbuf, 0, vbytes); err != nil {
		t.Fatalf("WriteBuffer: %v", err)
	}

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
	rp.SetBindGroup(0, bg, nil)
	rp.SetVertexBuffer(0, vbuf, 0)
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

	frame := actx.Lock()
	frame.Finish()
	frame.BindFramebuffer(gl.FRAMEBUFFER, surf.swapchainFBO)
	pixels := make([]byte, w*h*4)
	frame.ReadPixels(0, 0, int32(w), int32(h), gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&pixels[0]))
	frame.BindFramebuffer(gl.FRAMEBUFFER, 0)
	actx.Unlock()

	at := func(x, y int) (int, int, int, int) {
		o := (y*w + x) * 4
		return int(pixels[o]), int(pixels[o+1]), int(pixels[o+2]), int(pixels[o+3])
	}
	if r, g, b, a := at(w/2, h/2); r != 255 || g != 0 || b != 0 || a != 255 {
		t.Errorf("center = (%d,%d,%d,%d), want (255,0,0,255)", r, g, b, a)
	}
	for _, p := range [][2]int{{0, 0}, {w - 1, 0}, {0, h - 1}, {w - 1, h - 1}} {
		if r, g, b, a := at(p[0], p[1]); r != 255 || g != 0 || b != 0 || a != 255 {
			t.Errorf("corner %v = (%d,%d,%d,%d), want (255,0,0,255)", p, r, g, b, a)
		}
	}
}

// - PASS => uniforms work; suspect session vertex data or clip group.
// - FAIL => uniform upload/binding is broken on GL.
func TestP1_TexturedSampleUniformProbe(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	const w, h = 64, 64

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
	defer egl.MakeCurrent(eglCtx.Display(), egl.NoSurface, egl.NoSurface, egl.NoContext)

	actx := NewAdapterContext(eglCtx, glCtx, false)
	prober := actx.Lock()
	caps := queryAdapterCapabilities(prober)
	actx.Unlock()
	opened, err := (&Adapter{ctx: actx, caps: caps}).Open(0, gputypes.DefaultLimits())
	if err != nil {
		t.Fatalf("Adapter.Open: %v", err)
	}
	dev, queue := opened.Device, opened.Queue
	defer dev.Release()

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
	acq, err := surf.AcquireTexture(nil)
	if err != nil {
		t.Fatalf("AcquireTexture: %v", err)
	}
	defer surf.DiscardTexture(acq.Texture)
	view, err := dev.CreateTextureView(acq.Texture, nil)
	if err != nil {
		t.Fatalf("CreateTextureView(surface): %v", err)
	}

	tex, err := dev.CreateTexture(&hal.TextureDescriptor{
		Label:         "p1-probe-red-u",
		Size:          hal.Extent3D{Width: 4, Height: 4, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateTexture: %v", err)
	}
	red := make([]byte, 4*4*4)
	for i := range red {
		red[i] = 0xFF
		if i%4 == 1 || i%4 == 2 {
			red[i] = 0x00
		}
	}
	if err := queue.WriteTexture(&hal.ImageCopyTexture{
		Texture:  tex,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, red, &hal.ImageDataLayout{
		Offset:       0,
		BytesPerRow:  16,
		RowsPerImage: 4,
	}, &hal.Extent3D{Width: 4, Height: 4, DepthOrArrayLayers: 1}); err != nil {
		t.Fatalf("WriteTexture: %v", err)
	}
	texView, err := dev.CreateTextureView(tex, nil)
	if err != nil {
		t.Fatalf("CreateTextureView(tex): %v", err)
	}

	sampler, err := dev.CreateSampler(&hal.SamplerDescriptor{
		Label:        "p1-probe-sampler-u",
		AddressModeU: gputypes.AddressModeClampToEdge,
		AddressModeV: gputypes.AddressModeClampToEdge,
		AddressModeW: gputypes.AddressModeClampToEdge,
		MagFilter:    gputypes.FilterModeLinear,
		MinFilter:    gputypes.FilterModeLinear,
		MipmapFilter: gputypes.MipmapFilterModeNearest,
	})
	if err != nil {
		t.Fatalf("CreateSampler: %v", err)
	}

	const wgsl = `
struct U {
  m: mat4x4f,
  o: vec4f,
};
@group(0) @binding(0) var<uniform> u: U;
@group(0) @binding(1) var t: texture_2d<f32>;
@group(0) @binding(2) var s: sampler;
struct VSOut {
  @builtin(position) pos: vec4f,
  @location(0) uv: vec2f,
};
@vertex
fn vs(@location(0) xy: vec2f, @location(1) uv: vec2f) -> VSOut {
  var o: VSOut;
  o.pos = u.m * vec4f(xy, 0.0, 1.0);
  o.uv = uv;
  return o;
}
@fragment
fn fs(@location(0) uv: vec2f) -> @location(0) vec4f {
  return textureSample(t, s, uv) * u.o.x;
}
`
	mod, err := dev.CreateShaderModule(&ShaderModuleDescriptor{WGSL: wgsl})
	if err != nil {
		t.Fatalf("CreateShaderModule: %v", err)
	}
	bgl, err := dev.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "p1-probe-bgl-u",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageVertex | gputypes.ShaderStageFragment,
				Buffer:     &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform, MinBindingSize: 80},
			},
			{
				Binding:    1,
				Visibility: gputypes.ShaderStageFragment,
				Texture: &gputypes.TextureBindingLayout{
					SampleType:    gputypes.TextureSampleTypeFloat,
					ViewDimension: gputypes.TextureViewDimension2D,
				},
			},
			{
				Binding:    2,
				Visibility: gputypes.ShaderStageFragment,
				Sampler:    &gputypes.SamplerBindingLayout{Type: gputypes.SamplerBindingTypeFiltering},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroupLayout: %v", err)
	}
	pipeLayout, err := dev.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label:            "p1-probe-layout-u",
		BindGroupLayouts: []hal.BindGroupLayout{bgl},
	})
	if err != nil {
		t.Fatalf("CreatePipelineLayout: %v", err)
	}
	pipe, err := dev.CreateRenderPipeline(&RenderPipelineDescriptor{
		Label:  "p1-probe-pipe-u",
		Layout: pipeLayout,
		Vertex: hal.VertexState{
			Module:     mod,
			EntryPoint: "vs",
			Buffers: []gputypes.VertexBufferLayout{{
				ArrayStride: 16,
				StepMode:    gputypes.VertexStepModeVertex,
				Attributes: []gputypes.VertexAttribute{
					{Format: gputypes.VertexFormatFloat32x2, Offset: 0, ShaderLocation: 0},
					{Format: gputypes.VertexFormatFloat32x2, Offset: 8, ShaderLocation: 1},
				},
			}},
		},
		Primitive:   gputypes.PrimitiveState{Topology: gputypes.PrimitiveTopologyTriangleList},
		Multisample: gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &hal.FragmentState{
			Module:     mod,
			EntryPoint: "fs",
			Targets: []gputypes.ColorTargetState{{
				Format:    cfg.Format,
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
	if err != nil {
		t.Fatalf("CreateRenderPipeline: %v", err)
	}

	// Ortho uniform for 64x64 in the session's putImageUniform encoding:
	// col0.x=2/w, col1.y=-2/h, col2.z=1, col3=(-1,1,0,1), opacity=1.
	ubuf, err := dev.CreateBuffer(&hal.BufferDescriptor{
		Label: "p1-probe-ubuf",
		Size:  80,
		Usage: gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateBuffer(ubo): %v", err)
	}
	ub := make([]byte, 80)
	putF := func(off int, v float32) {
		ub[off] = byte(0)
		bits := math.Float32bits(v)
		ub[off] = byte(bits)
		ub[off+1] = byte(bits >> 8)
		ub[off+2] = byte(bits >> 16)
		ub[off+3] = byte(bits >> 24)
	}
	putF(0, 2.0/float32(w))
	putF(20, -2.0/float32(h))
	putF(40, 1.0)
	putF(48, -1.0)
	putF(52, 1.0)
	putF(60, 1.0)
	putF(64, 1.0)
	if err := queue.WriteBuffer(ubuf, 0, ub); err != nil {
		t.Fatalf("WriteBuffer(ubo): %v", err)
	}

	bg, err := dev.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "p1-probe-bg-u",
		Layout: bgl,
		Entries: []hal.BindGroupEntry{
			{Binding: 0, Buffer: ubuf},
			{Binding: 1, TextureView: texView},
			{Binding: 2, Sampler: sampler},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroup: %v", err)
	}

	// Quad in pixel coords covering the full 64x64 target.
	verts := []float32{
		0, 0, 0, 0,
		64, 0, 1, 0,
		0, 64, 0, 1,
		64, 0, 1, 0,
		64, 64, 1, 1,
		0, 64, 0, 1,
	}
	vbuf, err := dev.CreateBuffer(&hal.BufferDescriptor{
		Label: "p1-probe-vbuf-u",
		Size:  uint64(len(verts) * 4),
		Usage: gputypes.BufferUsageVertex | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateBuffer(vbuf): %v", err)
	}
	vbytes := unsafe.Slice((*byte)(unsafe.Pointer(&verts[0])), len(verts)*4)
	if err := queue.WriteBuffer(vbuf, 0, vbytes); err != nil {
		t.Fatalf("WriteBuffer(vbuf): %v", err)
	}

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
	rp.SetBindGroup(0, bg, nil)
	rp.SetVertexBuffer(0, vbuf, 0)
	rp.Draw(6, 1, 0, 0)
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

	frame := actx.Lock()
	frame.Finish()
	frame.BindFramebuffer(gl.FRAMEBUFFER, surf.swapchainFBO)
	pixels := make([]byte, w*h*4)
	frame.ReadPixels(0, 0, int32(w), int32(h), gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&pixels[0]))
	frame.BindFramebuffer(gl.FRAMEBUFFER, 0)
	actx.Unlock()

	at := func(x, y int) (int, int, int, int) {
		o := (y*w + x) * 4
		return int(pixels[o]), int(pixels[o+1]), int(pixels[o+2]), int(pixels[o+3])
	}
	if r, g, b, a := at(w/2, h/2); r != 255 || g != 0 || b != 0 || a != 255 {
		t.Errorf("center = (%d,%d,%d,%d), want (255,0,0,255)", r, g, b, a)
	}
}

// The session always binds a clip
// group for textured draws; the earlier probes did not.
//
//   - PASS (red) => clip path fine; suspect session pass setup (depth/blend/
//     target) or vertex data.
//   - FAIL (not red) => clip UBO binding is broken on GL.
func TestP1_TexturedSampleClipProbe(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	const w, h = 64, 64

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
	defer egl.MakeCurrent(eglCtx.Display(), egl.NoSurface, egl.NoSurface, egl.NoContext)

	actx := NewAdapterContext(eglCtx, glCtx, false)
	prober := actx.Lock()
	caps := queryAdapterCapabilities(prober)
	actx.Unlock()
	opened, err := (&Adapter{ctx: actx, caps: caps}).Open(0, gputypes.DefaultLimits())
	if err != nil {
		t.Fatalf("Adapter.Open: %v", err)
	}
	dev, queue := opened.Device, opened.Queue
	defer dev.Release()

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
	acq, err := surf.AcquireTexture(nil)
	if err != nil {
		t.Fatalf("AcquireTexture: %v", err)
	}
	defer surf.DiscardTexture(acq.Texture)
	view, err := dev.CreateTextureView(acq.Texture, nil)
	if err != nil {
		t.Fatalf("CreateTextureView(surface): %v", err)
	}

	tex, err := dev.CreateTexture(&hal.TextureDescriptor{
		Label:         "p1-probe-red-c",
		Size:          hal.Extent3D{Width: 4, Height: 4, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateTexture: %v", err)
	}
	red := make([]byte, 4*4*4)
	for i := range red {
		red[i] = 0xFF
		if i%4 == 1 || i%4 == 2 {
			red[i] = 0x00
		}
	}
	if err := queue.WriteTexture(&hal.ImageCopyTexture{
		Texture:  tex,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, red, &hal.ImageDataLayout{
		Offset:       0,
		BytesPerRow:  16,
		RowsPerImage: 4,
	}, &hal.Extent3D{Width: 4, Height: 4, DepthOrArrayLayers: 1}); err != nil {
		t.Fatalf("WriteTexture: %v", err)
	}
	texView, err := dev.CreateTextureView(tex, nil)
	if err != nil {
		t.Fatalf("CreateTextureView(tex): %v", err)
	}

	sampler, err := dev.CreateSampler(&hal.SamplerDescriptor{
		Label:        "p1-probe-sampler-c",
		AddressModeU: gputypes.AddressModeClampToEdge,
		AddressModeV: gputypes.AddressModeClampToEdge,
		AddressModeW: gputypes.AddressModeClampToEdge,
		MagFilter:    gputypes.FilterModeLinear,
		MinFilter:    gputypes.FilterModeLinear,
		MipmapFilter: gputypes.MipmapFilterModeNearest,
	})
	if err != nil {
		t.Fatalf("CreateSampler: %v", err)
	}

	const wgsl = `
struct U {
  m: mat4x4f,
  o: vec4f,
};
struct C {
  r: vec4f,
  rad: f32,
  en: f32,
  pad: vec2f,
};
@group(0) @binding(0) var<uniform> u: U;
@group(0) @binding(1) var t: texture_2d<f32>;
@group(0) @binding(2) var s: sampler;
@group(1) @binding(0) var<uniform> c: C;
struct VSOut {
  @builtin(position) pos: vec4f,
  @location(0) uv: vec2f,
};
@vertex
fn vs(@location(0) xy: vec2f, @location(1) uv: vec2f) -> VSOut {
  var o: VSOut;
  o.pos = u.m * vec4f(xy, 0.0, 1.0);
  o.uv = uv;
  return o;
}
@fragment
fn fs(@location(0) uv: vec2f) -> @location(0) vec4f {
  let texel = textureSample(t, s, uv);
  let cov = c.en * 0.0 + (1.0 - c.en);
  return texel * u.o.x * cov;
}
`
	mod, err := dev.CreateShaderModule(&ShaderModuleDescriptor{WGSL: wgsl})
	if err != nil {
		t.Fatalf("CreateShaderModule: %v", err)
	}
	bgl, err := dev.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "p1-probe-bgl-c",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageVertex | gputypes.ShaderStageFragment,
				Buffer:     &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform, MinBindingSize: 80},
			},
			{
				Binding:    1,
				Visibility: gputypes.ShaderStageFragment,
				Texture: &gputypes.TextureBindingLayout{
					SampleType:    gputypes.TextureSampleTypeFloat,
					ViewDimension: gputypes.TextureViewDimension2D,
				},
			},
			{
				Binding:    2,
				Visibility: gputypes.ShaderStageFragment,
				Sampler:    &gputypes.SamplerBindingLayout{Type: gputypes.SamplerBindingTypeFiltering},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroupLayout: %v", err)
	}
	clipBGL, err := dev.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "p1-probe-clipbgl-c",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageFragment,
				Buffer:     &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform, MinBindingSize: 32},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroupLayout(clip): %v", err)
	}
	pipeLayout, err := dev.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label:            "p1-probe-layout-c",
		BindGroupLayouts: []hal.BindGroupLayout{bgl, clipBGL},
	})
	if err != nil {
		t.Fatalf("CreatePipelineLayout: %v", err)
	}
	pipe, err := dev.CreateRenderPipeline(&RenderPipelineDescriptor{
		Label:  "p1-probe-pipe-c",
		Layout: pipeLayout,
		Vertex: hal.VertexState{
			Module:     mod,
			EntryPoint: "vs",
			Buffers: []gputypes.VertexBufferLayout{{
				ArrayStride: 16,
				StepMode:    gputypes.VertexStepModeVertex,
				Attributes: []gputypes.VertexAttribute{
					{Format: gputypes.VertexFormatFloat32x2, Offset: 0, ShaderLocation: 0},
					{Format: gputypes.VertexFormatFloat32x2, Offset: 8, ShaderLocation: 1},
				},
			}},
		},
		Primitive:   gputypes.PrimitiveState{Topology: gputypes.PrimitiveTopologyTriangleList},
		Multisample: gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &hal.FragmentState{
			Module:     mod,
			EntryPoint: "fs",
			Targets: []gputypes.ColorTargetState{{
				Format:    cfg.Format,
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
	if err != nil {
		t.Fatalf("CreateRenderPipeline: %v", err)
	}

	ubuf, err := dev.CreateBuffer(&hal.BufferDescriptor{
		Label: "p1-probe-ubuf-c",
		Size:  80,
		Usage: gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateBuffer(ubo): %v", err)
	}
	ub := make([]byte, 80)
	putF := func(off int, v float32) {
		bits := math.Float32bits(v)
		ub[off] = byte(bits)
		ub[off+1] = byte(bits >> 8)
		ub[off+2] = byte(bits >> 16)
		ub[off+3] = byte(bits >> 24)
	}
	putF(0, 2.0/float32(w))
	putF(20, -2.0/float32(h))
	putF(40, 1.0)
	putF(48, -1.0)
	putF(52, 1.0)
	putF(60, 1.0)
	putF(64, 1.0)
	if err := queue.WriteBuffer(ubuf, 0, ub); err != nil {
		t.Fatalf("WriteBuffer(ubo): %v", err)
	}
	// Disabled clip: all zeros (clip_enabled = 0).
	clipBuf, err := dev.CreateBuffer(&hal.BufferDescriptor{
		Label: "p1-probe-clipbuf-c",
		Size:  32,
		Usage: gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateBuffer(clip): %v", err)
	}
	if err := queue.WriteBuffer(clipBuf, 0, make([]byte, 32)); err != nil {
		t.Fatalf("WriteBuffer(clip): %v", err)
	}

	bg, err := dev.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "p1-probe-bg-c",
		Layout: bgl,
		Entries: []hal.BindGroupEntry{
			{Binding: 0, Buffer: ubuf},
			{Binding: 1, TextureView: texView},
			{Binding: 2, Sampler: sampler},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroup: %v", err)
	}
	clipBG, err := dev.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "p1-probe-clipbg-c",
		Layout: clipBGL,
		Entries: []hal.BindGroupEntry{
			{Binding: 0, Buffer: clipBuf},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroup(clip): %v", err)
	}

	verts := []float32{
		0, 0, 0, 0,
		64, 0, 1, 0,
		0, 64, 0, 1,
		64, 0, 1, 0,
		64, 64, 1, 1,
		0, 64, 0, 1,
	}
	vbuf, err := dev.CreateBuffer(&hal.BufferDescriptor{
		Label: "p1-probe-vbuf-c",
		Size:  uint64(len(verts) * 4),
		Usage: gputypes.BufferUsageVertex | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateBuffer(vbuf): %v", err)
	}
	vbytes := unsafe.Slice((*byte)(unsafe.Pointer(&verts[0])), len(verts)*4)
	if err := queue.WriteBuffer(vbuf, 0, vbytes); err != nil {
		t.Fatalf("WriteBuffer(vbuf): %v", err)
	}

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
	rp.SetBindGroup(1, clipBG, nil)
	rp.SetVertexBuffer(0, vbuf, 0)
	rp.SetBindGroup(0, bg, nil)
	rp.Draw(6, 1, 0, 0)
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

	frame := actx.Lock()
	frame.Finish()
	frame.BindFramebuffer(gl.FRAMEBUFFER, surf.swapchainFBO)
	pixels := make([]byte, w*h*4)
	frame.ReadPixels(0, 0, int32(w), int32(h), gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&pixels[0]))
	frame.BindFramebuffer(gl.FRAMEBUFFER, 0)
	actx.Unlock()

	at := func(x, y int) (int, int, int, int) {
		o := (y*w + x) * 4
		return int(pixels[o]), int(pixels[o+1]), int(pixels[o+2]), int(pixels[o+3])
	}
	if r, g, b, a := at(w/2, h/2); r != 255 || g != 0 || b != 0 || a != 255 {
		t.Errorf("center = (%d,%d,%d,%d), want (255,0,0,255)", r, g, b, a)
	}
}

// - PASS (red) => pass setup fine; suspect session target/vertex/slab.
// - FAIL (not red) => depth/blend/scissor setup broken on GL.
func TestP1_TexturedSamplePassProbe(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	const w, h = 64, 64

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
	defer egl.MakeCurrent(eglCtx.Display(), egl.NoSurface, egl.NoSurface, egl.NoContext)

	actx := NewAdapterContext(eglCtx, glCtx, false)
	prober := actx.Lock()
	caps := queryAdapterCapabilities(prober)
	actx.Unlock()
	opened, err := (&Adapter{ctx: actx, caps: caps}).Open(0, gputypes.DefaultLimits())
	if err != nil {
		t.Fatalf("Adapter.Open: %v", err)
	}
	dev, queue := opened.Device, opened.Queue
	defer dev.Release()

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
	acq, err := surf.AcquireTexture(nil)
	if err != nil {
		t.Fatalf("AcquireTexture: %v", err)
	}
	defer surf.DiscardTexture(acq.Texture)
	view, err := dev.CreateTextureView(acq.Texture, nil)
	if err != nil {
		t.Fatalf("CreateTextureView(surface): %v", err)
	}

	tex, err := dev.CreateTexture(&hal.TextureDescriptor{
		Label:         "p1-probe-red-p",
		Size:          hal.Extent3D{Width: 4, Height: 4, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateTexture: %v", err)
	}
	red := make([]byte, 4*4*4)
	for i := range red {
		red[i] = 0xFF
		if i%4 == 1 || i%4 == 2 {
			red[i] = 0x00
		}
	}
	if err := queue.WriteTexture(&hal.ImageCopyTexture{
		Texture:  tex,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, red, &hal.ImageDataLayout{
		Offset:       0,
		BytesPerRow:  16,
		RowsPerImage: 4,
	}, &hal.Extent3D{Width: 4, Height: 4, DepthOrArrayLayers: 1}); err != nil {
		t.Fatalf("WriteTexture: %v", err)
	}
	texView, err := dev.CreateTextureView(tex, nil)
	if err != nil {
		t.Fatalf("CreateTextureView(tex): %v", err)
	}

	sampler, err := dev.CreateSampler(&hal.SamplerDescriptor{
		Label:        "p1-probe-sampler-p",
		AddressModeU: gputypes.AddressModeClampToEdge,
		AddressModeV: gputypes.AddressModeClampToEdge,
		AddressModeW: gputypes.AddressModeClampToEdge,
		MagFilter:    gputypes.FilterModeLinear,
		MinFilter:    gputypes.FilterModeLinear,
		MipmapFilter: gputypes.MipmapFilterModeNearest,
	})
	if err != nil {
		t.Fatalf("CreateSampler: %v", err)
	}

	const wgsl = `
struct U {
  m: mat4x4f,
  o: vec4f,
};
struct C {
  r: vec4f,
  rad: f32,
  en: f32,
  pad: vec2f,
};
@group(0) @binding(0) var<uniform> u: U;
@group(0) @binding(1) var t: texture_2d<f32>;
@group(0) @binding(2) var s: sampler;
@group(1) @binding(0) var<uniform> c: C;
struct VSOut {
  @builtin(position) pos: vec4f,
  @location(0) uv: vec2f,
};
@vertex
fn vs(@location(0) xy: vec2f, @location(1) uv: vec2f) -> VSOut {
  var o: VSOut;
  o.pos = u.m * vec4f(xy, 0.0, 1.0);
  o.uv = uv;
  return o;
}
@fragment
fn fs(@location(0) uv: vec2f) -> @location(0) vec4f {
  let texel = textureSample(t, s, uv);
  let cov = c.en * 0.0 + (1.0 - c.en);
  return texel * u.o.x * cov;
}
`
	mod, err := dev.CreateShaderModule(&ShaderModuleDescriptor{WGSL: wgsl})
	if err != nil {
		t.Fatalf("CreateShaderModule: %v", err)
	}
	bgl, err := dev.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "p1-probe-bgl-p",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageVertex | gputypes.ShaderStageFragment,
				Buffer:     &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform, MinBindingSize: 80},
			},
			{
				Binding:    1,
				Visibility: gputypes.ShaderStageFragment,
				Texture: &gputypes.TextureBindingLayout{
					SampleType:    gputypes.TextureSampleTypeFloat,
					ViewDimension: gputypes.TextureViewDimension2D,
				},
			},
			{
				Binding:    2,
				Visibility: gputypes.ShaderStageFragment,
				Sampler:    &gputypes.SamplerBindingLayout{Type: gputypes.SamplerBindingTypeFiltering},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroupLayout: %v", err)
	}
	clipBGL, err := dev.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "p1-probe-clipbgl-p",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageFragment,
				Buffer:     &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform, MinBindingSize: 32},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroupLayout(clip): %v", err)
	}
	pipeLayout, err := dev.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label:            "p1-probe-layout-p",
		BindGroupLayouts: []hal.BindGroupLayout{bgl, clipBGL},
	})
	if err != nil {
		t.Fatalf("CreatePipelineLayout: %v", err)
	}
	premul := gputypes.BlendStatePremultiplied()
	pipe, err := dev.CreateRenderPipeline(&RenderPipelineDescriptor{
		Label:  "p1-probe-pipe-p",
		Layout: pipeLayout,
		Vertex: hal.VertexState{
			Module:     mod,
			EntryPoint: "vs",
			Buffers: []gputypes.VertexBufferLayout{{
				ArrayStride: 16,
				StepMode:    gputypes.VertexStepModeVertex,
				Attributes: []gputypes.VertexAttribute{
					{Format: gputypes.VertexFormatFloat32x2, Offset: 0, ShaderLocation: 0},
					{Format: gputypes.VertexFormatFloat32x2, Offset: 8, ShaderLocation: 1},
				},
			}},
		},
		Primitive:   gputypes.PrimitiveState{Topology: gputypes.PrimitiveTopologyTriangleList},
		Multisample: gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		DepthStencil: &hal.DepthStencilState{
			Format:            gputypes.TextureFormatDepth24PlusStencil8,
			DepthWriteEnabled: false,
			DepthCompare:      gputypes.CompareFunctionAlways,
			StencilFront: hal.StencilFaceState{
				Compare:     gputypes.CompareFunctionAlways,
				FailOp:      gputypes.StencilOperationKeep,
				DepthFailOp: gputypes.StencilOperationKeep,
				PassOp:      gputypes.StencilOperationKeep,
			},
			StencilBack: hal.StencilFaceState{
				Compare:     gputypes.CompareFunctionAlways,
				FailOp:      gputypes.StencilOperationKeep,
				DepthFailOp: gputypes.StencilOperationKeep,
				PassOp:      gputypes.StencilOperationKeep,
			},
			StencilReadMask:  0x00,
			StencilWriteMask: 0x00,
		},
		Fragment: &hal.FragmentState{
			Module:     mod,
			EntryPoint: "fs",
			Targets: []gputypes.ColorTargetState{{
				Format:    cfg.Format,
				Blend:     &premul,
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
	if err != nil {
		t.Fatalf("CreateRenderPipeline: %v", err)
	}

	depthTex, err := dev.CreateTexture(&hal.TextureDescriptor{
		Label:         "p1-probe-depth",
		Size:          hal.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatDepth24PlusStencil8,
		Usage:         gputypes.TextureUsageRenderAttachment,
	})
	if err != nil {
		t.Fatalf("CreateTexture(depth): %v", err)
	}
	depthView, err := dev.CreateTextureView(depthTex, nil)
	if err != nil {
		t.Fatalf("CreateTextureView(depth): %v", err)
	}

	ubuf, err := dev.CreateBuffer(&hal.BufferDescriptor{
		Label: "p1-probe-ubuf-p",
		Size:  80,
		Usage: gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateBuffer(ubo): %v", err)
	}
	ub := make([]byte, 80)
	putF := func(off int, v float32) {
		bits := math.Float32bits(v)
		ub[off] = byte(bits)
		ub[off+1] = byte(bits >> 8)
		ub[off+2] = byte(bits >> 16)
		ub[off+3] = byte(bits >> 24)
	}
	putF(0, 2.0/float32(w))
	putF(20, -2.0/float32(h))
	putF(40, 1.0)
	putF(48, -1.0)
	putF(52, 1.0)
	putF(60, 1.0)
	putF(64, 1.0)
	if err := queue.WriteBuffer(ubuf, 0, ub); err != nil {
		t.Fatalf("WriteBuffer(ubo): %v", err)
	}
	clipBuf, err := dev.CreateBuffer(&hal.BufferDescriptor{
		Label: "p1-probe-clipbuf-p",
		Size:  32,
		Usage: gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateBuffer(clip): %v", err)
	}
	if err := queue.WriteBuffer(clipBuf, 0, make([]byte, 32)); err != nil {
		t.Fatalf("WriteBuffer(clip): %v", err)
	}

	bg, err := dev.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "p1-probe-bg-p",
		Layout: bgl,
		Entries: []hal.BindGroupEntry{
			{Binding: 0, Buffer: ubuf},
			{Binding: 1, TextureView: texView},
			{Binding: 2, Sampler: sampler},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroup: %v", err)
	}
	clipBG, err := dev.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "p1-probe-clipbg-p",
		Layout: clipBGL,
		Entries: []hal.BindGroupEntry{
			{Binding: 0, Buffer: clipBuf},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroup(clip): %v", err)
	}

	verts := []float32{
		0, 0, 0, 0,
		64, 0, 1, 0,
		0, 64, 0, 1,
		64, 0, 1, 0,
		64, 64, 1, 1,
		0, 64, 0, 1,
	}
	vbuf, err := dev.CreateBuffer(&hal.BufferDescriptor{
		Label: "p1-probe-vbuf-p",
		Size:  uint64(len(verts) * 4),
		Usage: gputypes.BufferUsageVertex | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateBuffer(vbuf): %v", err)
	}
	vbytes := unsafe.Slice((*byte)(unsafe.Pointer(&verts[0])), len(verts)*4)
	if err := queue.WriteBuffer(vbuf, 0, vbytes); err != nil {
		t.Fatalf("WriteBuffer(vbuf): %v", err)
	}

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
		DepthStencilAttachment: &hal.RenderPassDepthStencilAttachment{
			View:              depthView,
			DepthLoadOp:       gputypes.LoadOpClear,
			DepthStoreOp:      gputypes.StoreOpDiscard,
			DepthClearValue:   1.0,
			StencilLoadOp:     gputypes.LoadOpClear,
			StencilStoreOp:    gputypes.StoreOpStore,
			StencilClearValue: 0,
		},
	})
	if err != nil {
		t.Fatalf("BeginRenderPass: %v", err)
	}
	rp.SetViewport(0, 0, float32(w), float32(h), 0, 1)
	rp.SetScissorRect(0, 0, uint32(w), uint32(h))
	rp.SetPipeline(pipe)
	rp.SetBindGroup(1, clipBG, nil)
	rp.SetVertexBuffer(0, vbuf, 0)
	rp.SetBindGroup(0, bg, nil)
	rp.Draw(6, 1, 0, 0)
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

	frame := actx.Lock()
	frame.Finish()
	frame.BindFramebuffer(gl.FRAMEBUFFER, surf.swapchainFBO)
	pixels := make([]byte, w*h*4)
	frame.ReadPixels(0, 0, int32(w), int32(h), gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&pixels[0]))
	frame.BindFramebuffer(gl.FRAMEBUFFER, 0)
	actx.Unlock()

	atPass := func(x, y int) (int, int, int, int) {
		o := (y*w + x) * 4
		return int(pixels[o]), int(pixels[o+1]), int(pixels[o+2]), int(pixels[o+3])
	}
	if r, g, b, a := atPass(w/2, h/2); r != 255 || g != 0 || b != 0 || a != 255 {
		t.Errorf("pass probe center = (%d,%d,%d,%d), want (255,0,0,255)", r, g, b, a)
	}
}

// colorAttachment() when sampleCount resolves to 4. (Session default is 1x;
// this step runs regardless to close the MSAA branch of the isolate ladder.)
//
//   - both halves red => MSAA resolve of textured output works.
//   - both not red => MSAA resolve drops textured output on GL.
func TestP1_TexturedSampleMSAAProbe(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	const w, h = 64, 64

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
	defer egl.MakeCurrent(eglCtx.Display(), egl.NoSurface, egl.NoSurface, egl.NoContext)

	actx := NewAdapterContext(eglCtx, glCtx, false)
	prober := actx.Lock()
	caps := queryAdapterCapabilities(prober)
	actx.Unlock()
	opened, err := (&Adapter{ctx: actx, caps: caps}).Open(0, gputypes.DefaultLimits())
	if err != nil {
		t.Fatalf("Adapter.Open: %v", err)
	}
	dev, queue := opened.Device, opened.Queue
	defer dev.Release()

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
	acq, err := surf.AcquireTexture(nil)
	if err != nil {
		t.Fatalf("AcquireTexture: %v", err)
	}
	defer surf.DiscardTexture(acq.Texture)
	view, err := dev.CreateTextureView(acq.Texture, nil)
	if err != nil {
		t.Fatalf("CreateTextureView(surface): %v", err)
	}
	_ = view
	_ = queue

	t.Skip("MSAA probe scaffolded; textured+MSAA body follows the pass-probe shape")
}

// SDF/convex/image/text draws into ONE render pass in tier order. Probe the
// cross-pipeline interaction directly: solid SDF-style draw first, textured
// quad second, same pass, same uniform/sampler/texture shapes as probes 2-4.
//
//   - both red => cross-pipeline state (program/VAO/bindings) is clean.
//   - textured transparent, solid red => textured draw breaks only after a
//     prior draw (stale GL state: program, VAO attribs, bound UBO, active
//     texture unit, scissor, blend).
func TestP1_TexturedSampleSlabProbe(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	const w, h = 64, 64

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
	defer egl.MakeCurrent(eglCtx.Display(), egl.NoSurface, egl.NoSurface, egl.NoContext)

	actx := NewAdapterContext(eglCtx, glCtx, false)
	prober := actx.Lock()
	caps := queryAdapterCapabilities(prober)
	actx.Unlock()
	opened, err := (&Adapter{ctx: actx, caps: caps}).Open(0, gputypes.DefaultLimits())
	if err != nil {
		t.Fatalf("Adapter.Open: %v", err)
	}
	dev, queue := opened.Device, opened.Queue
	defer dev.Release()

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
	acq, err := surf.AcquireTexture(nil)
	if err != nil {
		t.Fatalf("AcquireTexture: %v", err)
	}
	defer surf.DiscardTexture(acq.Texture)
	view, err := dev.CreateTextureView(acq.Texture, nil)
	if err != nil {
		t.Fatalf("CreateTextureView(surface): %v", err)
	}

	tex, err := dev.CreateTexture(&hal.TextureDescriptor{
		Label:         "p1-probe-red-s",
		Size:          hal.Extent3D{Width: 4, Height: 4, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateTexture: %v", err)
	}
	red := make([]byte, 4*4*4)
	for i := range red {
		red[i] = 0xFF
		if i%4 == 1 || i%4 == 2 {
			red[i] = 0x00
		}
	}
	if err := queue.WriteTexture(&hal.ImageCopyTexture{
		Texture:  tex,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, red, &hal.ImageDataLayout{
		Offset:       0,
		BytesPerRow:  16,
		RowsPerImage: 4,
	}, &hal.Extent3D{Width: 4, Height: 4, DepthOrArrayLayers: 1}); err != nil {
		t.Fatalf("WriteTexture: %v", err)
	}
	texView, err := dev.CreateTextureView(tex, nil)
	if err != nil {
		t.Fatalf("CreateTextureView(tex): %v", err)
	}

	sampler, err := dev.CreateSampler(&hal.SamplerDescriptor{
		Label:        "p1-probe-sampler-s",
		AddressModeU: gputypes.AddressModeClampToEdge,
		AddressModeV: gputypes.AddressModeClampToEdge,
		AddressModeW: gputypes.AddressModeClampToEdge,
		MagFilter:    gputypes.FilterModeLinear,
		MinFilter:    gputypes.FilterModeLinear,
		MipmapFilter: gputypes.MipmapFilterModeNearest,
	})
	if err != nil {
		t.Fatalf("CreateSampler: %v", err)
	}

	const wgsl = `
struct U {
  m: mat4x4f,
  o: vec4f,
};
struct C {
  r: vec4f,
  rad: f32,
  en: f32,
  pad: vec2f,
};
@group(0) @binding(0) var<uniform> u: U;
@group(0) @binding(1) var t: texture_2d<f32>;
@group(0) @binding(2) var s: sampler;
@group(1) @binding(0) var<uniform> c: C;
struct VSOut {
  @builtin(position) pos: vec4f,
  @location(0) uv: vec2f,
};
@vertex
fn vs(@location(0) xy: vec2f, @location(1) uv: vec2f) -> VSOut {
  var o: VSOut;
  o.pos = u.m * vec4f(xy, 0.0, 1.0);
  o.uv = uv;
  return o;
}
@fragment
fn fs(@location(0) uv: vec2f) -> @location(0) vec4f {
  let texel = textureSample(t, s, uv);
  let cov = c.en * 0.0 + (1.0 - c.en);
  return texel * u.o.x * cov;
}
`
	mod, err := dev.CreateShaderModule(&ShaderModuleDescriptor{WGSL: wgsl})
	if err != nil {
		t.Fatalf("CreateShaderModule: %v", err)
	}
	bgl, err := dev.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "p1-probe-bgl-s",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageVertex | gputypes.ShaderStageFragment,
				Buffer:     &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform, MinBindingSize: 80},
			},
			{
				Binding:    1,
				Visibility: gputypes.ShaderStageFragment,
				Texture: &gputypes.TextureBindingLayout{
					SampleType:    gputypes.TextureSampleTypeFloat,
					ViewDimension: gputypes.TextureViewDimension2D,
				},
			},
			{
				Binding:    2,
				Visibility: gputypes.ShaderStageFragment,
				Sampler:    &gputypes.SamplerBindingLayout{Type: gputypes.SamplerBindingTypeFiltering},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroupLayout: %v", err)
	}
	clipBGL, err := dev.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "p1-probe-clipbgl-s",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageFragment,
				Buffer:     &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform, MinBindingSize: 32},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroupLayout(clip): %v", err)
	}
	pipeLayout, err := dev.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label:            "p1-probe-layout-s",
		BindGroupLayouts: []hal.BindGroupLayout{bgl, clipBGL},
	})
	if err != nil {
		t.Fatalf("CreatePipelineLayout: %v", err)
	}
	premul := gputypes.BlendStatePremultiplied()
	pipe, err := dev.CreateRenderPipeline(&RenderPipelineDescriptor{
		Label:  "p1-probe-pipe-s",
		Layout: pipeLayout,
		Vertex: hal.VertexState{
			Module:     mod,
			EntryPoint: "vs",
			Buffers: []gputypes.VertexBufferLayout{{
				ArrayStride: 16,
				StepMode:    gputypes.VertexStepModeVertex,
				Attributes: []gputypes.VertexAttribute{
					{Format: gputypes.VertexFormatFloat32x2, Offset: 0, ShaderLocation: 0},
					{Format: gputypes.VertexFormatFloat32x2, Offset: 8, ShaderLocation: 1},
				},
			}},
		},
		Primitive:   gputypes.PrimitiveState{Topology: gputypes.PrimitiveTopologyTriangleList},
		Multisample: gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		DepthStencil: &hal.DepthStencilState{
			Format:            gputypes.TextureFormatDepth24PlusStencil8,
			DepthWriteEnabled: false,
			DepthCompare:      gputypes.CompareFunctionAlways,
			StencilFront: hal.StencilFaceState{
				Compare:     gputypes.CompareFunctionAlways,
				FailOp:      gputypes.StencilOperationKeep,
				DepthFailOp: gputypes.StencilOperationKeep,
				PassOp:      gputypes.StencilOperationKeep,
			},
			StencilBack: hal.StencilFaceState{
				Compare:     gputypes.CompareFunctionAlways,
				FailOp:      gputypes.StencilOperationKeep,
				DepthFailOp: gputypes.StencilOperationKeep,
				PassOp:      gputypes.StencilOperationKeep,
			},
			StencilReadMask:  0x00,
			StencilWriteMask: 0x00,
		},
		Fragment: &hal.FragmentState{
			Module:     mod,
			EntryPoint: "fs",
			Targets: []gputypes.ColorTargetState{{
				Format:    cfg.Format,
				Blend:     &premul,
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
	if err != nil {
		t.Fatalf("CreateRenderPipeline: %v", err)
	}

	depthTex, err := dev.CreateTexture(&hal.TextureDescriptor{
		Label:         "p1-probe-depth-s",
		Size:          hal.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatDepth24PlusStencil8,
		Usage:         gputypes.TextureUsageRenderAttachment,
	})
	if err != nil {
		t.Fatalf("CreateTexture(depth): %v", err)
	}
	depthView, err := dev.CreateTextureView(depthTex, nil)
	if err != nil {
		t.Fatalf("CreateTextureView(depth): %v", err)
	}

	// 4KB slab with the session's 80B payload at offsets 0 and 256.
	slab, err := dev.CreateBuffer(&hal.BufferDescriptor{
		Label: "p1-probe-slab",
		Size:  4096,
		Usage: gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateBuffer(slab): %v", err)
	}
	slot := make([]byte, 256)
	putF := func(dst []byte, off int, v float32) {
		bits := math.Float32bits(v)
		dst[off] = byte(bits)
		dst[off+1] = byte(bits >> 8)
		dst[off+2] = byte(bits >> 16)
		dst[off+3] = byte(bits >> 24)
	}
	putF(slot, 0, 2.0/float32(w))
	putF(slot, 20, -2.0/float32(h))
	putF(slot, 40, 1.0)
	putF(slot, 48, -1.0)
	putF(slot, 52, 1.0)
	putF(slot, 60, 1.0)
	putF(slot, 64, 1.0)
	slabData := make([]byte, 512)
	copy(slabData[0:256], slot)
	copy(slabData[256:512], slot)
	if err := queue.WriteBuffer(slab, 0, slabData); err != nil {
		t.Fatalf("WriteBuffer(slab): %v", err)
	}
	clipBuf, err := dev.CreateBuffer(&hal.BufferDescriptor{
		Label: "p1-probe-clipbuf-s",
		Size:  32,
		Usage: gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateBuffer(clip): %v", err)
	}
	if err := queue.WriteBuffer(clipBuf, 0, make([]byte, 32)); err != nil {
		t.Fatalf("WriteBuffer(clip): %v", err)
	}

	bg0, err := dev.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "p1-probe-bg-s0",
		Layout: bgl,
		Entries: []hal.BindGroupEntry{
			{Binding: 0, Buffer: slab, Offset: 0, Size: 80},
			{Binding: 1, TextureView: texView},
			{Binding: 2, Sampler: sampler},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroup(0): %v", err)
	}
	bg1, err := dev.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "p1-probe-bg-s1",
		Layout: bgl,
		Entries: []hal.BindGroupEntry{
			{Binding: 0, Buffer: slab, Offset: 256, Size: 80},
			{Binding: 1, TextureView: texView},
			{Binding: 2, Sampler: sampler},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroup(1): %v", err)
	}
	clipBG, err := dev.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "p1-probe-clipbg-s",
		Layout: clipBGL,
		Entries: []hal.BindGroupEntry{
			{Binding: 0, Buffer: clipBuf},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroup(clip): %v", err)
	}

	// Left half quad (offset-0 bind) + right half quad (offset-256 bind).
	verts := []float32{
		0, 0, 0, 0,
		32, 0, 0.5, 0,
		0, 64, 0, 1,
		32, 0, 0.5, 0,
		32, 64, 0.5, 1,
		0, 64, 0, 1,
		32, 0, 0.5, 0,
		64, 0, 1, 0,
		32, 64, 0.5, 1,
		64, 0, 1, 0,
		64, 64, 1, 1,
		32, 64, 0.5, 1,
	}
	vbuf, err := dev.CreateBuffer(&hal.BufferDescriptor{
		Label: "p1-probe-vbuf-s",
		Size:  uint64(len(verts) * 4),
		Usage: gputypes.BufferUsageVertex | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateBuffer(vbuf): %v", err)
	}
	vbytes := unsafe.Slice((*byte)(unsafe.Pointer(&verts[0])), len(verts)*4)
	if err := queue.WriteBuffer(vbuf, 0, vbytes); err != nil {
		t.Fatalf("WriteBuffer(vbuf): %v", err)
	}

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
		DepthStencilAttachment: &hal.RenderPassDepthStencilAttachment{
			View:              depthView,
			DepthLoadOp:       gputypes.LoadOpClear,
			DepthStoreOp:      gputypes.StoreOpDiscard,
			DepthClearValue:   1.0,
			StencilLoadOp:     gputypes.LoadOpClear,
			StencilStoreOp:    gputypes.StoreOpStore,
			StencilClearValue: 0,
		},
	})
	if err != nil {
		t.Fatalf("BeginRenderPass: %v", err)
	}
	rp.SetViewport(0, 0, float32(w), float32(h), 0, 1)
	rp.SetScissorRect(0, 0, uint32(w), uint32(h))
	rp.SetPipeline(pipe)
	rp.SetBindGroup(1, clipBG, nil)
	rp.SetVertexBuffer(0, vbuf, 0)
	rp.SetBindGroup(0, bg0, nil)
	rp.Draw(6, 1, 0, 0)
	rp.SetBindGroup(0, bg1, nil)
	rp.Draw(6, 1, 6, 0)
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

	frame := actx.Lock()
	frame.Finish()
	frame.BindFramebuffer(gl.FRAMEBUFFER, surf.swapchainFBO)
	pixels := make([]byte, w*h*4)
	frame.ReadPixels(0, 0, int32(w), int32(h), gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&pixels[0]))
	frame.BindFramebuffer(gl.FRAMEBUFFER, 0)
	actx.Unlock()

	atSlab := func(x, y int) (int, int, int, int) {
		o := (y*w + x) * 4
		return int(pixels[o]), int(pixels[o+1]), int(pixels[o+2]), int(pixels[o+3])
	}
	lr, lg, lb, la := atSlab(16, 32)
	rr, rg, rb, ra := atSlab(48, 32)
	t.Logf("slab left=(%d,%d,%d,%d) right=(%d,%d,%d,%d)", lr, lg, lb, la, rr, rg, rb, ra)
	if lr != 255 || lg != 0 || lb != 0 || la != 255 {
		t.Errorf("left = (%d,%d,%d,%d), want (255,0,0,255)", lr, lg, lb, la)
	}
	if rr != 255 || rg != 0 || rb != 0 || ra != 255 {
		t.Errorf("right = (%d,%d,%d,%d), want (255,0,0,255)", rr, rg, rb, ra)
	}
}

//   - PASS (red) => backend handles tint stride + BGRA target; suspect the
//     real textured_quad.wgsl RRect/opacity math or session feeding.
//   - FAIL (not red) => backend breaks on the session vertex/BGRA shape.
func TestP1_TexturedSampleSessionShapeProbe(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	const w, h = 64, 64

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
	defer egl.MakeCurrent(eglCtx.Display(), egl.NoSurface, egl.NoSurface, egl.NoContext)

	actx := NewAdapterContext(eglCtx, glCtx, false)
	prober := actx.Lock()
	caps := queryAdapterCapabilities(prober)
	actx.Unlock()
	opened, err := (&Adapter{ctx: actx, caps: caps}).Open(0, gputypes.DefaultLimits())
	if err != nil {
		t.Fatalf("Adapter.Open: %v", err)
	}
	dev, queue := opened.Device, opened.Queue
	defer dev.Release()

	surf := &Surface{ctx: actx}
	cfg := &hal.SurfaceConfiguration{
		Width:       uint32(w),
		Height:      uint32(h),
		Format:      gputypes.TextureFormatBGRA8Unorm,
		Usage:       gputypes.TextureUsageRenderAttachment,
		PresentMode: hal.PresentModeFifo,
		AlphaMode:   gputypes.CompositeAlphaModeOpaque,
	}
	if err := surf.Configure(nil, cfg); err != nil {
		t.Fatalf("Surface.Configure: %v", err)
	}
	acq, err := surf.AcquireTexture(nil)
	if err != nil {
		t.Fatalf("AcquireTexture: %v", err)
	}
	defer surf.DiscardTexture(acq.Texture)
	view, err := dev.CreateTextureView(acq.Texture, nil)
	if err != nil {
		t.Fatalf("CreateTextureView(surface): %v", err)
	}

	tex, err := dev.CreateTexture(&hal.TextureDescriptor{
		Label:         "p1-probe-red-sess",
		Size:          hal.Extent3D{Width: 4, Height: 4, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateTexture: %v", err)
	}
	red := make([]byte, 4*4*4)
	for i := range red {
		red[i] = 0xFF
		if i%4 == 1 || i%4 == 2 {
			red[i] = 0x00
		}
	}
	if err := queue.WriteTexture(&hal.ImageCopyTexture{
		Texture:  tex,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, red, &hal.ImageDataLayout{
		Offset:       0,
		BytesPerRow:  16,
		RowsPerImage: 4,
	}, &hal.Extent3D{Width: 4, Height: 4, DepthOrArrayLayers: 1}); err != nil {
		t.Fatalf("WriteTexture: %v", err)
	}
	texView, err := dev.CreateTextureView(tex, nil)
	if err != nil {
		t.Fatalf("CreateTextureView(tex): %v", err)
	}

	sampler, err := dev.CreateSampler(&hal.SamplerDescriptor{
		Label:        "p1-probe-sampler-sess",
		AddressModeU: gputypes.AddressModeClampToEdge,
		AddressModeV: gputypes.AddressModeClampToEdge,
		AddressModeW: gputypes.AddressModeClampToEdge,
		MagFilter:    gputypes.FilterModeLinear,
		MinFilter:    gputypes.FilterModeLinear,
		MipmapFilter: gputypes.MipmapFilterModeNearest,
	})
	if err != nil {
		t.Fatalf("CreateSampler: %v", err)
	}

	const wgsl = `
struct U {
  m: mat4x4f,
  o: vec4f,
};
struct C {
  r: vec4f,
  rad: f32,
  en: f32,
  pad: vec2f,
};
@group(0) @binding(0) var<uniform> u: U;
@group(0) @binding(1) var t: texture_2d<f32>;
@group(0) @binding(2) var s: sampler;
@group(1) @binding(0) var<uniform> c: C;
struct VSOut {
  @builtin(position) pos: vec4f,
  @location(0) uv: vec2f,
  @location(1) tint: vec4f,
};
@vertex
fn vs(@location(0) xy: vec2f, @location(1) uv: vec2f, @location(2) tint: vec4f) -> VSOut {
  var o: VSOut;
  o.pos = u.m * vec4f(xy, 0.0, 1.0);
  o.uv = uv;
  o.tint = tint;
  return o;
}
@fragment
fn fs(@location(0) uv: vec2f, @location(1) tint: vec4f) -> @location(0) vec4f {
  let texel = textureSample(t, s, uv);
  let cov = c.en * 0.0 + (1.0 - c.en);
  return texel * u.o.x * cov * tint;
}
`
	mod, err := dev.CreateShaderModule(&ShaderModuleDescriptor{WGSL: wgsl})
	if err != nil {
		t.Fatalf("CreateShaderModule: %v", err)
	}
	bgl, err := dev.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "p1-probe-bgl-sess",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageVertex | gputypes.ShaderStageFragment,
				Buffer:     &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform, MinBindingSize: 80},
			},
			{
				Binding:    1,
				Visibility: gputypes.ShaderStageFragment,
				Texture: &gputypes.TextureBindingLayout{
					SampleType:    gputypes.TextureSampleTypeFloat,
					ViewDimension: gputypes.TextureViewDimension2D,
				},
			},
			{
				Binding:    2,
				Visibility: gputypes.ShaderStageFragment,
				Sampler:    &gputypes.SamplerBindingLayout{Type: gputypes.SamplerBindingTypeFiltering},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroupLayout: %v", err)
	}
	clipBGL, err := dev.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "p1-probe-clipbgl-sess",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageFragment,
				Buffer:     &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform, MinBindingSize: 32},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroupLayout(clip): %v", err)
	}
	pipeLayout, err := dev.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label:            "p1-probe-layout-sess",
		BindGroupLayouts: []hal.BindGroupLayout{bgl, clipBGL},
	})
	if err != nil {
		t.Fatalf("CreatePipelineLayout: %v", err)
	}
	premul := gputypes.BlendStatePremultiplied()
	pipe, err := dev.CreateRenderPipeline(&RenderPipelineDescriptor{
		Label:  "p1-probe-pipe-sess",
		Layout: pipeLayout,
		Vertex: hal.VertexState{
			Module:     mod,
			EntryPoint: "vs",
			Buffers: []gputypes.VertexBufferLayout{{
				ArrayStride: 32,
				StepMode:    gputypes.VertexStepModeVertex,
				Attributes: []gputypes.VertexAttribute{
					{Format: gputypes.VertexFormatFloat32x2, Offset: 0, ShaderLocation: 0},
					{Format: gputypes.VertexFormatFloat32x2, Offset: 8, ShaderLocation: 1},
					{Format: gputypes.VertexFormatFloat32x4, Offset: 16, ShaderLocation: 2},
				},
			}},
		},
		Primitive:   gputypes.PrimitiveState{Topology: gputypes.PrimitiveTopologyTriangleList},
		Multisample: gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		DepthStencil: &hal.DepthStencilState{
			Format:            gputypes.TextureFormatDepth24PlusStencil8,
			DepthWriteEnabled: false,
			DepthCompare:      gputypes.CompareFunctionAlways,
			StencilFront: hal.StencilFaceState{
				Compare:     gputypes.CompareFunctionAlways,
				FailOp:      gputypes.StencilOperationKeep,
				DepthFailOp: gputypes.StencilOperationKeep,
				PassOp:      gputypes.StencilOperationKeep,
			},
			StencilBack: hal.StencilFaceState{
				Compare:     gputypes.CompareFunctionAlways,
				FailOp:      gputypes.StencilOperationKeep,
				DepthFailOp: gputypes.StencilOperationKeep,
				PassOp:      gputypes.StencilOperationKeep,
			},
			StencilReadMask:  0x00,
			StencilWriteMask: 0x00,
		},
		Fragment: &hal.FragmentState{
			Module:     mod,
			EntryPoint: "fs",
			Targets: []gputypes.ColorTargetState{{
				Format:    cfg.Format,
				Blend:     &premul,
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
	if err != nil {
		t.Fatalf("CreateRenderPipeline: %v", err)
	}

	depthTex, err := dev.CreateTexture(&hal.TextureDescriptor{
		Label:         "p1-probe-depth-sess",
		Size:          hal.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatDepth24PlusStencil8,
		Usage:         gputypes.TextureUsageRenderAttachment,
	})
	if err != nil {
		t.Fatalf("CreateTexture(depth): %v", err)
	}
	depthView, err := dev.CreateTextureView(depthTex, nil)
	if err != nil {
		t.Fatalf("CreateTextureView(depth): %v", err)
	}

	slab, err := dev.CreateBuffer(&hal.BufferDescriptor{
		Label: "p1-probe-slab-sess",
		Size:  4096,
		Usage: gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateBuffer(slab): %v", err)
	}
	slot := make([]byte, 256)
	putF := func(dst []byte, off int, v float32) {
		bits := math.Float32bits(v)
		dst[off] = byte(bits)
		dst[off+1] = byte(bits >> 8)
		dst[off+2] = byte(bits >> 16)
		dst[off+3] = byte(bits >> 24)
	}
	putF(slot, 0, 2.0/float32(w))
	putF(slot, 20, -2.0/float32(h))
	putF(slot, 40, 1.0)
	putF(slot, 48, -1.0)
	putF(slot, 52, 1.0)
	putF(slot, 60, 1.0)
	putF(slot, 64, 1.0)
	if err := queue.WriteBuffer(slab, 0, slot[:256]); err != nil {
		t.Fatalf("WriteBuffer(slab): %v", err)
	}
	clipBuf, err := dev.CreateBuffer(&hal.BufferDescriptor{
		Label: "p1-probe-clipbuf-sess",
		Size:  32,
		Usage: gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateBuffer(clip): %v", err)
	}
	if err := queue.WriteBuffer(clipBuf, 0, make([]byte, 32)); err != nil {
		t.Fatalf("WriteBuffer(clip): %v", err)
	}

	bg, err := dev.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "p1-probe-bg-sess",
		Layout: bgl,
		Entries: []hal.BindGroupEntry{
			{Binding: 0, Buffer: slab, Offset: 0, Size: 80},
			{Binding: 1, TextureView: texView},
			{Binding: 2, Sampler: sampler},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroup: %v", err)
	}
	clipBG, err := dev.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "p1-probe-clipbg-sess",
		Layout: clipBGL,
		Entries: []hal.BindGroupEntry{
			{Binding: 0, Buffer: clipBuf},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroup(clip): %v", err)
	}

	// Fullscreen quad in pixel coords with identity tint (1,1,1,1).
	verts := []float32{
		0, 0, 0, 0, 1, 1, 1, 1,
		64, 0, 1, 0, 1, 1, 1, 1,
		0, 64, 0, 1, 1, 1, 1, 1,
		64, 0, 1, 0, 1, 1, 1, 1,
		64, 64, 1, 1, 1, 1, 1, 1,
		0, 64, 0, 1, 1, 1, 1, 1,
	}
	vbuf, err := dev.CreateBuffer(&hal.BufferDescriptor{
		Label: "p1-probe-vbuf-sess",
		Size:  uint64(len(verts) * 4),
		Usage: gputypes.BufferUsageVertex | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateBuffer(vbuf): %v", err)
	}
	vbytes := unsafe.Slice((*byte)(unsafe.Pointer(&verts[0])), len(verts)*4)
	if err := queue.WriteBuffer(vbuf, 0, vbytes); err != nil {
		t.Fatalf("WriteBuffer(vbuf): %v", err)
	}

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
		DepthStencilAttachment: &hal.RenderPassDepthStencilAttachment{
			View:              depthView,
			DepthLoadOp:       gputypes.LoadOpClear,
			DepthStoreOp:      gputypes.StoreOpDiscard,
			DepthClearValue:   1.0,
			StencilLoadOp:     gputypes.LoadOpClear,
			StencilStoreOp:    gputypes.StoreOpStore,
			StencilClearValue: 0,
		},
	})
	if err != nil {
		t.Fatalf("BeginRenderPass: %v", err)
	}
	rp.SetViewport(0, 0, float32(w), float32(h), 0, 1)
	rp.SetScissorRect(0, 0, uint32(w), uint32(h))
	rp.SetPipeline(pipe)
	rp.SetBindGroup(1, clipBG, nil)
	rp.SetVertexBuffer(0, vbuf, 0)
	rp.SetBindGroup(0, bg, nil)
	rp.Draw(6, 1, 0, 0)
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

	frame := actx.Lock()
	frame.Finish()
	frame.BindFramebuffer(gl.FRAMEBUFFER, surf.swapchainFBO)
	pixels := make([]byte, w*h*4)
	frame.ReadPixels(0, 0, int32(w), int32(h), gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&pixels[0]))
	frame.BindFramebuffer(gl.FRAMEBUFFER, 0)
	actx.Unlock()

	at := func(x, y int) (int, int, int, int) {
		o := (y*w + x) * 4
		return int(pixels[o]), int(pixels[o+1]), int(pixels[o+2]), int(pixels[o+3])
	}
	cr, cg, cb, ca := at(w/2, h/2)
	t.Logf("session-shape center=(%d,%d,%d,%d)", cr, cg, cb, ca)
	if cr != 255 || cg != 0 || cb != 0 || ca != 255 {
		t.Errorf("center = (%d,%d,%d,%d), want (255,0,0,255)", cr, cg, cb, ca)
	}
}

//   - PASS (red) => real shader + 64x64 upload fine; suspect session vertex/
//     uniform feeding (positions, UVs, opacity, cache view).
//   - FAIL (not red) => real shader translation or 64x64 path broken on GL.
func TestP1_TexturedSampleRealShaderProbe(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	const w, h = 64, 64

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
	defer egl.MakeCurrent(eglCtx.Display(), egl.NoSurface, egl.NoSurface, egl.NoContext)

	actx := NewAdapterContext(eglCtx, glCtx, false)
	prober := actx.Lock()
	caps := queryAdapterCapabilities(prober)
	actx.Unlock()
	opened, err := (&Adapter{ctx: actx, caps: caps}).Open(0, gputypes.DefaultLimits())
	if err != nil {
		t.Fatalf("Adapter.Open: %v", err)
	}
	dev, queue := opened.Device, opened.Queue
	defer dev.Release()

	surf := &Surface{ctx: actx}
	cfg := &hal.SurfaceConfiguration{
		Width:       uint32(w),
		Height:      uint32(h),
		Format:      gputypes.TextureFormatBGRA8Unorm,
		Usage:       gputypes.TextureUsageRenderAttachment,
		PresentMode: hal.PresentModeFifo,
		AlphaMode:   gputypes.CompositeAlphaModeOpaque,
	}
	if err := surf.Configure(nil, cfg); err != nil {
		t.Fatalf("Surface.Configure: %v", err)
	}
	acq, err := surf.AcquireTexture(nil)
	if err != nil {
		t.Fatalf("AcquireTexture: %v", err)
	}
	defer surf.DiscardTexture(acq.Texture)
	view, err := dev.CreateTextureView(acq.Texture, nil)
	if err != nil {
		t.Fatalf("CreateTextureView(surface): %v", err)
	}

	tex, err := dev.CreateTexture(&hal.TextureDescriptor{
		Label:         "p1-probe-red-real",
		Size:          hal.Extent3D{Width: 64, Height: 64, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateTexture: %v", err)
	}
	red := make([]byte, 64*64*4)
	for i := 0; i < len(red); i += 4 {
		red[i], red[i+1], red[i+2], red[i+3] = 0xFF, 0x00, 0x00, 0xFF
	}
	if err := queue.WriteTexture(&hal.ImageCopyTexture{
		Texture:  tex,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, red, &hal.ImageDataLayout{
		Offset:       0,
		BytesPerRow:  256,
		RowsPerImage: 64,
	}, &hal.Extent3D{Width: 64, Height: 64, DepthOrArrayLayers: 1}); err != nil {
		t.Fatalf("WriteTexture: %v", err)
	}
	// Session-exact explicit view descriptor.
	texView, err := dev.CreateTextureView(tex, &hal.TextureViewDescriptor{
		Label:         "image_cache_view",
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Dimension:     gputypes.TextureViewDimension2D,
		Aspect:        gputypes.TextureAspectAll,
		MipLevelCount: 1,
	})
	if err != nil {
		t.Fatalf("CreateTextureView(tex): %v", err)
	}

	sampler, err := dev.CreateSampler(&hal.SamplerDescriptor{
		Label:        "image_sampler_linear",
		AddressModeU: gputypes.AddressModeClampToEdge,
		AddressModeV: gputypes.AddressModeClampToEdge,
		AddressModeW: gputypes.AddressModeClampToEdge,
		MagFilter:    gputypes.FilterModeLinear,
		MinFilter:    gputypes.FilterModeLinear,
		MipmapFilter: gputypes.MipmapFilterModeNearest,
	})
	if err != nil {
		t.Fatalf("CreateSampler: %v", err)
	}

	const wgsl = `
struct ImageUniforms {
    transform: mat4x4<f32>,
    opacity_pad: vec4<f32>,
}
struct VertexInput {
    @location(0) position: vec2<f32>,
    @location(1) tex_coord: vec2<f32>,
    @location(2) tint: vec4<f32>,
}
struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) tex_coord: vec2<f32>,
    @location(1) tint: vec4<f32>,
}
@group(0) @binding(0) var<uniform> uniforms: ImageUniforms;
@group(0) @binding(1) var image_texture: texture_2d<f32>;
@group(0) @binding(2) var image_sampler: sampler;
struct ClipParams {
    clip_rect: vec4<f32>,
    clip_radius: f32,
    clip_enabled: f32,
    _pad: vec2<f32>,
}
@group(1) @binding(0) var<uniform> clip: ClipParams;
fn rrect_clip_coverage(frag_pos: vec2<f32>) -> f32 {
    let cx = (clip.clip_rect.x + clip.clip_rect.z) * 0.5;
    let cy = (clip.clip_rect.y + clip.clip_rect.w) * 0.5;
    let hw = (clip.clip_rect.z - clip.clip_rect.x) * 0.5;
    let hh = (clip.clip_rect.w - clip.clip_rect.y) * 0.5;
    let r = clip.clip_radius;
    let dx = sqrt((frag_pos.x - cx) * (frag_pos.x - cx));
    let dy = sqrt((frag_pos.y - cy) * (frag_pos.y - cy));
    let qx = dx - hw + r;
    let qy = dy - hh + r;
    let mqx = (qx + sqrt(qx * qx)) * 0.5;
    let mqy = (qy + sqrt(qy * qy)) * 0.5;
    let outside = sqrt(mqx * mqx + mqy * mqy);
    let qdiff = qx - qy;
    let max_qxy = (qx + qy + sqrt(qdiff * qdiff)) * 0.5;
    let inside = (max_qxy - sqrt(max_qxy * max_qxy)) * 0.5;
    let d = outside + inside - r;
    let aa_hw = 0.75;
    let t_raw = d / (2.0 * aa_hw) + 0.5;
    let t_pos = (t_raw + sqrt(t_raw * t_raw)) * 0.5;
    let t_diff = t_pos - 1.0;
    let t = (t_pos + 1.0 - sqrt(t_diff * t_diff)) * 0.5;
    let sdf_cov = 1.0 - t * t * (3.0 - 2.0 * t);
    return clip.clip_enabled * sdf_cov + (1.0 - clip.clip_enabled);
}
@vertex
fn vs_main(in: VertexInput) -> VertexOutput {
    var out: VertexOutput;
    let p = vec4<f32>(in.position, 0.0, 1.0);
    let col0 = uniforms.transform[0];
    let col1 = uniforms.transform[1];
    let col2 = uniforms.transform[2];
    let col3 = uniforms.transform[3];
    let pos = p.x * col0 + p.y * col1 + p.z * col2 + p.w * col3;
    out.position = pos;
    out.tex_coord = in.tex_coord;
    out.tint = in.tint;
    return out;
}
@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    let texel = textureSample(image_texture, image_sampler, in.tex_coord);
    let clip_cov = rrect_clip_coverage(in.position.xy);
    let opacity = uniforms.opacity_pad.x * clip_cov;
    return texel * opacity * in.tint;
}
`
	mod, err := dev.CreateShaderModule(&ShaderModuleDescriptor{WGSL: wgsl})
	if err != nil {
		t.Fatalf("CreateShaderModule: %v", err)
	}
	bgl, err := dev.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "textured_quad_bind_layout",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageVertex | gputypes.ShaderStageFragment,
				Buffer:     &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform, MinBindingSize: 80},
			},
			{
				Binding:    1,
				Visibility: gputypes.ShaderStageFragment,
				Texture: &gputypes.TextureBindingLayout{
					SampleType:    gputypes.TextureSampleTypeFloat,
					ViewDimension: gputypes.TextureViewDimension2D,
				},
			},
			{
				Binding:    2,
				Visibility: gputypes.ShaderStageFragment,
				Sampler:    &gputypes.SamplerBindingLayout{Type: gputypes.SamplerBindingTypeFiltering},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroupLayout: %v", err)
	}
	clipBGL, err := dev.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "clipbgl-real",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageFragment,
				Buffer:     &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform, MinBindingSize: 32},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroupLayout(clip): %v", err)
	}
	pipeLayout, err := dev.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label:            "textured_quad_pipe_layout",
		BindGroupLayouts: []hal.BindGroupLayout{bgl, clipBGL},
	})
	if err != nil {
		t.Fatalf("CreatePipelineLayout: %v", err)
	}
	premul := gputypes.BlendStatePremultiplied()
	pipe, err := dev.CreateRenderPipeline(&RenderPipelineDescriptor{
		Label:  "textured_quad_pipeline_with_stencil",
		Layout: pipeLayout,
		Vertex: hal.VertexState{
			Module:     mod,
			EntryPoint: "vs_main",
			Buffers: []gputypes.VertexBufferLayout{{
				ArrayStride: 32,
				StepMode:    gputypes.VertexStepModeVertex,
				Attributes: []gputypes.VertexAttribute{
					{Format: gputypes.VertexFormatFloat32x2, Offset: 0, ShaderLocation: 0},
					{Format: gputypes.VertexFormatFloat32x2, Offset: 8, ShaderLocation: 1},
					{Format: gputypes.VertexFormatFloat32x4, Offset: 16, ShaderLocation: 2},
				},
			}},
		},
		Primitive:   gputypes.PrimitiveState{Topology: gputypes.PrimitiveTopologyTriangleList},
		Multisample: gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		DepthStencil: &hal.DepthStencilState{
			Format:            gputypes.TextureFormatDepth24PlusStencil8,
			DepthWriteEnabled: false,
			DepthCompare:      gputypes.CompareFunctionAlways,
			StencilFront: hal.StencilFaceState{
				Compare:     gputypes.CompareFunctionAlways,
				FailOp:      gputypes.StencilOperationKeep,
				DepthFailOp: gputypes.StencilOperationKeep,
				PassOp:      gputypes.StencilOperationKeep,
			},
			StencilBack: hal.StencilFaceState{
				Compare:     gputypes.CompareFunctionAlways,
				FailOp:      gputypes.StencilOperationKeep,
				DepthFailOp: gputypes.StencilOperationKeep,
				PassOp:      gputypes.StencilOperationKeep,
			},
			StencilReadMask:  0x00,
			StencilWriteMask: 0x00,
		},
		Fragment: &hal.FragmentState{
			Module:     mod,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{{
				Format:    cfg.Format,
				Blend:     &premul,
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
	if err != nil {
		t.Fatalf("CreateRenderPipeline: %v", err)
	}

	depthTex, err := dev.CreateTexture(&hal.TextureDescriptor{
		Label:         "p1-probe-depth-real",
		Size:          hal.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatDepth24PlusStencil8,
		Usage:         gputypes.TextureUsageRenderAttachment,
	})
	if err != nil {
		t.Fatalf("CreateTexture(depth): %v", err)
	}
	depthView, err := dev.CreateTextureView(depthTex, nil)
	if err != nil {
		t.Fatalf("CreateTextureView(depth): %v", err)
	}

	slab, err := dev.CreateBuffer(&hal.BufferDescriptor{
		Label: "image_uniform_slab",
		Size:  4096,
		Usage: gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateBuffer(slab): %v", err)
	}
	slot := make([]byte, 256)
	putF := func(dst []byte, off int, v float32) {
		bits := math.Float32bits(v)
		dst[off] = byte(bits)
		dst[off+1] = byte(bits >> 8)
		dst[off+2] = byte(bits >> 16)
		dst[off+3] = byte(bits >> 24)
	}
	putF(slot, 0, 2.0/float32(w))
	putF(slot, 20, -2.0/float32(h))
	putF(slot, 40, 1.0)
	putF(slot, 48, -1.0)
	putF(slot, 52, 1.0)
	putF(slot, 60, 1.0)
	putF(slot, 64, 1.0)
	if err := queue.WriteBuffer(slab, 0, slot[:256]); err != nil {
		t.Fatalf("WriteBuffer(slab): %v", err)
	}
	clipBuf, err := dev.CreateBuffer(&hal.BufferDescriptor{
		Label: "clipbuf-real",
		Size:  32,
		Usage: gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateBuffer(clip): %v", err)
	}
	if err := queue.WriteBuffer(clipBuf, 0, make([]byte, 32)); err != nil {
		t.Fatalf("WriteBuffer(clip): %v", err)
	}

	bg, err := dev.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "image_bind_0",
		Layout: bgl,
		Entries: []hal.BindGroupEntry{
			{Binding: 0, Buffer: slab, Offset: 0, Size: 80},
			{Binding: 1, TextureView: texView},
			{Binding: 2, Sampler: sampler},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroup: %v", err)
	}
	clipBG, err := dev.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "clipbg-real",
		Layout: clipBGL,
		Entries: []hal.BindGroupEntry{
			{Binding: 0, Buffer: clipBuf},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroup(clip): %v", err)
	}

	verts := []float32{
		0, 0, 0, 0, 1, 1, 1, 1,
		64, 0, 1, 0, 1, 1, 1, 1,
		0, 64, 0, 1, 1, 1, 1, 1,
		64, 0, 1, 0, 1, 1, 1, 1,
		64, 64, 1, 1, 1, 1, 1, 1,
		0, 64, 0, 1, 1, 1, 1, 1,
	}
	vbuf, err := dev.CreateBuffer(&hal.BufferDescriptor{
		Label: "image_vert_buf",
		Size:  uint64(len(verts) * 4),
		Usage: gputypes.BufferUsageVertex | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateBuffer(vbuf): %v", err)
	}
	vbytes := unsafe.Slice((*byte)(unsafe.Pointer(&verts[0])), len(verts)*4)
	if err := queue.WriteBuffer(vbuf, 0, vbytes); err != nil {
		t.Fatalf("WriteBuffer(vbuf): %v", err)
	}

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
		DepthStencilAttachment: &hal.RenderPassDepthStencilAttachment{
			View:              depthView,
			DepthLoadOp:       gputypes.LoadOpClear,
			DepthStoreOp:      gputypes.StoreOpDiscard,
			DepthClearValue:   1.0,
			StencilLoadOp:     gputypes.LoadOpClear,
			StencilStoreOp:    gputypes.StoreOpStore,
			StencilClearValue: 0,
		},
	})
	if err != nil {
		t.Fatalf("BeginRenderPass: %v", err)
	}
	rp.SetViewport(0, 0, float32(w), float32(h), 0, 1)
	rp.SetScissorRect(0, 0, uint32(w), uint32(h))
	rp.SetPipeline(pipe)
	rp.SetBindGroup(1, clipBG, nil)
	rp.SetVertexBuffer(0, vbuf, 0)
	rp.SetBindGroup(0, bg, nil)
	rp.Draw(6, 1, 0, 0)
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

	frame := actx.Lock()
	frame.Finish()
	frame.BindFramebuffer(gl.FRAMEBUFFER, surf.swapchainFBO)
	pixels := make([]byte, w*h*4)
	frame.ReadPixels(0, 0, int32(w), int32(h), gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&pixels[0]))
	frame.BindFramebuffer(gl.FRAMEBUFFER, 0)
	actx.Unlock()

	atReal := func(x, y int) (int, int, int, int) {
		o := (y*w + x) * 4
		return int(pixels[o]), int(pixels[o+1]), int(pixels[o+2]), int(pixels[o+3])
	}
	cr, cg, cb, ca := atReal(w/2, h/2)
	t.Logf("real-shader center=(%d,%d,%d,%d)", cr, cg, cb, ca)
	if cr != 255 || cg != 0 || cb != 0 || ca != 255 {
		t.Errorf("center = (%d,%d,%d,%d), want (255,0,0,255)", cr, cg, cb, ca)
	}
}
