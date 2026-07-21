// Package platform is the cross-platform host SPI (L2).
// core/primitive never import OS backends; they consume Host + events.
//
// M0: Caps, events, Headless host, Linux thin X11 adapter.
// Win/mac: stubs compile; real adapters in M6.
//
// IME (W4): CapIME is set on Headless (InjectIME for CI). LinuxHost does not
// advertise CapIME until XIM/XIC is wired — see ime.go for the formal contract.
// See docs/UI_FRAMEWORK_MAP.md §9.4, §12 M0, §12.3 W4.
package platform
