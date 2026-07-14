package gpu

import (
	"testing"

	"github.com/energye/gpui/render"
)

func TestEffectiveStrokeWidthMatchesSoftwareSemantics(t *testing.T) {
	tests := []struct {
		name string
		p    render.Paint
		want float64
	}{
		{
			name: "default_transform",
			p:    render.Paint{LineWidth: 2},
			want: 2,
		},
		{
			name: "scaled_up",
			p:    render.Paint{LineWidth: 2, TransformScale: 3},
			want: 6,
		},
		{
			name: "hairline_clamp",
			p:    render.Paint{LineWidth: 0.5, TransformScale: 1},
			want: 1,
		},
		{
			name: "scaled_down_clamp",
			p:    render.Paint{LineWidth: 1, TransformScale: 0.5},
			want: 1,
		},
		{
			name: "stroke_object",
			p: render.Paint{
				LineWidth:      2,
				TransformScale: 2,
				Stroke:         &render.Stroke{Width: 3},
			},
			want: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := effectiveStrokeWidth(&tt.p); got != tt.want {
				t.Fatalf("effectiveStrokeWidth() = %v, want %v", got, tt.want)
			}
		})
	}
}
