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
)

// Application wraps the LCL application lifecycle.
// Follows the standard LCL pattern: lcl.Init() → lcl.RunApp().
type Application struct {
	windows []*Window
}

// NewApplication creates a new Application.
func NewApplication() *Application {
	return &Application{}
}

// AddWindow adds a window to the application.
func (a *Application) AddWindow(w *Window) *Window {
	if a == nil || w == nil {
		return w
	}
	a.windows = append(a.windows, w)
	return w
}

// Run starts the application.
// It calls lcl.Init() and lcl.RunApp() with all registered windows.
func (a *Application) Run() {
	if a == nil {
		return
	}
	// Build the form list for RunApp
	forms := make([]lcl.IEngForm, len(a.windows))
	for i, w := range a.windows {
		forms[i] = &windowForm{window: w}
	}
	lcl.Init()
	lcl.RunApp(forms...)
}

// windowForm implements lcl.IEngForm for the Window.
type windowForm struct {
	lcl.TEngForm
	window *Window
}

// FormCreate is called by LCL when the form is created.
func (f *windowForm) FormCreate(sender lcl.IObject) {
	if f.window == nil {
		return
	}
	form, ok := sender.(lcl.IEngForm)
	if !ok || form == nil {
		return
	}
	f.window.setupForm(form)
}