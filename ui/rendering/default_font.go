package rendering

import (
	"github.com/energye/gpui/render/text"
)

// TryLoadDefaultFace loads one common system UI font (library default).
//
// Configure fonts with functions only (no env):
//
//	text.SetDefaultFontPath("/path/App.ttf")           // process
//	r := text.NewFontResolver().SetFontFile("a.ttf") // instance
//	face, _, _ := r.Load(16)
//	// or rendering.TryLoadDefaultFaceWith(r, 16)
//
// Multi-script is opt-in in the app/example: text.LoadMultiFace(16).
func TryLoadDefaultFace(points float64) (text.Face, string, error) {
	return text.LoadDefaultFace(points)
}

// TryLoadDefaultFaceWith uses an app-owned FontResolver.
func TryLoadDefaultFaceWith(r *text.FontResolver, points float64) (text.Face, string, error) {
	if r == nil {
		return text.LoadDefaultFace(points)
	}
	return r.Load(points)
}
