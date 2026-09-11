package input

// Kind classifies the cross-platform input event.
type Kind int

const (
	// KindNone is the zero value; not a real event.
	KindNone Kind = iota
	// KindClose is the destroyed window (external kill notification).
	// The interceptable ask arrives as KindCloseRequested instead.
	KindClose
	// KindResize carries new logical size / dpr.
	KindResize
	// KindKey is a key press/release (logical key code).
	KindKey
	// KindPointer is a mouse pointer sample (ID 0 = primary mouse cursor).
	KindPointer
	// KindTouch is a multi-touch sample (ID = touch slot, ≥ 1).
	KindTouch
	// KindScroll is a wheel / trackpad scroll delta.
	KindScroll
	// KindText is committed text (keyboard chars, paste, or IME commit).
	KindText
	// KindIME is an in-progress IME session event
	// (compose/commit/caret/delete-surrounding).
	KindIME
	// KindWake is a cross-thread wake-up (no input payload).
	KindWake
	// KindCloseRequested is the interceptable close ask (title-bar close,
	// WM_DELETE, xdg close). The window survives unless the app closes it;
	// KindClose below then reports the destroyed window.
	KindCloseRequested
	// KindMove carries the client-area top-left screen position.
	KindMove
	// KindScale carries an independent dpr change (reuses Scale).
	KindScale
	// KindOccluded reports fully-obscured/minimized state (reuses Occluded).
	KindOccluded
	// KindHidden reports app-driven hide/show (reuses Hidden).
	KindHidden
	// KindFocus reports keyboard-focus change (reuses Focused).
	KindFocus
	// KindStateChanged reports derived minimized/maximized/fullscreen state.
	KindStateChanged
	// KindThemeChanged reports dark/light switch (reuses Dark).
	KindThemeChanged
	// KindFramePresented reports the compositor showed a frame (no payload).
	KindFramePresented
	// KindResizeSync reports an X11 sync-resize frame request (no payload).
	KindResizeSync
	// KindMonitorChanged reports display add/remove (P2 placeholder shape).
	KindMonitorChanged
	// KindModifiersChanged reports a lone modifier-state change (reuses Modifiers).
	KindModifiersChanged
	// KindLocaleChanged reports system language/direction change.
	KindLocaleChanged
	// KindStylus is pen input; Down/Move/Up phase lives in Stylus.Kind.
	KindStylus
	// KindPinch reports a two-finger zoom gesture step.
	KindPinch
	// KindRotate reports a two-finger rotation gesture step.
	KindRotate
	// KindSmartMagnify reports a smart-zoom tap (no payload, P2 placeholder).
	KindSmartMagnify
	// KindDragEnter/KindDragOver report a drag hovering the window (reuses Drag).
	KindDragEnter
	KindDragOver
	// KindDragLeave reports the drag left the window (Drag stays zero).
	KindDragLeave
	// KindDrop reports an external drop onto the window (reuses Drag).
	KindDrop
	// KindDeviceAdded/KindDeviceRemoved report hot-plug (reuses Device).
	KindDeviceAdded
	KindDeviceRemoved
)

func (k Kind) String() string {
	switch k {
	case KindClose:
		return "close"
	case KindResize:
		return "resize"
	case KindKey:
		return "key"
	case KindPointer:
		return "pointer"
	case KindTouch:
		return "touch"
	case KindScroll:
		return "scroll"
	case KindText:
		return "text"
	case KindIME:
		return "ime"
	case KindWake:
		return "wake"
	case KindCloseRequested:
		return "close-requested"
	case KindMove:
		return "move"
	case KindScale:
		return "scale"
	case KindOccluded:
		return "occluded"
	case KindHidden:
		return "hidden"
	case KindFocus:
		return "focus"
	case KindStateChanged:
		return "state-changed"
	case KindThemeChanged:
		return "theme-changed"
	case KindFramePresented:
		return "frame-presented"
	case KindResizeSync:
		return "resize-sync"
	case KindMonitorChanged:
		return "monitor-changed"
	case KindModifiersChanged:
		return "modifiers-changed"
	case KindLocaleChanged:
		return "locale-changed"
	case KindStylus:
		return "stylus"
	case KindPinch:
		return "pinch"
	case KindRotate:
		return "rotate"
	case KindSmartMagnify:
		return "smart-magnify"
	case KindDragEnter:
		return "drag-enter"
	case KindDragOver:
		return "drag-over"
	case KindDragLeave:
		return "drag-leave"
	case KindDrop:
		return "drop"
	case KindDeviceAdded:
		return "device-added"
	case KindDeviceRemoved:
		return "device-removed"
	default:
		return "none"
	}
}

// Modifiers is the cross-platform modifier state at event time.
// Every backend fills the same four booleans; upper layers never map keysyms.
type Modifiers struct {
	Shift   bool
	Control bool
	Alt     bool
	Meta    bool
}

// IsEmpty reports no modifier is held.
func (m Modifiers) IsEmpty() bool {
	return !m.Shift && !m.Control && !m.Alt && !m.Meta
}

// Event is the normalized cross-platform input event. Exactly one payload
// field is meaningful per Kind; the rest are zero.
//
// All coordinates are logical pixels, Y-down (same convention as the rest of
// the UI engine). WindowID is assigned by the application layer for routing
// in multi-window setups.
type Event struct {
	Kind      Kind
	WindowID  int
	Modifiers Modifiers // held modifiers at event time (also KindModifiersChanged)

	// KindResize: new client-area size in logical pixels (Width/Height ≥ 1).
	Width  int
	Height int
	Scale  float64 // device pixel ratio; 0 = unchanged/unknown (also KindScale)

	// KindKey: logical key (Keys) + printable rune when one exists.
	Key KeyEvent

	// KindPointer / KindScroll: mouse sample.
	Pointer PointerEvent

	// KindTouch: multi-touch sample.
	Touch TouchEvent

	// KindText: committed character(s).
	Text TextEvent

	// KindIME: in-progress IME session event.
	IME IMEEvent

	// KindMove: client-area top-left in screen logical pixels (Y-down).
	MoveX, MoveY int

	// KindOccluded: true = fully obscured/minimized (stop rendering).
	Occluded bool
	// KindHidden: true = app hid the window (stop rendering + detach).
	Hidden bool
	// KindFocus: keyboard focus state.
	Focused bool
	// KindStateChanged: derived window state.
	State WindowState
	// KindThemeChanged: true = dark mode.
	Dark bool
	// KindMonitorChanged: display set snapshot (P2 placeholder shape).
	Monitor MonitorEvent
	// KindLocaleChanged: system language/direction.
	Locale LocaleEvent

	// KindStylus: pen sample (phase in Stylus.Kind).
	Stylus StylusEvent
	// KindPinch / KindRotate: gesture steps.
	Pinch  PinchEvent
	Rotate RotateEvent

	// KindDragEnter / KindDragOver / KindDrop: drag payload
	// (stays zero for KindDragLeave).
	Drag DragEvent

	// KindDeviceAdded / KindDeviceRemoved: hot-plugged device.
	Device DeviceEvent
}

// WindowState is the derived window state for KindStateChanged
// (minimize/maximize/fullscreen buttons all report through it).
type WindowState struct {
	Minimized  bool
	Maximized  bool
	Fullscreen bool
}

// MonitorEvent is the display-set snapshot for KindMonitorChanged.
type MonitorEvent struct {
	Count        int     // attached display count
	PrimaryScale float64 // primary display dpr (0 = unknown)
}

// LocaleEvent is the system language/direction for KindLocaleChanged.
type LocaleEvent struct {
	Language string // BCP 47 tag, empty = unknown
	RTL      bool   // true = right-to-left layout direction
}

// DeviceClass identifies a hot-plugged device family.
type DeviceClass int

const (
	DeviceUnknown DeviceClass = iota
	DeviceKeyboard
	DeviceMouse
	DeviceTouch
	DevicePen
)

func (c DeviceClass) String() string {
	switch c {
	case DeviceKeyboard:
		return "keyboard"
	case DeviceMouse:
		return "mouse"
	case DeviceTouch:
		return "touch"
	case DevicePen:
		return "pen"
	default:
		return "unknown"
	}
}

// DeviceEvent describes a hot-plugged device for
// KindDeviceAdded / KindDeviceRemoved.
type DeviceEvent struct {
	Class DeviceClass
	Name  string // OS device name, empty = unknown
}

// Phase is a gesture step phase shared by scroll, pinch and rotate.
// Unknown covers backends that report no phase (classic wheel ticks).
type Phase int

const (
	PhaseUnknown Phase = iota
	PhaseStarted
	PhaseMoved
	PhaseEnded
)

func (p Phase) String() string {
	switch p {
	case PhaseStarted:
		return "started"
	case PhaseMoved:
		return "moved"
	case PhaseEnded:
		return "ended"
	default:
		return "unknown"
	}
}

// StylusEvent is a pen sample for KindStylus. Kind carries the
// Down/Move/Up phase; Pressure is 0–1 (0 = unknown, sensor-less pens
// report 1); Tilt is in degrees (0 = unknown).
type StylusEvent struct {
	Kind         PointerKind
	ID           int // pen tool id (0 = primary pen, extra pens ≥ 1)
	X, Y         float64
	Pressure     float64
	TiltX, TiltY float64
	Eraser       bool
}

// PinchEvent is a two-finger zoom step for KindPinch.
type PinchEvent struct {
	ScaleDelta float64 // multiplicative factor since the last step
	Phase      Phase
}

// RotateEvent is a two-finger rotation step for KindRotate.
type RotateEvent struct {
	AngleDelta float64 // degrees since the last step
	Phase      Phase
}

// DragEvent is the drag-and-drop payload for KindDragEnter / KindDragOver /
// KindDrop (position in window logical pixels). Files lands first; Data is
// the P2 per-MIME extension (nil = unsupported).
type DragEvent struct {
	X, Y      float64
	MIMETypes []string
	Files     []string
	Data      map[string][]byte
}
