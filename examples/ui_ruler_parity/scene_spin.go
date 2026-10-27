package main

import (
	"fmt"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/kit"
)

// buildSpinScene mirrors tools/ruler_flutter spin: 480x320, #1A1E29, one
// 64px white sync glyph rotating once per 2s. Flutter uses Material Icons;
// gpui uses the kit sync glyph, so the silhouette differs but the edge,
// color, center and angular period are identical.
func buildSpinScene() *sceneSpec {
	root := newRoot(winSpinW, winSpinH, colBg)

	ctx := kit.DefaultScopeCtx()
	props := kit.DefaultIconProps("sync")
	props.Variant = "outlined"
	props.Spin = true
	props.Color, props.ColorSet = "#ffffff", true
	spin := kit.BuildIconBox(ctx, props, 64, "ruler-parity-spin-sync")
	root.Place(spin.Outer, float64(winSpinW)/2-32, float64(winSpinH)/2-32)

	return &sceneSpec{
		name:   sceneSpin,
		ref:    "tools/ruler_flutter RULER_SCENE=spin",
		winW:   winSpinW,
		winH:   winSpinH,
		title:  "gpui ruler parity — Flutter spin (480x320)",
		root:   root,
		clearR: colBg.R, clearG: colBg.G, clearB: colBg.B,
		tick: func(dt float64) bool {
			if spin.Inst.Tick(dt) {
				spin.SyncIconTransform()
			}
			return true
		},
		goldenRects: []wrkit.GoldenRect{{X: 0, Y: 0, W: winSpinW, H: winSpinH}},
		probes: []wrkit.PixelProbe{
			{Name: "spin_bg_tl", X: 10, Y: 10, R: colBg.R, G: colBg.G, B: colBg.B, Tol: 4},
			{Name: "spin_bg_br", X: 470, Y: 310, R: colBg.R, G: colBg.G, B: colBg.B, Tol: 4},
		},
		extraCheck: func(snapshot string) error {
			img, err := loadSnapshot(snapshot)
			if err != nil {
				return fmt.Errorf("decode spin snapshot: %w", err)
			}
			n := inkCount(img, 208, 128, 64, 64, colBg, 8)
			if n < 40 {
				return fmt.Errorf("spin glyph ink too small: %d px (want >=40)", n)
			}
			return nil
		},
	}
}
