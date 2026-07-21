// Package platform is the cross-platform host SPI (L2).
// core/primitive never import OS backends; they consume Host + events.
//
// M0: Caps, events, Headless host, Linux thin X11 adapter.
// Win/mac: stubs compile; real adapters in M6.
// See docs/UI_FRAMEWORK_MAP.md §9.4, §12 M0.
package platform
