//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package night freezes the S68 night-stage interfaces only (W13b).
//
// No logic, no window: the second stage reuses the chase frame
// (examples/game_stage_chase) and combines lights plus postfx on top.
// Everything here is additive: no existing caller changes, old code
// compiles untouched. The window comes later and fills these in.
//
// Layers (bottom-up): NightLights describes the moon plus the player
// flashlight; NightGrade pins exposure and the frozen tonemap curve;
// NightStage applies both over one chase frame and reports the frozen
// CPU fallback mark (S66 rule: a set bit without a log line is fake).
package night

import (
	"github.com/energye/gpui/engine/fx"
	"github.com/energye/gpui/engine/light"
)

// NightLights is the frozen light rig: one dim moon (directional) plus
// one player flashlight (point with a cone cookie). Night floor keeps
// the S63 promise (never below 0.03, never a black screen).
type NightLights struct {
	Moon  light.Light
	Torch light.Light
	Night [3]float64
}

// NightGrade pins the post look: exposure plus the frozen default curve
// (fx.DefaultTonemap, Reinhard). Selectable in code, default frozen.
type NightGrade struct {
	Exposure float64
	Curve    fx.Tonemap
}

// NightFrame is one frozen chase frame handed to the night stage:
// base colors plus per-pixel layers. The night stage never mutates it.
type NightFrame struct {
	W, H   int
	Pix    []float64
	Layers []int
}

// NightResult is what the stage returns: lit colors plus the frozen CPU
// fallback mark and a log line when the mark is set.
type NightResult struct {
	Pix      []float64
	CPUMark  bool
	LogLines []string
}

// NightStage applies the rig plus the grade over one frame.
// Implementations reuse the chase sim read-only; the chase files stay
// untouched. A set CPUMark must come with at least one LogLines entry.
type NightStage interface {
	// Build validates the rig and grade (bad rig is an error, never a guess).
	Build(lights NightLights, grade NightGrade) error
	// Apply lights one frame then grades it, returning the mark and logs.
	Apply(frame NightFrame) (NightResult, error)
}
