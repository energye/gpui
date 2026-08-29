package textinput

// 全局选中色，未设置时用引擎默认 0.22,0.45,0.85,0.35（Flutter 选区蓝）
var (
	defaultSelR, defaultSelG, defaultSelB, defaultSelA = 0.22, 0.45, 0.85, 0.35
	hasDefaultSel                                        bool
)

// SetDefaultSelectionColor 设置全局选中色（全局未设时回退到引擎默认）
func SetDefaultSelectionColor(r, g, b, a float64) {
	defaultSelR, defaultSelG, defaultSelB, defaultSelA = r, g, b, a
	hasDefaultSel = true
}

// ClearDefaultSelectionColor 清掉全局，回到引擎默认
func ClearDefaultSelectionColor() { hasDefaultSel = false }

// DefaultSelectionColor 返回当前全局色（未设时即引擎默认）
func DefaultSelectionColor() (r, g, b, a float64) {
	if hasDefaultSel {
		return defaultSelR, defaultSelG, defaultSelB, defaultSelA
	}
	return 0.22, 0.45, 0.85, 0.35
}

func resolveSelColor(hasCustom bool, cr, cg, cb, ca float64) (r, g, b, a float64) {
	if hasCustom {
		return cr, cg, cb, ca
	}
	return DefaultSelectionColor()
}
