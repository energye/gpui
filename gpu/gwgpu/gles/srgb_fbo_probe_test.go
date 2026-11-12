//go:build linux && !(js && wasm)

package gles

import (
	"runtime"
	"testing"
	"unsafe"

	"github.com/energye/gpui/gpu/gwgpu/gles/egl"
	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
	"github.com/energye/gpui/gpu/shader/glsl"
)

// P3.1b-0 GLES 探针：sRGB FBO 的写/编码/混合/读回，以及 GL_FRAMEBUFFER_SRGB
// 开关的实际语义。不是交付能力，只回答「GLES 腿能不能按 sRGB 目标工作」。
//
// 预期（GL_FRAMEBUFFER_SRGB 开启）：
//
//	base 线性 (0.5,0.25,0.1) 写 sRGB FBO → 读回 (188,137,89)；
//	50% alpha 线性灰叠加 → 线性混合 (0.5,0.375,0.3) → (188,165,149)。
//
// 关闭时：base 读回原始值 (128,64,26)；混合在编码域约 (127,95,76)。
func TestP31bSrgbFBOProbe(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := egl.Init(); err != nil {
		t.Skipf("egl.Init unavailable: %v", err)
	}
	cfg := egl.DefaultContextConfig()
	cfg.GLES = false
	eglCtx, err := egl.NewContext(cfg)
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
	t.Logf("GL offscreen: %s", glCtx.GetString(gl.VERSION))

	vsWGSL := `
@vertex fn vs_main(@builtin(vertex_index) vi: u32) -> @builtin(position) vec4<f32> {
    var pos = array<vec2<f32>, 3>(vec2<f32>(-1.0, -1.0), vec2<f32>(3.0, -1.0), vec2<f32>(-1.0, 3.0));
    return vec4<f32>(pos[vi], 0.0, 1.0);
}`
	fsBaseWGSL := `
@fragment fn fs_base() -> @location(0) vec4<f32> { return vec4<f32>(0.5, 0.25, 0.1, 1.0); }`
	fsBlendWGSL := `
@fragment fn fs_blend() -> @location(0) vec4<f32> { return vec4<f32>(0.5, 0.5, 0.5, 0.5); }`
	vsGLSL, _, err := compileWGSLToGLSL(glsl.Version330, vsWGSL, "vs_main", nil)
	if err != nil {
		t.Fatalf("vertex WGSL→GLSL: %v", err)
	}
	fsBaseGLSL, _, err := compileWGSLToGLSL(glsl.Version330, fsBaseWGSL, "fs_base", nil)
	if err != nil {
		t.Fatalf("base WGSL→GLSL: %v", err)
	}
	fsBlendGLSL, _, err := compileWGSLToGLSL(glsl.Version330, fsBlendWGSL, "fs_blend", nil)
	if err != nil {
		t.Fatalf("blend WGSL→GLSL: %v", err)
	}

	program := func(vs, fs string) uint32 {
		vsID := h4cCompileGL(t, glCtx, gl.VERTEX_SHADER, vs)
		fsID := h4cCompileGL(t, glCtx, gl.FRAGMENT_SHADER, fs)
		p := glCtx.CreateProgram()
		glCtx.AttachShader(p, vsID)
		glCtx.AttachShader(p, fsID)
		glCtx.LinkProgram(p)
		var status int32
		glCtx.GetProgramiv(p, gl.LINK_STATUS, &status)
		if status == gl.FALSE {
			t.Fatalf("link failed: %s", glCtx.GetProgramInfoLog(p))
		}
		glCtx.DeleteShader(vsID)
		glCtx.DeleteShader(fsID)
		return p
	}
	progBase := program(vsGLSL, fsBaseGLSL)
	defer glCtx.DeleteProgram(progBase)
	progBlend := program(vsGLSL, fsBlendGLSL)
	defer glCtx.DeleteProgram(progBlend)

	vao := glCtx.GenVertexArrays(1)
	if vao == 0 {
		t.Fatal("GenVertexArrays returned 0")
	}
	defer glCtx.DeleteVertexArrays(vao)
	glCtx.BindVertexArray(vao)

	const size = 32
	const glFramebufferSrgb = 0x8DB9
	run := func(fboSrgb bool) ([4]byte, [4]byte) {
		tex := glCtx.GenTextures(1)
		glCtx.BindTexture(gl.TEXTURE_2D, tex)
		internal := uint32(gl.SRGB8_ALPHA8)
		if !fboSrgb {
			internal = gl.RGBA8
		}
		glCtx.TexImage2D(gl.TEXTURE_2D, 0, int32(internal), size, size, 0, gl.RGBA, gl.UNSIGNED_BYTE, 0)
		glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, int32(gl.NEAREST))
		glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, int32(gl.NEAREST))
		fbo := glCtx.GenFramebuffers(1)
		glCtx.BindFramebuffer(gl.FRAMEBUFFER, fbo)
		glCtx.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, tex, 0)
		if got := glCtx.CheckFramebufferStatus(gl.FRAMEBUFFER); got != gl.FRAMEBUFFER_COMPLETE {
			t.Fatalf("FBO incomplete: 0x%x (srgb=%v)", got, fboSrgb)
		}
		glCtx.Viewport(0, 0, size, size)
		if fboSrgb {
			glCtx.Enable(glFramebufferSrgb)
		} else {
			glCtx.Disable(glFramebufferSrgb)
		}
		glCtx.ClearColor(0, 0, 0, 1)
		glCtx.Clear(gl.COLOR_BUFFER_BIT)
		glCtx.Disable(gl.BLEND)
		glCtx.UseProgram(progBase)
		glCtx.DrawArrays(gl.TRIANGLES, 0, 3)
		buf := make([]byte, size*size*4)
		glCtx.ReadPixels(0, 0, size, size, gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&buf[0]))
		var base [4]byte
		copy(base[:], buf[(size/2*size+size/2)*4:])
		glCtx.Enable(gl.BLEND)
		glCtx.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
		glCtx.UseProgram(progBlend)
		glCtx.DrawArrays(gl.TRIANGLES, 0, 3)
		glCtx.ReadPixels(0, 0, size, size, gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&buf[0]))
		var blend [4]byte
		copy(blend[:], buf[(size/2*size+size/2)*4:])
		glCtx.Disable(gl.BLEND)
		glCtx.Disable(glFramebufferSrgb)
		glCtx.BindFramebuffer(gl.FRAMEBUFFER, 0)
		glCtx.DeleteFramebuffers(fbo)
		glCtx.DeleteTextures(tex)
		return base, blend
	}

	srgbBase, srgbBlend := run(true)
	unormBase, unormBlend := run(false)
	t.Logf("P3.1b-0 GLES sRGB FBO base=%v blend=%v; Unorm FBO base=%v blend=%v", srgbBase, srgbBlend, unormBase, unormBlend)

	wantSrgbBase := [3]int{188, 137, 89}
	for i := 0; i < 3; i++ {
		if d := int(srgbBase[i]) - wantSrgbBase[i]; d < -2 || d > 2 {
			t.Fatalf("sRGB FBO base channel %d = %d, want %d±2 (sRGB encoding); bytes=%v", i, srgbBase[i], wantSrgbBase[i], srgbBase)
		}
	}
	wantSrgbBlend := [3]int{188, 165, 149}
	for i := 0; i < 3; i++ {
		if d := int(srgbBlend[i]) - wantSrgbBlend[i]; d < -2 || d > 2 {
			t.Fatalf("sRGB FBO blend channel %d = %d, want %d±2 (linear blend); bytes=%v", i, srgbBlend[i], wantSrgbBlend[i], srgbBlend)
		}
	}
	wantUnormBase := [3]int{128, 64, 26}
	for i := 0; i < 3; i++ {
		if d := int(unormBase[i]) - wantUnormBase[i]; d < -2 || d > 2 {
			t.Fatalf("Unorm FBO base channel %d = %d, want %d±2; bytes=%v", i, unormBase[i], wantUnormBase[i], unormBase)
		}
	}
	wantUnormBlend := [3]int{127, 95, 76}
	for i := 0; i < 3; i++ {
		if d := int(unormBlend[i]) - wantUnormBlend[i]; d < -2 || d > 2 {
			t.Fatalf("Unorm FBO blend channel %d = %d, want %d±2; bytes=%v", i, unormBlend[i], wantUnormBlend[i], unormBlend)
		}
	}
}
