// Package kit hosts Ant Design product controls for gpui.
//
// Rules (W0 foundation):
//
//   - One control, one directory: ui/kit/button/, ui/kit/select/, ...
//     Change only your own directory in a task; shared changes go through
//     the owning mechanism package.
//   - No second event/frame system: reuse ui/rendering (RenderObject),
//     ui/scene (Layer), ui/theme (tokens), ui/overlay (placement),
//     ui/focus (keyboard) and ui/gestures (pointer). Follow ui to render to
//     gpu direction only and keep CGO_ENABLED=0 builds green.
//   - Every §6.10 Go signature from docs/antd/<name>.md lands in
//     ui/kit/<name>/; old primitive/core names map to ui/rendering and
//     friends instead of new packages.
//   - Demos: each control owns one gallery page in
//     examples/ui_polish_gallery (one tab per control, sections follow the
//     official examples). A page must open standalone (flag/filter), test
//     standalone and screenshot standalone; the combined window is preview
//     only and never replaces a single-capability assertion.
//   - L1-L4 (behavior/token/golden/eye) are acceptance tiers per control
//     §6.1; W0-W5 are build waves per docs/antd/README. They do not conflict.
package kit
