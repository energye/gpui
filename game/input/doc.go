// Package input freezes the CPU-side action mapping every 2.5D play feeds.
//
// Frozen 2026-09-15 (capability 13.1, P0, S07/W1): Source, Valid,
// DeviceAny, Binding, NewKeyBinding, NewPadButtonBinding,
// NewPadAxisBinding, NewPinchBinding, ParseSource, AllSources, Map, NewMap, AddAction, Has,
// RemoveAction, Actions, ActionCount, SetDeadzone, Deadzone, Bind, Unbind,
// Rebind, ClearBindings, Bindings, SetKey, SetPadButton, SetPadAxis,
// AddPinchDelta, Pinch, ClearPinch, ResetInputs, Strength, Pressed, Vector,
// StartRumble, StopRumble, UpdateRumbles, RumbleLevels, RumbleActive.
// Additive changes only.
//
// The package draws nothing and makes no sound; it only turns hardware
// numbers into action numbers. The caller maps platform codes to the int
// ids stored here once at the boundary (for example ui/input.Key to int)
// and feeds stick values, pinch deltas, and rumble time; render and audio
// stay on the caller side.
//
// Actions: one name owns a deadzone plus a binding list. Any bound input
// fires the action; the reported strength is the maximum over all
// bindings after the per-action deadzone. Empty binding lists are legal
// and stay silent (never pressed, strength 0).
//
// Sources: key (digital code > 0), pad button (pad >= 0, button >= 0),
// pad axis (pad >= 0, axis >= 0, side +1/-1), pinch (spread +1, close -1).
// Device -1 (DeviceAny) matches any pad; key and pinch require DeviceAny.
// Only core numbers are used; bad inputs are core InvalidArg errors and
// change nothing, unknown actions are core NotFound.
package input
