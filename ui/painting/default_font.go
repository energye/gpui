package painting

import (
	"fmt"
	"os"

	"github.com/energye/gpui/render/text"
)

// common Linux font paths for UI demos (first existing wins).
var defaultFontCandidates = []string{
	"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
	"/usr/share/fonts/TTF/DejaVuSans.ttf",
	"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
	"/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf",
	"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
	"/usr/share/fonts/truetype/wqy/wqy-microhei.ttc",
	"/usr/share/fonts/truetype/arphic/uming.ttc",
}

// TryLoadDefaultFace loads a system UI font at the given point size.
// Returns face, path used, or error if none found / load failed.
// Override search with GPUI_UI_FONT=/path/to/font.ttf.
func TryLoadDefaultFace(points float64) (text.Face, string, error) {
	if points <= 0 {
		points = 14
	}
	candidates := defaultFontCandidates
	if p := os.Getenv("GPUI_UI_FONT"); p != "" {
		candidates = append([]string{p}, candidates...)
	}
	var lastErr error
	for _, path := range candidates {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		src, err := text.NewFontSourceFromFile(path)
		if err != nil {
			lastErr = err
			continue
		}
		return src.Face(points), path, nil
	}
	if lastErr != nil {
		return nil, "", fmt.Errorf("painting: no default font (last err: %w)", lastErr)
	}
	return nil, "", fmt.Errorf("painting: no default font found (set GPUI_UI_FONT)")
}
