//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package loop is the generic runway every 2.5D play runs on.
//
// Frozen 2026-09-30 (E05): Loop, New,
// AddSystem, Systems, SystemCount, SetPacketHook, InjectPacket,
// PendingPackets, SetSubmit, Frame, Dt, Steps, Elapsed, Alpha,
// MaxPendingPackets. Additive changes only.
//
// One frame, always the same order: drain queued net packets, run every
// registered system in registration order once per fixed tick, then hand
// the blend factor to the submit hook. The loop writes no gameplay: no
// monsters, no scenes, no wins. Plays register systems; the loop only
// runs the table.
//
// Time is core.Duration integer milliseconds on a step.Fixed beat, so
// slow and fast machines agree tick by tick. Only core numbers cross
// the boundary; render types are converted by the submit hook's owner,
// never here.
package loop
