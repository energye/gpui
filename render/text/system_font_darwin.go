//go:build darwin

package text

// platformSystemFontCandidates lists common macOS system / supplemental fonts.
// Apple often ships .ttc collections; FontSource supports collection files.
func platformSystemFontCandidates(b FontRole) []string {
	switch b {
	case FontRoleLatin:
		return []string{
			"/System/Library/Fonts/Supplemental/Arial.ttf",
			"/Library/Fonts/Arial.ttf",
			"/System/Library/Fonts/Helvetica.ttc",
			"/System/Library/Fonts/SFNS.ttf",
			"/System/Library/Fonts/SFNSText.ttf",
			"/Library/Fonts/Arial Unicode.ttf",
			"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
		}
	case FontRoleCJK:
		return []string{
			"/System/Library/Fonts/PingFang.ttc",
			"/System/Library/Fonts/Hiragino Sans GB.ttc",
			"/System/Library/Fonts/Hiragino Sans.ttc",
			"/System/Library/Fonts/AppleSDGothicNeo.ttc",
			"/System/Library/Fonts/Supplemental/Songti.ttc",
			"/System/Library/Fonts/STHeiti Light.ttc",
			"/System/Library/Fonts/STHeiti Medium.ttc",
			"/Library/Fonts/Arial Unicode.ttf",
			"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
		}
	case FontRoleThai:
		return []string{
			"/System/Library/Fonts/Thonburi.ttc",
			"/System/Library/Fonts/Supplemental/Thonburi.ttc",
			"/Library/Fonts/Arial Unicode.ttf",
		}
	case FontRoleDevanagari:
		return []string{
			"/System/Library/Fonts/Supplemental/Devanagari Sangam MN.ttc",
			"/System/Library/Fonts/Devanagari Sangam MN.ttc",
			"/System/Library/Fonts/Supplemental/DevanagariMT.ttc",
			"/Library/Fonts/Arial Unicode.ttf",
		}
	case FontRoleArabic:
		return []string{
			"/System/Library/Fonts/Supplemental/Arial.ttf",
			"/System/Library/Fonts/GeezaPro.ttc",
			"/Library/Fonts/Arial.ttf",
			"/Library/Fonts/Arial Unicode.ttf",
		}
	default:
		return nil
	}
}
