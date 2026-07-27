// Package platform is the cross-platform host SPI (L2) for Linux, Windows, and macOS.
// core/primitive never import OS backends; they consume Host + events.
//
// Hosts:
//   - Headless: CI / unit tests (all Caps needed for kit tests)
//   - LinuxHost: X11 and/or Wayland (purego, no cgo); CapIME deferred (XIM)
//   - WindowsHost / DarwinHost: API-complete stubs (+ CapClipboard); real HWND/AppKit later
//
// Linux display selection (mirrors examples/exhost reference):
//
//	GPUI_DISPLAY=wayland|x11|auto   (default auto)
//	Auto: prefer Wayland when WAYLAND_DISPLAY is set; fall back to X11 (DISPLAY)
//
// Native handles for wgpu:
//
//	ns, ok := platform.SurfaceOf(host)  // Kind + Display + Window
//
// Clipboard (CapClipboard): NewSystemClipboard()
//   - Linux: xclip/xsel + memory fallback
//   - Windows: PowerShell Get/Set-Clipboard + memory fallback
//   - macOS: pbcopy/pbpaste + memory fallback
//
// app.Attach bridges ClipboardProvider → core.Tree.SetClipboard automatically.
//
// See docs/UI_FRAMEWORK_MAP.md §9.4, §12 M0/M6, docs/UI_FOUNDATION_P0.md.
package platform
