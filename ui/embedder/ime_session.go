//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package embedder

import (
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/textinput"
)

// One per window; the ImeSession facade drives it exclusively.
type imeAdapter struct {
	ime platform.IME
}

// NewImeAdapter wraps a platform.IME capability into the facade's adapter
// (exported: examples/kit assemble ImeSession themselves).
func NewImeAdapter(ime platform.IME) *imeAdapter { return &imeAdapter{ime: ime} }

func (a *imeAdapter) Enable(f platform.FieldSnapshot)    { a.ime.EnableIME(f.Rect) }
func (a *imeAdapter) Disable()                           { a.ime.DisableIME() }
func (a *imeAdapter) CaretMoved(rect platform.Rect)      { a.ime.UpdateCursorRect(rect) }
func (a *imeAdapter) SetPurpose(ct platform.ContentType) { a.ime.SetContentType(ct.Purpose) }
func (a *imeAdapter) PushSurrounding(text string, cursor int) {
	a.ime.SetComposing(text, cursor)
}

var _ textinput.PlatformAdapter = (*imeAdapter)(nil)
