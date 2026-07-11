// Package gl provides OpenGL bindings via purego (cross-platform, no CGo).
//
// Call gl.Init() once with the OpenGL context current to load all function
// pointers. After Init, all functions are safe to call concurrently.
package gl

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/ebitengine/purego"
)

// OpenGL constants
const (
	GL_FALSE = 0
	GL_TRUE  = 1

	GL_ZERO                = 0
	GL_ONE                 = 1
	GL_SRC_ALPHA           = 0x0302
	GL_ONE_MINUS_SRC_ALPHA = 0x0303

	GL_BLEND        = 0x0BE2
	GL_SCISSOR_TEST = 0x0C11
	GL_DEPTH_TEST   = 0x0B71
	GL_CULL_FACE    = 0x0B44
	GL_STENCIL_TEST = 0x0B90

	GL_ALWAYS    = 0x0207
	GL_EQUAL     = 0x0202
	GL_NOTEQUAL  = 0x0205
	GL_KEEP      = 0x1E00
	GL_INVERT    = 0x150A
	GL_INCR_WRAP = 0x8507
	GL_DECR_WRAP = 0x8508

	GL_TEXTURE_2D           = 0x0DE1
	GL_RGBA                 = 0x1908
	GL_UNSIGNED_BYTE        = 0x1401
	GL_LINEAR               = 0x2601
	GL_NEAREST              = 0x2600
	GL_LINEAR_MIPMAP_LINEAR = 0x2703
	GL_CLAMP_TO_EDGE        = 0x812F
	GL_REPEAT               = 0x2901
	GL_TEXTURE_MIN_FILTER   = 0x2801
	GL_TEXTURE_MAG_FILTER   = 0x2800
	GL_TEXTURE_WRAP_S       = 0x2802
	GL_TEXTURE_WRAP_T       = 0x2803

	GL_ARRAY_BUFFER         = 0x8892
	GL_ELEMENT_ARRAY_BUFFER = 0x8893
	GL_STATIC_DRAW          = 0x88E4
	GL_DYNAMIC_DRAW         = 0x88E8
	GL_STREAM_DRAW          = 0x88E0
	GL_FLOAT                = 0x1406
	GL_UNSIGNED_INT         = 0x1405
	GL_TRIANGLES            = 0x0004
	GL_TRIANGLE_STRIP       = 0x0005
	GL_TRIANGLE_FAN         = 0x0006

	GL_VERTEX_SHADER   = 0x8B31
	GL_FRAGMENT_SHADER = 0x8B30
	GL_COMPILE_STATUS  = 0x8B81
	GL_LINK_STATUS     = 0x8B82
	GL_INFO_LOG_LENGTH = 0x8B84

	GL_COLOR_BUFFER_BIT   = 0x00004000
	GL_DEPTH_BUFFER_BIT   = 0x00000100
	GL_STENCIL_BUFFER_BIT = 0x00000400

	GL_TEXTURE0 = 0x84C0
	GL_TEXTURE1 = 0x84C1
	GL_TEXTURE2 = 0x84C2
	GL_TEXTURE3 = 0x84C3

	GL_UNPACK_ROW_LENGTH = 0x0CF2
	GL_UNPACK_ALIGNMENT  = 0x0CF5

	// FBO constants
	GL_FRAMEBUFFER              = 0x8D40
	GL_READ_FRAMEBUFFER         = 0x8CA8
	GL_DRAW_FRAMEBUFFER         = 0x8CA9
	GL_COLOR_ATTACHMENT0        = 0x8CE0
	GL_DEPTH_ATTACHMENT         = 0x8D00
	GL_STENCIL_ATTACHMENT       = 0x8D20
	GL_DEPTH_STENCIL_ATTACHMENT = 0x821A
	GL_FRAMEBUFFER_COMPLETE     = 0x8CD5
	GL_RENDERBUFFER             = 0x8D41
	GL_DEPTH_COMPONENT24        = 0x81A6
	GL_DEPTH24_STENCIL8         = 0x88F0
)

const cglCPSwapInterval = 222

// GL function pointers — callable after Init().
var (
	// State management
	Viewport     func(x, y, width, height int32)
	ClearColor   func(r, g, b, a float32)
	Clear        func(mask uint32)
	Enable       func(cap uint32)
	Disable      func(cap uint32)
	BlendFunc    func(sfactor, dfactor uint32)
	Scissor      func(x, y, width, height int32)
	ColorMask    func(red, green, blue, alpha bool)
	ClearStencil func(s int32)
	StencilFunc  func(fn uint32, ref int32, mask uint32)
	StencilOp    func(sfail, dpfail, dppass uint32)
	GetError     func() uint32

	// Shader functions
	CreateShader      func(shaderType uint32) uint32
	ShaderSource      func(shader, count uint32, str *uintptr, length *int32)
	CompileShader     func(shader uint32)
	GetShaderiv       func(shader, pname uint32, params *int32)
	GetShaderInfoLog  func(shader uint32, bufSize int32, length *int32, infoLog *byte)
	CreateProgram     func() uint32
	AttachShader      func(program, shader uint32)
	LinkProgram       func(program uint32)
	GetProgramiv      func(program uint32, pname uint32, params *int32)
	GetProgramInfoLog func(program uint32, bufSize int32, length *int32, infoLog *byte)
	UseProgram        func(program uint32)
	DeleteShader      func(shader uint32)
	DeleteProgram     func(program uint32)

	// Uniform functions
	GetUniformLocation       func(program uint32, name *byte) int32
	Uniform1i                func(location int32, v0 int32)
	Uniform1f                func(location int32, v0 float32)
	Uniform2f                func(location int32, v0, v1 float32)
	Uniform4f                func(location int32, v0, v1, v2, v3 float32)
	UniformMatrix4fv         func(location int32, count int32, transpose bool, value *float32)
	GetAttribLocation        func(program uint32, name *byte) int32
	BindAttribLocation       func(program uint32, index uint32, name *byte)
	EnableVertexAttribArray  func(index uint32)
	DisableVertexAttribArray func(index uint32)
	VertexAttribPointer      func(index uint32, size int32, xtype uint32, normalized bool, stride int32, pointer uintptr)

	// Buffer functions
	GenBuffers    func(n int32, buffers *uint32)
	BindBuffer    func(target uint32, buffer uint32)
	BufferData    func(target uint32, size int32, data uintptr, usage uint32)
	BufferSubData func(target uint32, offset int32, size int32, data uintptr)
	DeleteBuffers func(n int32, buffers *uint32)

	// VAO functions
	GenVertexArrays    func(n int32, arrays *uint32)
	BindVertexArray    func(array uint32)
	DeleteVertexArrays func(n int32, arrays *uint32)

	// Draw functions
	DrawArrays   func(mode uint32, first int32, count int32)
	DrawElements func(mode uint32, count int32, xtype uint32, indices uintptr)
	ReadPixels   func(x, y, width, height int32, format uint32, xtype uint32, pixels uintptr)

	// Texture functions
	GenTextures    func(n int32, textures *uint32)
	DeleteTextures func(n int32, textures *uint32)
	BindTexture    func(target uint32, texture uint32)
	TexImage2D     func(target uint32, level int32, internalformat int32, width int32, height int32, border int32, format uint32, xtype uint32, pixels uintptr)
	TexSubImage2D  func(target uint32, level int32, xoffset int32, yoffset int32, width int32, height int32, format uint32, xtype uint32, pixels uintptr)
	TexParameteri  func(target uint32, pname uint32, param int32)
	ActiveTexture  func(texture uint32)

	// FBO functions (GL 3.0+)
	GenFramebuffers         func(n int32, framebuffers *uint32)
	DeleteFramebuffers      func(n int32, framebuffers *uint32)
	BindFramebuffer         func(target uint32, framebuffer uint32)
	FramebufferTexture2D    func(target uint32, attachment uint32, textarget uint32, texture uint32, level int32)
	CheckFramebufferStatus  func(target uint32) uint32
	GenRenderbuffers        func(n int32, renderbuffers *uint32)
	DeleteRenderbuffers     func(n int32, renderbuffers *uint32)
	BindRenderbuffer        func(target uint32, renderbuffer uint32)
	RenderbufferStorage     func(target uint32, internalformat uint32, width int32, height int32)
	FramebufferRenderbuffer func(target uint32, attachment uint32, renderbuffertarget uint32, renderbuffer uint32)
)

var (
	glXGetProcAddressARB  func(name *byte) uintptr
	glXGetProcAddress     func(name *byte) uintptr
	glXGetCurrentDisplay  func() uintptr
	glXGetCurrentDrawable func() uintptr
	glXSwapIntervalEXT    func(display uintptr, drawable uintptr, interval int32)
	glXSwapIntervalMESA   func(interval uint32) int32
	glXSwapIntervalSGI    func(interval int32) int32

	wglGetProcAddress  func(name *byte) uintptr
	wglSwapIntervalEXT func(interval int32) int32

	cglGetCurrentContext func() uintptr
	cglSetParameter      func(ctx uintptr, pname int32, params *int32) int32
)

var (
	initMu      sync.Mutex
	initialized bool
)

// Init loads OpenGL function pointers using purego.
// Must be called once with the OpenGL context current.
func Init() error {
	initMu.Lock()
	defer initMu.Unlock()
	if initialized {
		return nil
	}

	lib, err := openGLLibrary()
	if err != nil {
		return fmt.Errorf("gl: %v", err)
	}

	initOK := false
	defer func() {
		if !initOK {
			resetGLFuncs()
		}
	}()

	bind := func(fn any, name string) {
		purego.RegisterLibFunc(fn, lib, name)
	}

	// Core OpenGL 1.x functions
	bind(&Viewport, "glViewport")
	bind(&ClearColor, "glClearColor")
	bind(&Clear, "glClear")
	bind(&Enable, "glEnable")
	bind(&Disable, "glDisable")
	bind(&BlendFunc, "glBlendFunc")
	bind(&Scissor, "glScissor")
	bind(&ColorMask, "glColorMask")
	bind(&ClearStencil, "glClearStencil")
	bind(&StencilFunc, "glStencilFunc")
	bind(&StencilOp, "glStencilOp")
	bind(&GetError, "glGetError")

	// Shader functions (GL 2.0+)
	bind(&CreateShader, "glCreateShader")
	bind(&ShaderSource, "glShaderSource")
	bind(&CompileShader, "glCompileShader")
	bind(&GetShaderiv, "glGetShaderiv")
	bind(&GetShaderInfoLog, "glGetShaderInfoLog")
	bind(&CreateProgram, "glCreateProgram")
	bind(&AttachShader, "glAttachShader")
	bind(&LinkProgram, "glLinkProgram")
	bind(&GetProgramiv, "glGetProgramiv")
	bind(&GetProgramInfoLog, "glGetProgramInfoLog")
	bind(&UseProgram, "glUseProgram")
	bind(&DeleteShader, "glDeleteShader")
	bind(&DeleteProgram, "glDeleteProgram")

	// Uniform functions
	bind(&GetUniformLocation, "glGetUniformLocation")
	bind(&Uniform1i, "glUniform1i")
	bind(&Uniform1f, "glUniform1f")
	bind(&Uniform2f, "glUniform2f")
	bind(&Uniform4f, "glUniform4f")
	bind(&UniformMatrix4fv, "glUniformMatrix4fv")
	bind(&GetAttribLocation, "glGetAttribLocation")
	bind(&BindAttribLocation, "glBindAttribLocation")
	bind(&EnableVertexAttribArray, "glEnableVertexAttribArray")
	bind(&DisableVertexAttribArray, "glDisableVertexAttribArray")
	bind(&VertexAttribPointer, "glVertexAttribPointer")

	// Buffer functions (GL 1.5+)
	if fn := getGLFunc(lib, "glGenBuffers"); fn != 0 {
		bind(&GenBuffers, "glGenBuffers")
		bind(&BindBuffer, "glBindBuffer")
		bind(&BufferData, "glBufferData")
		bind(&BufferSubData, "glBufferSubData")
		bind(&DeleteBuffers, "glDeleteBuffers")
	} else {
		return fmt.Errorf("GL 1.5+ not supported (glGenBuffers missing)")
	}

	// VAO (GL 3.0+ core or extension)
	if fn := getGLFunc(lib, "glGenVertexArrays"); fn != 0 {
		bind(&GenVertexArrays, "glGenVertexArrays")
		bind(&BindVertexArray, "glBindVertexArray")
		bind(&DeleteVertexArrays, "glDeleteVertexArrays")
	} else if fn := getGLFunc(lib, "glGenVertexArraysAPPLE"); fn != 0 {
		purego.RegisterLibFunc(&GenVertexArrays, lib, "glGenVertexArraysAPPLE")
		purego.RegisterLibFunc(&BindVertexArray, lib, "glBindVertexArrayAPPLE")
		purego.RegisterLibFunc(&DeleteVertexArrays, lib, "glDeleteVertexArraysAPPLE")
	} else {
		return fmt.Errorf("VAO not supported (needs GL 3.0+ or ARB_vertex_array_object)")
	}

	// Draw functions
	bind(&DrawArrays, "glDrawArrays")
	bind(&DrawElements, "glDrawElements")
	bind(&ReadPixels, "glReadPixels")

	// Texture functions
	bind(&GenTextures, "glGenTextures")
	bind(&DeleteTextures, "glDeleteTextures")
	bind(&BindTexture, "glBindTexture")
	bind(&TexImage2D, "glTexImage2D")
	bind(&TexSubImage2D, "glTexSubImage2D")
	bind(&TexParameteri, "glTexParameteri")
	bind(&ActiveTexture, "glActiveTexture")

	// FBO functions (GL 3.0+)
	if fn := getGLFunc(lib, "glGenFramebuffers"); fn != 0 {
		bind(&GenFramebuffers, "glGenFramebuffers")
		bind(&DeleteFramebuffers, "glDeleteFramebuffers")
		bind(&BindFramebuffer, "glBindFramebuffer")
		bind(&FramebufferTexture2D, "glFramebufferTexture2D")
		bind(&CheckFramebufferStatus, "glCheckFramebufferStatus")
		bind(&GenRenderbuffers, "glGenRenderbuffers")
		bind(&DeleteRenderbuffers, "glDeleteRenderbuffers")
		bind(&BindRenderbuffer, "glBindRenderbuffer")
		bind(&RenderbufferStorage, "glRenderbufferStorage")
		bind(&FramebufferRenderbuffer, "glFramebufferRenderbuffer")
	}

	loadSwapIntervalFuncs(lib)

	initOK = true
	initialized = true
	return nil
}

// SetVSync enables or disables presentation pacing through the platform swap
// interval API. The OpenGL context must be current on the calling thread.
func SetVSync(enabled bool) error {
	if enabled {
		return SetSwapInterval(1)
	}
	return SetSwapInterval(0)
}

// SetSwapInterval sets the platform swap interval for the current OpenGL
// context. interval=1 enables VSync pacing; interval=0 disables it when the
// platform extension supports that operation.
func SetSwapInterval(interval int) error {
	if !initialized {
		return fmt.Errorf("gl: Init must be called before SetSwapInterval")
	}

	switch runtime.GOOS {
	case "linux":
		return setGLXSwapInterval(int32(interval))
	case "windows":
		return setWGLSwapInterval(int32(interval))
	case "darwin":
		return setCGLSwapInterval(int32(interval))
	default:
		return fmt.Errorf("gl: swap interval unsupported on %s", runtime.GOOS)
	}
}

// SwapIntervalSupported reports whether a platform swap interval binding was
// found during Init. It does not guarantee that a later SetSwapInterval call
// succeeds, because some APIs also require a current drawable/context.
func SwapIntervalSupported() bool {
	switch runtime.GOOS {
	case "linux":
		return glXSwapIntervalEXT != nil || glXSwapIntervalMESA != nil || glXSwapIntervalSGI != nil
	case "windows":
		return wglSwapIntervalEXT != nil
	case "darwin":
		return cglGetCurrentContext != nil && cglSetParameter != nil
	default:
		return false
	}
}

func resetGLFuncs() {
	Viewport = nil
	ClearColor = nil
	Clear = nil
	Enable = nil
	Disable = nil
	BlendFunc = nil
	Scissor = nil
	ColorMask = nil
	ClearStencil = nil
	StencilFunc = nil
	StencilOp = nil
	GetError = nil

	CreateShader = nil
	ShaderSource = nil
	CompileShader = nil
	GetShaderiv = nil
	GetShaderInfoLog = nil
	CreateProgram = nil
	AttachShader = nil
	LinkProgram = nil
	GetProgramiv = nil
	GetProgramInfoLog = nil
	UseProgram = nil
	DeleteShader = nil
	DeleteProgram = nil

	GetUniformLocation = nil
	Uniform1i = nil
	Uniform1f = nil
	Uniform2f = nil
	Uniform4f = nil
	UniformMatrix4fv = nil
	GetAttribLocation = nil
	BindAttribLocation = nil
	EnableVertexAttribArray = nil
	DisableVertexAttribArray = nil
	VertexAttribPointer = nil

	GenBuffers = nil
	BindBuffer = nil
	BufferData = nil
	BufferSubData = nil
	DeleteBuffers = nil

	GenVertexArrays = nil
	BindVertexArray = nil
	DeleteVertexArrays = nil

	DrawArrays = nil
	DrawElements = nil
	ReadPixels = nil

	GenTextures = nil
	DeleteTextures = nil
	BindTexture = nil
	TexImage2D = nil
	TexSubImage2D = nil
	TexParameteri = nil
	ActiveTexture = nil

	GenFramebuffers = nil
	DeleteFramebuffers = nil
	BindFramebuffer = nil
	FramebufferTexture2D = nil
	CheckFramebufferStatus = nil
	GenRenderbuffers = nil
	DeleteRenderbuffers = nil
	BindRenderbuffer = nil
	RenderbufferStorage = nil
	FramebufferRenderbuffer = nil

	glXGetProcAddressARB = nil
	glXGetProcAddress = nil
	glXGetCurrentDisplay = nil
	glXGetCurrentDrawable = nil
	glXSwapIntervalEXT = nil
	glXSwapIntervalMESA = nil
	glXSwapIntervalSGI = nil

	wglGetProcAddress = nil
	wglSwapIntervalEXT = nil

	cglGetCurrentContext = nil
	cglSetParameter = nil
}

func openGLLibrary() (uintptr, error) {
	flags := purego.RTLD_LAZY | purego.RTLD_GLOBAL
	switch runtime.GOOS {
	case "linux":
		lib, err := purego.Dlopen("libGL.so.1", flags)
		if err != nil {
			lib, err = purego.Dlopen("libGL.so", flags)
			if err != nil {
				return 0, fmt.Errorf("cannot open libGL.so: %w", err)
			}
		}
		return lib, nil
	case "darwin":
		lib, err := purego.Dlopen("/System/Library/Frameworks/OpenGL.framework/OpenGL", flags)
		if err != nil {
			return 0, fmt.Errorf("cannot open OpenGL.framework: %w", err)
		}
		return lib, nil
	case "windows":
		lib, err := purego.Dlopen("opengl32.dll", flags)
		if err != nil {
			return 0, fmt.Errorf("cannot open opengl32.dll: %w", err)
		}
		return lib, nil
	default:
		return 0, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func getGLFunc(lib uintptr, name string) uintptr {
	fn, err := purego.Dlsym(lib, name)
	if err != nil {
		return 0
	}
	return fn
}

func loadSwapIntervalFuncs(lib uintptr) {
	switch runtime.GOOS {
	case "linux":
		loadGLXSwapIntervalFuncs(lib)
	case "windows":
		loadWGLSwapIntervalFuncs(lib)
	case "darwin":
		loadCGLSwapIntervalFuncs(lib)
	}
}

func loadGLXSwapIntervalFuncs(lib uintptr) {
	if fn := getGLFunc(lib, "glXGetProcAddressARB"); fn != 0 {
		purego.RegisterFunc(&glXGetProcAddressARB, fn)
	}
	if fn := getGLFunc(lib, "glXGetProcAddress"); fn != 0 {
		purego.RegisterFunc(&glXGetProcAddress, fn)
	}
	if fn := getGLFunc(lib, "glXGetCurrentDisplay"); fn != 0 {
		purego.RegisterFunc(&glXGetCurrentDisplay, fn)
	}
	if fn := getGLFunc(lib, "glXGetCurrentDrawable"); fn != 0 {
		purego.RegisterFunc(&glXGetCurrentDrawable, fn)
	}

	if fn := getGLXProcAddress(lib, "glXSwapIntervalEXT"); validProcAddress(fn) {
		purego.RegisterFunc(&glXSwapIntervalEXT, fn)
	}
	if fn := getGLXProcAddress(lib, "glXSwapIntervalMESA"); validProcAddress(fn) {
		purego.RegisterFunc(&glXSwapIntervalMESA, fn)
	}
	if fn := getGLXProcAddress(lib, "glXSwapIntervalSGI"); validProcAddress(fn) {
		purego.RegisterFunc(&glXSwapIntervalSGI, fn)
	}
}

func getGLXProcAddress(lib uintptr, name string) uintptr {
	if glXGetProcAddressARB != nil {
		if fn := glXGetProcAddressARB(cString(name)); validProcAddress(fn) {
			return fn
		}
	}
	if glXGetProcAddress != nil {
		if fn := glXGetProcAddress(cString(name)); validProcAddress(fn) {
			return fn
		}
	}
	return getGLFunc(lib, name)
}

func setGLXSwapInterval(interval int32) error {
	if glXSwapIntervalEXT != nil {
		if glXGetCurrentDisplay == nil || glXGetCurrentDrawable == nil {
			return fmt.Errorf("gl: GLX current display/drawable bindings unavailable")
		}
		display := glXGetCurrentDisplay()
		drawable := glXGetCurrentDrawable()
		if display == 0 || drawable == 0 {
			return fmt.Errorf("gl: GLX context or drawable is not current")
		}
		glXSwapIntervalEXT(display, drawable, interval)
		return nil
	}
	if glXSwapIntervalMESA != nil {
		if interval < 0 {
			return fmt.Errorf("gl: glXSwapIntervalMESA does not support negative intervals")
		}
		if status := glXSwapIntervalMESA(uint32(interval)); status != 0 {
			return fmt.Errorf("gl: glXSwapIntervalMESA failed with status %d", status)
		}
		return nil
	}
	if glXSwapIntervalSGI != nil {
		if interval <= 0 {
			return fmt.Errorf("gl: glXSwapIntervalSGI only supports positive intervals")
		}
		if status := glXSwapIntervalSGI(interval); status != 0 {
			return fmt.Errorf("gl: glXSwapIntervalSGI failed with status %d", status)
		}
		return nil
	}
	return fmt.Errorf("gl: GLX swap interval extension unavailable")
}

func loadWGLSwapIntervalFuncs(lib uintptr) {
	if fn := getGLFunc(lib, "wglGetProcAddress"); fn != 0 {
		purego.RegisterFunc(&wglGetProcAddress, fn)
	}
	if wglGetProcAddress == nil {
		return
	}
	if fn := wglGetProcAddress(cString("wglSwapIntervalEXT")); validProcAddress(fn) {
		purego.RegisterFunc(&wglSwapIntervalEXT, fn)
	}
}

func setWGLSwapInterval(interval int32) error {
	if wglSwapIntervalEXT == nil {
		return fmt.Errorf("gl: WGL_EXT_swap_control unavailable")
	}
	if status := wglSwapIntervalEXT(interval); status == 0 {
		return fmt.Errorf("gl: wglSwapIntervalEXT failed")
	}
	return nil
}

func loadCGLSwapIntervalFuncs(lib uintptr) {
	if fn := getGLFunc(lib, "CGLGetCurrentContext"); fn != 0 {
		purego.RegisterFunc(&cglGetCurrentContext, fn)
	}
	if fn := getGLFunc(lib, "CGLSetParameter"); fn != 0 {
		purego.RegisterFunc(&cglSetParameter, fn)
	}
}

func setCGLSwapInterval(interval int32) error {
	if cglGetCurrentContext == nil || cglSetParameter == nil {
		return fmt.Errorf("gl: CGL swap interval bindings unavailable")
	}
	ctx := cglGetCurrentContext()
	if ctx == 0 {
		return fmt.Errorf("gl: CGL context is not current")
	}
	if status := cglSetParameter(ctx, cglCPSwapInterval, &interval); status != 0 {
		return fmt.Errorf("gl: CGLSetParameter(kCGLCPSwapInterval) failed with status %d", status)
	}
	return nil
}

func validProcAddress(fn uintptr) bool {
	return fn != 0 && fn != 1 && fn != 2 && fn != 3 && fn != ^uintptr(0)
}

func cString(s string) *byte {
	b := append([]byte(s), 0)
	return &b[0]
}
