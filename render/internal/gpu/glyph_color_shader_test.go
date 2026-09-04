//go:build !nogpu

package gpu

import (
	_ "embed"
	"strings"
	"testing"
)

//go:embed shaders/glyph_color.wgsl
var glyphColorShaderTestSource string

func TestGlyphColorShaderContent(t *testing.T) {
	for _, req := range []string{
		"@vertex",
		"@fragment",
		"vs_main",
		"fs_main",
		"texture_2d<f32>",
		"sampler",
		"textureSample",
		"tex.rgb",
		"tex.a",
		"uniforms.color.a",
	} {
		if !strings.Contains(glyphColorShaderTestSource, req) {
			t.Errorf("glyph_color shader missing required element: %q", req)
		}
	}
}
