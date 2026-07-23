// X11-compatible modifier bit parsing shared by LinuxHost and unit tests.
// Masks match X.h ShiftMask / ControlMask / Mod1Mask / Mod4Mask.
package platform

// ParseModifierState maps a platform modifier bitfield into unified Event flags.
// Linux passes X event state; other hosts may pass an equivalent mask or set
// Event fields directly without this helper.
func ParseModifierState(state uint32) (shift, ctrl, alt, meta bool) {
	const (
		shiftMask = uint32(1 << 0)
		ctrlMask  = uint32(1 << 2)
		altMask   = uint32(1 << 3)
		metaMask  = uint32(1 << 6)
	)
	return state&shiftMask != 0, state&ctrlMask != 0, state&altMask != 0, state&metaMask != 0
}
