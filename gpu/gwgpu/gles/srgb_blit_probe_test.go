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

// P3.1b-0 GLES 探针：sRGB FBO ↔ Unorm FBO 的 glBlitFramebuffer 语义。
//
// 这是 swapchain/present 路径的前置风险：若 opt-in linear 目标的 FBO 是 sRGB，
// 最终 blit 到 Unorm 目标/FBO 0 时，GL_FRAMEBUFFER_SRGB 的开关决定是「解码成
// 线性」还是「原样拷贝」。本探针把四种组合的字节量出来。
func TestP31bSrgbBlitProbe(t *testing.T) {
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

	vsWGSL := `
@vertex fn vs_main(@builtin(vertex_index) vi: u32) -> @builtin(position) vec4<f32> {
    var pos = array<vec2<f32>, 3>(vec2<f32>(-1.0, -1.0), vec2<f32>(3.0, -1.0), vec2<f32>(-1.0, 3.0));
    return vec4<f32>(pos[vi], 0.0, 1.0);
}`
	fsWGSL := `
@fragment fn fs_base() -> @location(0) vec4<f32> { return vec4<f32>(0.5, 0.25, 0.1, 1.0); }`
	vsGLSL, _, err := compileWGSLToGLSL(glsl.Version330, vsWGSL, "vs_main", nil)
	if err != nil {
		t.Fatalf("vertex WGSL→GLSL: %v", err)
	}
	fsGLSL, _, err := compileWGSLToGLSL(glsl.Version330, fsWGSL, "fs_base", nil)
	if err != nil {
		t.Fatalf("fragment WGSL→GLSL: %v", err)
	}
	vsID := h4cCompileGL(t, glCtx, gl.VERTEX_SHADER, vsGLSL)
	fsID := h4cCompileGL(t, glCtx, gl.FRAGMENT_SHADER, fsGLSL)
	prog := glCtx.CreateProgram()
	glCtx.AttachShader(prog, vsID)
	glCtx.AttachShader(prog, fsID)
	glCtx.LinkProgram(prog)
	var link int32
	glCtx.GetProgramiv(prog, gl.LINK_STATUS, &link)
	if link == gl.FALSE {
		t.Fatalf("link failed: %s", glCtx.GetProgramInfoLog(prog))
	}
	glCtx.DeleteShader(vsID)
	glCtx.DeleteShader(fsID)
	defer glCtx.DeleteProgram(prog)
	vao := glCtx.GenVertexArrays(1)
	defer glCtx.DeleteVertexArrays(vao)
	glCtx.BindVertexArray(vao)

	const size = 32
	const glFramebufferSrgb = 0x8DB9
	newFBO := func(internal uint32) (uint32, uint32) {
		tex := glCtx.GenTextures(1)
		glCtx.BindTexture(gl.TEXTURE_2D, tex)
		glCtx.TexImage2D(gl.TEXTURE_2D, 0, int32(internal), size, size, 0, gl.RGBA, gl.UNSIGNED_BYTE, 0)
		glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, int32(gl.NEAREST))
		glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, int32(gl.NEAREST))
		fbo := glCtx.GenFramebuffers(1)
		glCtx.BindFramebuffer(gl.FRAMEBUFFER, fbo)
		glCtx.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, tex, 0)
		if got := glCtx.CheckFramebufferStatus(gl.FRAMEBUFFER); got != gl.FRAMEBUFFER_COMPLETE {
			t.Fatalf("FBO incomplete: 0x%x", got)
		}
		return tex, fbo
	}
	srgbTex, srgbFBO := newFBO(gl.SRGB8_ALPHA8)
	defer glCtx.DeleteTextures(srgbTex)
	defer glCtx.DeleteFramebuffers(srgbFBO)
	unormTex, unormFBO := newFBO(gl.RGBA8)
	defer glCtx.DeleteTextures(unormTex)
	defer glCtx.DeleteFramebuffers(unormFBO)

	drawBase := func(fbo uint32, srgb bool) {
		glCtx.BindFramebuffer(gl.FRAMEBUFFER, fbo)
		glCtx.Viewport(0, 0, size, size)
		if srgb {
			glCtx.Enable(glFramebufferSrgb)
		} else {
			glCtx.Disable(glFramebufferSrgb)
		}
		glCtx.ClearColor(0, 0, 0, 1)
		glCtx.Clear(gl.COLOR_BUFFER_BIT)
		glCtx.Disable(gl.BLEND)
		glCtx.UseProgram(prog)
		glCtx.DrawArrays(gl.TRIANGLES, 0, 3)
	}
	readCenter := func(fbo uint32) [4]byte {
		glCtx.BindFramebuffer(gl.READ_FRAMEBUFFER, fbo)
		buf := make([]byte, size*size*4)
		glCtx.ReadPixels(0, 0, size, size, gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&buf[0]))
		var out [4]byte
		copy(out[:], buf[(size/2*size+size/2)*4:])
		return out
	}
	blit := func(src, dst uint32, srgbState bool) {
		if srgbState {
			glCtx.Enable(glFramebufferSrgb)
		} else {
			glCtx.Disable(glFramebufferSrgb)
		}
		glCtx.BindFramebuffer(gl.READ_FRAMEBUFFER, src)
		glCtx.BindFramebuffer(gl.DRAW_FRAMEBUFFER, dst)
		glCtx.BlitFramebuffer(0, 0, size, size, 0, 0, size, size, gl.COLOR_BUFFER_BIT, gl.NEAREST)
		if err := glCtx.GetError(); err != gl.NO_ERROR {
			t.Fatalf("BlitFramebuffer GL error 0x%x (srgbState=%v)", err, srgbState)
		}
	}

	// Base images: sRGB FBO = encoded 188/137/89; Unorm FBO = raw 128/64/26.
	drawBase(srgbFBO, true)
	drawBase(unormFBO, false)
	srgbBase := readCenter(srgbFBO)
	unormBase := readCenter(unormFBO)
	t.Logf("P3.1b-0 GLES blit base: sRGB FBO=%v Unorm FBO=%v", srgbBase, unormBase)

	// sRGB -> Unorm: enabled should decode to linear (128/64/26); disabled raw (188/137/89).
	blit(srgbFBO, unormFBO, true)
	decoded := readCenter(unormFBO)
	blit(srgbFBO, unormFBO, false)
	raw := readCenter(unormFBO)
	// Unorm -> sRGB: enabled should encode (188/137/89); disabled raw (128/64/26).
	drawBase(unormFBO, false)
	blit(unormFBO, srgbFBO, true)
	encoded := readCenter(srgbFBO)
	drawBase(unormFBO, false)
	blit(unormFBO, srgbFBO, false)
	rawToSrgb := readCenter(srgbFBO)
	t.Logf("P3.1b-0 GLES blit sRGB→Unorm enabled=%v disabled=%v; Unorm→sRGB enabled=%v disabled=%v",
		decoded, raw, encoded, rawToSrgb)

	wantDecoded := [3]int{127, 64, 25}
	wantRaw := [3]int{188, 137, 89}
	wantEncoded := [3]int{188, 137, 89}
	wantRawToSrgb := [3]int{127, 64, 25}
	check := func(label string, got [4]byte, want [3]int) {
		for i := 0; i < 3; i++ {
			if d := int(got[i]) - want[i]; d < -2 || d > 2 {
				t.Fatalf("%s channel %d = %d, want %d±2; bytes=%v", label, i, got[i], want[i], got)
			}
		}
	}
	check("sRGB→Unorm enabled", decoded, wantDecoded)
	check("sRGB→Unorm disabled", raw, wantRaw)
	check("Unorm→sRGB enabled", encoded, wantEncoded)
	check("Unorm→sRGB disabled", rawToSrgb, wantRawToSrgb)
}
