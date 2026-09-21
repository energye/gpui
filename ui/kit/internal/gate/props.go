package gate

// PropsCoverage is the window-reported Props audit: Total API params
// from the antd doc table, Covered actually exercised, Missing named.
type PropsCoverage struct {
	Component string
	Total     int
	Covered   int
	Missing   []string
}

// CheckProps reports whether every documented param is covered.
// Missing names or Covered<Total both fail.
func CheckProps(c PropsCoverage) (bool, []string) {
	if c.Total <= 0 {
		return false, []string{"total"}
	}
	if c.Covered < c.Total {
		m := append([]string{}, c.Missing...)
		if len(m) == 0 {
			m = []string{"uncovered"}
		}
		return false, m
	}
	if len(c.Missing) > 0 {
		return false, append([]string{}, c.Missing...)
	}
	return true, nil
}
