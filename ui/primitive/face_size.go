package primitive

import "github.com/energye/gpui/render/text"

// faceAtSize returns a Face at the requested point size when possible.
//
// FontSize on Text/Typography was ignored whenever Face was set at another size
// (e.g. gallery SetFace(14) then SetFontSize(16/20)) — glyphs were painted with
// a mismatched face → uneven stroke weight / blur, especially on CJK titles.
//
// MultiFace: Source() is nil, so we must call AtSize to resize every component
// (Latin + CJK fallback). Plain faces re-derive via FontSource.Face(size).
func faceAtSize(face text.Face, size float64) text.Face {
	if face == nil {
		return nil
	}
	if size <= 0 {
		return face
	}
	// Already at target (tolerance for float noise).
	if fs := face.Size(); fs > 0 && absf(fs-size) < 0.25 {
		return face
	}
	// MultiFace / any AtSize implementation: keep full fallback chain.
	type atSizer interface {
		AtSize(float64) text.Face
	}
	if a, ok := face.(atSizer); ok {
		return a.AtSize(size)
	}
	if mf, ok := face.(*text.MultiFace); ok {
		return mf.AtSize(size)
	}
	src := face.Source()
	if src == nil {
		return face
	}
	return src.Face(size)
}

func absf(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
