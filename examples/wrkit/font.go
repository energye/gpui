package wrkit

import (
	"fmt"
	"os"
	"sync"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// UIFace is a process-wide default face for ui_wr_* chrome / LiveHUD.
// DrawString / RenderText Paint **no-op without a Face** — labels placed at
// absolute coords would look like empty layout holes until this is set.
var (
	uiFaceOnce sync.Once
	uiFace     text.Face
	uiFacePath string
	uiFaceErr  error
)

// EnsureUIFace loads a system UI font once (size 14 base). Safe to call often.
// Returns (face, path, err). On err, text will still layout by rune estimate
// but will not paint — callers should log and avoid relying on invisible labels.
func EnsureUIFace() (text.Face, string, error) {
	uiFaceOnce.Do(func() {
		uiFace, uiFacePath, uiFaceErr = rendering.TryLoadDefaultFace(14)
		if uiFaceErr != nil {
			fmt.Fprintf(os.Stderr, "wrkit: UI font load failed: %v (text will be invisible)\n", uiFaceErr)
		} else {
			fmt.Fprintf(os.Stderr, "wrkit: UI font %s\n", uiFacePath)
		}
	})
	return uiFace, uiFacePath, uiFaceErr
}

// FaceAt returns a face scaled to points (falls back to EnsureUIFace base).
func FaceAt(points float64) text.Face {
	base, _, err := EnsureUIFace()
	if err != nil || base == nil {
		return nil
	}
	if points <= 0 {
		points = 14
	}
	// Prefer Source re-face when available so size tracks points.
	if src := base.Source(); src != nil {
		if f := src.Face(points); f != nil {
			return f
		}
	}
	return base
}

// Label builds a RenderText chrome label **with Face set** so Paint draws.
// Without Face, DrawString no-ops → Place coords look empty.
func Label(s string, size float64, r, g, b float64) *rendering.RenderText {
	t := rendering.NewRenderText(s)
	t.FontSize = size
	t.R, t.G, t.B, t.A = r, g, b, 1
	t.ApproxCharW = 0.55
	if face := FaceAt(size); face != nil {
		t.SetFace(face)
	}
	return t
}

// ApplyFace sets Face on an existing RenderText (no-op if face unavailable).
func ApplyFace(t *rendering.RenderText, points float64) {
	if t == nil {
		return
	}
	if face := FaceAt(points); face != nil {
		t.SetFace(face)
	}
}
