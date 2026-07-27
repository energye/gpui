//go:build !linux && !windows && !darwin

package text

// platformSystemFontCandidates: no built-in paths on this GOOS.
// Use text.SetDefaultFontPath / SetSystemFontPaths / FontResolver.SetFontFile.
func platformSystemFontCandidates(b FontRole) []string {
	return nil
}
