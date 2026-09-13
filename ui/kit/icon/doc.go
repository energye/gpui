// Package icon implements the Icon control (docs/antd/icon.md §6).
//
// Composition over new frameworks: the widget owns a rendering.RenderBox
// node (Node) and reuses ui/rendering for draw, ui/theme for color,
// ui/scheduler only as a Ticker host. No new event or frame system.
package icon
