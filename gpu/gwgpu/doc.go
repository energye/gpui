// Package gwgpu hosts gpui's self-maintained pure-Go GPU backend family.
//
// Layout mirrors the ported source so fixes stay comparable:
//   - hal/      — HAL interfaces (ported from gogpu wgpu/hal; gputypes
//     references rewritten to gpu/types, plus gwgpu_compat additions).
//   - hal/noop  — test-double backend for hal conformance tests.
//   - gles/     — OpenGL ES backend (ported from gogpu wgpu/hal/gles).
//   - gles/egl  — EGL display/context/surface on Linux.
//   - gles/gl   — GL entry-point table.
//   - gles/wgl  — Windows WGL helpers.
//   - ffishim/  — purego-backed replacement for the goffi C-ABI call layer
//     (LoadLibrary/GetSymbol/PrepareCallInterface/CallFunction). No libffi,
//     no external dependency.
//
// Maintenance: this tree is owned by gpui. It does not track the upstream
// gogpu tree; fixes land here directly with unit tests.
//
// Status: P0 port complete (build + unit + live EGL init green). Wiring into
// render/present (GPUI_BACKEND=gl path) is P1.
package gwgpu
