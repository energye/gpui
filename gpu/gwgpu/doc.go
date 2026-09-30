//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package gwgpu hosts gpui's self-maintained pure-Go GPU backend family.
//
// Layout mirrors the ported source so fixes stay comparable:
//   - gpu/hal (+noop) — HAL interfaces live one level up (sibling of
//     gpu/types); gputypes references rewritten to gpu/types, plus
//     gwgpu_compat additions.
//   - gles/ — OpenGL ES backend.
//   - gles/egl — EGL display/context/surface on Linux.
//   - gles/gl — GL entry-point table.
//   - gles/wgl — Windows WGL helpers.
//   - ffishim/ — purego-backed replacement for the goffi C-ABI call layer
//     (LoadLibrary/GetSymbol/PrepareCallInterface/CallFunction). No libffi,
//     no external dependency.
//
// Maintenance: this tree is owned by gpui. It does not track the upstream
// gogpu tree; fixes land here directly with unit tests.
//
// Status: P0 port complete (build + unit + live EGL init green).
package gwgpu
