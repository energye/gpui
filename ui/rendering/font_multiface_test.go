package rendering_test

import (
	"testing"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

func TestTryLoadDefaultFace_SingleSystemFont(t *testing.T) {
	text.ClearSystemFontPaths()
	face, desc, err := rendering.TryLoadDefaultFace(16)
	if err != nil {
		t.Skip(err)
	}
	t.Log("face:", desc)
	if face == nil {
		t.Fatal("nil face")
	}
	if !face.HasGlyph('A') {
		t.Fatal("expected Latin A on system UI font")
	}
	if _, ok := face.(*text.MultiFace); ok {
		t.Fatal("library default must be a single face, not MultiFace")
	}
}
