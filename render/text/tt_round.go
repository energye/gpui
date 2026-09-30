//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// TrueType bytecode interpreter — round state.
//
// Implements all TrueType rounding modes: Grid, HalfGrid, DoubleGrid,
// DownToGrid, UpToGrid, Off, Super, Super45.
package text

// ttRoundMode selects the rounding strategy.
type ttRoundMode uint8

const (
	// ttRoundGrid rounds to nearest grid line. Set by RTG.
	ttRoundGrid ttRoundMode = iota
	// ttRoundHalfGrid rounds to nearest half grid line. Set by RTHG.
	ttRoundHalfGrid
	// ttRoundDoubleGrid rounds to nearest half or integer pixel. Set by RTDG.
	ttRoundDoubleGrid
	// ttRoundDownToGrid rounds down to nearest integer. Set by RDTG.
	ttRoundDownToGrid
	// ttRoundUpToGrid rounds up to nearest integer. Set by RUTG.
	ttRoundUpToGrid
	// ttRoundOff disables rounding. Set by ROFF.
	ttRoundOff
	// ttRoundSuper uses custom period/phase/threshold. Set by SROUND.
	ttRoundSuper
	// ttRoundSuper45 like Super but with sqrt(2)/2 period. Set by S45ROUND.
	ttRoundSuper45
)

// ttRoundState controls rounding behavior.
type ttRoundState struct {
	mode      ttRoundMode
	threshold int32
	phase     int32
	period    int32
}

// defaultRoundState returns the default round state (Grid mode, period=64).
func defaultRoundState() ttRoundState {
	return ttRoundState{
		mode:      ttRoundGrid,
		threshold: 0,
		phase:     0,
		period:    64,
	}
}

// round applies the current rounding mode to a 26.6 distance value.
func (rs *ttRoundState) round(distance int32) int32 {
	switch rs.mode {
	case ttRoundGrid:
		if distance >= 0 {
			r := ttRound26Dot6(distance)
			if r < 0 {
				return 0
			}
			return r
		}
		r := -ttRound26Dot6(-distance)
		if r > 0 {
			return 0
		}
		return r

	case ttRoundHalfGrid:
		// round modes clamp to 0. FT 2.11 sets all compensations to
		// 0 (ttobjs.c:1172-1175) so the clamp branch is unreachable
		// in practice, but keep FT semantics for correctness.
		if distance >= 0 {
			r := ttFloor26Dot6(distance) + 32
			if r < 0 {
				return 32
			}
			return r
		}
		r := -(ttFloor26Dot6(-distance) + 32)
		if r > 0 {
			return -32
		}
		return r

	case ttRoundDoubleGrid:
		if distance >= 0 {
			r := ttRoundPad(distance, 32)
			if r < 0 {
				return 0
			}
			return r
		}
		r := -ttRoundPad(-distance, 32)
		if r > 0 {
			return 0
		}
		return r

	case ttRoundDownToGrid:
		if distance >= 0 {
			r := ttFloor26Dot6(distance)
			if r < 0 {
				return 0
			}
			return r
		}
		r := -ttFloor26Dot6(-distance)
		if r > 0 {
			return 0
		}
		return r

	case ttRoundUpToGrid:
		if distance >= 0 {
			r := ttCeil26Dot6(distance)
			if r < 0 {
				return 0
			}
			return r
		}
		r := -ttCeil26Dot6(-distance)
		if r > 0 {
			return 0
		}
		return r

	case ttRoundSuper:
		if distance >= 0 {
			val := ((distance + (rs.threshold - rs.phase)) & -rs.period) + rs.phase
			if val < 0 {
				return rs.phase
			}
			return val
		}
		val := -(((rs.threshold - rs.phase) - distance) & -rs.period) - rs.phase
		if val > 0 {
			return -rs.phase
		}
		return val

	case ttRoundSuper45:
		if distance >= 0 {
			val := (((distance + (rs.threshold - rs.phase)) / rs.period) *
				rs.period) + rs.phase
			if val < 0 {
				return rs.phase
			}
			return val
		}
		val := -((((rs.threshold - rs.phase) - distance) / rs.period) *
			rs.period) - rs.phase
		if val > 0 {
			return -rs.phase
		}
		return val

	case ttRoundOff:
		return distance

	default:
		return distance
	}
}
