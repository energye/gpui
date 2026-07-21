// Package visualtest is track-2 component visual regression for ui polish.
//
// Harness: CPU render.Context → PaintContext (or minimal tree) → Image().
// Compare: warehouse baseline under testdata/visual/<id>.png with channel
// tolerance; on failure writes actual/diff under testdata/out/.
// Update baselines intentionally with UPDATE_VISUAL=1.
//
// See docs/UI_FRAMEWORK_MAP.md §12.2 (轨 2) and §12.3 W1.
package visualtest
