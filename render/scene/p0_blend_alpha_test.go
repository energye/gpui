package scene

import (
	"testing"

	"github.com/energye/gpui/render/internal/blend"
)

// P0.3: scene-level UI blend modes must map to fixed-pixel blend results.

func TestP03SceneModes_FixedPixels(t *testing.T) {
	type px struct{ r, g, b, a byte }
	cases := []struct {
		name string
		mode BlendMode
		src  px
		dst  px
		want px
	}{
		{
			name: "Normal == SourceOver opaque red over blue",
			mode: BlendNormal,
			src:  px{255, 0, 0, 255},
			dst:  px{0, 0, 255, 255},
			want: px{255, 0, 0, 255},
		},
		{
			name: "SourceOver opaque red over blue",
			mode: BlendSourceOver,
			src:  px{255, 0, 0, 255},
			dst:  px{0, 0, 255, 255},
			want: px{255, 0, 0, 255},
		},
		{
			name: "Copy replaces",
			mode: BlendCopy,
			src:  px{1, 2, 3, 4},
			dst:  px{9, 8, 7, 6},
			want: px{1, 2, 3, 4},
		},
		{
			name: "Plus clamps",
			mode: BlendPlus,
			src:  px{200, 10, 0, 200},
			dst:  px{200, 20, 5, 200},
			want: px{255, 30, 5, 255},
		},
		{
			name: "Multiply black * white",
			mode: BlendMultiply,
			src:  px{0, 0, 0, 255},
			dst:  px{255, 255, 255, 255},
			want: px{0, 0, 0, 255},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fn := tc.mode.GetBlendFunc()
			r, g, b, a := fn(tc.src.r, tc.src.g, tc.src.b, tc.src.a, tc.dst.r, tc.dst.g, tc.dst.b, tc.dst.a)
			if r != tc.want.r || g != tc.want.g || b != tc.want.b || a != tc.want.a {
				t.Fatalf("%v = rgba(%d,%d,%d,%d), want rgba(%d,%d,%d,%d)",
					tc.mode, r, g, b, a, tc.want.r, tc.want.g, tc.want.b, tc.want.a)
			}
		})
	}
}

func TestP03SceneModeMapping_Internal(t *testing.T) {
	checks := []struct {
		scene    BlendMode
		internal blend.BlendMode
	}{
		{BlendNormal, blend.BlendSourceOver},
		{BlendSourceOver, blend.BlendSourceOver},
		{BlendCopy, blend.BlendSource},
		{BlendPlus, blend.BlendPlus},
		{BlendMultiply, blend.BlendMultiply},
	}
	for _, c := range checks {
		got := c.scene.ToInternalBlendMode()
		if got != c.internal {
			t.Fatalf("%s.ToInternalBlendMode() = %v, want %v", c.scene, got, c.internal)
		}
	}
}
