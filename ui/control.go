//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License 2.0
//
//----------------------------------------

package ui

import (
	"unsafe"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/gl"
	"github.com/energye/lcl/lcl"
)

// TGPUControl wraps an LCL OpenGL control with GPU-accelerated 2D rendering.
type TGPUControl struct {
	ctrl      lcl.IOpenGLControl
	ggCtx     *render.Context // gg rendering context
	texture   uint32
	vao       uint32
	vbo       uint32
	program   uint32
	width     int32
	height    int32
	onRender  func(*render.Context) // user render callback (receives render.Context)
	initDone  bool
	animating bool // continuous render loop flag (OnDraw style)
}

// NewGPUControl creates a new TGPUControl.
func NewGPUControl() *TGPUControl {
	return &TGPUControl{}
}

// Attach binds this TGPUControl to an LCL OpenGL control.
func (c *TGPUControl) Attach(ctrl lcl.IOpenGLControl) {
	c.ctrl = ctrl
	ctrl.SetOnPaint(c.onPaint)
}

// GLControl returns the underlying LCL OpenGL control.
func (c *TGPUControl) GLControl() lcl.IOpenGLControl { return c.ctrl }

// SetOnRender sets the render callback. Called each frame with the gg context.
func (c *TGPUControl) SetOnRender(fn func(*render.Context)) { c.onRender = fn }

// Width returns the control width in pixels.
func (c *TGPUControl) Width() int32 { return c.width }

// Height returns the control height in pixels.
func (c *TGPUControl) Height() int32 { return c.height }

// MakeCurrent activates the OpenGL context.
func (c *TGPUControl) MakeCurrent() bool {
	return c.ctrl != nil && c.ctrl.MakeCurrent(true)
}

// ReleaseContext deactivates the OpenGL context.
func (c *TGPUControl) ReleaseContext() {
	if c.ctrl != nil {
		c.ctrl.ReleaseContext()
	}
}

// SwapBuffers swaps the OpenGL buffers.
func (c *TGPUControl) SwapBuffers() {
	if c.ctrl != nil {
		c.ctrl.SwapBuffers()
	}
}

// Invalidate requests a repaint.
func (c *TGPUControl) Invalidate() {
	if c.ctrl != nil {
		c.ctrl.Invalidate()
	}
}

// InitGL initializes OpenGL function pointers and resources.
// Must be called with the OpenGL context current.
func (c *TGPUControl) InitGL() error {
	if err := gl.Init(); err != nil {
		return err
	}

	// Generate display texture
	gl.GenTextures(1, &c.texture)
	gl.BindTexture(gl.GL_TEXTURE_2D, c.texture)
	gl.TexParameteri(gl.GL_TEXTURE_2D, gl.GL_TEXTURE_MIN_FILTER, gl.GL_LINEAR)
	gl.TexParameteri(gl.GL_TEXTURE_2D, gl.GL_TEXTURE_MAG_FILTER, gl.GL_LINEAR)
	gl.TexParameteri(gl.GL_TEXTURE_2D, gl.GL_TEXTURE_WRAP_S, gl.GL_CLAMP_TO_EDGE)
	gl.TexParameteri(gl.GL_TEXTURE_2D, gl.GL_TEXTURE_WRAP_T, gl.GL_CLAMP_TO_EDGE)

	c.initQuad()
	c.initShader()
	return nil
}

// onPaint handles the LCL OnPaint event.
func (c *TGPUControl) onPaint(sender lcl.IObject) {
	if !c.MakeCurrent() {
		return
	}
	defer c.ReleaseContext()

	if !c.initDone {
		if err := c.InitGL(); err != nil {
			return
		}
		c.initDone = true
	}

	if c.ctrl != nil {
		c.width = c.ctrl.ClientWidth()
		c.height = c.ctrl.ClientHeight()
	}
	if c.width <= 0 || c.height <= 0 {
		return
	}

	// Create or recreate gg context if size changed
	w := int(c.width)
	h := int(c.height)
	if c.ggCtx == nil || c.ggCtx.Width() != w || c.ggCtx.Height() != h {
		if c.ggCtx != nil {
			c.ggCtx.Close()
		}
		c.ggCtx = render.NewContext(w, h)
	}

	// Clear the gg context
	c.ggCtx.Clear()

	// Call user render callback
	if c.onRender != nil {
		c.onRender(c.ggCtx)
	}

	// Get pixmap data and upload to OpenGL texture
	pixmap := c.ggCtx.Pixmap()
	data := pixmap.Data()
	c.UploadPixmap(data, w, h)

	// Display
	c.doPresent()

	// Continuous render loop (OnDraw style): schedule next frame when animation is active
	// This ties rendering to the display refresh rate via SwapBuffers vsync,
	// avoiding timer precision issues from lcl.TTimer.
	if c.animating {
		c.Invalidate()
	}
}

// SetSize updates the control's size.
func (c *TGPUControl) SetSize(w, h int32) {
	c.width = w
	c.height = h
}

// doPresent renders the current texture to the screen.
func (c *TGPUControl) doPresent() {
	gl.Viewport(0, 0, c.width, c.height)
	gl.Clear(gl.GL_COLOR_BUFFER_BIT)

	gl.UseProgram(c.program)
	gl.ActiveTexture(gl.GL_TEXTURE0)
	gl.BindTexture(gl.GL_TEXTURE_2D, c.texture)
	loc := gl.GetUniformLocation(c.program, strPtr("uTexture"))
	gl.Uniform1i(loc, 0)

	gl.BindVertexArray(c.vao)
	gl.DrawArrays(gl.GL_TRIANGLES, 0, 6)
	gl.BindVertexArray(0)
	gl.UseProgram(0)

	c.SwapBuffers()
}

// UploadPixmap uploads RGBA pixel data to the display texture.
func (c *TGPUControl) UploadPixmap(data []byte, width, height int) {
	if width <= 0 || height <= 0 || len(data) == 0 {
		return
	}
	gl.BindTexture(gl.GL_TEXTURE_2D, c.texture)
	gl.TexImage2D(gl.GL_TEXTURE_2D, 0, gl.GL_RGBA, int32(width), int32(height),
		0, gl.GL_RGBA, gl.GL_UNSIGNED_BYTE, uintptr(unsafe.Pointer(&data[0])))
}

// PresentPixmap uploads RGBA data and displays it.
func (c *TGPUControl) PresentPixmap(data []byte, width, height int) {
	c.UploadPixmap(data, width, height)
	c.doPresent()
}

// SavePNG saves the current gg context content to a PNG file.
func (c *TGPUControl) SavePNG(path string) error {
	if c.ggCtx == nil {
		return nil
	}
	return c.ggCtx.SavePNG(path)
}

func (c *TGPUControl) initQuad() {
	vertices := []float32{
		-1, -1, 0, 1,
		1, -1, 1, 1,
		-1, 1, 0, 0,
		-1, 1, 0, 0,
		1, -1, 1, 1,
		1, 1, 1, 0,
	}
	gl.GenVertexArrays(1, &c.vao)
	gl.GenBuffers(1, &c.vbo)
	gl.BindVertexArray(c.vao)
	gl.BindBuffer(gl.GL_ARRAY_BUFFER, c.vbo)
	gl.BufferData(gl.GL_ARRAY_BUFFER, int32(len(vertices)*4), uintptr(unsafe.Pointer(&vertices[0])), gl.GL_STATIC_DRAW)
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointer(0, 2, gl.GL_FLOAT, false, 16, 0)
	gl.EnableVertexAttribArray(1)
	gl.VertexAttribPointer(1, 2, gl.GL_FLOAT, false, 16, 8)
	gl.BindVertexArray(0)
	gl.BindBuffer(gl.GL_ARRAY_BUFFER, 0)
}

func (c *TGPUControl) initShader() {
	vsSrc := "#version 330 core\nlayout(location=0)in vec2 aPos;layout(location=1)in vec2 aUV;out vec2 vUV;void main(){gl_Position=vec4(aPos,0,1);vUV=aUV;}\x00"
	fsSrc := "#version 330 core\nin vec2 vUV;out vec4 fC;uniform sampler2D uTexture;void main(){fC=texture(uTexture,vUV);}\x00"

	vs := gl.CreateShader(gl.GL_VERTEX_SHADER)
	vsPtr := uintptr(unsafe.Pointer(strPtr(vsSrc)))
	gl.ShaderSource(vs, 1, &vsPtr, nil)
	fs := gl.CreateShader(gl.GL_FRAGMENT_SHADER)
	fsPtr := uintptr(unsafe.Pointer(strPtr(fsSrc)))
	gl.ShaderSource(fs, 1, &fsPtr, nil)

	gl.CompileShader(vs)
	gl.CompileShader(fs)

	c.program = gl.CreateProgram()
	gl.AttachShader(c.program, vs)
	gl.AttachShader(c.program, fs)
	gl.LinkProgram(c.program)
	gl.DeleteShader(vs)
	gl.DeleteShader(fs)
}

// StartAnimation begins continuous rendering (OnDraw style).
// Each frame triggers Invalidate() at the end of onPaint, creating a
// render loop tied to the display refresh rate via SwapBuffers vsync.
// This avoids the timer precision issues of lcl.TTimer.
func (c *TGPUControl) StartAnimation() {
	if c == nil || c.ctrl == nil {
		return
	}
	c.animating = true
	c.Invalidate() // start the render loop
}

// StopAnimation stops continuous rendering.
func (c *TGPUControl) StopAnimation() {
	if c == nil {
		return
	}
	c.animating = false
}

// strPtr returns a *byte pointer to a null-terminated string.
func strPtr(s string) *byte {
	return &[]byte(s)[0]
}
