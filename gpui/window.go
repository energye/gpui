//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License 2.0
//
//----------------------------------------

package gpui

import (
	"github.com/energye/lcl/lcl"
	"github.com/energye/lcl/types"
)

// WindowConfig configures a Window.
type WindowConfig struct {
	Title  string
	Width  int32
	Height int32
}

// Window wraps a form with an OpenGL control and a TGPUControl.
// It follows the standard LCL pattern: TEngForm + OpenGL control.
type Window struct {
	config  WindowConfig
	form    lcl.IEngForm
	control *TGPUControl
	onInit  func(*TGPUControl) // called after GL initialization
}

// NewWindow creates a new Window with the given configuration.
func NewWindow(config WindowConfig) *Window {
	return &Window{config: config}
}

// OnInit registers a callback called after the OpenGL control is initialized.
// The callback receives the TGPUControl for rendering setup.
func (w *Window) OnInit(fn func(*TGPUControl)) {
	w.onInit = fn
}

// Control returns the TGPUControl associated with this window.
func (w *Window) Control() *TGPUControl { return w.control }

// Form returns the LCL form associated with this window.
func (w *Window) Form() lcl.IEngForm { return w.form }

// setupForm is called by windowForm.FormCreate.
func (w *Window) setupForm(form lcl.IEngForm) {
	w.form = form
	w.form.SetCaption(w.config.Title)
	w.form.SetWidth(w.config.Width)
	w.form.SetHeight(w.config.Height)
	w.form.ScreenCenter()

	// Create OpenGL control
	glCtrl := lcl.NewOpenGLControl(w.form)
	configureOpenGLControl(glCtrl)
	glCtrl.SetParent(w.form)
	glCtrl.SetAlign(types.AlClient)

	// Create TGPUControl and attach
	w.control = NewGPUControl()
	w.control.Attach(glCtrl)

	// Set up events
	glCtrl.SetOnPaint(func(sender lcl.IObject) {
		w.control.onPaint(sender)
	})

	// Handle resize
	w.form.SetOnResize(func(sender lcl.IObject) {
		if w.control != nil {
			w.control.SetSize(w.form.ClientWidth(), w.form.ClientHeight())
		}
	})

	// Auto-init on first show
	w.form.SetOnShow(func(sender lcl.IObject) {
		if w.control != nil {
			w.control.Invalidate()
		}
	})
}

// configureOpenGLControl configures the OpenGL control's visual settings.
func configureOpenGLControl(ctrl lcl.IOpenGLControl) {
	if ctrl == nil {
		return
	}
	ctrl.SetRGBA(true)
	ctrl.SetOpenGLMajorVersion(3)
	ctrl.SetOpenGLMinorVersion(2)
	ctrl.SetAlphaBits(8)
	ctrl.SetDepthBits(0)
	ctrl.SetStencilBits(8)
	ctrl.SetMultiSampling(0)
	ctrl.SetAutoResizeViewport(false)
}