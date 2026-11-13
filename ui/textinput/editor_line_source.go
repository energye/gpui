//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package textinput

// editorLineSource exposes the Editor buffer as a rendering.LineSource so the
// viewport model can materialize only visible lines without GetText().
type editorLineSource struct{ ed *Editor }

func (s editorLineSource) LineCount() int {
	if s.ed == nil {
		return 0
	}
	return s.ed.ensureBuf().LineCount()
}

func (s editorLineSource) LineAt(k int) string {
	if s.ed == nil {
		return ""
	}
	return s.ed.ensureBuf().LineAt(k)
}

func (s editorLineSource) LineStart(k int) int {
	if s.ed == nil {
		return 0
	}
	return s.ed.ensureBuf().LineStart(k)
}
