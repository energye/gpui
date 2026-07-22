package primitive

import "github.com/energye/gpui/render/text"

// faceAtSize returns a Face at the requested point size when Face.Source is
// available. FontSize on Text/EditableText was previously ignored whenever Face
// was set (Face is baked at creation size, e.g. Face(14)), so Style.FontSize=8
// had no effect. Re-derive from the same FontSource instead.
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
