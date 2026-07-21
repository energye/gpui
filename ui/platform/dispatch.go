package platform

import "github.com/energye/gpui/ui/core"

// Dispatch translates platform events into core.Tree pointer/key routing.
// Resize/focus/close are returned for the app loop to handle.
func Dispatch(tree *core.Tree, ev Event) (resize *Event, close bool) {
	if tree == nil {
		if ev.Type == EventClose {
			return nil, true
		}
		if ev.Type == EventResize {
			e := ev
			return &e, false
		}
		return nil, false
	}
	switch ev.Type {
	case EventPointer:
		pe := &core.PointerEvent{
			Type:      mapPointer(ev.Pointer),
			X:         ev.X,
			Y:         ev.Y,
			Button:    mapButton(ev.Button),
			PointerID: ev.PointerID,
		}
		tree.DispatchPointer(pe)
	case EventKey:
		ke := &core.KeyEvent{
			Key:  ev.Key,
			Text: ev.Text,
		}
		if ev.Down {
			ke.Type = core.KeyDown
		} else {
			ke.Type = core.KeyUp
		}
		tree.DispatchKey(ke)
	case EventText:
		ke := &core.KeyEvent{Type: core.KeyDown, Text: ev.Text, Key: "Text"}
		tree.DispatchKey(ke)
	case EventResize:
		e := ev
		return &e, false
	case EventClose:
		return nil, true
	}
	return nil, false
}

func mapPointer(k PointerKind) core.PointerType {
	switch k {
	case PointerDown:
		return core.PointerDown
	case PointerUp:
		return core.PointerUp
	case PointerCancel:
		return core.PointerCancel
	default:
		return core.PointerMove
	}
}

func mapButton(b PointerBtn) core.PointerButton {
	switch b {
	case BtnLeft:
		return core.ButtonLeft
	case BtnMiddle:
		return core.ButtonMiddle
	case BtnRight:
		return core.ButtonRight
	default:
		return core.ButtonNone
	}
}
