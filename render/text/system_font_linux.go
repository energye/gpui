//go:build linux

package text

import (
	"os"
	"path/filepath"
)

// platformSystemFontCandidates lists common distro font paths (probe order).
// No machine-specific home hardcoding; user fonts are expanded at runtime.
func platformSystemFontCandidates(b FontRole) []string {
	switch b {
	case FontRoleLatin:
		return withUserFontDirs([]string{
			"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
			"/usr/share/fonts/TTF/DejaVuSans.ttf",
			"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
			"/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf",
			"/usr/share/fonts/opentype/noto/NotoSans-Regular.ttf",
			"/usr/share/fonts/truetype/freefont/FreeSans.ttf",
			"/usr/share/fonts/truetype/ubuntu/Ubuntu-R.ttf",
		}, []string{
			"DejaVuSans.ttf",
			"NotoSans-Regular.ttf",
			"LiberationSans-Regular.ttf",
		})
	case FontRoleCJK:
		return withUserFontDirs([]string{
			"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf",
			"/usr/share/fonts/truetype/wqy/wqy-microhei.ttc",
			"/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc",
			"/usr/share/fonts/truetype/arphic/uming.ttc",
			"/usr/share/fonts/opentype/source-han-sans/SourceHanSansSC-Regular.otf",
		}, []string{
			"NotoSansCJK-Regular.ttc",
			"NotoSansCJKsc-Regular.otf",
			"NotoSansCJKsc-Regular.ttc",
			"SourceHanSansSC-Regular.otf",
		})
	case FontRoleThai:
		return withUserFontDirs([]string{
			"/usr/share/fonts/truetype/tlwg/Garuda.ttf",
			"/usr/share/fonts/truetype/tlwg/Loma.ttf",
			"/usr/share/fonts/truetype/tlwg/TlwgTypo.ttf",
			"/usr/share/fonts/truetype/noto/NotoSansThai-Regular.ttf",
			"/usr/share/fonts/opentype/noto/NotoSansThai-Regular.ttf",
		}, []string{
			"Garuda.ttf",
			"NotoSansThai-Regular.ttf",
		})
	case FontRoleDevanagari:
		return withUserFontDirs([]string{
			"/usr/share/fonts/truetype/lohit-devanagari/Lohit-Devanagari.ttf",
			"/usr/share/fonts/truetype/Sahadeva/sahadeva.ttf",
			"/usr/share/fonts/truetype/noto/NotoSansDevanagari-Regular.ttf",
			"/usr/share/fonts/opentype/noto/NotoSansDevanagari-Regular.ttf",
		}, []string{
			"Lohit-Devanagari.ttf",
			"NotoSansDevanagari-Regular.ttf",
		})
	case FontRoleArabic:
		return withUserFontDirs([]string{
			"/usr/share/fonts/truetype/noto/NotoSansArabic-Regular.ttf",
			"/usr/share/fonts/opentype/noto/NotoSansArabic-Regular.ttf",
			"/usr/share/fonts/truetype/noto/NotoNaskhArabic-Regular.ttf",
			"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
			"/usr/share/fonts/TTF/DejaVuSans.ttf",
		}, []string{
			"NotoSansArabic-Regular.ttf",
			"NotoNaskhArabic-Regular.ttf",
		})
	default:
		return nil
	}
}

// withUserFontDirs appends $XDG_DATA_HOME/fonts and ~/.local/share/fonts
// (and nested one level) for the given basenames — portable user installs.
func withUserFontDirs(system []string, basenames []string) []string {
	out := append([]string{}, system...)
	dirs := userFontDirs()
	for _, dir := range dirs {
		for _, base := range basenames {
			out = append(out, filepath.Join(dir, base))
		}
		// one-level subdirs (e.g. ~/.local/share/fonts/noto/Foo.ttf)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			sub := filepath.Join(dir, e.Name())
			for _, base := range basenames {
				out = append(out, filepath.Join(sub, base))
			}
		}
	}
	return out
}

func userFontDirs() []string {
	var dirs []string
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		dirs = append(dirs, filepath.Join(xdg, "fonts"))
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		dirs = append(dirs,
			filepath.Join(home, ".local", "share", "fonts"),
			filepath.Join(home, ".fonts"),
		)
	}
	return dirs
}
