package platform

import "github.com/energye/gpui/ui/core"

// Dispatch translates platform events into core.Tree pointer/key routing.
// Resize/focus/close are returned for the app loop to handle.
// EventRedraw marks the tree dirty (demand-driven paint; gogpu RequestRedraw).
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
			Key:   ev.Key,
			Text:  ev.Text,
			Shift: ev.Shift,
			Ctrl:  ev.Ctrl,
			Alt:   ev.Alt,
			Meta:  ev.Meta,
		}
		if ev.Down {
			ke.Type = core.KeyDown
		} else {
			ke.Type = core.KeyUp
		}
		tree.DispatchKey(ke)
	case EventText:
		tree.DispatchTextInput(&core.TextInputEvent{Text: ev.Text})
	case EventScroll:
		tree.DispatchScroll(&core.ScrollEvent{
			X: ev.X, Y: ev.Y, DX: ev.ScrollDX, DY: ev.ScrollDY,
		})
	case EventIME:
		tree.DispatchIME(&core.IMECompositionEvent{
			Text: ev.IMEText, Cursor: ev.IMECursor, End: ev.IMEEnd,
		})
	case EventResize:
		e := ev
		return &e, false
	case EventClose:
		return nil, true
	case EventRedraw:
		// Expose / RequestRedraw: OS may have discarded window pixels — force a
		// full paint on the next frame (Phase A retained present).
		tree.MarkFullPaintRequired()
	case EventFocus:
		// Focus alone does not force a frame; widgets mark paint if needed.
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
