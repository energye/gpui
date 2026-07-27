// Package focus provides a minimal focus tree and keyboard routing skeleton (P5c).
//
//	FocusManager  — one primary focus per window/scope
//	FocusNode     — RequestFocus / Unfocus / OnKey / OnFocusChange
//	Tab order     — registration order; optional TabIndex > 0 sorts first
//
// Non-goals: Shortcuts/Actions system, IME, platform a11y bridge.
//
// Dependency: focus → platform only (no rendering/gpu). Paint dirty is signaled
// via OnFocusChange callbacks so callers can MarkNeedsPaint without layout.
package focus
