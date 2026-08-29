package textinput

// 文本内边距，默认 8px（对齐 Flutter InputDecoration.contentPadding 8）
// 可全局或单框覆盖，未设走 8
var (
	defaultPad    float64 = 8
	hasDefaultPad bool
)

// SetDefaultPadding 设置全局内边距
func SetDefaultPadding(pad float64) {
	if pad < 0 {
		pad = 0
	}
	defaultPad = pad
	hasDefaultPad = true
}
func ClearDefaultPadding() { hasDefaultPad = false; defaultPad = 8 }
func DefaultPadding() float64 {
	if hasDefaultPad {
		return defaultPad
	}
	return 8
}
func resolvePad(hasCustom bool, custom float64) float64 {
	if hasCustom {
		if custom < 0 {
			return 0
		}
		return custom
	}
	return DefaultPadding()
}
