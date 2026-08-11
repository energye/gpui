// Package input defines the cross-platform normalized input event layer.
//
// Every platform backend (X11/Wayland/Win32/AppKit) produces raw-ish
// platform.Event values; input.FromPlatform converts them into a single
// platform-agnostic Event that upper layers (gestures / focus / textinput /
// kit) consume. Upper layers never see keysyms, HWNDs, or other native
// identifiers.
//
// The two cross-cutting goals are:
//  1. One event vocabulary for every platform (logical keys, one pointer
//     model with ID-based multi-touch, text/IME events).
//  2. A single conversion point (FromPlatform) where platform differences
//     are absorbed, so UI code stays untouched per-platform.
package input