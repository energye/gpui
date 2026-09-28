// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

//go:build linux && !(js && wasm)

package gles

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"unsafe"

	"github.com/energye/gpui/gpu/gwgpu/gles/egl"
	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
	"github.com/energye/gpui/gpu/gwgpu/naga/glsl"
)

// H4-c 离屏像素证：三角经 WGSL→GLSL 缓存路径编译，真建 GL 管线，
// 离屏 FBO 读回。画面语义对 P1 三角指纹（红三角蓝底：中心红、四角蓝），
// 全像素 md5 对 testdata 金文件的指纹，容差为 0（逐位一致，差一像素就停）。
//
// 说明：只认缓存编译出的 GLSL，不手写 GLSL；Y 翻转标记
// （WriterFlagAdjustCoordinateSpace） baked 在着色器里，但中心/四角探针
// 与翻转无关；无 EGL 时 t.Skipf（缺真机数据不假绿）。

type h4cTriangleGolden struct {
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Tolerance int    `json:"tolerance"`
	Center    []int  `json:"center_rgba"`
	Corner    []int  `json:"corner_rgba"`
	PixelsMD5 string `json:"pixels_md5"`
}

func h4cCompileGL(t *testing.T, glCtx *gl.Context, shaderType uint32, src string) uint32 {
	t.Helper()
	id := glCtx.CreateShader(shaderType)
	if id == 0 {
		t.Fatal("CreateShader returned 0")
	}
	glCtx.ShaderSource(id, src)
	glCtx.CompileShader(id)
	var status int32
	glCtx.GetShaderiv(id, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		log := glCtx.GetShaderInfoLog(id)
		glCtx.DeleteShader(id)
		t.Fatalf("shader compile failed: %s", log)
	}
	return id
}

func TestH4C_TriangleOffscreenPixel(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// WGSL 与期望探针一律读数据文件。
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

	// 两次编译走缓存：第二次必须命中且输出逐字节一致。
	vsGLSL1, _, err := compileWGSLToGLSL(glsl.Version330, vsWGSL, "vs_main", nil)
	if err != nil {
		t.Fatalf("vertex WGSL→GLSL: %v", err)
	}
	h0, _, _ := shaderCacheStats()
	vsGLSL2, _, err := compileWGSLToGLSL(glsl.Version330, vsWGSL, "vs_main", nil)
	if err != nil {
		t.Fatalf("vertex WGSL→GLSL (cached): %v", err)
	}
	if vsGLSL2 != vsGLSL1 {
		t.Fatal("cached vertex GLSL differs from first translation")
	}
	h1, _, _ := shaderCacheStats()
	if h1-h0 != 1 {
		t.Fatalf("vertex cache hits delta = %d, want 1", h1-h0)
	}
	fsGLSL, _, err := compileWGSLToGLSL(glsl.Version330, fsWGSL, "fs_main", nil)
	if err != nil {
		t.Fatalf("fragment WGSL→GLSL: %v", err)
	}

	// 真机 EGL 上下文（桌面 GL）；无 EGL 时跳过并写明原因。
	if err := egl.Init(); err != nil {
		t.Skipf("egl.Init unavailable: %v", err)
	}
	config := egl.DefaultContextConfig()
	config.GLES = false
	eglCtx, err := egl.NewContext(config)
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

	// Core Profile 下 DrawArrays 要求有 VAO。
	vao := glCtx.GenVertexArrays(1)
	if vao == 0 {
		t.Fatal("GenVertexArrays returned 0")
	}
	defer glCtx.DeleteVertexArrays(vao)
	glCtx.BindVertexArray(vao)

	// 真建管线：编译 + 链接（与 Device.CreateRenderPipeline 同一路调用）。
	vsID := h4cCompileGL(t, glCtx, gl.VERTEX_SHADER, vsGLSL1)
	defer glCtx.DeleteShader(vsID)
	fsID := h4cCompileGL(t, glCtx, gl.FRAGMENT_SHADER, fsGLSL)
	defer glCtx.DeleteShader(fsID)
	prog := glCtx.CreateProgram()
	if prog == 0 {
		t.Fatal("CreateProgram returned 0")
	}
	defer glCtx.DeleteProgram(prog)
	glCtx.AttachShader(prog, vsID)
	glCtx.AttachShader(prog, fsID)
	glCtx.LinkProgram(prog)
	var linked int32
	glCtx.GetProgramiv(prog, gl.LINK_STATUS, &linked)
	if linked == gl.FALSE {
		t.Fatalf("program link failed: %s", glCtx.GetProgramInfoLog(prog))
	}

	// 离屏 FBO（单采样 RGBA8 纹理，无 MSAA，不确定因素为零）。
	tex := glCtx.GenTextures(1)
	if tex == 0 {
		t.Fatal("GenTextures returned 0")
	}
	defer glCtx.DeleteTextures(tex)
	glCtx.BindTexture(gl.TEXTURE_2D, tex)
	glCtx.TexImage2D(gl.TEXTURE_2D, 0, int32(gl.RGBA8), int32(w), int32(h), 0, gl.RGBA, gl.UNSIGNED_BYTE, 0)
	glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	fbo := glCtx.GenFramebuffers(1)
	if fbo == 0 {
		t.Fatal("GenFramebuffers returned 0")
	}
	defer glCtx.DeleteFramebuffers(fbo)
	glCtx.BindFramebuffer(gl.FRAMEBUFFER, fbo)
	glCtx.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, tex, 0)
	if st := glCtx.CheckFramebufferStatus(gl.FRAMEBUFFER); st != gl.FRAMEBUFFER_COMPLETE {
		t.Fatalf("framebuffer status = %#x, want COMPLETE", st)
	}

	glCtx.Viewport(0, 0, int32(w), int32(h))
	glCtx.ClearColor(0, 0, 1, 1)
	glCtx.Clear(gl.COLOR_BUFFER_BIT)
	glCtx.UseProgram(prog)
	glCtx.DrawArrays(gl.TRIANGLES, 0, 3)
	glCtx.Finish()

	pixels := make([]byte, w*h*4)
	glCtx.ReadPixels(0, 0, int32(w), int32(h), gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&pixels[0]))
	glCtx.BindFramebuffer(gl.FRAMEBUFFER, 0)

	// 探针：中心红、四角蓝（GL 原点在左下，行按 bottom-up 取）。
	at := func(x, y int) []int {
		o := (y*w + x) * 4
		return []int{int(pixels[o]), int(pixels[o+1]), int(pixels[o+2]), int(pixels[o+3])}
	}
	eq := func(a, b []int) bool {
		return len(a) == 4 && len(b) == 4 && a[0] == b[0] && a[1] == b[1] && a[2] == b[2] && a[3] == b[3]
	}
	if got := at(w/2, h/2); !eq(got, golden.Center) {
		t.Errorf("center = %v, want %v", got, golden.Center)
	}
	for _, p := range [][2]int{{0, 0}, {w - 1, 0}, {0, h - 1}, {w - 1, h - 1}} {
		if got := at(p[0], p[1]); !eq(got, golden.Corner) {
			t.Errorf("corner %v = %v, want %v", p, got, golden.Corner)
		}
	}

	// 逐位回归：全像素 md5 对金文件指纹。
	sum := md5.Sum(pixels)
	gotMD5 := hex.EncodeToString(sum[:])
	t.Logf("offscreen pixels md5: %s", gotMD5)
	if gotMD5 != golden.PixelsMD5 {
		t.Errorf("pixels md5 = %s, want %s (golden testdata/h4c_triangle_golden.json)", gotMD5, golden.PixelsMD5)
	}
}
