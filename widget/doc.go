// Package widget provides a small, composable UI control layer on top of
// github.com/energye/gpui/render.
//
// Architecture constraints (S5.5 / S6):
//   - All painting goes through render.Context (no alternate rasterizer).
//   - Prefer PresentFrame / PresentFrameAuto / damage retained frames.
//   - No silent CPU path claims; GPU correctness is owned by render.
//
// W0 scope: paint-only first batch (Button, Input, Modal, ListRow, TableCell)
// plus theme tokens and hit-test helpers. Full interaction / layout engines
// are later slices.
package widget
