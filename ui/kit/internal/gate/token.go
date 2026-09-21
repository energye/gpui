package gate

// TokenUse is the window-reported theme audit: colors resolve from
// Ctx tokens with zero hardcoded user-facing values in Render.
type TokenUse struct {
	FromTheme      bool
	HardcodedCount int
}

// CheckToken passes only when values come from theme and nothing is
// hardcoded in Render.
func CheckToken(u TokenUse) bool {
	return u.FromTheme && u.HardcodedCount == 0
}
