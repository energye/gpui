//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build linux

package platform

// Wayland tablet placeholder.
//
// The zwp_tablet_manager_v2 / tablet_tool protocol batch (pen pressure, tilt,
// eraser, tool serials) is intentionally unbound in this phase: seat
// capabilities keep flowing for keyboard/pointer/touch, tablet stays silent
// with zero behavior change. X11 leads via XInput2 (see x11_stylus_linux.go);
// this file reserves the home for the phase-2 binding so the port has a
// single landing spot and reviewers do not mistake "missing file" for
// "forgotten protocol".
//
// Phase-2 work (tracked, not started): bind zwp_tablet_manager_v2 from the
// registry, track tablet seats/tools, decode tablet_tool axes into
// EventStylus (Pressure 0–1 with sensor-less fallback 1, Tilt degrees,
// Eraser), and surface tool add/remove alongside seat capabilities.
const waylandTabletPhase = "phase-2-placeholder"

var _ = waylandTabletPhase
