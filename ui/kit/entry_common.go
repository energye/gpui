package kit

import (
	"fmt"
	"time"

	"github.com/energye/gpui/ui/core"
)

// Package-level helpers for Data Entry controls.

func containsFold(s, sub string) bool {
	if sub == "" {
		return true
	}
	return len(s) >= len(sub) && (s == sub || indexASCII(s, sub) >= 0)
}

func indexASCII(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		ok := true
		for j := 0; j < len(sub); j++ {
			a, b := s[i+j], sub[j]
			if a >= 'A' && a <= 'Z' {
				a += 32
			}
			if b >= 'A' && b <= 'Z' {
				b += 32
			}
			if a != b {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return -1
}

func daysInMonth(year int, m time.Month) int {
	t := time.Date(year, m+1, 0, 0, 0, 0, 0, time.UTC)
	return t.Day()
}

func formatNum(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return fmt.Sprintf("%g", v)
}

func findInput(n core.Node) (*Input, bool) {
	return nil, false
}
