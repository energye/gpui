package wrkit

import (
	"fmt"
	"os"
	"path/filepath"
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
//
// Default chain puts Source Han Sans SC (Noto Sans CJK SC — the open-source
// Microsoft YaHei equivalent, covering Latin too) first, then the Latin UI
// sans, Thai, Devanagari and Arabic role fallbacks. The system NotoSansCJK
// TTC resolves to the JP sub-font by default collection index, so the SC
// single-language file in the user font dir is preferred when present —
// simplified-Han glyph forms with the YaHei-style design.
func EnsureUIFace() (text.Face, string, error) {
	uiFaceOnce.Do(func() {
		uiFace, uiFacePath, uiFaceErr = loadUIChain()
		if uiFaceErr != nil {
			fmt.Fprintf(os.Stderr, "wrkit: UI font load failed: %v (text will be invisible)\n", uiFaceErr)
		} else {
			fmt.Fprintf(os.Stderr, "wrkit: UI font %s\n", uiFacePath)
		}
	})
	return uiFace, uiFacePath, uiFaceErr
}

// loadUIChain builds the ui_wr_* chrome MultiFace: CJK role first so the
// YaHei-style Source Han Sans SC face drives both Han and Latin text.
func loadUIChain() (text.Face, string, error) {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		sc := filepath.Join(home, ".local", "share", "fonts", "NotoSansCJKsc-Regular.otf")
		text.SetSystemFontPaths(text.FontRoleCJK, sc,
			"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc")
	}
	return text.LoadDefaultFaceFor(14, []text.FontRole{
		text.FontRoleCJK,
		text.FontRoleUI,
		text.FontRoleThai,
		text.FontRoleDevanagari,
		text.FontRoleArabic,
	})
}

// FaceAt returns a face scaled to points (falls back to EnsureUIFace base).
// The MultiFace chain is re-pinned to the production hinting mode
// (HintingVertical = FT light, matching the GPU glyph-mask pipeline, engine
// default is HintingFull) — MultiFace.Source() is nil, so the re-pin goes
// through the engine's MultiFace.WithHinting + AtSize (options preserved).
func FaceAt(points float64) text.Face {
	base, _, err := EnsureUIFace()
	if err != nil || base == nil {
		return nil
	}
	if points <= 0 {
		points = 14
	}
	if mf, ok := base.(*text.MultiFace); ok {
		if f := mf.WithHinting(text.HintingVertical); f != nil {
			if f2 := f.AtSize(points); f2 != nil {
				return f2
			}
		}
	}
	if src := base.Source(); src != nil {
		if f := src.Face(points, text.WithHinting(text.HintingVertical)); f != nil {
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
