package behavior

import (
	"math"

	"github.com/energye/gpui/ui/semantics"
	"github.com/energye/gpui/ui/theme"
)

// Semantics facade (F0-5 §6.2): F0 only checks named-plus-role.
// Merge/exclude actions stay in the engine backlog; this file never
// reimplements the semantics tree, it only audits it.

// AuditOne checks a single node: it must carry a non-empty name and
// a non-empty role. Unnamed or role-less nodes fail (screen readers
// would see a nameless control).
func AuditOne(n *semantics.Node) (ok bool, missing string) {
	if n == nil {
		return false, "nil"
	}
	if n.Label == "" {
		return false, "name"
	}
	if n.Role == "" || n.Role == semantics.RoleNone {
		return false, "role"
	}
	return true, ""
}

// AuditTree counts named-and-roled nodes over a flattened tree.
func AuditTree(root *semantics.Node) (total, named int) {
	flat := semantics.Flatten(root)
	total = len(flat)
	for _, f := range flat {
		if f.Label != "" && f.Role != "" && f.Role != semantics.RoleNone {
			named++
		}
	}
	return total, named
}

// MinTouchOK is the advisory minimum touch target (44px). F0 reports
// it as a warning, never a gate: small targets advise, they do not fail.
func MinTouchOK(w, h float64) bool { return w >= 44 && h >= 44 }

// ContrastRatio returns the WCAG relative-luminance ratio of fg over bg.
func ContrastRatio(fg, bg theme.Color) float64 {
	l1 := luminance(fg)
	l2 := luminance(bg)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

// PassContrast reports ratio >= 4.5 (advisory only in F0).
func PassContrast(ratio float64) bool { return ratio >= 4.5 }

func luminance(c theme.Color) float64 {
	r := lin(c.R)
	g := lin(c.G)
	b := lin(c.B)
	return 0.2126*r + 0.7152*g + 0.0722*b
}

func lin(v float64) float64 {
	if v <= 0.03928 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}
